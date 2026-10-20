// Package hotkey registers system-wide keyboard shortcuts, i.e. shortcuts
// that fire even when another application (a game, Discord...) has focus.
//
// macOS uses Carbon's RegisterEventHotKey, which needs no Accessibility
// permission; Windows uses RegisterHotKey on a dedicated message-loop thread.
package hotkey

import (
	"context"
	"strings"
)

type Modifier uint8

const (
	ModAlt   Modifier = 1 << iota // ⌥ Option on macOS
	ModShift                      // ⇧
	ModCtrl                       // ⌃
	ModCmd                        // ⌘ on macOS, Windows key on Windows
)

// Key is an uppercase letter 'A'..'Z' or KeySpace.
type Key rune

const KeySpace Key = ' '

// Binding associates a shortcut with a callback. Callbacks run on their own
// goroutine, never on a UI thread, so they may freely call back into the
// window layer.
type Binding struct {
	Mod Modifier
	Key Key
	Fn  func()
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
	if b.Key == KeySpace {
		s.WriteString("Space")
	} else {
		s.WriteRune(rune(b.Key))
	}
	return s.String()
}

// Listen registers the bindings and keeps them active until ctx is
// cancelled. It fails if any shortcut is already taken by another app.
func Listen(ctx context.Context, bindings []Binding) error {
	return listen(ctx, bindings)
}
