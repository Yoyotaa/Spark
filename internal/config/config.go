// Package config loads Spark settings (API keys, hotkeys, voice...).
// Secrets come from the environment, never from the repository.
package config

import "os"

type Config struct {
	LLMAPIKey string // SPARK_LLM_API_KEY, or GEMINI_API_KEY
	LLMModel  string // SPARK_LLM_MODEL, provider default if empty
	LLMURL    string // SPARK_LLM_BASE_URL: proxy or local fake server
	Debug     bool
	Mute      bool   // SPARK_MUTE=1: no voice
	Voice     string // SPARK_VOICE: force a system voice by name
}

func Load() Config {
	return Config{
		LLMAPIKey: firstNonEmpty(os.Getenv("SPARK_LLM_API_KEY"), os.Getenv("GEMINI_API_KEY")),
		LLMModel:  os.Getenv("SPARK_LLM_MODEL"),
		LLMURL:    os.Getenv("SPARK_LLM_BASE_URL"),
		Debug:     os.Getenv("SPARK_DEBUG") == "1",
		Mute:      os.Getenv("SPARK_MUTE") == "1",
		Voice:     os.Getenv("SPARK_VOICE"),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
