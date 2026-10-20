// Package ai talks to a multimodal LLM on behalf of A.R.I.A.
//
// Responses are streamed: text chunks are pushed to the UI and to the TTS as
// they arrive, so A.R.I.A starts speaking before the answer is complete.
package ai

import "context"

// Persona is the system prompt shared by every provider. Answers are read
// aloud, hence the "no formatting" rules.
const Persona = `Tu es A.R.I.A (Augmented Reality & Intelligence Agent), la coéquipière de l'utilisateur, façon Jarvis.
Tu vois son écran grâce à la capture jointe à sa question (jeu vidéo, navigation, travail...).
Règles :
- Réponds en français et tutoie l'utilisateur, de façon concise et directe : une à trois phrases, sauf demande explicite.
- Va droit au conseil actionnable ; appuie-toi sur ce que tu vois à l'écran.
- Tes réponses sont lues à voix haute : pas de markdown, pas de listes, pas d'emoji.
- Si l'image ne permet pas de répondre, dis-le en une phrase.`

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

// Provider is a multimodal LLM backend. Stream must have finished reading
// the history (images included) when it returns: callers may reuse the
// image buffers right after.
type Provider interface {
	Name() string
	Stream(ctx context.Context, system string, history []Message) (<-chan Chunk, error)
}
