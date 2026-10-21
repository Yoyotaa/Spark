package app

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"spark/internal/ai"
	"spark/internal/audio"
	"spark/internal/config"
	"spark/internal/vision"
)

// Voice events (mirrored in frontend/src/lib/bridge.ts).
const (
	EventVoiceState   = "voice:state"   // "idle" | "listening" | "transcribing"
	EventVoiceLevel   = "voice:level"   // float 0-1, ~20 Hz while the mic runs
	EventVoicePartial = "voice:partial" // live transcription
	EventVoiceTurn    = "voice:turn"    // VoiceTurn, a spoken question was sent
	EventVoiceError   = "voice:error"   // string
)

// VoiceTurn is a spoken question turned into an AI request.
type VoiceTurn struct {
	Text string `json:"text"`
	Turn Turn   `json:"turn"`
}

const (
	vadStart      = 120 * time.Millisecond // voice above threshold to start
	vadStop       = 900 * time.Millisecond // silence to end an utterance
	maxUtterance  = 15 * time.Second
	echoGuard     = 400 * time.Millisecond // ignore the mic right after A.R.I.A spoke
	finalTimeout  = 8 * time.Second        // recognition must conclude by then
	captureBudget = 3 * time.Second
)

// voice turns microphone input into questions: push-to-talk (key held) or
// continuous listening with voice-activity detection and a wake phrase.
type voice struct {
	a  *App
	in audio.Input
	// allowed reports whether the mic and speech permissions are granted
	// (replaced in tests).
	allowed func() bool

	mu        sync.Mutex
	cfg       config.MicSettings
	lang      string
	micOn     bool
	testing   bool // settings page meter
	active    bool // an utterance is being recognized
	byKey     bool // started by push-to-talk
	started   time.Time
	above     time.Time // first loud sample of a candidate utterance
	below     time.Time // first quiet sample during an utterance
	shot      *pendingShot
	finalWait *time.Timer

	speaking  atomic.Bool  // A.R.I.A's TTS is playing
	spokeAt   atomic.Int64 // unix nanos when the TTS last stopped
	lastLevel atomic.Int64 // throttles UI level events

	// levels decouples the audio thread from everything else: it only
	// queues the level; a goroutine runs detection and emits events.
	levels chan float32
}

// pendingShot is a screenshot taken when the user starts speaking, so the
// question is about the screen at that moment, not when transcription ends.
type pendingShot struct {
	done  chan struct{}
	frame *vision.Frame
	capt  *Capture
}

func newVoice(a *App) *voice {
	v := &voice{a: a, allowed: func() bool {
		return audio.MicPermission() == audio.PermissionGranted && audio.SpeechPermission() == audio.PermissionGranted
	}}
	// Native callbacks must return at once and never call into Wails or
	// AppKit: the level comes from CoreAudio's real-time thread (calling
	// EventsEmit from there crashed Wails' ExecJS after a few minutes), and
	// stopping the engine waits for that thread. Everything is queued.
	v.levels = make(chan float32, 64)
	go v.pumpLevels(a.ctx)
	v.in = audio.NewInput(audio.InputEvents{
		Level:   v.pushLevel,
		Partial: func(t string) { go v.onPartial(t) },
		Final:   func(t string) { go v.onFinal(t) },
		Error:   func(err error) { go v.onError(err) },
	}, transcriberFor(a))
	return v
}

// transcriberFor returns the cloud transcriber used where speech is not
// recognized natively (Windows): Gemini, if a key is available.
func transcriberFor(a *App) audio.Transcriber {
	return transcriberFunc(func(ctx context.Context, wav []byte, lang string) (string, error) {
		key, _ := config.APIKey(config.ProviderGemini)
		if key == "" {
			return "", errors.New("la reconnaissance vocale passe par Gemini : ajoute une clé Gemini dans Paramètres › Intelligence")
		}
		return ai.NewGeminiTranscriber(key, "").Transcribe(ctx, wav, lang)
	})
}

type transcriberFunc func(ctx context.Context, wav []byte, lang string) (string, error)

func (f transcriberFunc) Transcribe(ctx context.Context, wav []byte, lang string) (string, error) {
	return f(ctx, wav, lang)
}

func (v *voice) emitState(state string) {
	v.a.emit(EventVoiceState, state)
}

// configure applies the microphone settings: the continuous mode keeps the
// mic open, the other modes open it on demand.
func (v *voice) configure(cfg config.MicSettings, lang string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cfg, v.lang = cfg, lang
	v.syncMicLocked()
}

// syncMicLocked opens or closes the mic to match the mode. Caller holds mu.
func (v *voice) syncMicLocked() {
	want := v.active || v.testing ||
		(v.cfg.Mode == config.MicContinuous && v.allowed())
	switch {
	case want && !v.micOn:
		if err := v.in.StartMic(); err != nil {
			log.Printf("mic: %v", err)
			go v.onError(err)
			return
		}
		v.micOn = true
		v.a.debugf("mic: on (%s)", v.cfg.Mode)
	case !want && v.micOn:
		v.in.StopMic()
		v.micOn = false
		v.a.debugf("mic: off")
	}
}

// ensurePermissions asks for the microphone and speech permissions if
// needed (macOS shows its dialogs once).
func (v *voice) ensurePermissions() bool {
	if v.allowed() {
		return true
	}
	if audio.MicPermission() == audio.PermissionDenied || audio.SpeechPermission() == audio.PermissionDenied {
		v.onError(errors.New("accès au micro ou à la reconnaissance vocale refusé : autorise Spark dans Réglages Système › Confidentialité"))
		return false
	}
	v.a.requestPermissions(func(ok bool) {
		if ok {
			v.mu.Lock()
			v.syncMicLocked()
			v.mu.Unlock()
		}
	})
	v.onError(errors.New("autorise le micro et la reconnaissance vocale, puis réessaie"))
	return false
}

// pressPTT starts an utterance (push-to-talk key down). It also stops
// A.R.I.A if she is talking: pressing the key means "my turn".
func (v *voice) pressPTT() {
	v.a.Interrupt()
	if !v.ensurePermissions() {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.active {
		return
	}
	if err := v.beginLocked(true); err != nil {
		go v.onError(err)
	}
}

// releasePTT ends the utterance (key up); the transcription follows.
func (v *voice) releasePTT() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.active && v.byKey {
		v.endLocked()
	}
}

// beginLocked starts recognizing. Caller holds mu.
func (v *voice) beginLocked(byKey bool) error {
	v.active, v.byKey, v.started = true, byKey, time.Now()
	v.syncMicLocked()
	if !v.micOn {
		v.active = false
		return errors.New("micro indisponible")
	}
	if err := v.in.Begin(v.lang); err != nil {
		v.active = false
		v.syncMicLocked()
		return err
	}
	// Push-to-talk captures the screen right away. With a wake phrase the
	// capture waits until the phrase is heard: most continuous-mode
	// utterances are not for A.R.I.A.
	if byKey || !v.cfg.WakeWord {
		v.startShotLocked()
	}
	if v.cfg.Cues {
		audio.PlayCue(true)
	}
	v.emitState("listening")
	return nil
}

// endLocked stops listening; onFinal follows. Caller holds mu.
func (v *voice) endLocked() {
	v.in.End()
	v.below = time.Time{}
	if v.cfg.Cues && v.byKey {
		audio.PlayCue(false)
	}
	v.emitState("transcribing")
	// Recognition engines sometimes never conclude: do not hang.
	v.finalWait = time.AfterFunc(finalTimeout, func() {
		v.mu.Lock()
		stuck := v.active
		v.mu.Unlock()
		if stuck {
			v.in.Cancel()
			v.onFinal("")
		}
	})
}

func (v *voice) startShotLocked() {
	if v.shot != nil || !v.a.current().Vision.Enabled {
		return
	}
	shot := &pendingShot{done: make(chan struct{})}
	v.shot = shot
	go func() {
		defer close(shot.done)
		ctx, cancel := context.WithTimeout(v.a.ctx, captureBudget)
		defer cancel()
		f, c, err := v.a.capture(ctx)
		if err != nil {
			v.a.debugf("voice: capture failed: %v", err)
			return
		}
		shot.frame, shot.capt = f, c
	}()
}

// pushLevel runs on the audio thread: queue the sample, drop it if the
// consumer lags (a level is worthless once stale).
func (v *voice) pushLevel(rms float32) {
	select {
	case v.levels <- rms:
	default:
	}
}

func (v *voice) pumpLevels(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case rms := <-v.levels:
			v.onLevel(rms)
		}
	}
}

// onLevel drives the UI meter and, in continuous mode, voice detection.
// Runs on the pump goroutine (tests call it directly).
func (v *voice) onLevel(rms float32) {
	now := time.Now()
	if now.UnixNano()-v.lastLevel.Load() >= int64(50*time.Millisecond) {
		v.lastLevel.Store(now.UnixNano())
		v.a.emit(EventVoiceLevel, min(1, float64(rms)*8))
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cfg.Mode != config.MicContinuous || v.testing {
		return
	}
	// A.R.I.A's own voice through the speakers must not trigger her.
	if v.speaking.Load() || now.Sub(time.Unix(0, v.spokeAt.Load())) < echoGuard {
		v.above = time.Time{}
		return
	}
	loud := rms > v.threshold()

	if !v.active {
		switch {
		case !loud:
			v.above = time.Time{}
		case v.above.IsZero():
			v.above = now
		case now.Sub(v.above) >= vadStart:
			v.above = time.Time{}
			if err := v.beginLocked(false); err != nil {
				log.Printf("voice: %v", err)
			}
		}
		return
	}
	if v.byKey {
		return
	}
	switch {
	case now.Sub(v.started) > maxUtterance:
		v.endLocked()
	case loud:
		v.below = time.Time{}
	case v.below.IsZero():
		v.below = now
	case now.Sub(v.below) >= vadStop:
		v.endLocked()
	}
}

// threshold maps the sensitivity (0-100) to an RMS level: speech is
// typically 0.02-0.1, room noise 0.001-0.005.
func (v *voice) threshold() float32 {
	return 0.004 + 0.05*(1-float32(v.cfg.Sensitivity)/100)
}

func (v *voice) onPartial(text string) {
	v.mu.Lock()
	if !v.active {
		v.mu.Unlock()
		return
	}
	// The wake phrase is heard: take the screenshot now.
	if !v.byKey && v.cfg.WakeWord && v.shot == nil {
		if _, ok := stripWakePhrase(text, v.cfg.WakePhrase); ok {
			v.startShotLocked()
		}
	}
	v.mu.Unlock()
	v.a.emit(EventVoicePartial, text)
}

func (v *voice) onFinal(text string) {
	v.mu.Lock()
	if !v.active {
		v.mu.Unlock()
		return
	}
	if v.finalWait != nil {
		v.finalWait.Stop()
		v.finalWait = nil
	}
	byKey, wake, phrase := v.byKey, v.cfg.WakeWord, v.cfg.WakePhrase
	shot := v.shot
	v.active, v.byKey, v.shot = false, false, nil
	v.syncMicLocked()
	v.mu.Unlock()

	v.a.debugf("voice: heard %q", text)
	if !byKey && wake {
		req, ok := stripWakePhrase(text, phrase)
		if !ok {
			v.discard(shot)
			v.emitState("idle")
			return
		}
		text = req
		if shot == nil {
			v.mu.Lock()
			v.startShotLocked()
			shot = v.shot
			v.shot = nil
			v.mu.Unlock()
		}
	}
	if text == "" && (byKey || !wake) {
		// Nothing intelligible: a mis-press or noise.
		v.discard(shot)
		v.emitState("idle")
		return
	}

	var frame *vision.Frame
	var capt *Capture
	if shot != nil {
		<-shot.done
		frame, capt = shot.frame, shot.capt
	}
	v.emitState("idle")
	// The UI must know about the question before the first answer token
	// (the answer starts streaming before ask returns).
	_, err := v.a.askThen(text, false, frame, capt, func(t Turn) {
		v.a.emit(EventVoiceTurn, VoiceTurn{Text: text, Turn: t})
	})
	if err != nil {
		v.onError(err)
	}
}

func (v *voice) discard(shot *pendingShot) {
	if shot == nil {
		return
	}
	go func() {
		<-shot.done
		shot.frame.Release()
	}()
}

func (v *voice) onError(err error) {
	log.Printf("voice: %v", err)
	v.mu.Lock()
	if v.active {
		v.active, v.byKey = false, false
		v.discard(v.shot)
		v.shot = nil
		v.in.Cancel()
		v.syncMicLocked()
	}
	v.mu.Unlock()
	v.emitState("idle")
	v.a.emit(EventVoiceError, err.Error())
}

// setSpeaking is called by the TTS so voice detection ignores A.R.I.A.
func (v *voice) setSpeaking(on bool) {
	v.speaking.Store(on)
	if !on {
		v.spokeAt.Store(time.Now().UnixNano())
	}
}

// setTesting opens the mic for the level meter of the settings page.
func (v *voice) setTesting(on bool) error {
	if on && !v.ensurePermissions() {
		return errors.New("micro non autorisé")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.testing = on
	v.syncMicLocked()
	if on && !v.micOn {
		return errors.New("micro indisponible")
	}
	return nil
}

// requestPermissions shows the OS permission dialogs above the overlay
// (which otherwise covers them) and restores it once they are answered.
func (a *App) requestPermissions(done func(ok bool)) {
	if a.overlay == nil {
		audio.RequestPermissions(done)
		return
	}
	// The UI steps aside (hidden, click-through) until the dialogs are
	// answered, or after a while in any case.
	a.overlay.Yield(true)
	a.emit(EventPermission, true)
	var once sync.Once
	restore := func() {
		once.Do(func() {
			a.overlay.Yield(false)
			a.emit(EventPermission, false)
		})
	}
	timer := time.AfterFunc(permissionTimeout, restore)
	audio.RequestPermissions(func(ok bool) {
		timer.Stop()
		restore()
		done(ok)
	})
}

const permissionTimeout = 90 * time.Second
