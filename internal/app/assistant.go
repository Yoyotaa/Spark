package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"spark/internal/ai"
	"spark/internal/vision"
)

const captureTimeout = 5 * time.Second

// Used when the user sends the screen without a question.
var defaultQuestion = map[string]string{
	"fr": "Que vois-tu, et que me conseilles-tu ?",
	"en": "What do you see, and what do you advise?",
}

// Capture describes the screenshot attached to a question.
type Capture struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Bytes   int    `json:"bytes"`
	Millis  int64  `json:"millis"`
	DataURL string `json:"dataUrl"` // thumbnail source for the chat
}

// Turn is returned by Ask; the answer itself streams through events.
type Turn struct {
	ID      int      `json:"id"`
	Capture *Capture `json:"capture,omitempty"`
	// Interrupted: superseded by a newer question before any answer.
	Interrupted bool `json:"interrupted,omitempty"`
}

// Delta is a piece of streamed answer (EventDelta).
type Delta struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// Done ends an answer (EventDone).
type Done struct {
	ID          int    `json:"id"`
	Error       string `json:"error,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
}

// Ask sends a question to A.R.I.A, with a screenshot taken right now when
// withScreen is set. A new question interrupts the previous answer (text and
// voice). It returns once the request is accepted; the answer streams through
// EventDelta / EventDone.
func (a *App) Ask(text string, withScreen bool) (Turn, error) {
	return a.ask(text, withScreen, nil, nil)
}

// askThen is ask with a hook called once the request is accepted, right
// before the answer starts streaming.
func (a *App) askThen(text string, withScreen bool, frame *vision.Frame, capt *Capture, accepted func(Turn)) (Turn, error) {
	return a.askWith(text, withScreen, frame, capt, accepted)
}

// interruptLocked stops the current question (request, answer, voice)
// and waits for it, so the history keeps a strict user/model alternation.
// Caller holds askMu; the waited goroutine never needs it before done.
func (a *App) interruptLocked() {
	if a.askCancel != nil {
		a.askCancel()
		<-a.askDone
		a.askCancel, a.askDone = nil, nil
	}
}

// Interrupt stops A.R.I.A (request pending, answer being generated or
// spoken). It returns at once even if the request is stuck on the network.
func (a *App) Interrupt() {
	a.askMu.Lock()
	a.interruptLocked()
	a.askMu.Unlock()
}

// busy reports whether a question is in progress.
func (a *App) busy() bool {
	a.askMu.Lock()
	defer a.askMu.Unlock()
	return a.askCancel != nil
}

// finish marks a question as over (done is closed first, see
// interruptLocked), unless a newer one already took its place.
func (a *App) finish(done chan struct{}) {
	close(done)
	a.askMu.Lock()
	if a.askDone == done {
		a.askCancel, a.askDone = nil, nil
	}
	a.askMu.Unlock()
}

// ask is Ask with an optional screenshot already taken (voice questions
// capture the screen when the user starts speaking). It takes ownership of
// frame.
//
// The lock is only held to register the question: capture and network run
// outside of it, so a new question (or the push-to-talk key) interrupts a
// stuck request immediately instead of waiting for its timeout.
func (a *App) ask(text string, withScreen bool, frame *vision.Frame, capt *Capture) (Turn, error) {
	return a.askWith(text, withScreen, frame, capt, nil)
}

func (a *App) askWith(text string, withScreen bool, frame *vision.Frame, capt *Capture, accepted func(Turn)) (turn Turn, err error) {
	defer func() {
		if err != nil {
			a.debugf("ai: question refusée : %v", err)
		}
	}()
	// Safe once ask returns: the provider has serialized the image when
	// Stream returns, and the frame is not kept.
	defer func() { frame.Release() }()

	a.mu.RLock()
	llm, llmErr, cfg := a.llm, a.llmErr, a.cfg
	a.mu.RUnlock()
	if llm == nil {
		if llmErr == nil {
			llmErr = errors.New("IA non configurée")
		}
		return Turn{}, llmErr
	}
	text = strings.TrimSpace(text)
	if text == "" {
		if !withScreen && frame == nil {
			return Turn{}, errors.New("question vide")
		}
		text = defaultQuestion[cfg.AI.Language]
	}

	a.askMu.Lock()
	a.interruptLocked()
	ctx, cancel := context.WithCancel(a.ctx)
	done := make(chan struct{})
	a.askCancel, a.askDone = cancel, done
	a.askID++
	turn = Turn{ID: a.askID, Capture: capt}
	a.askMu.Unlock()

	// From here, every exit must end with finish(done).
	interrupted := func() bool { return ctx.Err() != nil && a.ctx.Err() == nil }
	fail := func(e error) (Turn, error) {
		cancel()
		a.finish(done)
		if interrupted() {
			// Superseded by a newer question: not an error for the user.
			return Turn{ID: turn.ID, Interrupted: true}, nil
		}
		return Turn{}, e
	}

	if frame == nil && withScreen {
		frame, turn.Capture, err = a.capture(ctx)
		if err != nil {
			return fail(err)
		}
	}

	// Past exchanges sent as context: text only, screenshots are never kept
	// (only the current question carries one) to bound tokens.
	a.convMu.Lock()
	keep := 2 * cfg.AI.HistoryTurns
	history := slices.Clone(a.history[max(0, len(a.history)-keep):])
	a.convMu.Unlock()
	question := ai.Message{Role: ai.RoleUser, Text: text}
	if frame != nil {
		question.Image = frame.Data
	}
	history = append(history, question)

	start := time.Now()
	system := ai.SystemPrompt(ai.PromptOptions{
		Language: cfg.AI.Language, Length: cfg.AI.Length, Custom: cfg.AI.CustomInstructions,
	})
	stream, err := llm.Stream(ctx, system, history)
	if err != nil {
		return fail(err)
	}
	a.debugf("ai: request accepted in %v", time.Since(start))

	if frame != nil {
		question.Text += " [capture d'écran jointe]"
	}
	question.Image = nil
	a.convMu.Lock()
	a.history = append(a.history, question)
	a.convMu.Unlock()

	if accepted != nil {
		accepted(turn)
	}
	go a.answer(ctx, cancel, done, turn.ID, stream, start, cfg.Voice.ReadAnswers)
	return turn, nil
}

// answer forwards the stream to the UI and, sentence by sentence, to the
// voice, then records the reply in the history.
func (a *App) answer(ctx context.Context, cancel context.CancelFunc, done chan struct{}, id int, stream <-chan ai.Chunk, start time.Time, readAloud bool) {
	defer a.finish(done)
	defer cancel()

	sentences := make(chan string, 16)
	spoken := make(chan struct{})
	go func() {
		a.speakAll(ctx, sentences)
		close(spoken)
	}()

	var full strings.Builder
	var splitter ai.Sentences
	var streamErr error
	first := true
	for c := range stream {
		if c.Err != nil {
			streamErr = c.Err
			break
		}
		if first {
			first = false
			a.debugf("ai: first token after %v", time.Since(start))
		}
		full.WriteString(c.Text)
		a.emit(EventDelta, Delta{ID: id, Text: c.Text})
		for _, s := range splitter.Push(c.Text) {
			if readAloud {
				sentences <- s
			}
		}
	}
	if rest := splitter.Flush(); rest != "" && streamErr == nil && readAloud {
		sentences <- rest
	}
	close(sentences)

	d := Done{ID: id, Interrupted: ctx.Err() != nil && a.ctx.Err() == nil}
	if streamErr != nil && !d.Interrupted {
		d.Error = streamErr.Error()
	}
	a.emit(EventDone, d)
	a.debugf("ai: answer %d done in %v (%d chars)", id, time.Since(start), full.Len())

	reply := full.String()
	if reply == "" {
		reply = "…"
	}
	a.convMu.Lock()
	a.history = append(a.history, ai.Message{Role: ai.RoleModel, Text: reply})
	// The settings cap what is sent; this only bounds memory.
	if n := len(a.history); n > 40 {
		a.history = slices.Delete(a.history, 0, n-40)
	}
	a.convMu.Unlock()

	<-spoken
}

// capture takes a screenshot without Spark itself.
func (a *App) capture(parent context.Context) (*vision.Frame, *Capture, error) {
	ctx, cancel := context.WithTimeout(parent, captureTimeout)
	defer cancel()

	f, err := a.vision.Capture(ctx)
	if errors.Is(err, vision.ErrPermission) {
		vision.RequestPermission()
		return nil, nil, errors.New("autorise Spark dans Réglages Système › Confidentialité › Enregistrement de l'écran, puis relance l'app")
	}
	if err != nil {
		return nil, nil, err
	}
	a.debugf("vision: %dx%d, %d KB in %v", f.Width, f.Height, len(f.Data)>>10, f.Took)

	const prefix = "data:image/jpeg;base64,"
	url := make([]byte, 0, len(prefix)+(len(f.Data)+2)/3*4)
	url = f.AppendBase64(append(url, prefix...))
	return f, &Capture{
		Width: f.Width, Height: f.Height, Bytes: len(f.Data),
		Millis: f.Took.Milliseconds(), DataURL: string(url),
	}, nil
}
