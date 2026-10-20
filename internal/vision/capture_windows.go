//go:build windows

package vision

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GDI capture of the primary monitor. Spark's overlay is excluded by
// WDA_EXCLUDEFROMCAPTURE (see package overlay). DXGI Desktop Duplication
// would be faster but needs D3D11 plumbing; GDI is plenty for on-demand
// screenshots.

const (
	smCxScreen   = 0
	smCyScreen   = 1
	halftone     = 4
	srcCopy      = 0x00CC0020
	biRGB        = 0
	dibRGBColors = 0
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procGetDC            = user32.NewProc("GetDC")
	procReleaseDC        = user32.NewProc("ReleaseDC")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procStretchBlt         = gdi32.NewProc("StretchBlt")
	procSetStretchBltMode  = gdi32.NewProc("SetStretchBltMode")
	procSetBrushOrgEx      = gdi32.NewProc("SetBrushOrgEx")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

type windowsPlatform struct {
	rgba *image.RGBA // reused between captures (Service serializes calls)
}

func newPlatform() platform { return &windowsPlatform{} }

func hasPermission() bool { return true }

func requestPermission() {}

func (p *windowsPlatform) captureJPEG(_ context.Context, opts Options, dst []byte) ([]byte, int, int, error) {
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	if sw == 0 || sh == 0 {
		return dst, 0, 0, errors.New("vision: no screen")
	}
	w, h := fitSize(int(sw), int(sh), opts.MaxSide)

	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return dst, 0, 0, errors.New("vision: GetDC failed")
	}
	defer procReleaseDC.Call(0, screen)

	mem, _, _ := procCreateCompatibleDC.Call(screen)
	if mem == 0 {
		return dst, 0, 0, errors.New("vision: CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(mem)

	bi := bitmapInfo{Header: bitmapInfoHeader{
		Width: int32(w), Height: -int32(h), // negative: top-down rows
		Planes: 1, BitCount: 32, Compression: biRGB,
	}}
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	var bits unsafe.Pointer
	dib, _, _ := procCreateDIBSection.Call(mem, uintptr(unsafe.Pointer(&bi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 || bits == nil {
		return dst, 0, 0, errors.New("vision: CreateDIBSection failed")
	}
	defer procDeleteObject.Call(dib)
	old, _, _ := procSelectObject.Call(mem, dib)
	defer procSelectObject.Call(mem, old)

	// HALFTONE gives a proper downscale (averaging) instead of dropping pixels.
	procSetStretchBltMode.Call(mem, halftone)
	procSetBrushOrgEx.Call(mem, 0, 0, 0)
	if r, _, _ := procStretchBlt.Call(mem, 0, 0, uintptr(w), uintptr(h),
		screen, 0, 0, sw, sh, srcCopy); r == 0 {
		return dst, 0, 0, errors.New("vision: StretchBlt failed")
	}

	if p.rgba == nil || p.rgba.Rect.Dx() != w || p.rgba.Rect.Dy() != h {
		p.rgba = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	bgra := unsafe.Slice((*byte)(bits), w*h*4)
	pix := p.rgba.Pix
	for i := 0; i < len(bgra); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = bgra[i+2], bgra[i+1], bgra[i], 0xff
	}

	buf := bytes.NewBuffer(dst)
	if err := jpeg.Encode(buf, p.rgba, &jpeg.Options{Quality: opts.Quality}); err != nil {
		return dst, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}
