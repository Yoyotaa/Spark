package config

import (
	"os"
	"path/filepath"
	"testing"
)

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())    // macOS / Linux
	t.Setenv("XDG_CONFIG_HOME", "")  // Linux
	t.Setenv("AppData", t.TempDir()) // Windows
}

func TestStoreRoundTrip(t *testing.T) {
	isolate(t)
	st, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if st.Get().Orb != nil || st.Get().AI.Provider != ProviderGemini {
		t.Fatal("fresh store should hold the defaults")
	}
	err = st.Update(func(s *Settings) {
		s.Orb = &Point{X: 12, Y: 34}
		s.AI.Provider = ProviderClaude
		s.AI.Models[ProviderClaude] = "claude-haiku-5-5"
		s.Voice.Rate = 9 // out of range
	})
	if err != nil {
		t.Fatal(err)
	}

	again, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	s := again.Get()
	if s.Orb == nil || *s.Orb != (Point{12, 34}) || s.AI.Model(ProviderClaude) != "claude-haiku-5-5" {
		t.Fatalf("not persisted: %+v", s)
	}
	if s.Voice.Rate != 2 {
		t.Fatalf("rate not clamped: %v", s.Voice.Rate)
	}
}

func TestOlderFileKeepsDefaults(t *testing.T) {
	isolate(t)
	dir, _ := os.UserConfigDir()
	path := filepath.Join(dir, "Spark", "settings.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	// A file from the first versions only had the orb position.
	_ = os.WriteFile(path, []byte(`{"orb":{"x":1,"y":2},"mic":{"mode":"bogus"}}`), 0o644)

	st, err := OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	s := st.Get()
	d := Defaults()
	if s.Hotkeys != d.Hotkeys || s.Vision != d.Vision || s.Mic.Mode != d.Mic.Mode || s.Orb == nil {
		t.Fatalf("defaults lost: %+v", s)
	}
}

func TestGetReturnsACopy(t *testing.T) {
	isolate(t)
	st, _ := OpenStore()
	s := st.Get()
	s.AI.Models["gemini"] = "mutated"
	if st.Get().AI.Models["gemini"] == "mutated" {
		t.Fatal("Get must not expose internal maps")
	}
}

func TestShortcutValidation(t *testing.T) {
	s := Defaults()
	s.Hotkeys.ToggleChat = Shortcut{Key: "Escape", Alt: true}
	s.Mic.PushToTalk = Shortcut{Key: "F12"}
	n := s.Normalize()
	if n.Hotkeys.ToggleChat.Key != "" || n.Mic.PushToTalk.Key != "F12" {
		t.Fatalf("got %+v / %+v", n.Hotkeys.ToggleChat, n.Mic.PushToTalk)
	}
}
