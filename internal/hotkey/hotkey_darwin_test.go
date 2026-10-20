//go:build darwin

package hotkey

import "testing"

func TestKeyCodeResolvesEveryKey(t *testing.T) {
	seen := map[uint32]Key{}
	var keys []Key
	for c := 'A'; c <= 'Z'; c++ {
		keys = append(keys, Key(string(c)))
	}
	for c := '0'; c <= '9'; c++ {
		keys = append(keys, Key(string(c)))
	}
	keys = append(keys, "F1", "F5", "F12", KeySpace)
	for _, k := range keys {
		code, ok := keyCode(k)
		if !ok {
			t.Fatalf("no key code for %q", k)
		}
		if other, dup := seen[code]; dup {
			t.Fatalf("%q and %q share key code %#x", k, other, code)
		}
		seen[code] = k
	}
	a, _ := keyCode("A")
	t.Logf("layout: 'A' is key code %#x (QWERTY 0x00, AZERTY 0x0C)", a)
}

func TestFunctionKey(t *testing.T) {
	for k, want := range map[Key]int{"F1": 1, "F12": 12, "F13": 0, "F": 0, "A": 0, "F0": 0} {
		if got := functionKey(k); got != want {
			t.Errorf("functionKey(%q) = %d, want %d", k, got, want)
		}
	}
}

func TestLetterForCodeRoundTrip(t *testing.T) {
	found := map[Key]string{}
	for code, kc := range physicalCodes {
		letter, ok := LetterForCode(code)
		if !ok {
			continue // e.g. AZERTY's QWERTY-M key types ","
		}
		// The letter typed by a physical key must map back to that key.
		if back, _ := keyCode(letter); back != kc {
			t.Fatalf("%s types %q but %q maps to %#x, not %#x", code, letter, letter, back, kc)
		}
		found[letter] = code
	}
	if len(found) != 26 {
		t.Fatalf("only %d letters reachable: %v", len(found), found)
	}
	t.Logf("on this layout: A=%s M=%s Q=%s", found["A"], found["M"], found["Q"])
}
