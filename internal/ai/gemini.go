package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const (
	geminiBaseURL      = "https://generativelanguage.googleapis.com/v1beta"
	DefaultGeminiModel = "gemini-3.8-flash"
)

// Gemini calls the Google Generative Language REST API (streamGenerateContent
// over SSE). No SDK: one HTTP request, one line-based reader.
type Gemini struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client

	// Thinking costs latency; "low" keeps answers snappy. If the model
	// rejects the field, it is dropped for the rest of the session.
	thinkingLevel string
	noThinking    atomic.Bool
}

// NewGemini creates a client; empty model and baseURL select the defaults.
func NewGemini(apiKey, model, baseURL string) *Gemini {
	if model == "" {
		model = DefaultGeminiModel
	}
	if baseURL == "" {
		baseURL = geminiBaseURL
	}
	return &Gemini{
		apiKey:        apiKey,
		model:         model,
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		thinkingLevel: "low",
		// No global timeout: a stream may legitimately last a while; the
		// caller's context bounds it.
		client: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			ForceAttemptHTTP2:     true,
		}},
	}
}

func (g *Gemini) Name() string { return "Gemini · " + g.model }

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
	Thought    bool              `json:"thought,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data"` // encoding/json emits standard base64
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  map[string]any  `json:"generation_config,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

type geminiError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (g *Gemini) body(system string, history []Message) ([]byte, error) {
	req := geminiRequest{Contents: make([]geminiContent, 0, len(history))}
	if system != "" {
		req.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: system}}}
	}
	for _, m := range history {
		c := geminiContent{Role: string(m.Role)}
		if m.Image != nil {
			c.Parts = append(c.Parts, geminiPart{InlineData: &geminiInlineData{MimeType: "image/jpeg", Data: m.Image}})
		}
		if m.Text != "" {
			c.Parts = append(c.Parts, geminiPart{Text: m.Text})
		}
		if len(c.Parts) > 0 {
			req.Contents = append(req.Contents, c)
		}
	}
	if g.thinkingLevel != "" && !g.noThinking.Load() {
		req.GenerationConfig = map[string]any{"thinking_level": g.thinkingLevel}
	}
	return json.Marshal(req)
}

func (g *Gemini) Stream(ctx context.Context, system string, history []Message) (<-chan Chunk, error) {
	resp, err := g.post(ctx, system, history)
	if err != nil {
		return nil, err
	}
	ch := make(chan Chunk, 16)
	go g.read(ctx, resp.Body, ch)
	return ch, nil
}

func (g *Gemini) post(ctx context.Context, system string, history []Message) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		body, err := g.body(system, history)
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", g.baseURL, g.model)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", g.apiKey) // header, never in the URL/logs

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Gemini injoignable : %w", err)
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		apiErr := decodeError(resp)
		if resp.StatusCode == http.StatusBadRequest && attempt == 0 && !g.noThinking.Load() &&
			strings.Contains(strings.ToLower(apiErr.Error.Message), "thinking") {
			g.noThinking.Store(true)
			continue
		}
		return nil, friendlyError(resp.StatusCode, apiErr)
	}
}

func decodeError(resp *http.Response) geminiError {
	defer resp.Body.Close()
	var e geminiError
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	_ = json.Unmarshal(data, &e)
	if e.Error.Message == "" {
		e.Error.Message = strings.TrimSpace(string(data))
	}
	return e
}

func friendlyError(status int, e geminiError) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("clé API Gemini refusée : vérifie SPARK_LLM_API_KEY")
	case http.StatusTooManyRequests:
		return errors.New("quota Gemini atteint (niveau gratuit) : réessaie dans une minute")
	case http.StatusNotFound:
		return fmt.Errorf("modèle Gemini introuvable : %s", e.Error.Message)
	}
	return fmt.Errorf("Gemini %d : %s", status, e.Error.Message)
}

// read parses the SSE stream: one "data: {json}" line per chunk.
func (g *Gemini) read(ctx context.Context, body io.ReadCloser, ch chan<- Chunk) {
	defer close(ch)
	defer body.Close()

	send := func(c Chunk) bool {
		select {
		case ch <- c:
			return true
		case <-ctx.Done():
			return false
		}
	}

	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		var r geminiResponse
		if err := json.Unmarshal(bytes.TrimSpace(line[5:]), &r); err != nil {
			send(Chunk{Err: fmt.Errorf("réponse Gemini illisible : %w", err)})
			return
		}
		if r.PromptFeedback != nil && r.PromptFeedback.BlockReason != "" {
			send(Chunk{Err: fmt.Errorf("demande bloquée par Gemini (%s)", r.PromptFeedback.BlockReason)})
			return
		}
		for _, c := range r.Candidates {
			for _, p := range c.Content.Parts {
				if p.Text != "" && !p.Thought && !send(Chunk{Text: p.Text}) {
					return
				}
			}
			if c.FinishReason == "SAFETY" || c.FinishReason == "PROHIBITED_CONTENT" {
				send(Chunk{Err: errors.New("réponse interrompue par les filtres de Gemini")})
				return
			}
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		send(Chunk{Err: fmt.Errorf("flux Gemini interrompu : %w", err)})
	}
}

var _ Provider = (*Gemini)(nil)
