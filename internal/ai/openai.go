package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
)

const openAIBaseURL = "https://api.openai.com/v1"

// OpenAI streams from the Chat Completions API, which most providers also
// expose (OpenRouter, Mistral, Groq, Ollama, LM Studio...): set BaseURL.
type OpenAI struct {
	apiKey  string
	model   string
	baseURL string
	effort  string
	client  *http.Client

	// Non-reasoning models reject reasoning_effort: dropped on first 400.
	noEffort atomic.Bool
}

func NewOpenAI(cfg Config) *OpenAI {
	base := strings.TrimSuffix(cfg.BaseURL, "/")
	if base == "" {
		base = openAIBaseURL
	}
	return &OpenAI{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: base,
		effort:  cfg.Thinking,
		client: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: headerTimeout,
			ForceAttemptHTTP2:     true,
		}},
	}
}

func (o *OpenAI) Name() string {
	if o.baseURL == openAIBaseURL {
		return "OpenAI · " + o.model
	}
	return o.model + " · " + strings.TrimPrefix(strings.TrimPrefix(o.baseURL, "https://"), "http://")
}

type oaPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *oaImgURL `json:"image_url,omitempty"`
}

type oaImgURL struct {
	URL string `json:"url"`
}

type oaMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []oaPart
}

type oaRequest struct {
	Model           string      `json:"model"`
	Messages        []oaMessage `json:"messages"`
	Stream          bool        `json:"stream"`
	ReasoningEffort string      `json:"reasoning_effort,omitempty"`
}

func (o *OpenAI) body(system string, history []Message) ([]byte, error) {
	req := oaRequest{Model: o.model, Stream: true}
	if system != "" {
		req.Messages = append(req.Messages, oaMessage{Role: "system", Content: system})
	}
	for _, m := range history {
		role := "user"
		if m.Role == RoleModel {
			role = "assistant"
		}
		if m.Image == nil {
			req.Messages = append(req.Messages, oaMessage{Role: role, Content: m.Text})
			continue
		}
		url := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(m.Image)
		parts := []oaPart{{Type: "image_url", ImageURL: &oaImgURL{URL: url}}}
		if m.Text != "" {
			parts = append(parts, oaPart{Type: "text", Text: m.Text})
		}
		req.Messages = append(req.Messages, oaMessage{Role: role, Content: parts})
	}
	if o.effort != "" && !o.noEffort.Load() {
		req.ReasoningEffort = o.effort
	}
	return json.Marshal(req)
}

func (o *OpenAI) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, o.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	resp, err := o.client.Do(req)
	switch {
	case err != nil && ctx.Err() != nil:
		return nil, ctx.Err()
	case err != nil && isTimeout(err):
		return nil, fmt.Errorf("%s ne répond pas : essaie un autre modèle", o.model)
	case err != nil:
		return nil, fmt.Errorf("%s injoignable : %w", o.baseURL, err)
	}
	return resp, nil
}

type oaError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func oaDecodeError(resp *http.Response) string {
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var e oaError
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return strings.TrimSpace(string(data))
}

func oaFriendly(status int, msg string) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("clé API refusée : vérifie-la dans les paramètres")
	case http.StatusTooManyRequests:
		return errors.New("quota ou limite de requêtes atteint : réessaie dans un instant")
	case http.StatusNotFound:
		return fmt.Errorf("modèle ou adresse introuvable : %s", msg)
	}
	return fmt.Errorf("erreur %d : %s", status, msg)
}

func (o *OpenAI) Stream(ctx context.Context, system string, history []Message) (<-chan Chunk, error) {
	if o.model == "" {
		return nil, errors.New("aucun modèle choisi : sélectionne-en un dans Paramètres › Intelligence")
	}
	var resp *http.Response
	for retried := false; ; retried = true {
		body, err := o.body(system, history)
		if err != nil {
			return nil, err
		}
		resp, err = o.do(ctx, http.MethodPost, "/chat/completions", body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		msg := oaDecodeError(resp)
		if !retried && resp.StatusCode == http.StatusBadRequest && !o.noEffort.Load() &&
			strings.Contains(strings.ToLower(msg), "reasoning") {
			o.noEffort.Store(true)
			continue
		}
		return nil, oaFriendly(resp.StatusCode, msg)
	}

	ch := make(chan Chunk, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		send := func(c Chunk) bool {
			select {
			case ch <- c:
				return true
			case <-ctx.Done():
				return false
			}
		}
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			data := bytes.TrimSpace(line[5:])
			if bytes.Equal(data, []byte("[DONE]")) {
				return
			}
			var ev struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(data, &ev); err != nil {
				send(Chunk{Err: fmt.Errorf("réponse illisible : %w", err)})
				return
			}
			if ev.Error != nil {
				send(Chunk{Err: errors.New(ev.Error.Message)})
				return
			}
			for _, c := range ev.Choices {
				if c.Delta.Content != "" && !send(Chunk{Text: c.Delta.Content}) {
					return
				}
			}
		}
		if err := sc.Err(); err != nil && ctx.Err() == nil {
			send(Chunk{Err: fmt.Errorf("flux interrompu : %w", err)})
		}
	}()
	return ch, nil
}

func (o *OpenAI) ListModels(ctx context.Context) ([]string, error) {
	resp, err := o.do(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, oaFriendly(resp.StatusCode, oaDecodeError(resp))
	}
	defer resp.Body.Close()
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list.Data))
	for _, m := range list.Data {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	return ids, nil
}

var _ Provider = (*OpenAI)(nil)
