//go:build darwin

package audio

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strconv"
)

// French female voices, best first. Premium/Enhanced variants are free
// downloads: System Settings › Accessibility › Spoken Content › Voices.
var preferredVoices = []string{
	"Audrey (Premium)", "Audrey (Enhanced)", "Aurélie (Premium)", "Aurélie (Enhanced)",
	"Amélie (Premium)", "Amélie (Enhanced)", "Audrey", "Aurélie", "Amélie",
	"Flo (Français (France))", "Sandy (Français (France))",
}

// `say -v ?` lines look like: "Amélie              fr_CA    # Bonjour!"
var voiceLine = regexp.MustCompile(`^(.+?)\s+([a-z]{2,3}_[A-Z0-9]{2,3})\s+#`)

// say's default speaking rate, in words per minute.
const baseWPM = 185

func listVoices() ([]Voice, error) {
	out, err := exec.Command("say", "-v", "?").Output()
	if err != nil {
		return nil, err
	}
	var voices []Voice
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if m := voiceLine.FindStringSubmatch(sc.Text()); m != nil {
			voices = append(voices, Voice{Name: m[1], Lang: m[2]})
		}
	}
	return voices, nil
}

func bestVoice() string {
	voices, err := listVoices()
	if err != nil {
		return ""
	}
	installed := map[string]bool{}
	for _, v := range voices {
		installed[v.Name] = true
	}
	for _, v := range preferredVoices {
		if installed[v] {
			return v
		}
	}
	return ""
}

// Speak blocks until the sentence is spoken; cancelling ctx stops it.
func (t *SystemTTS) Speak(ctx context.Context, text string) error {
	cfg := t.cfg.Load()
	args := []string{"-r", strconv.Itoa(int(baseWPM * cfg.rate))}
	if cfg.voice != "" {
		args = append(args, "-v", cfg.voice)
	}
	// "--" so a text starting with "-" is never parsed as a flag.
	args = append(args, "--", text)
	return exec.CommandContext(ctx, "say", args...).Run()
}
