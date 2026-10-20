//go:build darwin

package hotkey

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Carbon -framework Foundation
#include "hotkey_darwin.h"
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
)

// Carbon modifier masks (Events.h).
const (
	cmdKey     = 0x0100
	shiftKey   = 0x0200
	optionKey  = 0x0800
	controlKey = 0x1000
)

// Virtual key codes of the ANSI (QWERTY) layout, from HIToolbox/Events.h.
// Only a fallback: codes are physical positions, so letters are first
// resolved through the active layout (AZERTY 'A' is QWERTY 'Q').
var ansiKeyCodes = map[Key]uint32{
	'A': 0x00, 'S': 0x01, 'D': 0x02, 'F': 0x03, 'H': 0x04, 'G': 0x05,
	'Z': 0x06, 'X': 0x07, 'C': 0x08, 'V': 0x09, 'B': 0x0B, 'Q': 0x0C,
	'W': 0x0D, 'E': 0x0E, 'R': 0x0F, 'Y': 0x10, 'T': 0x11, 'O': 0x1F,
	'U': 0x20, 'I': 0x22, 'P': 0x23, 'L': 0x25, 'J': 0x26, 'K': 0x28,
	'N': 0x2D, 'M': 0x2E, KeySpace: 0x31,
}

var (
	mu       sync.Mutex
	handlers        = map[uint32]func(){}
	nextID   uint32 = 1
)

func keyCode(k Key) (uint32, bool) {
	if k != KeySpace {
		if code := C.sparkKeyCodeForChar(C.uint16_t(k)); code >= 0 {
			return uint32(code), true
		}
	}
	code, ok := ansiKeyCodes[k]
	return code, ok
}

//export sparkHotkeyFired
func sparkHotkeyFired(id C.uint32_t) {
	mu.Lock()
	fn := handlers[uint32(id)]
	mu.Unlock()
	if fn != nil {
		// Runs on the AppKit main thread: hand off immediately, callbacks
		// may dispatch_sync back to the main queue.
		go fn()
	}
}

func listen(ctx context.Context, bindings []Binding) error {
	mu.Lock()
	defer mu.Unlock()

	for _, b := range bindings {
		code, ok := keyCode(b.Key)
		if !ok {
			return fmt.Errorf("hotkey: unsupported key %q", rune(b.Key))
		}
		var mods uint32
		if b.Mod&ModAlt != 0 {
			mods |= optionKey
		}
		if b.Mod&ModShift != 0 {
			mods |= shiftKey
		}
		if b.Mod&ModCtrl != 0 {
			mods |= controlKey
		}
		if b.Mod&ModCmd != 0 {
			mods |= cmdKey
		}
		id := nextID
		nextID++
		if st := C.sparkRegisterHotkey(C.uint32_t(id), C.uint32_t(code), C.uint32_t(mods)); st != 0 {
			return fmt.Errorf("hotkey: cannot register %s (status %d)", b, st)
		}
		handlers[id] = b.Fn
	}

	go func() {
		<-ctx.Done()
		C.sparkUnregisterHotkeys()
		mu.Lock()
		clear(handlers)
		mu.Unlock()
	}()
	return nil
}
