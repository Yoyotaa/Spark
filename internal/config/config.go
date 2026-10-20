// Package config holds Spark's settings: user preferences (settings.go,
// persisted by store.go), API keys (secrets.go) and environment overrides.
package config

import "os"

// Env are process-level overrides, mostly for development.
type Env struct {
	Debug   bool   // SPARK_DEBUG=1
	Mute    bool   // SPARK_MUTE=1: no voice regardless of settings
	Model   string // SPARK_LLM_MODEL: overrides the selected model
	BaseURL string // SPARK_LLM_BASE_URL: proxy or local fake server
}

func LoadEnv() Env {
	return Env{
		Debug:   os.Getenv("SPARK_DEBUG") == "1",
		Mute:    os.Getenv("SPARK_MUTE") == "1",
		Model:   os.Getenv("SPARK_LLM_MODEL"),
		BaseURL: os.Getenv("SPARK_LLM_BASE_URL"),
	}
}
