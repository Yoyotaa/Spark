package audio

import (
	"context"
	"encoding/binary"
)

// InputEvents receives what the microphone produces. Callbacks run on
// arbitrary goroutines and must return quickly.
type InputEvents struct {
	Level   func(rms float32) // ~20 Hz while the mic runs
	Partial func(text string) // live transcription (native engines only)
	Final   func(text string) // end of an utterance ("" if nothing heard)
	Error   func(err error)
}

// Input is the microphone plus speech recognition.
//
// The mic (StartMic/StopMic) can run without recognizing, which is how the
// continuous mode watches the level for voice activity; Begin/End delimit
// one utterance to transcribe.
type Input interface {
	StartMic() error
	StopMic()
	Begin(lang string) error // "fr" | "en"
	End()                    // stop listening, the Final event follows
	Cancel()                 // drop the utterance, no Final event
	// Live reports whether partial transcriptions are streamed.
	Live() bool
}

// Transcriber turns a recorded utterance into text (cloud engines).
type Transcriber interface {
	Transcribe(ctx context.Context, wav []byte, lang string) (string, error)
}

// Permission states.
type Permission int

const (
	PermissionUnknown Permission = iota // never asked
	PermissionDenied
	PermissionGranted
)

func (p Permission) String() string {
	return [...]string{"unknown", "denied", "granted"}[p]
}

func locale(lang string) string {
	if lang == "en" {
		return "en-US"
	}
	return "fr-FR"
}

// wav wraps 16-bit mono PCM samples in a WAV container.
func wav(pcm []byte, sampleRate int) []byte {
	out := make([]byte, 44+len(pcm))
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1) // PCM
	binary.LittleEndian.PutUint16(out[22:], 1) // mono
	binary.LittleEndian.PutUint32(out[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(out[28:], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(out[32:], 2)
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm)))
	copy(out[44:], pcm)
	return out
}
