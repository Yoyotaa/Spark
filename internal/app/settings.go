package app

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"time"

	"spark/internal/ai"
	"spark/internal/audio"
	"spark/internal/config"
	"spark/internal/hotkey"
	"spark/internal/startup"
)

// GetSettings returns the persisted user preferences.
func (a *App) GetSettings() config.Settings {
	return a.settings.Get()
}

// SaveResult is the outcome of SaveSettings.
type SaveResult struct {
	Settings config.Settings `json:"settings"` // as normalized and applied
	// HotkeyError is set when a shortcut could not be registered (taken by
	// another app). The settings are saved anyway.
	HotkeyError string `json:"hotkeyError,omitempty"`
}

// SaveSettings persists and applies the settings immediately. The orb
// position is managed separately (SetOrbPosition) and kept as is.
func (a *App) SaveSettings(s config.Settings) (SaveResult, error) {
	err := a.settings.Update(func(cur *config.Settings) {
		orb := cur.Orb
		*cur = s
		cur.Orb = orb
	})
	if err != nil {
		return SaveResult{}, err
	}
	saved := a.settings.Get()
	a.apply(saved, false)

	res := SaveResult{Settings: saved}
	a.mu.RLock()
	if a.hotkeyErr != nil {
		res.HotkeyError = a.hotkeyErr.Error()
	}
	a.mu.RUnlock()
	return res, nil
}

// ResetSettings restores every default (orb position and API keys kept).
func (a *App) ResetSettings() (SaveResult, error) {
	return a.SaveSettings(config.Defaults())
}

// KeyInfo describes a provider's key without ever exposing it.
type KeyInfo struct {
	Source   string   `json:"source"` // "keychain" | "env" | ""
	Hint     string   `json:"hint"`   // last 4 characters
	EnvNames []string `json:"envNames"`
}

// AIStatus is shown at the top of the AI settings and in the chat footer.
type AIStatus struct {
	Ready bool               `json:"ready"`
	Name  string             `json:"name"`
	Error string             `json:"error,omitempty"`
	Keys  map[string]KeyInfo `json:"keys"`
}

func (a *App) GetAIStatus() AIStatus {
	st := AIStatus{Keys: map[string]KeyInfo{}}
	for _, p := range config.Providers {
		key, src := config.APIKey(p)
		info := KeyInfo{Source: string(src), EnvNames: config.EnvKeyNames(p)}
		if len(key) >= 8 {
			info.Hint = key[len(key)-4:]
		}
		st.Keys[p] = info
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.llm != nil {
		st.Ready, st.Name = true, a.llm.Name()
	} else if a.llmErr != nil {
		st.Error = a.llmErr.Error()
	}
	return st
}

// SetAPIKey stores (or, if empty, deletes) a provider's key in the OS
// keychain and reconnects A.R.I.A.
func (a *App) SetAPIKey(provider, key string) (AIStatus, error) {
	if !validProvider(provider) {
		return AIStatus{}, errors.New("fournisseur inconnu")
	}
	if err := config.SetAPIKey(provider, strings.TrimSpace(key)); err != nil {
		return AIStatus{}, err
	}
	a.mu.Lock()
	a.rebuildLLM()
	a.rebuildNeuralLocked()
	a.mu.Unlock()
	return a.GetAIStatus(), nil
}

func validProvider(p string) bool {
	for _, x := range config.Providers {
		if x == p {
			return true
		}
	}
	return false
}

// ListModels asks the provider which models this key can use.
func (a *App) ListModels(provider string) ([]string, error) {
	if !validProvider(provider) {
		return nil, errors.New("fournisseur inconnu")
	}
	a.mu.RLock()
	p, err := a.newProvider(provider)
	a.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	return p.ListModels(ctx)
}

// TestResult reports a connection test.
type TestResult struct {
	Millis int64  `json:"millis"` // until the first token
	Reply  string `json:"reply"`
	Model  string `json:"model"`
}

// TestAI sends a tiny text request with the current settings.
func (a *App) TestAI(provider string) (TestResult, error) {
	if !validProvider(provider) {
		return TestResult{}, errors.New("fournisseur inconnu")
	}
	a.mu.RLock()
	p, err := a.newProvider(provider)
	lang := a.cfg.AI.Language
	a.mu.RUnlock()
	if err != nil {
		return TestResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	q := "Test de connexion : réponds en une phrase très courte."
	if lang == "en" {
		q = "Connection test: answer in one very short sentence."
	}
	start := time.Now()
	ch, err := p.Stream(ctx, ai.SystemPrompt(ai.PromptOptions{Language: lang, Length: "short"}), []ai.Message{{Role: ai.RoleUser, Text: q}})
	if err != nil {
		return TestResult{}, err
	}
	res := TestResult{Model: p.Name()}
	var b strings.Builder
	for c := range ch {
		if c.Err != nil {
			return TestResult{}, c.Err
		}
		if res.Millis == 0 {
			res.Millis = time.Since(start).Milliseconds()
		}
		b.WriteString(c.Text)
	}
	res.Reply = strings.TrimSpace(b.String())
	return res, nil
}

// ListVoices returns the system voices, the selected one first.
func (a *App) ListVoices() ([]audio.Voice, error) {
	return audio.ListVoices()
}

// TestVoice says a sample sentence with the current voice settings (only
// if the voice is enabled).
func (a *App) TestVoice() {
	text := "Bonjour, je suis A.R.I.A. Voici ma voix."
	if a.current().AI.Language == "en" {
		text = "Hello, I am A.R.I.A. This is my voice."
	}
	// Spoken only: the sample must not land in the conversation.
	sentences := make(chan string, 1)
	sentences <- text
	close(sentences)
	go a.speakAll(a.ctx, sentences)
}

// ClearConversation forgets the context sent to the AI.
func (a *App) ClearConversation() {
	a.convMu.Lock()
	a.history = nil
	a.convMu.Unlock()
}

// AppInfo is shown in the "About" section.
type AppInfo struct {
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	SettingsPath string `json:"settingsPath"`
}

// Version is set at build time (-ldflags "-X spark/internal/app.Version=...").
var Version = "dev"

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{Version: Version, Platform: runtime.GOOS + "/" + runtime.GOARCH, SettingsPath: a.settings.Path()}
}

// KeyLabel resolves the letter of a physical key (DOM KeyboardEvent.code)
// for the shortcut recorder; "" if it is not a letter key.
func (a *App) KeyLabel(code string) string {
	k, _ := hotkey.LetterForCode(code)
	return string(k)
}

// MicStatus describes the voice input for the settings page.
type MicStatus struct {
	Mic    string `json:"mic"`    // "unknown" | "denied" | "granted"
	Speech string `json:"speech"` // speech recognition permission
	Engine string `json:"engine"` // who transcribes
	Live   bool   `json:"live"`   // live partial transcription
}

func (a *App) GetMicStatus() MicStatus {
	st := MicStatus{
		Mic:    audio.MicPermission().String(),
		Speech: audio.SpeechPermission().String(),
		Live:   a.voice.in.Live(),
	}
	if st.Live {
		st.Engine = "Reconnaissance vocale d’Apple, sur l’appareil quand c’est possible"
	} else {
		st.Engine = "Gemini (nécessite une clé Gemini)"
	}
	return st
}

// RequestMicPermission shows the OS permission dialogs and waits for the
// answer (or a timeout).
func (a *App) RequestMicPermission() MicStatus {
	done := make(chan struct{})
	a.requestPermissions(func(bool) { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
	}
	a.mu.RLock()
	mic, lang := a.cfg.Mic, a.cfg.AI.Language
	a.mu.RUnlock()
	a.voice.configure(mic, lang)
	return a.GetMicStatus()
}

// SetMicTest opens the mic for the settings level meter (EventVoiceLevel).
func (a *App) SetMicTest(on bool) error {
	return a.voice.setTesting(on)
}

// NeuralVoice is a Gemini prebuilt voice for the settings page.
type NeuralVoice struct {
	Name  string `json:"name"`
	Style string `json:"style"`
}

func (a *App) ListNeuralVoices() []NeuralVoice {
	out := make([]NeuralVoice, len(ai.NeuralVoices))
	for i, v := range ai.NeuralVoices {
		out[i] = NeuralVoice{Name: v.Name, Style: v.Style}
	}
	return out
}

// GetLaunchAtLogin reports whether Spark starts with the session.
func (a *App) GetLaunchAtLogin() bool { return startup.Enabled() }

// SetLaunchAtLogin registers Spark (or not) to start with the session.
func (a *App) SetLaunchAtLogin(on bool) (bool, error) {
	err := startup.Set(on)
	return startup.Enabled(), err
}
