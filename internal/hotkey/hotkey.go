// Package hotkey registers system-wide keyboard shortcuts, i.e. shortcuts
// that fire even when another application (a game, Discord...) has focus.
//
// macOS uses Carbon's RegisterEventHotKey, which needs no Accessibility
// permission; Windows uses RegisterHotKey on a dedicated message-loop thread.
package hotkey

import (
	"strings"
)

type Modifier uint8

const (
	ModAlt   Modifier = 1 << iota // ⌥ Option on macOS
	ModShift                      // ⇧
	ModCtrl                       // ⌃
	ModCmd                        // ⌘ on macOS, Windows key on Windows
)

// Key is "A"-"Z", "0"-"9", "F1"-"F12" or "Space". Letters follow the active
// keyboard layout; digits and function keys are physical keys.
type Key string

const KeySpace Key = "Space"

// Binding associates a shortcut with a callback. Callbacks run on their own
// goroutine, never on a UI thread, so they may freely call back into the
// window layer.
type Binding struct {
	Mod Modifier
	Key Key
	Fn  func() // key pressed
	// Release, if set, is called when the key is released (push-to-talk).
	Release func()
}

func (b Binding) String() string {
	var s strings.Builder
	for _, m := range []struct {
		mod  Modifier
		name string
	}{{ModCtrl, "Ctrl+"}, {ModAlt, "Alt+"}, {ModShift, "Shift+"}, {ModCmd, "Cmd+"}} {
		if b.Mod&m.mod != 0 {
			s.WriteString(m.name)
		}
	}
	s.WriteString(string(b.Key))
	return s.String()
}

// functionKey returns n for "F<n>" (1-12), 0 otherwise.
func functionKey(k Key) int {
	if len(k) < 2 || k[0] != 'F' {
		return 0
	}
	n := 0
	for _, c := range k[1:] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	if n < 1 || n > 12 {
		return 0
	}
	return n
}

// Listener holds registered shortcuts until Close.
type Listener struct {
	close func()
}

// Close unregisters the shortcuts and waits until it is done, so the same
// combinations can be registered again right after. Must not be called from
// the UI main thread.
func (l *Listener) Close() {
	if l != nil && l.close != nil {
		l.close()
		l.close = nil
	}
}

// Register activates the bindings. It fails, registering nothing, if a
// shortcut is invalid or already taken by another application.
func Register(bindings []Binding) (*Listener, error) {
	return register(bindings)
}

// LetterForCode returns the letter that the physical key identified by a DOM
// KeyboardEvent.code ("KeyQ"...) types with the current layout: on AZERTY,
// "KeyQ" is "A". Needed because Option+letter produces special characters
// on macOS, hiding the letter from the webview.
func LetterForCode(code string) (Key, bool) { return letterForCode(code) }
