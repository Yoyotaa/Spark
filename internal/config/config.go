// Package config loads Spark settings (API keys, hotkeys, voice...).
// Secrets come from the environment, never from the repository.
package config

import "os"

type Config struct {
	LLMAPIKey string
	Debug     bool
}

func Load() Config {
	return Config{
		LLMAPIKey: os.Getenv("SPARK_LLM_API_KEY"),
		Debug:     os.Getenv("SPARK_DEBUG") == "1",
	}
}
