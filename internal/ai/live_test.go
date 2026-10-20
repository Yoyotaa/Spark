package ai

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Real Gemini call, opt-in (uses the free quota):
//
//	SPARK_AI_LIVE=1 [SPARK_AI_LIVE_IMAGE=shot.jpg] go test -run Live -v ./internal/ai
func TestGeminiLive(t *testing.T) {
	key := os.Getenv("SPARK_LLM_API_KEY")
	if os.Getenv("SPARK_AI_LIVE") != "1" || key == "" {
		t.Skip("set SPARK_AI_LIVE=1 and SPARK_LLM_API_KEY")
	}
	g := NewGemini(key, os.Getenv("SPARK_LLM_MODEL"), "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	q := Message{Role: RoleUser, Text: "Test de connexion : réponds en une courte phrase."}
	if path := os.Getenv("SPARK_AI_LIVE_IMAGE"); path != "" {
		img, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		q = Message{Role: RoleUser, Text: "Que dois-je faire dans cette situation ?", Image: img}
	}

	start := time.Now()
	ch, err := g.Stream(ctx, Persona, []Message{q})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	var first time.Duration
	for c := range ch {
		if c.Err != nil {
			t.Fatal(c.Err)
		}
		if first == 0 {
			first = time.Since(start)
		}
		b.WriteString(c.Text)
	}
	t.Logf("%s | first token %v, total %v", g.Name(), first, time.Since(start))
	t.Logf("réponse : %q", b.String())
	if b.Len() == 0 {
		t.Fatal("empty answer")
	}
}
