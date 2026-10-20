//go:build !darwin && !windows

package audio

import (
	"context"
	"errors"
)

func bestVoice() string { return "" }

func listVoices() ([]Voice, error) { return nil, nil }

func (t *SystemTTS) Speak(context.Context, string) error {
	return errors.New("audio: no system TTS on this platform")
}
