//go:build darwin

package hotkey

import "testing"

func TestKeyCodeResolvesEveryLetter(t *testing.T) {
	seen := map[uint32]Key{}
	for k := Key('A'); k <= 'Z'; k++ {
		code, ok := keyCode(k)
		if !ok {
			t.Fatalf("no key code for %q", rune(k))
		}
		if other, dup := seen[code]; dup {
			t.Fatalf("%q and %q share key code %#x", rune(k), rune(other), code)
		}
		seen[code] = k
	}
	a, _ := keyCode('A')
	t.Logf("layout: 'A' is key code %#x (QWERTY 0x00, AZERTY 0x0C)", a)
}
