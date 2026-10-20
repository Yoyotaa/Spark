package app

import (
	"math/rand/v2"
	"time"
)

var greetingLines = map[string][]string{
	"fr": {
		"%s. A.R.I.A en ligne, je suis prête.",
		"%s. Systèmes opérationnels, je t'écoute.",
		"%s. A.R.I.A activée. Dis-moi ce dont tu as besoin.",
	},
	"en": {
		"%s. A.R.I.A online, ready when you are.",
		"%s. All systems go, I'm listening.",
		"%s. A.R.I.A activated. Tell me what you need.",
	},
}

func salutation(t time.Time, lang string) string {
	h := t.Hour()
	if lang == "en" {
		switch {
		case h >= 5 && h < 12:
			return "Good morning"
		case h >= 12 && h < 18:
			return "Good afternoon"
		default:
			return "Good evening"
		}
	}
	switch {
	case h >= 5 && h < 12:
		return "Bonjour"
	case h >= 12 && h < 18:
		return "Bon après-midi"
	default:
		return "Bonsoir"
	}
}

func greeting(t time.Time, lang string) (format, salut string) {
	lines, ok := greetingLines[lang]
	if !ok {
		lang, lines = "fr", greetingLines["fr"]
	}
	return lines[rand.IntN(len(lines))], salutation(t, lang)
}
