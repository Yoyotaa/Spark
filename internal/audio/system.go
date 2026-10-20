package audio

import "sync/atomic"

// SystemTTS speaks with the voices built into the OS. It is the zero-setup
// fallback; a neural cloud voice will replace it for A.R.I.A's final voice.
type SystemTTS struct {
	cfg atomic.Pointer[ttsConfig]
}

type ttsConfig struct {
	voice string  // "" = OS default
	rate  float64 // 1 = normal speed
}

// Voice is an installed system voice.
type Voice struct {
	Name string `json:"name"`
	Lang string `json:"lang"` // e.g. fr_FR, en-US
}

func NewSystemTTS() *SystemTTS {
	t := &SystemTTS{}
	t.Configure("", 1)
	return t
}

// Configure selects a voice ("" = best French female voice available) and
// a speed multiplier (0.5-2). Safe to call while speaking: it applies from
// the next sentence.
func (t *SystemTTS) Configure(voice string, rate float64) {
	if voice == "" {
		voice = bestVoice()
	}
	if rate <= 0 {
		rate = 1
	}
	t.cfg.Store(&ttsConfig{voice: voice, rate: rate})
}

// Voice returns the selected voice name.
func (t *SystemTTS) Voice() string { return t.cfg.Load().voice }

// ListVoices returns the voices installed on this machine.
func ListVoices() ([]Voice, error) { return listVoices() }

var _ TTS = (*SystemTTS)(nil)
