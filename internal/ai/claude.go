package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const fallbackBeta = "server-side-fallback-2026-07-01"

// Claude streams from the Anthropic Messages API through the official SDK.
type Claude struct {
	client   anthropic.Client
	model    string
	effort   string
	fallback bool
}

func NewClaude(cfg Config) *Claude {
	// One retry at most: a spoken assistant is better off with a quick,
	// clear error than with a long backoff.
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey), option.WithMaxRetries(1)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	model := cfg.Model
	if model == "" {
		model = "claude-opus-5-5"
	}
	return &Claude{
		client: anthropic.NewClient(opts...),
		model:  model,
		// Effort is the latency/depth knob on current models; "low" suits
		// a spoken assistant that must answer in seconds.
		effort: cfg.Thinking,
		// Server-side fallback is a Claude API feature (not on proxies) and
		// is not offered for Haiku.
		fallback: cfg.Fallback && cfg.BaseURL == "" && supportsFallback(model),
	}
}

func supportsFallback(model string) bool {
	for _, p := range []string{"claude-opus-5", "claude-fable-5", "claude-sonnet-5-5"} {
		if strings.HasPrefix(model, p) {
			return true
		}
	}
	return false
}

func (c *Claude) Name() string { return "Claude · " + c.model }

func (c *Claude) params(system string, history []Message) anthropic.MessageNewParams {
	msgs := make([]anthropic.MessageParam, 0, len(history))
	for _, m := range history {
		var blocks []anthropic.ContentBlockParamUnion
		if m.Image != nil {
			blocks = append(blocks, anthropic.NewImageBlockBase64("image/jpeg", base64.StdEncoding.EncodeToString(m.Image)))
		}
		if m.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(m.Text))
		}
		if len(blocks) == 0 {
			continue
		}
		if m.Role == RoleModel {
			msgs = append(msgs, anthropic.NewAssistantMessage(blocks...))
		} else {
			msgs = append(msgs, anthropic.NewUserMessage(blocks...))
		}
	}
	p := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 16000,
		Messages:  msgs,
	}
	if system != "" {
		p.System = []anthropic.TextBlockParam{{Text: system}}
	}
	if c.effort != "" {
		p.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(c.effort)}
	}
	return p
}

func (c *Claude) Stream(ctx context.Context, system string, history []Message) (<-chan Chunk, error) {
	var opts []option.RequestOption
	if c.fallback {
		opts = append(opts,
			option.WithHeaderAdd("anthropic-beta", fallbackBeta),
			option.WithJSONSet("fallbacks", "default"))
	}
	stream := c.client.Messages.NewStreaming(ctx, c.params(system, history), opts...)

	// Wait for the first event so auth/quota errors surface as a refused
	// request instead of a broken stream.
	if !stream.Next() {
		err := stream.Err()
		_ = stream.Close()
		if err == nil {
			err = errors.New("réponse vide")
		}
		return nil, claudeError(ctx, err)
	}

	ch := make(chan Chunk, 16)
	go func() {
		defer close(ch)
		defer stream.Close()
		send := func(ck Chunk) bool {
			select {
			case ch <- ck:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for {
			switch ev := stream.Current().AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				if d, ok := ev.Delta.AsAny().(anthropic.TextDelta); ok && d.Text != "" && !send(Chunk{Text: d.Text}) {
					return
				}
			case anthropic.MessageDeltaEvent:
				if ev.Delta.StopReason == anthropic.StopReasonRefusal {
					send(Chunk{Err: errors.New("Claude a refusé de répondre (filtre de sécurité)")})
					return
				}
			}
			if !stream.Next() {
				break
			}
		}
		if err := stream.Err(); err != nil && ctx.Err() == nil {
			send(Chunk{Err: claudeError(ctx, err)})
		}
	}()
	return ch, nil
}

func claudeError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("Claude injoignable : %w", err)
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return errors.New("clé API Claude refusée : vérifie-la dans les paramètres")
	case http.StatusTooManyRequests:
		return errors.New("limite de requêtes Claude atteinte : réessaie dans un instant")
	case http.StatusNotFound:
		return errors.New("modèle Claude introuvable : choisis-en un autre dans les paramètres")
	case 529:
		return errors.New("Claude est surchargé en ce moment : réessaie dans quelques secondes")
	}
	return fmt.Errorf("Claude %d : %s", apiErr.StatusCode, apiErr.Error())
}

func (c *Claude) ListModels(ctx context.Context) ([]string, error) {
	pager := c.client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{})
	var ids []string
	for pager.Next() {
		ids = append(ids, pager.Current().ID)
	}
	if err := pager.Err(); err != nil {
		return nil, claudeError(ctx, err)
	}
	return ids, nil
}

var _ Provider = (*Claude)(nil)
