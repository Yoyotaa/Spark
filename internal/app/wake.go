package app

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// fold lowercases and strips accents: "Ariá" -> "aria".
func fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// distance is the Levenshtein distance between two short strings.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// stripWakePhrase looks for the wake phrase among the first words of the
// transcript (speech engines write "Aria", "Arya", "Area"...) and returns the
// request that follows it. ok is false when the phrase is absent.
func stripWakePhrase(transcript, phrase string) (request string, ok bool) {
	target := words(fold(phrase))
	if len(target) == 0 {
		return transcript, true
	}
	joined := strings.Join(target, "")
	// One typo allowed per 4 letters, at least one.
	tolerance := max(1, len([]rune(joined))/4)

	// Word positions in the original text, to cut it without re-folding.
	type span struct{ start, end int }
	var spans []span
	inWord := false
	for i, r := range transcript {
		letter := unicode.IsLetter(r) || unicode.IsDigit(r)
		switch {
		case letter && !inWord:
			spans = append(spans, span{start: i})
			inWord = true
		case !letter && inWord:
			spans[len(spans)-1].end = i
			inWord = false
		}
	}
	if inWord {
		spans[len(spans)-1].end = len(transcript)
	}

	const lookahead = 3 // "Hé Aria", "OK Aria, ..."
	for i := 0; i < len(spans) && i < lookahead; i++ {
		// Compare the phrase with 1..len(target)+1 consecutive words, so
		// "A ria" or "Aria" both match "Aria".
		for n := 1; n <= len(target)+1 && i+n <= len(spans); n++ {
			var cand strings.Builder
			for _, sp := range spans[i : i+n] {
				cand.WriteString(fold(transcript[sp.start:sp.end]))
			}
			if distance(cand.String(), joined) <= tolerance {
				rest := strings.TrimLeft(transcript[spans[i+n-1].end:], " ,.;:!?-—")
				return strings.TrimSpace(rest), true
			}
		}
	}
	return "", false
}
