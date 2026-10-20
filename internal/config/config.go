// Package config loads Spark settings (API keys, hotkeys, voice...).
// Secrets come from the environment, never from the repository.
package config

import "os"

type Config struct {
	LLMAPIKey string
	Debug     bool
	Mute      bool   // SPARK_MUTE=1: no voice
	Voice     string // SPARK_VOICE: force a system voice by name
}

func Load() Config {
	return Config{
		LLMAPIKey: os.Getenv("SPARK_LLM_API_KEY"),
		Debug:     os.Getenv("SPARK_DEBUG") == "1",
		Mute:      os.Getenv("SPARK_MUTE") == "1",
		Voice:     os.Getenv("SPARK_VOICE"),
	}
}
