package audio

// SystemTTS speaks with the voices built into the OS. It is the zero-setup
// fallback; a neural cloud voice will replace it for A.R.I.A's final voice.
type SystemTTS struct {
	voice string // resolved voice name, "" = OS default
}

// NewSystemTTS picks the requested voice, or the best French female voice
// available on this machine.
func NewSystemTTS(voice string) *SystemTTS {
	if voice == "" {
		voice = bestVoice()
	}
	return &SystemTTS{voice: voice}
}

// Voice returns the selected voice name.
func (t *SystemTTS) Voice() string { return t.voice }

var _ TTS = (*SystemTTS)(nil)
