package app

import (
	"context"
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
