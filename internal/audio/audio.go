// Package audio holds the voice pipeline: STT for the user's commands and
// TTS for A.R.I.A's answers.
//
// Planned design (phase 4): push-to-talk first (no always-on mic), PCM frames
// flowing through channels, TTS fed sentence by sentence from the LLM stream.
package audio

import "context"

// STT turns speech into text.
type STT interface {
	// Transcribe consumes PCM16 mono frames until the channel is closed.
	Transcribe(ctx context.Context, pcm <-chan []int16) (string, error)
}

// TTS turns text into speech and plays it.
type TTS interface {
	Speak(ctx context.Context, text string) error
}
