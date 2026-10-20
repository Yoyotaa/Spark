package app

import "testing"

func TestStripWakePhrase(t *testing.T) {
	cases := []struct {
		in, phrase, want string
		ok               bool
	}{
		{"Aria, que dois-je faire ?", "Aria", "que dois-je faire ?", true},
		{"Arya que dois-je faire", "Aria", "que dois-je faire", true},
		{"Hé Area, regarde la minimap.", "Aria", "regarde la minimap.", true},
		{"Ok Ariá", "Aria", "", true},
		{"tu viens ce soir ?", "Aria", "", false},
		{"Il faut que Aria regarde", "Aria", "", false}, // not in the first words
		{"Jarvis, analyse", "Jarvis", "analyse", true},
		{"Hey Spark Assistant décris l'écran", "Spark Assistant", "décris l'écran", true},
	}
	for _, c := range cases {
		got, ok := stripWakePhrase(c.in, c.phrase)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("stripWakePhrase(%q, %q) = %q, %v; want %q, %v", c.in, c.phrase, got, ok, c.want, c.ok)
		}
	}
}
