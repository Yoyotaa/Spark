//go:build windows

package overlay

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsExLayered       = 0x00080000
	wsExTransparent   = 0x00000020
	wsExToolWindow    = 0x00000080 // hidden from taskbar and Alt+Tab
	wsExTopmost       = 0x00000008
	lwaAlpha          = 0x2
	wdaExcludeCapture = 0x11 // WDA_EXCLUDEFROMCAPTURE, Windows 10 2004+
	swpNoActivate     = 0x0010
	swpShowWindow     = 0x0040
	monitorNearest    = 0x2
)

var (
	gwlExStyle  = -20         // GWL_EXSTYLE; a var so it converts to uintptr
	hwndTopmost = ^uintptr(0) // (HWND)-1
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW              = user32.NewProc("FindWindowW")
	procGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW        = user32.NewProc("SetWindowLongPtrW")
	procSetLayeredWindowAttribs  = user32.NewProc("SetLayeredWindowAttributes")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procMonitorFromWindow        = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procScreenToClient           = user32.NewProc("ScreenToClient")
	procGetDpiForWindow          = user32.NewProc("GetDpiForWindow")
)

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

type windowsPlatform struct {
	hwnd  uintptr
	scale float64 // physical px per CSS px
}

func newPlatform() platform { return &windowsPlatform{scale: 1} }

func (p *windowsPlatform) attach() error {
	cls, _ := windows.UTF16PtrFromString(WindowClassName)
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)), 0)
	if hwnd == 0 {
		return errors.New("overlay: HWND not found")
	}
	p.hwnd = hwnd

	if dpi, _, _ := procGetDpiForWindow.Call(hwnd); dpi != 0 {
		p.scale = float64(dpi) / 96
	}

	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlExStyle))
	procSetWindowLongPtrW.Call(hwnd, uintptr(gwlExStyle), ex|wsExLayered|wsExToolWindow|wsExTopmost)
	// A layered window without attributes is invisible: force full opacity,
	// per-pixel transparency is then handled by WebView2.
	procSetLayeredWindowAttribs.Call(hwnd, 0, 255, lwaAlpha)
	// Keep A.R.I.A out of her own screenshots (no-op on older Windows).
	procSetWindowDisplayAffinity.Call(hwnd, wdaExcludeCapture)

	mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorNearest)
	mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r != 0 {
		m := mi.RcMonitor
		procSetWindowPos.Call(hwnd, hwndTopmost,
			uintptr(m.Left), uintptr(m.Top),
			uintptr(m.Right-m.Left), uintptr(m.Bottom-m.Top),
			swpNoActivate|swpShowWindow)
	}
	return nil
}

func (p *windowsPlatform) cursor() (float64, float64, bool) {
	var pt point
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return 0, 0, false
	}
	if r, _, _ := procScreenToClient.Call(p.hwnd, uintptr(unsafe.Pointer(&pt))); r == 0 {
		return 0, 0, false
	}
	return float64(pt.X) / p.scale, float64(pt.Y) / p.scale, true
}

func (p *windowsPlatform) setIgnoreMouse(ignore bool) {
	ex, _, _ := procGetWindowLongPtrW.Call(p.hwnd, uintptr(gwlExStyle))
	if ignore {
		ex |= wsExTransparent
	} else {
		ex &^= wsExTransparent
	}
	procSetWindowLongPtrW.Call(p.hwnd, uintptr(gwlExStyle), ex)
}
