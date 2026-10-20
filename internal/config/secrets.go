package config

import (
	"errors"
	"os"

	"github.com/zalando/go-keyring"
)

// API keys live in the OS credential store: macOS Keychain, Windows
// Credential Manager (Secret Service on Linux). Environment variables remain
// a fallback for development.

const keyringService = "Spark"

var envKeys = map[string][]string{
	ProviderGemini: {"SPARK_GEMINI_API_KEY", "GEMINI_API_KEY", "SPARK_LLM_API_KEY"},
	ProviderClaude: {"SPARK_CLAUDE_API_KEY", "ANTHROPIC_API_KEY"},
	ProviderOpenAI: {"SPARK_OPENAI_API_KEY", "OPENAI_API_KEY"},
}

// KeySource tells where a provider's key comes from.
type KeySource string

const (
	KeyNone     KeySource = ""
	KeyKeychain KeySource = "keychain"
	KeyEnv      KeySource = "env"
)

// APIKey returns the key for a provider: keychain first, then environment.
func APIKey(provider string) (string, KeySource) {
	if k, err := keyring.Get(keyringService, provider); err == nil && k != "" {
		return k, KeyKeychain
	}
	for _, name := range envKeys[provider] {
		if k := os.Getenv(name); k != "" {
			return k, KeyEnv
		}
	}
	return "", KeyNone
}

// SetAPIKey stores the key in the keychain; an empty key deletes it.
func SetAPIKey(provider, key string) error {
	if key == "" {
		err := keyring.Delete(keyringService, provider)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return err
	}
	return keyring.Set(keyringService, provider, key)
}

// EnvKeyNames lists the environment variables read for a provider.
func EnvKeyNames(provider string) []string { return envKeys[provider] }
