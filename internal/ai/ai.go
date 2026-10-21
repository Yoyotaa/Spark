// Package ai talks to a multimodal LLM on behalf of A.R.I.A.
//
// Responses are streamed: text chunks are pushed to the UI and to the TTS as
// they arrive, so A.R.I.A starts speaking before the answer is complete.
package ai

import (
	"context"
	"fmt"
	"strings"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
)

type Message struct {
	Role  Role
	Text  string
	Image []byte // optional JPEG screenshot
}

// Chunk is a piece of a streamed answer. Err is set on the final chunk if
// the stream failed.
type Chunk struct {
	Text string
	Err  error
}

// Provider is a multimodal LLM backend. Stream returns once the request is
// accepted, and must have finished reading the history (images included) by
// then: callers may reuse the image buffers right after.
type Provider interface {
	Name() string
	Stream(ctx context.Context, system string, history []Message) (<-chan Chunk, error)
	// ListModels returns the model ids usable with this provider and key.
	ListModels(ctx context.Context) ([]string, error)
}

// Config selects and tunes a provider.
type Config struct {
	Provider string // "gemini" | "claude" | "openai"
	APIKey   string
	Model    string
	BaseURL  string // "" = official endpoint
	// Thinking trades latency for depth: "low" | "medium" | "high".
	Thinking string
	// Fallback re-runs safety-declined requests on another model (Claude).
	Fallback bool
}

// New builds the provider described by cfg.
func New(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "gemini":
		if cfg.APIKey == "" {
			return nil, errNoKey("Gemini", "aistudio.google.com/apikey")
		}
		return NewGemini(cfg.APIKey, cfg.Model, cfg.BaseURL, cfg.Thinking), nil
	case "claude":
		if cfg.APIKey == "" {
			return nil, errNoKey("Claude", "console.anthropic.com")
		}
		return NewClaude(cfg), nil
	case "openai":
		if cfg.APIKey == "" && cfg.BaseURL == "" {
			return nil, errNoKey("OpenAI", "platform.openai.com/api-keys")
		}
		return NewOpenAI(cfg), nil
	}
	return nil, fmt.Errorf("fournisseur inconnu : %q", cfg.Provider)
}

func errNoKey(name, where string) error {
	return fmt.Errorf("clé API %s manquante : ajoute-la dans Paramètres › Intelligence (%s)", name, where)
}

// PromptOptions shape A.R.I.A's system prompt.
type PromptOptions struct {
	Language string // "fr" | "en"
	Length   string // "short" | "normal" | "detailed"
	Custom   string // user instructions
}

// Silence is what a proactive look answers when nothing deserves a remark.
const Silence = "RAS"

// ProactivePrompt is appended to the persona for unsolicited looks.
func ProactivePrompt(lang string) string {
	if lang == "en" {
		return "\nNobody asked you anything: you are glancing at the screen on your own. Speak only if you see something important, urgent or genuinely useful right now (a danger, an opportunity, a visible mistake). Then say it in one short sentence. Otherwise answer exactly " + Silence + "."
	}
	return "\nPersonne ne t'a rien demandé : tu jettes un œil à l'écran de toi-même. Ne parle que si tu vois quelque chose d'important, d'urgent ou de vraiment utile à cet instant (un danger, une opportunité, une erreur visible). Dis-le alors en une seule phrase courte. Sinon, réponds exactement " + Silence + "."
}

// SystemPrompt builds A.R.I.A's persona. Answers are read aloud, hence the
// "no formatting" rules.
func SystemPrompt(o PromptOptions) string {
	var b strings.Builder
	if o.Language == "en" {
		b.WriteString(`You are A.R.I.A (Augmented Reality & Intelligence Agent), the user's teammate, Jarvis-style.
You see their screen through the screenshot attached to their question (video game, browsing, work...).
Rules:
- Answer in English, casually, concise and direct; go straight to actionable advice based on what you see.
- Your answers are read aloud: no markdown, no lists, no emoji.
- If the image is not enough to answer, say so in one sentence.
`)
		switch o.Length {
		case "short":
			b.WriteString("- One or two short sentences at most.\n")
		case "detailed":
			b.WriteString("- Detailed answers are welcome when useful, in plain spoken sentences.\n")
		default:
			b.WriteString("- One to three sentences, unless explicitly asked for more.\n")
		}
	} else {
		b.WriteString(`Tu es A.R.I.A (Augmented Reality & Intelligence Agent), la coéquipière de l'utilisateur, façon Jarvis.
Tu vois son écran grâce à la capture jointe à sa question (jeu vidéo, navigation, travail...).
Règles :
- Réponds en français et tutoie l'utilisateur, de façon concise et directe ; va droit au conseil actionnable en t'appuyant sur ce que tu vois.
- Tes réponses sont lues à voix haute : pas de markdown, pas de listes, pas d'emoji.
- Si l'image ne permet pas de répondre, dis-le en une phrase.
`)
		switch o.Length {
		case "short":
			b.WriteString("- Une ou deux phrases courtes au maximum.\n")
		case "detailed":
			b.WriteString("- Tu peux détailler quand c'est utile, en phrases parlées.\n")
		default:
			b.WriteString("- Une à trois phrases, sauf demande explicite.\n")
		}
	}
	if c := strings.TrimSpace(o.Custom); c != "" {
		if o.Language == "en" {
			b.WriteString("\nAdditional instructions from the user:\n")
		} else {
			b.WriteString("\nInstructions supplémentaires de l'utilisateur :\n")
		}
		b.WriteString(c)
		b.WriteByte('\n')
	}
	return b.String()
}
