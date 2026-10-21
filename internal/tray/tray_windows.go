//go:build windows

package tray

import (
	"errors"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Notification-area icon: a message-only window receives the icon's
// clicks and shows a popup menu.

const (
	wmApp          = 0x8000
	wmTray         = wmApp + 1
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	wmNull         = 0x0000
	nimAdd         = 0x0
	nifMessage     = 0x1
	nifIcon        = 0x2
	nifTip         = 0x4
	mfString       = 0x0
	mfSeparator    = 0x800
	tpmRightButton = 0x2
	tpmNoNotify    = 0x80
	tpmReturnCmd   = 0x100
	idiApplication = 32512
)

var hwndMessage = ^uintptr(2) // HWND_MESSAGE (-3)

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconW     = shell32.NewProc("ExtractIconW")
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type notifyIconData struct {
	Size            uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        [16]byte
	BalloonIcon     uintptr
}

type point struct{ X, Y int32 }

type msg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

var items []Item

func showMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	for i, it := range items {
		if it.Title == "" {
			procAppendMenuW.Call(menu, mfSeparator, 0, 0)
			continue
		}
		title, _ := windows.UTF16PtrFromString(it.Title)
		procAppendMenuW.Call(menu, mfString, uintptr(i+1), uintptr(unsafe.Pointer(title)))
	}
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// Required for the menu to close when clicking elsewhere.
	procSetForegroundWnd.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmReturnCmd|tpmNoNotify|tpmRightButton,
		uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	procPostMessageW.Call(hwnd, wmNull, 0, 0)
	if i := int(cmd) - 1; i >= 0 && i < len(items) && items[i].Action != nil {
		go items[i].Action()
	}
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	if message == wmTray && (lParam == wmLButtonUp || lParam == wmRButtonUp) {
		showMenu(hwnd)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func start(tooltip string, list []Item) error {
	items = list
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread() // the window and its messages live on this thread
		className, _ := windows.UTF16PtrFromString("SparkTray")
		wc := wndClassEx{WndProc: windows.NewCallback(wndProc), ClassName: className}
		wc.Size = uint32(unsafe.Sizeof(wc))
		if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			ready <- errors.New("tray: RegisterClassEx failed")
			return
		}
		hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, 0, 0)
		if hwnd == 0 {
			ready <- errors.New("tray: CreateWindowEx failed")
			return
		}

		nid := notifyIconData{Wnd: hwnd, ID: 1, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmTray}
		nid.Size = uint32(unsafe.Sizeof(nid))
		exe, _ := os.Executable()
		exePtr, _ := windows.UTF16PtrFromString(exe)
		nid.Icon, _, _ = procExtractIconW.Call(0, uintptr(unsafe.Pointer(exePtr)), 0)
		if nid.Icon <= 1 {
			nid.Icon, _, _ = procLoadIconW.Call(0, idiApplication)
		}
		tip, _ := windows.UTF16FromString(tooltip)
		copy(nid.Tip[:len(nid.Tip)-1], tip)
		if r, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
			ready <- errors.New("tray: Shell_NotifyIcon failed")
			return
		}
		ready <- nil

		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	return <-ready
}
