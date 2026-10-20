package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func claudeSSE(texts ...string) string {
	var b strings.Builder
	ev := func(name, data string) { fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", name, data) }
	ev("message_start", `{"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}`)
	ev("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	for _, t := range texts {
		d, _ := json.Marshal(t)
		ev("content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%s}}`, d))
	}
	ev("content_block_stop", `{"type":"content_block_stop","index":0}`)
	ev("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}`)
	ev("message_stop", `{"type":"message_stop"}`)
	return b.String()
}

func TestClaudeStream(t *testing.T) {
	var body map[string]any
	var beta, key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		beta, key = r.Header.Get("anthropic-beta"), r.Header.Get("x-api-key")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, claudeSSE("Recule ", "sous ta tour."))
	}))
	defer srv.Close()

	// The real API has no fallback on proxies: force it to check the shape.
	c := NewClaude(Config{APIKey: "sk", Model: "claude-opus-5-5", BaseURL: srv.URL, Thinking: "low"})
	c.fallback = true
	ch, err := c.Stream(context.Background(), "sys", []Message{
		{Role: RoleUser, Text: "avant"}, {Role: RoleModel, Text: "ok"},
		{Role: RoleUser, Text: "Que faire ?", Image: []byte{0xff, 0xd8}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if text, err := collect(t, ch); err != nil || text != "Recule sous ta tour." {
		t.Fatalf("got %q, %v", text, err)
	}

	if key != "sk" || !strings.Contains(beta, fallbackBeta) || body["fallbacks"] != "default" {
		t.Fatalf("auth/fallback not sent: key=%q beta=%q fallbacks=%v", key, beta, body["fallbacks"])
	}
	if body["output_config"].(map[string]any)["effort"] != "low" {
		t.Fatalf("effort not sent: %v", body["output_config"])
	}
	msgs := body["messages"].([]any)
	if len(msgs) != 3 || msgs[1].(map[string]any)["role"] != "assistant" {
		t.Fatalf("bad roles: %v", msgs)
	}
	img := msgs[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	src := img["source"].(map[string]any)
	if img["type"] != "image" || src["media_type"] != "image/jpeg" || src["data"] != "/9g=" {
		t.Fatalf("bad image block: %v", img)
	}
}

func TestClaudeErrors(t *testing.T) {
	for status, want := range map[int]string{401: "clé API Claude", 529: "surchargé", 404: "introuvable"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			fmt.Fprint(w, `{"type":"error","error":{"type":"x","message":"boom"}}`)
		}))
		c := NewClaude(Config{APIKey: "sk", BaseURL: srv.URL})
		_, err := c.Stream(context.Background(), "", []Message{{Role: RoleUser, Text: "x"}})
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("status %d: got %v, want %q", status, err, want)
		}
	}
}

func TestClaudeFallbackOnlyWhereSupported(t *testing.T) {
	for model, want := range map[string]bool{
		"claude-opus-5-5": true, "claude-sonnet-5-5": true, "claude-fable-5-1": true,
		"claude-haiku-5-5": false, "claude-sonnet-5": false,
	} {
		if got := NewClaude(Config{APIKey: "k", Model: model, Fallback: true}).fallback; got != want {
			t.Errorf("%s: fallback=%v, want %v", model, got, want)
		}
	}
	if NewClaude(Config{APIKey: "k", Fallback: true, BaseURL: "http://proxy"}).fallback {
		t.Error("no server-side fallback through a proxy")
	}
}

func TestOpenAIStream(t *testing.T) {
	calls := 0
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer sk" {
			t.Errorf("bad request %s %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		body = map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, has := body["reasoning_effort"]; has {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"Unrecognized request argument supplied: reasoning_effort"}}`)
			return
		}
		for _, c := range []string{"Pose ", "une ward."} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	o := NewOpenAI(Config{APIKey: "sk", Model: "m", BaseURL: srv.URL, Thinking: "low"})
	ch, err := o.Stream(context.Background(), "sys", []Message{{Role: RoleUser, Text: "Que faire ?", Image: []byte{1}}})
	if err != nil {
		t.Fatal(err)
	}
	if text, err := collect(t, ch); err != nil || text != "Pose une ward." || calls != 2 {
		t.Fatalf("got %q, %v after %d calls", text, err, calls)
	}
	user := body["messages"].([]any)[1].(map[string]any)["content"].([]any)
	if url := user[0].(map[string]any)["image_url"].(map[string]any)["url"]; url != "data:image/jpeg;base64,AQ==" {
		t.Fatalf("bad image: %v", url)
	}
}

func TestListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/models": // Gemini & OpenAI share the path in this fake
			if r.Header.Get("x-goog-api-key") != "" {
				fmt.Fprint(w, `{"models":[{"name":"models/gemini-x","supportedGenerationMethods":["generateContent"]},{"name":"models/embed","supportedGenerationMethods":["embedContent"]}]}`)
				return
			}
			fmt.Fprint(w, `{"data":[{"id":"b"},{"id":"a"}]}`)
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"claude-opus-5-5","type":"model","display_name":"Opus","created_at":"2026-01-01T00:00:00Z"}],"has_more":false,"first_id":"claude-opus-5-5","last_id":"claude-opus-5-5"}`)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	for name, p := range map[string]Provider{
		"gemini": NewGemini("k", "", srv.URL, ""),
		"openai": NewOpenAI(Config{APIKey: "k", BaseURL: srv.URL}),
		"claude": NewClaude(Config{APIKey: "k", BaseURL: srv.URL}),
	} {
		ids, err := p.ListModels(ctx)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := map[string]string{"gemini": "[gemini-x]", "openai": "[a b]", "claude": "[claude-opus-5-5]"}[name]
		if fmt.Sprint(ids) != want {
			t.Errorf("%s: got %v, want %s", name, ids, want)
		}
	}
}

func TestSystemPrompt(t *testing.T) {
	fr := SystemPrompt(PromptOptions{Language: "fr", Length: "short", Custom: "Parle comme un coach."})
	en := SystemPrompt(PromptOptions{Language: "en"})
	if !strings.Contains(fr, "tutoie") || !strings.Contains(fr, "Une ou deux phrases") || !strings.Contains(fr, "coach") {
		t.Fatalf("fr prompt: %s", fr)
	}
	if !strings.Contains(en, "Answer in English") || !strings.Contains(en, "One to three") {
		t.Fatalf("en prompt: %s", en)
	}
}

func TestGeminiTranscriber(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models/"+transcribeModel+":generateContent") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"« Que dois-je faire ? »"}]}}]}`)
	}))
	defer srv.Close()

	text, err := NewGeminiTranscriber("k", srv.URL).Transcribe(context.Background(), []byte("RIFF"), "fr")
	if err != nil || text != "Que dois-je faire ?" {
		t.Fatalf("got %q, %v", text, err)
	}
	part := body["contents"].([]any)[0].(map[string]any)["parts"].([]any)[0].(map[string]any)
	if part["inline_data"].(map[string]any)["mime_type"] != "audio/wav" {
		t.Fatalf("audio not sent: %v", part)
	}
}
