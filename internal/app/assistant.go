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

// interrupt stops the current answer (request and voice) and waits for it,
// so the history keeps a strict user/model alternation. Caller holds askMu.
func (a *App) interruptLocked() {
	if a.askCancel != nil {
		a.askCancel()
		<-a.askDone
		a.askCancel = nil
	}
}

// Interrupt stops A.R.I.A (answer being generated or spoken).
func (a *App) Interrupt() {
	a.askMu.Lock()
	a.interruptLocked()
	a.askMu.Unlock()
}

// ask is Ask with an optional screenshot already taken (voice questions
// capture the screen when the user starts speaking). It takes ownership of
// frame.
func (a *App) ask(text string, withScreen bool, frame *vision.Frame, capt *Capture) (turn Turn, err error) {
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
	defer a.askMu.Unlock()
	a.interruptLocked()
	ctx, cancel := context.WithCancel(a.ctx)
	a.askID++
	turn = Turn{ID: a.askID, Capture: capt}

	if frame == nil && withScreen {
		frame, turn.Capture, err = a.capture(ctx)
		if err != nil {
			cancel()
			return Turn{}, err
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
		cancel()
		return Turn{}, err
	}
	a.debugf("ai: request accepted in %v", time.Since(start))

	if frame != nil {
		question.Text += " [capture d'écran jointe]"
	}
	question.Image = nil
	a.convMu.Lock()
	a.history = append(a.history, question)
	a.convMu.Unlock()

	done := make(chan struct{})
	a.askCancel, a.askDone = cancel, done
	go a.answer(ctx, cancel, done, turn.ID, stream, start, cfg.Voice.ReadAnswers)
	return turn, nil
}

// answer forwards the stream to the UI and, sentence by sentence, to the
// voice, then records the reply in the history.
func (a *App) answer(ctx context.Context, cancel context.CancelFunc, done chan struct{}, id int, stream <-chan ai.Chunk, start time.Time, readAloud bool) {
	defer close(done)
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
