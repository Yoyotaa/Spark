package ai

import "strings"

// Sentences cuts a token stream into sentences so the TTS can start on the
// first one while the rest is still being generated.
type Sentences struct {
	buf strings.Builder
}

// Push adds streamed text and returns the sentences it completed.
func (s *Sentences) Push(text string) []string {
	s.buf.WriteString(text)
	pending := s.buf.String()

	var out []string
	start := 0
	for i := 0; i < len(pending); i++ {
		switch pending[i] {
		case '.', '!', '?', '\n':
		default:
			continue
		}
		// A terminator ends a sentence only if followed by whitespace:
		// "3.5" or "A.R.I.A" must not be cut. Wait for more text otherwise.
		end := i + 1
		if pending[i] != '\n' {
			for end < len(pending) && strings.ContainsRune(".!?", rune(pending[end])) {
				end++
			}
			if end >= len(pending) || !isSpace(pending[end]) {
				i = end - 1
				continue
			}
		}
		if sentence := strings.TrimSpace(pending[start:end]); sentence != "" {
			out = append(out, sentence)
		}
		start = end
		i = end - 1
	}
	if start > 0 {
		rest := pending[start:]
		s.buf.Reset()
		s.buf.WriteString(rest)
	}
	return out
}

// Flush returns whatever is left once the stream ended.
func (s *Sentences) Flush() string {
	rest := strings.TrimSpace(s.buf.String())
	s.buf.Reset()
	return rest
}

func isSpace(b byte) bool { return b == ' ' || b == '\n' || b == '\t' || b == '\r' }
