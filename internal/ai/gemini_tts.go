package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"spark/internal/audio"
)

const (
	ttsModel      = "gemini-3.8-flash-tts"
	ttsSampleRate = 24000 // audio/l16 stream: 24 kHz, mono, 16-bit
)

// NeuralVoices are Gemini's prebuilt voices with their documented style
// (the API does not state genders).
var NeuralVoices = []struct{ Name, Style string }{
	{"Despina", "Douce"}, {"Achernar", "Feutrée"}, {"Vindemiatrix", "Délicate"}, {"Sulafat", "Chaleureuse"},
	{"Algieba", "Veloutée"}, {"Kore", "Ferme"}, {"Aoede", "Aérienne"}, {"Leda", "Jeune"},
	{"Zephyr", "Lumineuse"}, {"Autonoe", "Lumineuse"}, {"Erinome", "Claire"}, {"Iapetus", "Claire"},
	{"Callirrhoe", "Décontractée"}, {"Umbriel", "Décontractée"}, {"Enceladus", "Soufflée"}, {"Gacrux", "Mature"},
	{"Pulcherrima", "Directe"}, {"Achird", "Amicale"}, {"Schedar", "Posée"}, {"Charon", "Informative"},
	{"Rasalgethi", "Informative"}, {"Sadaltager", "Savante"}, {"Puck", "Enjouée"}, {"Laomedeia", "Enjouée"},
	{"Sadachbia", "Vive"}, {"Fenrir", "Enthousiaste"}, {"Orus", "Ferme"}, {"Alnilam", "Ferme"},
	{"Algenib", "Rocailleuse"}, {"Zubenelgenubi", "Familière"},
}

// GeminiVoice speaks with Gemini's neural text-to-speech, playing the audio
// stream as it arrives.
type GeminiVoice struct {
	g      *Gemini
	voice  string
	style  string
	player audio.Player
}

func NewGeminiVoice(apiKey, baseURL, voice, style string, player audio.Player) *GeminiVoice {
	return &GeminiVoice{g: NewGemini(apiKey, ttsModel, baseURL, ""), voice: voice, style: style, player: player}
}

type ttsRequest struct {
	Model          string         `json:"model"`
	Stream         bool           `json:"stream"`
	Input          []ttsInput     `json:"input"`
	ResponseFormat map[string]any `json:"response_format"`
	GenConfig      ttsGenConfig   `json:"generation_config"`
}

type ttsInput struct {
	Type    string       `json:"type"`
	Content []ttsContent `json:"content"`
}

type ttsContent struct {
	Type        string           `json:"type"`
	Text        string           `json:"text"`
	Annotations []map[string]any `json:"annotations,omitempty"`
}

type ttsGenConfig struct {
	SpeechConfig []map[string]string `json:"speech_config"`
}

// ErrNoAudio means the request failed before any sound was played, so the
// caller can fall back to another voice for the same text.
var ErrNoAudio = errors.New("aucun audio reçu")

// Speak blocks until the text is spoken; cancelling ctx stops the voice.
func (v *GeminiVoice) Speak(ctx context.Context, text string) error {
	content := ttsContent{Type: "text", Text: text}
	if v.style != "" {
		content.Annotations = []map[string]any{{"type": "speech_metadata", "style": v.style}}
	}
	body, err := json.Marshal(ttsRequest{
		Model:          v.g.model,
		Stream:         true,
		Input:          []ttsInput{{Type: "user_input", Content: []ttsContent{content}}},
		ResponseFormat: map[string]any{"type": "audio"},
		GenConfig:      ttsGenConfig{SpeechConfig: []map[string]string{{"voice": v.voice}}},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.g.baseURL+"/interactions?alt=sse", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", v.g.apiKey)
	resp, err := v.g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w : %v", ErrNoAudio, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w : %v", ErrNoAudio, friendlyError(resp.StatusCode, decodeError(resp)))
	}

	started := false
	defer func() {
		if started {
			v.player.Stop()
		}
	}()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 256<<10), 8<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		raw := bytes.TrimSpace(line[5:])
		if len(raw) == 0 || raw[0] != '{' { // "[DONE]"
			continue
		}
		var ev struct {
			Delta struct {
				MimeType string `json:"mime_type"`
				Data     string `json:"data"`
			} `json:"delta"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &ev) != nil {
			continue
		}
		if ev.Error != nil {
			if !started {
				return fmt.Errorf("%w : %s", ErrNoAudio, ev.Error.Message)
			}
			return errors.New(ev.Error.Message)
		}
		if ev.Delta.Data == "" || !strings.HasPrefix(ev.Delta.MimeType, "audio/") {
			continue
		}
		pcm, err := base64.StdEncoding.DecodeString(ev.Delta.Data)
		if err != nil {
			continue
		}
		if !started {
			if err := v.player.Start(ttsSampleRate); err != nil {
				return fmt.Errorf("%w : %v", ErrNoAudio, err)
			}
			started = true
		}
		v.player.Write(pcm)
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		if !started {
			return fmt.Errorf("%w : %v", ErrNoAudio, err)
		}
		return err
	}
	if !started {
		return ErrNoAudio
	}
	return v.player.Drain(ctx)
}
