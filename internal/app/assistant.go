package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"spark/internal/ai"
	"spark/internal/vision"
)

const (
	captureTimeout = 5 * time.Second
	// Text-only turns kept as context; screenshots are never kept in the
	// history (only the current question carries one) to bound tokens.
	maxHistory = 12
	// Used when the user sends the screen without a question.
	defaultQuestion = "Que vois-tu, et que me conseilles-tu ?"
)

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

// Status tells the UI which modules are available.
type Status struct {
	AI      string `json:"ai"`
	AIReady bool   `json:"aiReady"`
}

func (a *App) Status() Status {
	if a.llm == nil {
		return Status{AI: "non configurée"}
	}
	return Status{AI: a.llm.Name(), AIReady: true}
}

// Ask sends a question to A.R.I.A, with a screenshot taken right now when
// withScreen is set. A new question interrupts the previous answer (text and
// voice). It returns once the request is accepted; the answer streams through
// EventDelta / EventDone.
func (a *App) Ask(text string, withScreen bool) (Turn, error) {
	if a.llm == nil {
		return Turn{}, errors.New("IA non configurée : lance Spark avec SPARK_LLM_API_KEY (clé gratuite sur aistudio.google.com)")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		if !withScreen {
			return Turn{}, errors.New("question vide")
		}
		text = defaultQuestion
	}

	a.askMu.Lock()
	defer a.askMu.Unlock()
	// Interrupt and wait for the previous answer so the history keeps a
	// strict user/model alternation.
	if a.askCancel != nil {
		a.askCancel()
		<-a.askDone
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.askID++
	turn := Turn{ID: a.askID}

	var frame *vision.Frame
	if withScreen {
		var err error
		frame, turn.Capture, err = a.capture(ctx)
		if err != nil {
			cancel()
			return Turn{}, err
		}
		// Safe: the provider has serialized the image when Stream returns.
		defer frame.Release()
	}

	a.convMu.Lock()
	history := slices.Clone(a.history)
	a.convMu.Unlock()
	question := ai.Message{Role: ai.RoleUser, Text: text}
	if frame != nil {
		question.Image = frame.Data
	}
	history = append(history, question)

	start := time.Now()
	stream, err := a.llm.Stream(ctx, ai.Persona, history)
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
	go a.answer(ctx, cancel, done, turn.ID, stream, start)
	return turn, nil
}

// answer forwards the stream to the UI and, sentence by sentence, to the
// voice, then records the reply in the history.
func (a *App) answer(ctx context.Context, cancel context.CancelFunc, done chan struct{}, id int, stream <-chan ai.Chunk, start time.Time) {
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
		runtime.EventsEmit(a.ctx, EventDelta, Delta{ID: id, Text: c.Text})
		for _, s := range splitter.Push(c.Text) {
			sentences <- s
		}
	}
	if rest := splitter.Flush(); rest != "" && streamErr == nil {
		sentences <- rest
	}
	close(sentences)

	d := Done{ID: id, Interrupted: ctx.Err() != nil && a.ctx.Err() == nil}
	if streamErr != nil && !d.Interrupted {
		d.Error = streamErr.Error()
	}
	runtime.EventsEmit(a.ctx, EventDone, d)
	a.debugf("ai: answer %d done in %v (%d chars)", id, time.Since(start), full.Len())

	reply := full.String()
	if reply == "" {
		reply = "…"
	}
	a.convMu.Lock()
	a.history = append(a.history, ai.Message{Role: ai.RoleModel, Text: reply})
	if n := len(a.history); n > maxHistory {
		a.history = slices.Delete(a.history, 0, n-maxHistory)
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
