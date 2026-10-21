// Package app is the bridge between Wails and Spark's modules. Exported
// methods of App are bound to the frontend (frontend/wailsjs/go/app).
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"spark/internal/ai"
	"spark/internal/audio"
	"spark/internal/config"
	"spark/internal/hotkey"
	"spark/internal/overlay"
	"spark/internal/vision"
)

// Events emitted to the frontend (mirrored in frontend/src/lib/bridge.ts).
const (
	EventToggleChat   = "aria:toggle-chat"
	EventVisibility   = "spark:visibility"
	EventOpenSettings = "spark:open-settings"
	EventPermission   = "spark:permission" // bool, an OS permission dialog is pending
	EventSay          = "aria:say"         // text A.R.I.A is saying
	EventSpeaking     = "aria:speaking"    // bool, voice playing
	EventDelta        = "aria:delta"       // Delta, streamed answer text
	EventDone         = "aria:done"        // Done, end of an answer
)

type App struct {
	env      config.Env
	settings *config.Store
	overlay  *overlay.Controller
	vision   *vision.Service
	tts      *audio.SystemTTS
	neural   *ai.GeminiVoice // nil: system voice only. Guarded by mu.
	player   audio.Player
	speakMu  sync.Mutex // one voice at a time
	voice    *voice
	pro      proactive
	visible  atomic.Bool
	debug    atomic.Bool
	boot     sync.Once

	// Live configuration, rebuilt by apply. Guarded by mu.
	mu        sync.RWMutex
	cfg       config.Settings
	llm       ai.Provider // nil when not configured
	llmErr    error       // why llm is nil
	hotkeys   *hotkey.Listener
	hotkeyErr error

	// Conversation state, see assistant.go.
	askMu     sync.Mutex
	askID     int
	askCancel context.CancelFunc
	askDone   chan struct{}
	convMu    sync.Mutex
	history   []ai.Message

	ctx    context.Context
	cancel context.CancelFunc

	// emitFn replaces the Wails event bus in tests.
	emitFn func(event string, data ...any)
	emitMu sync.Mutex
	// closing: the webview is being torn down. Events sent then reach a
	// dead WKWebView and crash in Wails' ExecJS (seen when quitting).
	closing atomic.Bool
}

// emit sends an event to the frontend. Serialized: events come from many
// goroutines and Wails' ExecJS is not meant to be hammered concurrently.
func (a *App) emit(event string, data ...any) {
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	if a.closing.Load() {
		return
	}
	if a.emitFn != nil {
		a.emitFn(event, data...)
		return
	}
	runtime.EventsEmit(a.ctx, event, data...)
}

func New(env config.Env, settings *config.Store) *App {
	a := &App{env: env, settings: settings, overlay: overlay.NewController(), vision: vision.New(vision.DefaultOptions)}
	a.visible.Store(true)
	a.overlay.OnToggle = func(ignoring bool) { a.debugf("overlay: click-through=%v", ignoring) }
	a.overlay.OnRepair = func(what int) {
		log.Printf("overlay: fenêtre remise au premier plan (niveau=%v, ordre=%v)", what&1 != 0, what&2 != 0)
	}
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	// Ctrl+C / SIGTERM (wails dev): stop emitting before Wails tears the
	// webview down. Wails handles the signal too and quits the app.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		a.beginClose()
	}()
	// Created here rather than in New: Wails also runs New when generating
	// bindings, where listing the system voices is wasted work.
	a.tts = audio.NewSystemTTS()
	a.player = audio.NewPlayer()
	a.voice = newVoice(a)
	a.apply(a.settings.Get(), true)
}

// DomReady runs once the native window and the webview exist.
func (a *App) DomReady(context.Context) {
	if err := a.overlay.Start(a.ctx); err != nil {
		log.Printf("overlay disabled: %v", err)
	}
	a.mu.Lock()
	a.registerHotkeys()
	a.mu.Unlock()
	go a.proactiveLoop(a.ctx)
}

// BeforeClose runs first when the app quits: stop talking to the webview
// and stop the background work right away.
func (a *App) BeforeClose(context.Context) (prevent bool) {
	a.beginClose()
	return false
}

// Shutdown only cancels: it must not wait on goroutines that may be
// dispatching to the main thread, which is the one calling Shutdown.
func (a *App) Shutdown(context.Context) {
	a.beginClose()
}

func (a *App) beginClose() {
	a.emitMu.Lock() // no event in flight past this point
	a.closing.Store(true)
	a.emitMu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *App) debugf(format string, args ...any) {
	if a.debug.Load() {
		log.Printf(format, args...)
	}
}

// apply pushes settings into every module. Only what changed is rebuilt.
func (a *App) apply(s config.Settings, first bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.cfg
	a.cfg = s

	a.debug.Store(a.env.Debug || s.Advanced.Debug)
	a.overlay.SetPollRate(s.Advanced.PollHz)
	a.vision.SetOptions(vision.Options{MaxSide: s.Vision.MaxSide, Quality: s.Vision.Quality, ShowCursor: s.Vision.ShowCursor})
	if first || s.Voice.Voice != old.Voice.Voice || s.Voice.Rate != old.Voice.Rate {
		a.tts.Configure(s.Voice.Voice, s.Voice.Rate)
		a.debugf("voice: %q ×%.2f", a.tts.Voice(), s.Voice.Rate)
	}
	if first || s.Voice != old.Voice {
		a.rebuildNeuralLocked()
	}
	if first || !aiEqual(s.AI, old.AI) {
		a.rebuildLLM()
	}
	if first || s.Mic != old.Mic || s.AI.Language != old.AI.Language {
		a.voice.configure(s.Mic, s.AI.Language)
	}
	// Hotkeys need the native window: DomReady registers the first ones.
	pttChanged := s.Mic.Mode != old.Mic.Mode || s.Mic.PushToTalk != old.Mic.PushToTalk
	if !first && (s.Hotkeys != old.Hotkeys || pttChanged) {
		a.registerHotkeys()
	}
}

func aiEqual(x, y config.AISettings) bool {
	return fmt.Sprint(x) == fmt.Sprint(y) // maps print sorted: cheap deep equality
}

// rebuildLLM recreates the provider from the settings and the key store.
// Caller holds a.mu.
func (a *App) rebuildLLM() {
	a.llm, a.llmErr = a.newProvider(a.cfg.AI.Provider)
	if a.llm != nil {
		log.Printf("ai: %s", a.llm.Name())
	} else {
		log.Printf("ai: non configurée (%v)", a.llmErr)
	}
}

// rebuildNeuralLocked sets up the neural voice when selected and a Gemini
// key exists. Caller holds a.mu.
func (a *App) rebuildNeuralLocked() {
	a.neural = nil
	v := a.cfg.Voice
	if v.Engine != config.VoiceGemini {
		return
	}
	key, _ := config.APIKey(config.ProviderGemini)
	if key == "" {
		log.Printf("voice: neural voice needs a Gemini key, using the system voice")
		return
	}
	a.neural = ai.NewGeminiVoice(key, "", v.NeuralVoice, v.Style, a.player)
	a.debugf("voice: neural %s", v.NeuralVoice)
}

// newProvider builds a provider from the current settings and key store.
// Caller holds a.mu (read or write).
func (a *App) newProvider(provider string) (ai.Provider, error) {
	s := a.cfg.AI
	key, _ := config.APIKey(provider)
	model := s.Model(provider)
	baseURL := s.BaseURLs[provider]
	if provider == s.Provider {
		if a.env.Model != "" {
			model = a.env.Model
		}
		if a.env.BaseURL != "" {
			baseURL = a.env.BaseURL
		}
	}
	p, err := ai.New(ai.Config{
		Provider: provider, APIKey: key, Model: model, BaseURL: baseURL,
		Thinking: s.Thinking, Fallback: s.Fallback,
	})
	if g, ok := p.(*ai.Gemini); ok {
		g.OnFallback = func(from, to string, reason error) {
			log.Printf("ai: %s saturé (%v), réponse via %s", from, reason, to)
		}
	}
	return p, err
}

// registerHotkeys replaces the global shortcuts, push-to-talk included.
// Caller holds a.mu.
func (a *App) registerHotkeys() {
	a.hotkeys.Close()
	a.hotkeys = nil
	h := a.cfg.Hotkeys
	var bindings []hotkey.Binding
	for _, b := range []struct {
		s  config.Shortcut
		fn func()
	}{{h.ToggleChat, a.toggleChat}, {h.ToggleVisible, a.toggleVisible}, {h.OpenSettings, a.openSettings}} {
		if b.s.Key != "" {
			bindings = append(bindings, hotkey.Binding{Mod: modifiers(b.s), Key: hotkey.Key(b.s.Key), Fn: b.fn})
		}
	}
	if ptt := a.cfg.Mic.PushToTalk; a.cfg.Mic.Mode == config.MicPushToTalk && ptt.Key != "" {
		bindings = append(bindings, hotkey.Binding{
			Mod: modifiers(ptt), Key: hotkey.Key(ptt.Key),
			Fn: a.voice.pressPTT, Release: a.voice.releasePTT,
		})
	}
	a.hotkeys, a.hotkeyErr = hotkey.Register(bindings)
	if a.hotkeyErr != nil {
		log.Printf("hotkeys: %v", a.hotkeyErr)
	}
}

func modifiers(s config.Shortcut) hotkey.Modifier {
	var m hotkey.Modifier
	if s.Alt {
		m |= hotkey.ModAlt
	}
	if s.Shift {
		m |= hotkey.ModShift
	}
	if s.Ctrl {
		m |= hotkey.ModCtrl
	}
	if s.Cmd {
		m |= hotkey.ModCmd
	}
	return m
}

func (a *App) current() config.Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// Boot is called by the frontend once it listens to events. It plays the
// welcome sequence, only once per process (dev hot reloads call it again).
func (a *App) Boot() {
	a.boot.Do(func() {
		if a.current().Voice.Greeting {
			go a.greet()
		}
	})
}

func (a *App) greet() {
	format, salut := greeting(time.Now(), a.current().AI.Language)
	a.say(fmt.Sprintf(format, salut))
}

// say shows text in the UI and speaks it.
func (a *App) say(text string) {
	a.emit(EventSay, text)
	sentences := make(chan string, 1)
	sentences <- text
	close(sentences)
	a.speakAll(a.ctx, sentences)
}

func (a *App) voiceOn() bool {
	return !a.env.Mute && a.current().Voice.Enabled
}

// speakAll speaks sentences as they arrive, keeping the "speaking" state up
// for the whole batch. Once ctx is cancelled it only drains the channel, so
// producers never block.
func (a *App) speakAll(ctx context.Context, sentences <-chan string) {
	a.speakMu.Lock()
	defer a.speakMu.Unlock()

	a.mu.RLock()
	neural := a.neural
	a.mu.RUnlock()

	started := false
	begin := func() {
		if !started {
			started = true
			a.voice.setSpeaking(true)
			a.emit(EventSpeaking, true)
		}
	}
	system := func(text string) {
		if err := a.tts.Speak(ctx, spoken(text)); err != nil && ctx.Err() == nil {
			log.Printf("tts: %v", err)
		}
	}

	if neural != nil {
		// One request per answer: the free tier allows only a few TTS
		// requests per minute. The whole answer is spoken in one stream.
		var all []string
		for s := range sentences {
			if a.voiceOn() && ctx.Err() == nil {
				all = append(all, s)
			}
		}
		if len(all) > 0 && ctx.Err() == nil {
			begin()
			text := strings.Join(all, " ")
			if err := neural.Speak(ctx, spoken(text)); err != nil && ctx.Err() == nil {
				log.Printf("tts: neural voice failed (%v)", err)
				if errors.Is(err, ai.ErrNoAudio) {
					system(text) // never leave A.R.I.A mute
				}
			}
		}
	} else {
		for s := range sentences {
			if !a.voiceOn() || ctx.Err() != nil {
				continue
			}
			begin()
			system(s)
		}
	}
	if started {
		a.voice.setSpeaking(false)
		a.emit(EventSpeaking, false)
	}
}

// spoken adapts displayed text for the voice: "A.R.I.A" would be spelled
// out letter by letter.
func spoken(text string) string {
	return strings.ReplaceAll(text, "A.R.I.A", "Aria")
}

func (a *App) toggleChat() {
	a.debugf("hotkey: toggle chat")
	if !a.visible.Load() {
		a.setVisible(true)
	}
	a.emit(EventToggleChat)
}

func (a *App) toggleVisible() {
	a.debugf("hotkey: toggle visibility")
	a.setVisible(!a.visible.Load())
}

func (a *App) openSettings() {
	a.debugf("hotkey: open settings")
	if !a.visible.Load() {
		a.setVisible(true)
	}
	a.emit(EventOpenSettings)
}

func (a *App) setVisible(v bool) {
	a.visible.Store(v)
	a.overlay.SetPaused(!v)
	a.emit(EventVisibility, v)
}

// SetInteractiveRegions is called by the frontend whenever the layout of its
// clickable elements changes.
func (a *App) SetInteractiveRegions(regions []overlay.Rect) {
	a.overlay.SetRegions(regions)
}

// SetMouseCaptured forces the overlay to keep (or release) the mouse.
func (a *App) SetMouseCaptured(v bool) {
	a.overlay.SetCaptured(v)
}

// SetKeyboardFocus takes the keyboard focus while a text field is in use
// (chat, settings) and gives it back to the previous app (game, browser...)
// afterwards.
func (a *App) SetKeyboardFocus(focused bool) {
	a.debugf("keyboard focus=%v", focused)
	a.pro.focused.Store(focused)
	if focused {
		a.overlay.Focus()
	} else {
		a.overlay.RestoreFocus()
	}
}

// SetOrbPosition persists where the user dropped the A.R.I.A button.
func (a *App) SetOrbPosition(x, y float64) error {
	return a.settings.Update(func(s *config.Settings) { s.Orb = &config.Point{X: x, Y: y} })
}

func (a *App) Debug() bool {
	return a.debug.Load()
}
