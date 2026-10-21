package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"spark/internal/ai"
	"spark/internal/config"
)

type fakeInput struct {
	mu                         sync.Mutex
	micOn                      bool
	begins, ends, cancels, ons int
}

func (f *fakeInput) StartMic() error { f.mu.Lock(); f.micOn = true; f.ons++; f.mu.Unlock(); return nil }
func (f *fakeInput) StopMic()        { f.mu.Lock(); f.micOn = false; f.mu.Unlock() }
func (f *fakeInput) Begin(string) error {
	f.mu.Lock()
	f.begins++
	f.mu.Unlock()
	return nil
}
func (f *fakeInput) End()       { f.mu.Lock(); f.ends++; f.mu.Unlock() }
func (f *fakeInput) Cancel()    { f.mu.Lock(); f.cancels++; f.mu.Unlock() }
func (f *fakeInput) Live() bool { return true }

func (f *fakeInput) counts() (begins, ends int, on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.begins, f.ends, f.micOn
}

type fakeLLM struct {
	mu        sync.Mutex
	questions []string
}

func (f *fakeLLM) Name() string                                 { return "fake" }
func (f *fakeLLM) ListModels(context.Context) ([]string, error) { return nil, nil }
func (f *fakeLLM) Stream(_ context.Context, _ string, h []ai.Message) (<-chan ai.Chunk, error) {
	f.mu.Lock()
	f.questions = append(f.questions, h[len(h)-1].Text)
	f.mu.Unlock()
	ch := make(chan ai.Chunk, 1)
	ch <- ai.Chunk{Text: "Recule."}
	close(ch)
	return ch, nil
}

func (f *fakeLLM) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.questions...)
}

type recorder struct {
	mu     sync.Mutex
	events []string
	turns  []VoiceTurn
}

func (r *recorder) emit(event string, data ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if event == EventVoiceLevel {
		return
	}
	if event == EventVoiceState && len(data) == 1 {
		event += "=" + data[0].(string)
	}
	if t, ok := firstArg[VoiceTurn](data); ok {
		r.turns = append(r.turns, t)
	}
	r.events = append(r.events, event)
}

func firstArg[T any](data []any) (T, bool) {
	var zero T
	if len(data) == 0 {
		return zero, false
	}
	t, ok := data[0].(T)
	return t, ok
}

func (r *recorder) has(event string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e == event {
			return true
		}
	}
	return false
}

func newVoiceApp(t *testing.T, mic config.MicSettings) (*App, *fakeInput, *fakeLLM, *recorder) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Mic = mic
	cfg.Vision.Enabled = false // no real screen capture in tests
	cfg.Voice.Enabled = false
	llm, rec, in := &fakeLLM{}, &recorder{}, &fakeInput{}
	a := &App{ctx: context.Background(), cfg: cfg, llm: llm, emitFn: rec.emit}
	a.voice = &voice{a: a, in: in, allowed: func() bool { return true }}
	a.voice.configure(cfg.Mic, "fr")
	return a, in, llm, rec
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// feed sends a constant level for d, every 20 ms, like the audio thread.
func feed(v *voice, rms float32, d time.Duration) {
	for end := time.Now().Add(d); time.Now().Before(end); {
		v.onLevel(rms)
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPushToTalk(t *testing.T) {
	mic := config.Defaults().Mic
	mic.Cues = false
	a, in, llm, rec := newVoiceApp(t, mic)
	v := a.voice

	if _, _, on := in.counts(); on {
		t.Fatal("push-to-talk must not keep the mic open")
	}
	v.pressPTT()
	if b, _, on := in.counts(); b != 1 || !on || !rec.has(EventVoiceState+"=listening") {
		t.Fatalf("press: begins=%d mic=%v", b, on)
	}
	v.releasePTT()
	if _, e, _ := in.counts(); e != 1 || !rec.has(EventVoiceState+"=transcribing") {
		t.Fatalf("release: ends=%d", e)
	}
	v.onFinal("Que dois-je faire ici ?")

	waitFor(t, "question sent", func() bool { return len(llm.asked()) == 1 })
	if q := llm.asked()[0]; q != "Que dois-je faire ici ?" {
		t.Fatalf("asked %q", q)
	}
	waitFor(t, "voice turn event", func() bool { return rec.has(EventVoiceTurn) })
	if _, _, on := in.counts(); on {
		t.Fatal("mic must close after the utterance")
	}
}

func TestPushToTalkSilenceAsksNothing(t *testing.T) {
	a, _, llm, rec := newVoiceApp(t, config.Defaults().Mic)
	a.voice.pressPTT()
	a.voice.releasePTT()
	a.voice.onFinal("")
	if len(llm.asked()) != 0 || !rec.has(EventVoiceState+"=idle") {
		t.Fatalf("empty utterance must be dropped: %v", llm.asked())
	}
}

func TestContinuousWithWakeWord(t *testing.T) {
	mic := config.Defaults().Mic
	mic.Mode, mic.Cues, mic.Sensitivity = config.MicContinuous, false, 50
	a, in, llm, _ := newVoiceApp(t, mic)
	v := a.voice

	if _, _, on := in.counts(); !on {
		t.Fatal("continuous mode keeps the mic open")
	}
	feed(v, 0.001, 100*time.Millisecond) // room noise: nothing
	if b, _, _ := in.counts(); b != 0 {
		t.Fatal("noise must not start an utterance")
	}
	feed(v, 0.2, 250*time.Millisecond) // speech
	if b, _, _ := in.counts(); b != 1 {
		t.Fatalf("speech should start an utterance, begins=%d", b)
	}
	feed(v, 0.001, 1100*time.Millisecond) // silence ends it
	if _, e, _ := in.counts(); e != 1 {
		t.Fatalf("silence should end the utterance, ends=%d", e)
	}

	// Without the wake phrase: ignored (a chat with friends, Discord...).
	v.onFinal("on mange quoi ce soir")
	if len(llm.asked()) != 0 {
		t.Fatalf("no wake phrase, nothing asked: %v", llm.asked())
	}

	// With it: the rest of the sentence is the question.
	feed(v, 0.2, 250*time.Millisecond)
	feed(v, 0.001, 1100*time.Millisecond)
	v.onFinal("Arya, que dois-je faire ?")
	waitFor(t, "question sent", func() bool { return len(llm.asked()) == 1 })
	if q := llm.asked()[0]; q != "que dois-je faire ?" {
		t.Fatalf("asked %q", q)
	}
	if _, _, on := in.counts(); !on {
		t.Fatal("continuous mode keeps listening afterwards")
	}
}

func TestContinuousIgnoresAriasOwnVoice(t *testing.T) {
	mic := config.Defaults().Mic
	mic.Mode = config.MicContinuous
	a, in, _, _ := newVoiceApp(t, mic)
	a.voice.setSpeaking(true)
	feed(a.voice, 0.3, 300*time.Millisecond)
	a.voice.setSpeaking(false)
	feed(a.voice, 0.3, 200*time.Millisecond) // still inside the echo guard
	if b, _, _ := in.counts(); b != 0 {
		t.Fatalf("A.R.I.A's voice must not trigger her, begins=%d", b)
	}
}

func TestPressInterruptsAnswer(t *testing.T) {
	a, _, _, _ := newVoiceApp(t, config.Defaults().Mic)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.askCancel, a.askDone = cancel, done
	go func() { <-ctx.Done(); close(done) }()

	a.voice.pressPTT()
	if ctx.Err() == nil {
		t.Fatal("pressing push-to-talk must stop A.R.I.A")
	}
}

// The audio thread must never block nor call into the UI: pushLevel only
// queues, the pump goroutine does the rest.
func TestAudioThreadNeverBlocks(t *testing.T) {
	mic := config.Defaults().Mic
	mic.Mode = config.MicContinuous
	a, in, _, rec := newVoiceApp(t, mic)
	v := a.voice
	v.levels = make(chan float32, 64)

	start := time.Now()
	for range 100_000 { // no consumer yet: must drop, not block
		v.pushLevel(0.3)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("pushLevel blocked: %v", d)
	}
	if rec.has(EventVoiceState + "=listening") {
		t.Fatal("the audio thread must not run voice detection itself")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go v.pumpLevels(ctx)
	waitFor(t, "detection on the pump goroutine", func() bool {
		v.pushLevel(0.3)
		time.Sleep(20 * time.Millisecond)
		b, _, _ := in.counts()
		return b == 1
	})
}

// stuckLLM never answers until its context is cancelled (a saturated API).
type stuckLLM struct{ started chan struct{} }

func (s *stuckLLM) Name() string                                 { return "stuck" }
func (s *stuckLLM) ListModels(context.Context) ([]string, error) { return nil, nil }
func (s *stuckLLM) Stream(ctx context.Context, _ string, _ []ai.Message) (<-chan ai.Chunk, error) {
	close(s.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPushToTalkInterruptsAStuckRequest(t *testing.T) {
	a, in, _, _ := newVoiceApp(t, config.Defaults().Mic)
	stuck := &stuckLLM{started: make(chan struct{})}
	a.llm = stuck

	result := make(chan Turn, 1)
	go func() {
		turn, err := a.Ask("Que vois-tu ?", false)
		if err != nil {
			t.Errorf("an interrupted question is not an error: %v", err)
		}
		result <- turn
	}()
	<-stuck.started

	start := time.Now()
	a.voice.pressPTT() // must not wait for the network timeout
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("push-to-talk waited %v for the stuck request", d)
	}
	if b, _, _ := in.counts(); b != 1 {
		t.Fatal("the mic must open right away")
	}
	if turn := <-result; !turn.Interrupted {
		t.Fatalf("the stuck question should report Interrupted, got %+v", turn)
	}
}

func TestNotBusyAfterAnAnswer(t *testing.T) {
	a, _, _, _ := newVoiceApp(t, config.Defaults().Mic)
	if _, err := a.Ask("Bonjour", false); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "answer finished", func() bool { return !a.busy() })
}

// The UI creates the answer bubble on voice:turn: it must come before the
// first streamed token, or the bubble misses it and stays on "ANALYSE".
func TestVoiceTurnBeforeFirstToken(t *testing.T) {
	a, _, _, rec := newVoiceApp(t, config.Defaults().Mic)
	a.voice.pressPTT()
	a.voice.releasePTT()
	a.voice.onFinal("Que vois-tu ?")
	waitFor(t, "answer streamed", func() bool { return rec.has(EventDelta) })

	rec.mu.Lock()
	defer rec.mu.Unlock()
	turn, delta := -1, -1
	for i, e := range rec.events {
		if e == EventVoiceTurn && turn < 0 {
			turn = i
		}
		if e == EventDelta && delta < 0 {
			delta = i
		}
	}
	if turn < 0 || turn > delta {
		t.Fatalf("voice:turn at %d, first delta at %d: %v", turn, delta, rec.events)
	}
}

// failingLLM rejects every request (quota, bad key...).
type failingLLM struct{}

func (failingLLM) Name() string                                 { return "failing" }
func (failingLLM) ListModels(context.Context) ([]string, error) { return nil, nil }
func (failingLLM) Stream(context.Context, string, []ai.Message) (<-chan ai.Chunk, error) {
	return nil, errors.New("quota atteint")
}

// A real failure must reach the user, not pass for an interruption.
func TestAskReportsErrors(t *testing.T) {
	a, _, _, rec := newVoiceApp(t, config.Defaults().Mic)
	a.llm = failingLLM{}
	turn, err := a.Ask("Bonjour", false)
	if err == nil || turn.Interrupted {
		t.Fatalf("got turn %+v, err %v: the error was swallowed", turn, err)
	}
	if a.busy() {
		t.Fatal("a failed question must not leave the app busy")
	}

	a.voice.pressPTT()
	a.voice.releasePTT()
	a.voice.onFinal("Que vois-tu ?")
	waitFor(t, "voice error reported", func() bool { return rec.has(EventVoiceError) })
}
