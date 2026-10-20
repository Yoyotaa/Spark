//go:build !darwin && !windows

package audio

import (
	"context"
	"errors"
)

func bestVoice() string { return "" }

func (t *SystemTTS) Speak(context.Context, string) error {
	return errors.New("audio: no system TTS on this platform")
}
