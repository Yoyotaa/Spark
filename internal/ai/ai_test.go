package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestSentences(t *testing.T) {
	var s Sentences
	var got []string
	for _, chunk := range []string{"Recule sous ta to", "ur. A.R.I.A voit 3.5 ", "ennemis !", "! Attends", " le jungler\nPuis push."} {
		got = append(got, s.Push(chunk)...)
	}
	got = append(got, s.Flush())
	want := []string{"Recule sous ta tour.", "A.R.I.A voit 3.5 ennemis !!", "Attends le jungler", "Puis push."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func sse(chunks ...string) string {
	var b strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&b, "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":%q}]}}]}\r\n\r\n", c)
	}
	return b.String()
}

func collect(t *testing.T, ch <-chan Chunk) (string, error) {
	t.Helper()
	var b strings.Builder
	for c := range ch {
		if c.Err != nil {
			return b.String(), c.Err
		}
		b.WriteString(c.Text)
	}
	return b.String(), nil
}

func TestGeminiStream(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "k" || r.URL.Query().Get("key") != "" {
			t.Errorf("API key must travel in the header only")
		}
		if !strings.HasSuffix(r.URL.Path, "/models/m:streamGenerateContent") || r.URL.Query().Get("alt") != "sse" {
			t.Errorf("unexpected URL %s", r.URL)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sse("Pose une ", "ward."))
	}))
	defer srv.Close()

	g := NewGemini("k", "m", srv.URL)
	ch, err := g.Stream(context.Background(), "sys", []Message{{Role: RoleUser, Text: "Que faire ?", Image: []byte{0xff, 0xd8}}})
	if err != nil {
		t.Fatal(err)
	}
	text, err := collect(t, ch)
	if err != nil || text != "Pose une ward." {
		t.Fatalf("got %q, %v", text, err)
	}

	parts := body["contents"].([]any)[0].(map[string]any)["parts"].([]any)
	img := parts[0].(map[string]any)["inline_data"].(map[string]any)
	if img["mime_type"] != "image/jpeg" || img["data"] != base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8}) {
		t.Fatalf("bad image part: %v", img)
	}
	thinking := body["generation_config"].(map[string]any)["thinking_config"].(map[string]any)
	if thinking["thinking_level"] != "low" {
		t.Fatalf("thinking level not sent: %v", thinking)
	}
}

func TestGeminiRetries(t *testing.T) {
	for name, fail := range map[string]func(w http.ResponseWriter, body map[string]any) bool{
		// 503 "high demand" once, then OK.
		"503": func(w http.ResponseWriter, _ map[string]any) bool {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":{"message":"high demand"}}`)
			return true
		},
		// A model without thinking control: retried without the field.
		"thinking": func(w http.ResponseWriter, body map[string]any) bool {
			if _, has := body["generation_config"]; !has {
				return false
			}
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"thinking_level is not supported for this model"}}`)
			return true
		},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if calls == 1 && fail(w, body) {
					return
				}
				fmt.Fprint(w, sse("ok"))
			}))
			defer srv.Close()

			g := NewGemini("k", "m", srv.URL)
			ch, err := g.Stream(context.Background(), "", []Message{{Role: RoleUser, Text: "x"}})
			if err != nil {
				t.Fatal(err)
			}
			if text, err := collect(t, ch); err != nil || text != "ok" || calls != 2 {
				t.Fatalf("got %q, %v after %d calls", text, err, calls)
			}
		})
	}
}

func TestGeminiFriendlyErrors(t *testing.T) {
	for status, want := range map[int]string{429: "quota", 403: "clé API", 500: "Gemini 500", 503: "saturé"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"error":{"message":"boom"}}`)
		}))
		g := NewGemini("k", "m", srv.URL)
		_, err := g.Stream(context.Background(), "", []Message{{Role: RoleUser, Text: "x"}})
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("status %d: got %v, want %q", status, err, want)
		}
	}
}
