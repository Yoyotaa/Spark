//go:build darwin

package hotkey

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Carbon -framework Foundation
#include "hotkey_darwin.h"
*/
import "C"

import (
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

// Virtual key codes from HIToolbox/Events.h. They are physical positions:
// letters are first resolved through the active layout (AZERTY 'A' is the
// QWERTY 'Q' key), this ANSI table is the fallback.
var ansiKeyCodes = map[Key]uint32{
	"A": 0x00, "S": 0x01, "D": 0x02, "F": 0x03, "H": 0x04, "G": 0x05,
	"Z": 0x06, "X": 0x07, "C": 0x08, "V": 0x09, "B": 0x0B, "Q": 0x0C,
	"W": 0x0D, "E": 0x0E, "R": 0x0F, "Y": 0x10, "T": 0x11, "O": 0x1F,
	"U": 0x20, "I": 0x22, "P": 0x23, "L": 0x25, "J": 0x26, "K": 0x28,
	"N": 0x2D, "M": 0x2E, KeySpace: 0x31,
	// Top-row digits: physical keys on every layout (AZERTY types "&é"...).
	"1": 0x12, "2": 0x13, "3": 0x14, "4": 0x15, "5": 0x17,
	"6": 0x16, "7": 0x1A, "8": 0x1C, "9": 0x19, "0": 0x1D,
}

var functionKeyCodes = [13]uint32{0, 0x7A, 0x78, 0x63, 0x76, 0x60, 0x61, 0x62, 0x64, 0x65, 0x6D, 0x67, 0x6F}

var (
	mu       sync.Mutex
	handlers        = map[uint32]func(){}
	nextID   uint32 = 1
)

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

func keyCode(k Key) (uint32, bool) {
	if n := functionKey(k); n > 0 {
		return functionKeyCodes[n], true
	}
	if len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z' {
		if code := C.sparkKeyCodeForChar(C.uint16_t(k[0])); code >= 0 {
			return uint32(code), true
		}
	}
	code, ok := ansiKeyCodes[k]
	return code, ok
}

// physicalCodes maps DOM KeyboardEvent.code values to macOS key codes.
var physicalCodes = func() map[string]uint32 {
	m := map[string]uint32{}
	for k, code := range ansiKeyCodes {
		if len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z' {
			m["Key"+string(k)] = code
		}
	}
	// Punctuation positions also carry letters on some layouts (AZERTY "M"
	// sits on the QWERTY ";" key).
	for code, kc := range map[string]uint32{
		"Semicolon": 0x29, "Quote": 0x27, "Comma": 0x2B, "Period": 0x2F,
		"Slash": 0x2C, "BracketLeft": 0x21, "BracketRight": 0x1E,
		"Backslash": 0x2A, "Minus": 0x1B, "Equal": 0x18, "Backquote": 0x32,
		"IntlBackslash": 0x0A,
	} {
		m[code] = kc
	}
	return m
}()

func letterForCode(code string) (Key, bool) {
	kc, ok := physicalCodes[code]
	if !ok {
		return "", false
	}
	c := C.sparkCharForKeyCode(C.uint16_t(kc))
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	if c < 'A' || c > 'Z' {
		return "", false
	}
	return Key(string(rune(c))), true
}

func modifiers(m Modifier) uint32 {
	var mods uint32
	if m&ModAlt != 0 {
		mods |= optionKey
	}
	if m&ModShift != 0 {
		mods |= shiftKey
	}
	if m&ModCtrl != 0 {
		mods |= controlKey
	}
	if m&ModCmd != 0 {
		mods |= cmdKey
	}
	return mods
}

func register(bindings []Binding) (*Listener, error) {
	mu.Lock()
	defer mu.Unlock()

	var ids []uint32
	unregister := func() {
		for _, id := range ids {
			C.sparkUnregisterHotkey(C.uint32_t(id))
			delete(handlers, id)
		}
	}
	for _, b := range bindings {
		code, ok := keyCode(b.Key)
		if !ok {
			unregister()
			return nil, fmt.Errorf("touche non prise en charge : %q", b.Key)
		}
		id := nextID
		nextID = nextID%255 + 1 // ids are slots in a 256-entry table
		if st := C.sparkRegisterHotkey(C.uint32_t(id), C.uint32_t(code), C.uint32_t(modifiers(b.Mod))); st != 0 {
			unregister()
			return nil, fmt.Errorf("%s est déjà utilisé par une autre application", b)
		}
		ids = append(ids, id)
		handlers[id] = b.Fn
	}
	return &Listener{close: func() {
		mu.Lock()
		defer mu.Unlock()
		unregister()
	}}, nil
}
