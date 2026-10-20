//go:build windows

package audio

import (
	"bufio"
	"bytes"
	"context"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

// Windows picks the voice through SAPI hints (female, fr-FR), e.g. Hortense
// or Julie depending on the installed language packs.
func bestVoice() string { return "" }

func powershell(ctx context.Context, script string, env ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

const listScript = `Add-Type -AssemblyName System.Speech
$s = New-Object System.Speech.Synthesis.SpeechSynthesizer
$s.GetInstalledVoices() | ForEach-Object { $_.VoiceInfo.Name + "|" + $_.VoiceInfo.Culture.Name }`

func listVoices() ([]Voice, error) {
	out, err := powershell(context.Background(), listScript).Output()
	if err != nil {
		return nil, err
	}
	var voices []Voice
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if name, lang, ok := strings.Cut(strings.TrimSpace(sc.Text()), "|"); ok {
			voices = append(voices, Voice{Name: name, Lang: lang})
		}
	}
	return voices, nil
}

// The text and voice go through environment variables, never through the
// script itself, so no input can be interpreted as PowerShell code.
const speakScript = `Add-Type -AssemblyName System.Speech
$s = New-Object System.Speech.Synthesis.SpeechSynthesizer
if ($env:SPARK_TTS_VOICE) { $s.SelectVoice($env:SPARK_TTS_VOICE) }
else { try { $s.SelectVoiceByHints('Female', 'Adult', 0, [Globalization.CultureInfo]'fr-FR') } catch {} }
$s.Rate = [int]$env:SPARK_TTS_RATE
$s.Speak($env:SPARK_TTS_TEXT)`

func (t *SystemTTS) Speak(ctx context.Context, text string) error {
	cfg := t.cfg.Load()
	// SAPI rate is -10..10, roughly doubling every 10 steps.
	rate := int(math.Round(10 * math.Log2(cfg.rate)))
	rate = min(max(rate, -10), 10)
	return powershell(ctx, speakScript,
		"SPARK_TTS_TEXT="+text, "SPARK_TTS_VOICE="+cfg.voice, "SPARK_TTS_RATE="+strconv.Itoa(rate)).Run()
}
