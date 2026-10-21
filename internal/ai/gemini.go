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
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

const (
	geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	// Measured on the free tier (Oct 2026), question + screenshot, first
	// token: 3.6 Flash ≈ 1.8 s with thinking "low" (5-10 s by default),
	// 3.5 Flash-Lite ≈ 0.9 s, 3.8 Flash queued for ~40 s.
	DefaultGeminiModel = "gemini-3.6-flash"

	// The free tier answers 503 "high demand" in bursts: one quick retry.
	retryDelay = 800 * time.Millisecond

	// Without response headers after this delay, the model is considered
	// saturated (the free tier queues requests for tens of seconds).
	headerTimeout = 10 * time.Second

	// Used when the chosen model is saturated: the fastest one, which has
	// its own free-tier quota.
	FallbackGeminiModel = "gemini-3.5-flash-lite"
)

// Gemini calls the Google Generative Language REST API (streamGenerateContent
// over SSE). No SDK: one HTTP request, one line-based reader.
type Gemini struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client

	thinkingLevel string
	noThinking    atomic.Bool

	// fallback replaces model for one request when it is saturated
	// (timeout, 503, 429). OnFallback, if set, is told about it.
	fallback   string
	OnFallback func(from, to string, reason error)
	// skipUntil (unix nanos): the primary model is known to be saturated,
	// go straight to the fallback instead of wasting a round trip.
	skipUntil atomic.Int64
}

// How long a saturated model is skipped: quotas reset per minute, queues
// clear faster.
const (
	quotaCooldown = 60 * time.Second
	busyCooldown  = 30 * time.Second
)

// NewGemini creates a client; empty model and baseURL select the defaults.
// thinking is the thinking level ("low" recommended: on screenshots the
// model thinks 5-10x more than on text, "low" cuts the first-token latency
// by 3-5x); it is dropped if the model rejects it.
func NewGemini(apiKey, model, baseURL, thinking string) *Gemini {
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
		thinkingLevel: thinking,
		fallback:      FallbackGeminiModel,
		// No global timeout: a stream may legitimately last a while; the
		// caller's context bounds it.
		client: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: headerTimeout,
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
	SystemInstruction *geminiContent   `json:"system_instruction,omitempty"`
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  *geminiGenConfig `json:"generation_config,omitempty"`
}

type geminiGenConfig struct {
	ThinkingConfig *geminiThinking `json:"thinking_config,omitempty"`
}

type geminiThinking struct {
	ThinkingLevel string `json:"thinking_level"`
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
		req.GenerationConfig = &geminiGenConfig{ThinkingConfig: &geminiThinking{ThinkingLevel: g.thinkingLevel}}
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

// errSaturated wraps the failures that justify trying the fallback model.
type errSaturated struct {
	err   error
	quota bool // 429: per-minute quota exhausted
}

func (e errSaturated) Error() string { return e.err.Error() }
func (e errSaturated) Unwrap() error { return e.err }

func (g *Gemini) post(ctx context.Context, system string, history []Message) (*http.Response, error) {
	canFallback := g.fallback != "" && g.fallback != g.model
	if canFallback && time.Now().UnixNano() < g.skipUntil.Load() {
		return g.postModel(ctx, g.fallback, system, history)
	}
	resp, err := g.postModel(ctx, g.model, system, history)
	var sat errSaturated
	if err == nil || !errors.As(err, &sat) || !canFallback || ctx.Err() != nil {
		return resp, err
	}
	cooldown := busyCooldown
	if sat.quota {
		cooldown = quotaCooldown
	}
	g.skipUntil.Store(time.Now().Add(cooldown).UnixNano())
	if g.OnFallback != nil {
		g.OnFallback(g.model, g.fallback, err)
	}
	return g.postModel(ctx, g.fallback, system, history)
}

func (g *Gemini) postModel(ctx context.Context, model, system string, history []Message) (*http.Response, error) {
	retried := false
	for {
		body, err := g.body(system, history)
		if err != nil {
			return nil, err
		}
		url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", g.baseURL, model)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", g.apiKey) // header, never in the URL/logs

		resp, err := g.client.Do(req)
		switch {
		case err != nil && ctx.Err() != nil:
			return nil, ctx.Err()
		case err != nil && isTimeout(err):
			return nil, errSaturated{err: fmt.Errorf("%s ne répond pas (modèle saturé) : réessaie ou choisis un autre modèle dans les paramètres", model)}
		case err != nil:
			return nil, fmt.Errorf("Gemini injoignable : %w", err)
		case resp.StatusCode == http.StatusOK:
			return resp, nil
		}

		apiErr := decodeError(resp)
		switch {
		case retried:
		case resp.StatusCode == http.StatusBadRequest && !g.noThinking.Load() &&
			strings.Contains(strings.ToLower(apiErr.Error.Message), "thinking"):
			g.noThinking.Store(true) // this model has no thinking control
			retried = true
			continue
		case resp.StatusCode == http.StatusServiceUnavailable:
			retried = true
			select {
			case <-time.After(retryDelay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		err = friendlyError(resp.StatusCode, apiErr)
		if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
			err = errSaturated{err: err, quota: resp.StatusCode == http.StatusTooManyRequests}
		}
		return nil, err
	}
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
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
	case http.StatusServiceUnavailable:
		return errors.New("Gemini est saturé en ce moment (niveau gratuit) : réessaie dans quelques secondes")
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

// ListModels returns the models that can generate content.
func (g *Gemini) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"/models?pageSize=1000", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", g.apiKey)
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini injoignable : %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, friendlyError(resp.StatusCode, decodeError(resp))
	}
	defer resp.Body.Close()
	var list struct {
		Models []struct {
			Name    string   `json:"name"`
			Methods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range list.Models {
		if slices.Contains(m.Methods, "generateContent") {
			ids = append(ids, strings.TrimPrefix(m.Name, "models/"))
		}
	}
	return ids, nil
}
