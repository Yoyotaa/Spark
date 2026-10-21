package config

import (
	"slices"
	"strings"
)

// Settings are the user preferences persisted between launches. Every field
// is editable from the settings page; API keys live in the OS keychain (see
// secrets.go), never here.
type Settings struct {
	Orb        *Point             `json:"orb,omitempty"` // A.R.I.A button position
	AI         AISettings         `json:"ai"`
	Vision     VisionSettings     `json:"vision"`
	Voice      VoiceSettings      `json:"voice"`
	Mic        MicSettings        `json:"mic"`
	Proactive  ProactiveSettings  `json:"proactive"`
	Hotkeys    HotkeySettings     `json:"hotkeys"`
	Appearance AppearanceSettings `json:"appearance"`
	Advanced   AdvancedSettings   `json:"advanced"`
}

// Providers supported by Spark.
const (
	ProviderGemini = "gemini"
	ProviderClaude = "claude"
	ProviderOpenAI = "openai" // OpenAI or any compatible API (OpenRouter, Ollama...)
)

var Providers = []string{ProviderGemini, ProviderClaude, ProviderOpenAI}

type AISettings struct {
	Provider string            `json:"provider"`
	Models   map[string]string `json:"models"`   // per provider
	BaseURLs map[string]string `json:"baseUrls"` // per provider, "" = official endpoint
	// Thinking trades latency for depth: "low" | "medium" | "high".
	Thinking string `json:"thinking"`
	// Length of the answers: "short" | "normal" | "detailed".
	Length   string `json:"length"`
	Language string `json:"language"` // "fr" | "en"
	// Instructions appended to A.R.I.A's persona.
	CustomInstructions string `json:"customInstructions"`
	// HistoryTurns is how many past exchanges are sent as context.
	HistoryTurns int `json:"historyTurns"`
	// Fallback re-runs a request declined by a safety filter on another
	// model (Claude only, server-side).
	Fallback bool `json:"fallback"`
}

// ProactiveSettings: A.R.I.A watches the screen on her own and speaks only
// when something matters.
type ProactiveSettings struct {
	Enabled  bool `json:"enabled"`
	Interval int  `json:"interval"` // seconds between two looks
}

type VisionSettings struct {
	Enabled    bool `json:"enabled"` // screenshot attached to each question by default
	MaxSide    int  `json:"maxSide"` // px
	Quality    int  `json:"quality"` // JPEG 1-100
	ShowCursor bool `json:"showCursor"`
}

// Voice engines.
const (
	VoiceSystem = "system" // OS voices: instant, robotic
	VoiceGemini = "gemini" // neural, ~2.5 s more latency, free-tier quota
)

type VoiceSettings struct {
	Enabled bool   `json:"enabled"`
	Engine  string `json:"engine"`
	// Neural voice (Gemini): prebuilt voice name and delivery style.
	NeuralVoice string  `json:"neuralVoice"`
	Style       string  `json:"style"`
	Voice       string  `json:"voice"` // system voice, "" = best available
	Rate        float64 `json:"rate"`  // 0.5-2.0
	ReadAnswers bool    `json:"readAnswers"`
	Greeting    bool    `json:"greeting"`
}

// Microphone modes.
const (
	MicOff        = "off"
	MicPushToTalk = "ptt"
	MicContinuous = "continuous"
)

type MicSettings struct {
	Mode        string   `json:"mode"`
	PushToTalk  Shortcut `json:"pushToTalk"`
	Sensitivity int      `json:"sensitivity"` // 0-100, continuous mode
	// WakeWord makes the continuous mode only react to sentences that start
	// with WakePhrase ("Aria, que dois-je faire ?").
	WakeWord   bool   `json:"wakeWord"`
	WakePhrase string `json:"wakePhrase"`
	Cues       bool   `json:"cues"` // short sounds when listening starts/stops
}

// Shortcut is a key plus modifiers. Key is "A"-"Z", "0"-"9", "F1"-"F12" or
// "Space"; an empty Key disables the shortcut.
type Shortcut struct {
	Key   string `json:"key"`
	Alt   bool   `json:"alt"`
	Shift bool   `json:"shift"`
	Ctrl  bool   `json:"ctrl"`
	Cmd   bool   `json:"cmd"`
}

type HotkeySettings struct {
	ToggleChat    Shortcut `json:"toggleChat"`
	ToggleVisible Shortcut `json:"toggleVisible"`
	OpenSettings  Shortcut `json:"openSettings"`
}

type AppearanceSettings struct {
	Accent       string `json:"accent"`       // cyan | violet | emerald | amber | rose
	OrbSize      string `json:"orbSize"`      // small | medium | large
	PanelOpacity int    `json:"panelOpacity"` // 50-100 %
	ReduceMotion bool   `json:"reduceMotion"`
	ShowBubbles  bool   `json:"showBubbles"`
}

type AdvancedSettings struct {
	Debug  bool `json:"debug"`
	PollHz int  `json:"pollHz"` // cursor polling for click-through
}

// DefaultModels are the models selected when the user has not picked one.
// OpenAI-compatible endpoints vary too much: the model is chosen from the
// list fetched from the endpoint.
var DefaultModels = map[string]string{
	ProviderGemini: "gemini-3.6-flash",
	ProviderClaude: "claude-opus-5-5",
	ProviderOpenAI: "",
}

func Defaults() Settings {
	return Settings{
		AI: AISettings{
			Provider:     ProviderGemini,
			Models:       map[string]string{},
			BaseURLs:     map[string]string{},
			Thinking:     "low",
			Length:       "normal",
			Language:     "fr",
			HistoryTurns: 6,
			Fallback:     true,
		},
		Vision:    VisionSettings{Enabled: true, MaxSide: 1568, Quality: 72, ShowCursor: true},
		Proactive: ProactiveSettings{Enabled: false, Interval: 60},
		Voice: VoiceSettings{
			Enabled: true, Engine: VoiceSystem, NeuralVoice: "Despina",
			Style: "calme, posée et assurée, comme une assistante d'élite",
			Rate:  1, ReadAnswers: true, Greeting: true,
		},
		Mic: MicSettings{
			Mode:        MicPushToTalk,
			PushToTalk:  Shortcut{Key: "F8"},
			Sensitivity: 50,
			WakeWord:    true,
			WakePhrase:  "Aria",
			Cues:        true,
		},
		Hotkeys: HotkeySettings{
			ToggleChat:    Shortcut{Key: "A", Alt: true, Shift: true},
			ToggleVisible: Shortcut{Key: "H", Alt: true, Shift: true},
			OpenSettings:  Shortcut{Key: "P", Alt: true, Shift: true},
		},
		Appearance: AppearanceSettings{Accent: "cyan", OrbSize: "medium", PanelOpacity: 82, ShowBubbles: true},
		Advanced:   AdvancedSettings{PollHz: 60},
	}
}

// Model returns the model to use for a provider.
func (a AISettings) Model(provider string) string {
	if m := a.Models[provider]; m != "" {
		return m
	}
	return DefaultModels[provider]
}

func oneOf(v string, allowed []string, fallback string) string {
	if slices.Contains(allowed, v) {
		return v
	}
	return fallback
}

func clamp[T int | float64](v, lo, hi T) T {
	return min(max(v, lo), hi)
}

var validKeys = func() []string {
	keys := []string{"Space"}
	for c := 'A'; c <= 'Z'; c++ {
		keys = append(keys, string(c))
	}
	for c := '0'; c <= '9'; c++ {
		keys = append(keys, string(c))
	}
	for i := 1; i <= 12; i++ {
		keys = append(keys, "F"+itoa(i))
	}
	return keys
}()

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return "1" + string(rune('0'+i-10))
}

// ValidKey reports whether k can be bound to a shortcut.
func ValidKey(k string) bool { return slices.Contains(validKeys, k) }

func (s Shortcut) normalized() Shortcut {
	if !ValidKey(s.Key) {
		return Shortcut{}
	}
	return s
}

// Normalize clamps every value into its valid range and replaces unknown
// enum values with defaults, so a hand-edited or older file never breaks
// the app.
func (s Settings) Normalize() Settings {
	d := Defaults()

	s.AI.Provider = oneOf(s.AI.Provider, Providers, d.AI.Provider)
	if s.AI.Models == nil {
		s.AI.Models = map[string]string{}
	}
	if s.AI.BaseURLs == nil {
		s.AI.BaseURLs = map[string]string{}
	}
	s.AI.Thinking = oneOf(s.AI.Thinking, []string{"low", "medium", "high"}, d.AI.Thinking)
	s.AI.Length = oneOf(s.AI.Length, []string{"short", "normal", "detailed"}, d.AI.Length)
	s.AI.Language = oneOf(s.AI.Language, []string{"fr", "en"}, d.AI.Language)
	s.AI.HistoryTurns = clamp(s.AI.HistoryTurns, 0, 20)
	if len(s.AI.CustomInstructions) > 2000 {
		s.AI.CustomInstructions = s.AI.CustomInstructions[:2000]
	}

	s.Proactive.Interval = clamp(s.Proactive.Interval, 15, 600)
	s.Vision.MaxSide = clamp(s.Vision.MaxSide, 512, 2560)
	s.Vision.Quality = clamp(s.Vision.Quality, 30, 95)

	s.Voice.Rate = clamp(s.Voice.Rate, 0.5, 2)
	s.Voice.Engine = oneOf(s.Voice.Engine, []string{VoiceSystem, VoiceGemini}, d.Voice.Engine)
	if strings.TrimSpace(s.Voice.NeuralVoice) == "" {
		s.Voice.NeuralVoice = d.Voice.NeuralVoice
	}
	if len(s.Voice.Style) > 300 {
		s.Voice.Style = s.Voice.Style[:300]
	}

	s.Mic.Mode = oneOf(s.Mic.Mode, []string{MicOff, MicPushToTalk, MicContinuous}, d.Mic.Mode)
	s.Mic.PushToTalk = s.Mic.PushToTalk.normalized()
	s.Mic.Sensitivity = clamp(s.Mic.Sensitivity, 0, 100)
	s.Mic.WakePhrase = strings.TrimSpace(s.Mic.WakePhrase)
	if s.Mic.WakePhrase == "" || len(s.Mic.WakePhrase) > 40 {
		s.Mic.WakePhrase = d.Mic.WakePhrase
	}

	s.Hotkeys.ToggleChat = s.Hotkeys.ToggleChat.normalized()
	s.Hotkeys.ToggleVisible = s.Hotkeys.ToggleVisible.normalized()
	s.Hotkeys.OpenSettings = s.Hotkeys.OpenSettings.normalized()

	s.Appearance.Accent = oneOf(s.Appearance.Accent, []string{"cyan", "violet", "emerald", "amber", "rose"}, d.Appearance.Accent)
	s.Appearance.OrbSize = oneOf(s.Appearance.OrbSize, []string{"small", "medium", "large"}, d.Appearance.OrbSize)
	s.Appearance.PanelOpacity = clamp(s.Appearance.PanelOpacity, 50, 100)

	s.Advanced.PollHz = oneOfInt(s.Advanced.PollHz, []int{30, 60, 120}, d.Advanced.PollHz)
	return s
}

func oneOfInt(v int, allowed []int, fallback int) int {
	if slices.Contains(allowed, v) {
		return v
	}
	return fallback
}
