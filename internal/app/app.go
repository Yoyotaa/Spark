// Package app is the bridge between Wails and Spark's modules. Exported
// methods of App are bound to the frontend (frontend/wailsjs/go/app).
package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
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
	EventToggleChat = "aria:toggle-chat"
	EventVisibility = "spark:visibility"
	EventSay        = "aria:say"      // text A.R.I.A is saying
	EventSpeaking   = "aria:speaking" // bool, voice playing
	EventDelta      = "aria:delta"    // Delta, streamed answer text
	EventDone       = "aria:done"     // Done, end of an answer
)

type App struct {
	cfg      config.Config
	settings *config.Store // nil if the config dir is unusable
	overlay  *overlay.Controller
	tts      audio.TTS
	speakMu  sync.Mutex // one voice at a time
	vision   *vision.Service
	llm      ai.Provider // nil without API key
	visible  atomic.Bool
	boot     sync.Once

	// Conversation state, see assistant.go.
	askMu     sync.Mutex
	askID     int
	askCancel context.CancelFunc
	askDone   chan struct{}
	convMu    sync.Mutex
	history   []ai.Message

	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg config.Config, settings *config.Store) *App {
	ov := overlay.NewController()
	if cfg.Debug {
		ov.OnToggle = func(ignoring bool) { log.Printf("overlay: click-through=%v", ignoring) }
	}
	a := &App{cfg: cfg, settings: settings, overlay: ov, vision: vision.New(vision.DefaultOptions)}
	if cfg.LLMAPIKey != "" {
		a.llm = ai.NewGemini(cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMURL)
	}
	a.visible.Store(true)
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	// Resolved here rather than in New: Wails also runs New when generating
	// bindings, where listing the system voices is wasted work.
	if !a.cfg.Mute {
		tts := audio.NewSystemTTS(a.cfg.Voice)
		a.debugf("voice: %q", tts.Voice())
		a.tts = tts
	}
	if a.llm != nil {
		log.Printf("ai: %s", a.llm.Name())
	} else {
		log.Printf("ai: non configurée (SPARK_LLM_API_KEY absente de l'environnement)")
	}
}

// DomReady runs once the native window and the webview exist.
func (a *App) DomReady(context.Context) {
	if err := a.overlay.Start(a.ctx); err != nil {
		log.Printf("overlay disabled: %v", err)
	}
	err := hotkey.Listen(a.ctx, []hotkey.Binding{
		{Mod: hotkey.ModAlt | hotkey.ModShift, Key: 'A', Fn: a.toggleChat},
		{Mod: hotkey.ModAlt | hotkey.ModShift, Key: 'H', Fn: a.toggleVisible},
	})
	if err != nil {
		log.Printf("hotkeys disabled: %v", err)
	}
}

// Shutdown only cancels: it must not wait on goroutines that may be
// dispatching to the main thread, which is the one calling Shutdown.
func (a *App) Shutdown(context.Context) {
	a.cancel()
}

func (a *App) debugf(format string, args ...any) {
	if a.cfg.Debug {
		log.Printf(format, args...)
	}
}

// Boot is called by the frontend once it listens to events. It plays the
// welcome sequence, only once per process (dev hot reloads call it again).
func (a *App) Boot() {
	a.boot.Do(func() { go a.greet() })
}

func (a *App) greet() {
	format, salut := greeting(time.Now())
	a.say(fmt.Sprintf(format, salut))
}

// say shows text in the UI and speaks it.
func (a *App) say(text string) {
	runtime.EventsEmit(a.ctx, EventSay, text)
	sentences := make(chan string, 1)
	sentences <- text
	close(sentences)
	a.speakAll(a.ctx, sentences)
}

// speakAll speaks sentences as they arrive, keeping the "speaking" state up
// for the whole batch. Once ctx is cancelled it only drains the channel, so
// producers never block.
func (a *App) speakAll(ctx context.Context, sentences <-chan string) {
	a.speakMu.Lock()
	defer a.speakMu.Unlock()

	started := false
	for s := range sentences {
		if a.tts == nil || ctx.Err() != nil {
			continue
		}
		if !started {
			started = true
			runtime.EventsEmit(a.ctx, EventSpeaking, true)
		}
		if err := a.tts.Speak(ctx, spoken(s)); err != nil && ctx.Err() == nil {
			log.Printf("tts: %v", err)
		}
	}
	if started {
		runtime.EventsEmit(a.ctx, EventSpeaking, false)
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
	runtime.EventsEmit(a.ctx, EventToggleChat)
}

func (a *App) toggleVisible() {
	a.debugf("hotkey: toggle visibility")
	a.setVisible(!a.visible.Load())
}

func (a *App) setVisible(v bool) {
	a.visible.Store(v)
	a.overlay.SetPaused(!v)
	runtime.EventsEmit(a.ctx, EventVisibility, v)
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

// SetChatOpen takes the keyboard focus when the chat opens and gives it back
// to the previous app (game, browser...) when it closes.
func (a *App) SetChatOpen(open bool) {
	a.debugf("chat open=%v", open)
	if open {
		a.overlay.Focus()
	} else {
		a.overlay.RestoreFocus()
	}
}

// GetSettings returns the persisted user preferences.
func (a *App) GetSettings() config.Settings {
	if a.settings == nil {
		return config.Settings{}
	}
	return a.settings.Get()
}

// SetOrbPosition persists where the user dropped the A.R.I.A button.
func (a *App) SetOrbPosition(x, y float64) error {
	if a.settings == nil {
		return nil
	}
	return a.settings.Update(func(s *config.Settings) { s.Orb = &config.Point{X: x, Y: y} })
}

func (a *App) Debug() bool {
	return a.cfg.Debug
}
