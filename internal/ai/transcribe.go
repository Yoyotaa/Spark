package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Fastest model with audio understanding (no thinking): transcription must
// not add seconds before the actual question is even asked.
const transcribeModel = "gemini-3.5-flash-lite"

// GeminiTranscriber transcribes recorded utterances with Gemini. Used where
// no native speech recognition is wired (Windows).
type GeminiTranscriber struct {
	g *Gemini
}

func NewGeminiTranscriber(apiKey, baseURL string) *GeminiTranscriber {
	return &GeminiTranscriber{g: NewGemini(apiKey, transcribeModel, baseURL, "")}
}

func (t *GeminiTranscriber) Transcribe(ctx context.Context, wav []byte, lang string) (string, error) {
	prompt := "Transcris exactement ce que dit la personne, en français, sans rien ajouter. Si rien n'est dit, réponds par une chaîne vide."
	if lang == "en" {
		prompt = "Transcribe exactly what the person says, in English, adding nothing. If nothing is said, answer with an empty string."
	}
	req := geminiRequest{Contents: []geminiContent{{
		Role: "user",
		Parts: []geminiPart{
			{InlineData: &geminiInlineData{MimeType: "audio/wav", Data: wav}},
			{Text: prompt},
		},
	}}}
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/models/%s:generateContent", t.g.baseURL, t.g.model)
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("x-goog-api-key", t.g.apiKey)
	resp, err := t.g.client.Do(hr)
	if err != nil {
		return "", fmt.Errorf("transcription impossible : %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", friendlyError(resp.StatusCode, decodeError(resp))
	}
	defer resp.Body.Close()
	var r geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range r.Candidates {
		for _, p := range c.Content.Parts {
			if !p.Thought {
				b.WriteString(p.Text)
			}
		}
	}
	return strings.Trim(strings.TrimSpace(b.String()), `"«» `), nil
}
