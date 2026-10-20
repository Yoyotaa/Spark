package app

import (
	"math/rand/v2"
	"time"
)

var greetingLines = []string{
	"%s. A.R.I.A en ligne, je suis prête.",
	"%s. Systèmes opérationnels, je t'écoute.",
	"%s. A.R.I.A activée. Dis-moi ce dont tu as besoin.",
}

func salutation(t time.Time) string {
	switch h := t.Hour(); {
	case h >= 5 && h < 12:
		return "Bonjour"
	case h >= 12 && h < 18:
		return "Bon après-midi"
	default:
		return "Bonsoir"
	}
}

func greeting(t time.Time) (format, salut string) {
	return greetingLines[rand.IntN(len(greetingLines))], salutation(t)
}
