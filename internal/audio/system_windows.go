//go:build windows

package audio

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Windows picks the voice through SAPI hints (female, fr-FR), e.g. Hortense
// or Julie depending on the installed language packs.
func bestVoice() string { return "" }

// The text and voice go through environment variables, never through the
// script itself, so no input can be interpreted as PowerShell code.
const speakScript = `Add-Type -AssemblyName System.Speech
$s = New-Object System.Speech.Synthesis.SpeechSynthesizer
if ($env:SPARK_TTS_VOICE) { $s.SelectVoice($env:SPARK_TTS_VOICE) }
else { try { $s.SelectVoiceByHints('Female', 'Adult', 0, [Globalization.CultureInfo]'fr-FR') } catch {} }
$s.Speak($env:SPARK_TTS_TEXT)`

func (t *SystemTTS) Speak(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", speakScript)
	cmd.Env = append(os.Environ(), "SPARK_TTS_TEXT="+text, "SPARK_TTS_VOICE="+t.voice)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd.Run()
}
