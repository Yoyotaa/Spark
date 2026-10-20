package audio

import (
	"context"
	"encoding/binary"
)

// Player plays a stream of 16-bit little-endian mono PCM as it arrives.
type Player interface {
	Start(sampleRate int) error
	Write(pcm []byte)
	// Drain blocks until everything written is played, or ctx is done
	// (then playback stops at once).
	Drain(ctx context.Context) error
	Stop()
}

// NewPlayer returns the native streaming player.
func NewPlayer() Player { return newPlayer() }

// samples converts little-endian PCM bytes to int16 samples.
func samples(pcm []byte) []int16 {
	out := make([]int16, len(pcm)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(pcm[2*i:]))
	}
	return out
}
