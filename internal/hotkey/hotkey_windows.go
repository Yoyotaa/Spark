//go:build windows

package hotkey

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
	vkSpace     = 0x20
	vkF1        = 0x70
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
)

type msg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       struct{ X, Y int32 }
	LPrivate uint32
}

func vkOf(k Key) (uintptr, bool) {
	if n := functionKey(k); n > 0 {
		return uintptr(vkF1 + n - 1), true
	}
	switch {
	case k == KeySpace:
		return vkSpace, true
	case len(k) == 1 && (k[0] >= 'A' && k[0] <= 'Z' || k[0] >= '0' && k[0] <= '9'):
		return uintptr(k[0]), true // VK_A..VK_Z and VK_0..VK_9 match ASCII
	}
	return 0, false
}

// register runs a dedicated OS thread: hotkeys registered with a NULL HWND
// are delivered to the message queue of the registering thread, and
// unregistered when it exits.
func register(bindings []Binding) (*Listener, error) {
	ready := make(chan error, 1)
	exited := make(chan struct{})
	var tid uint32

	go func() {
		defer close(exited)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		var m msg
		// Force the creation of this thread's message queue so that
		// PostThreadMessage(WM_QUIT) cannot be lost.
		procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 0)
		tid = windows.GetCurrentThreadId()

		registered := 0
		defer func() {
			for id := 1; id <= registered; id++ {
				procUnregisterHotKey.Call(0, uintptr(id))
			}
		}()

		for i, b := range bindings {
			vk, ok := vkOf(b.Key)
			if !ok {
				ready <- fmt.Errorf("touche non prise en charge : %q", b.Key)
				return
			}
			mods := uintptr(modNoRepeat)
			if b.Mod&ModAlt != 0 {
				mods |= modAlt
			}
			if b.Mod&ModShift != 0 {
				mods |= modShift
			}
			if b.Mod&ModCtrl != 0 {
				mods |= modControl
			}
			if b.Mod&ModCmd != 0 {
				mods |= modWin
			}
			if r, _, _ := procRegisterHotKey.Call(0, uintptr(i+1), mods, vk); r == 0 {
				ready <- fmt.Errorf("%s est déjà utilisé par une autre application", b)
				return
			}
			registered++
		}
		ready <- nil

		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 { // WM_QUIT or error
				return
			}
			if m.Message == wmHotkey {
				if id := int(m.WParam); id >= 1 && id <= len(bindings) {
					go bindings[id-1].Fn()
				}
			}
		}
	}()

	if err := <-ready; err != nil {
		<-exited
		return nil, err
	}
	return &Listener{close: func() {
		procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0)
		<-exited
	}}, nil
}

// Windows reports layout letters in KeyboardEvent.key even with Alt: the
// physical code is only a fallback.
func letterForCode(code string) (Key, bool) {
	if len(code) == 4 && code[:3] == "Key" && code[3] >= 'A' && code[3] <= 'Z' {
		return Key(code[3:]), true
	}
	return "", false
}
