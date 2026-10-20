// Package ai talks to a multimodal LLM on behalf of A.R.I.A.
//
// Planned design (phase 3): provider-agnostic interface, streamed responses
// (tokens are pushed to the UI and to the TTS as they arrive, so A.R.I.A
// starts speaking before the full answer is generated).
package ai

import "context"

// Persona is the system prompt shared by every provider.
const Persona = `Tu es A.R.I.A (Augmented Reality & Intelligence Agent), la coéquipière de l'utilisateur.
Tu vois son écran. Réponds de façon concise, pertinente et directe : une à trois phrases, sauf demande explicite.`

type Message struct {
	Role  string // "user" | "assistant"
	Text  string
	Image []byte // optional JPEG screenshot
}

// Chunk is a piece of a streamed answer. Err is set on the final chunk if
// the stream failed.
type Chunk struct {
	Text string
	Err  error
}

// Provider is a multimodal LLM backend.
type Provider interface {
	Stream(ctx context.Context, history []Message) (<-chan Chunk, error)
}
