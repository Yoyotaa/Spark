//go:build darwin

package vision

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework CoreGraphics -framework ImageIO -framework ScreenCaptureKit
#include <stdlib.h>
#include "capture_darwin.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"
)

type darwinPlatform struct{}

func newPlatform() platform { return darwinPlatform{} }

func hasPermission() bool { return C.sparkHasScreenAccess() == 1 }

func requestPermission() { C.sparkRequestScreenAccess() }

const defaultTimeout = 5 * time.Second

func (darwinPlatform) captureJPEG(ctx context.Context, opts Options, dst []byte) ([]byte, int, int, error) {
	timeout := defaultTimeout
	if d, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(d))
	}
	var f C.sparkFrame
	cursor := C.int(0)
	if opts.ShowCursor {
		cursor = 1
	}
	rc := C.sparkCaptureScreen(C.int(opts.MaxSide), C.double(float64(opts.Quality)/100), cursor, C.double(timeout.Seconds()), &f)
	if rc != 0 {
		if f.denied == 1 || !hasPermission() {
			return dst, 0, 0, ErrPermission
		}
		return dst, 0, 0, fmt.Errorf("vision: %s", C.GoString(&f.err[0]))
	}
	defer C.free(f.data)
	if f.len == 0 {
		return dst, 0, 0, errors.New("vision: empty capture")
	}
	// One copy, into the pooled buffer.
	dst = append(dst, unsafe.Slice((*byte)(f.data), int(f.len))...)
	return dst, int(f.width), int(f.height), nil
}
