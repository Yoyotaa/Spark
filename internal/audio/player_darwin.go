//go:build darwin

package audio

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework Foundation
#include <stdlib.h>
#include "player_darwin.h"
*/
import "C"

import (
	"context"
	"errors"
	"unsafe"
)

type darwinPlayer struct{}

func newPlayer() Player { return darwinPlayer{} }

func (darwinPlayer) Start(rate int) error {
	buf := make([]byte, 256)
	if C.sparkPlayerStart(C.double(rate), (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))) != 0 {
		return errors.New(C.GoString((*C.char)(unsafe.Pointer(&buf[0]))))
	}
	return nil
}

func (darwinPlayer) Write(pcm []byte) {
	s := samples(pcm)
	if len(s) == 0 {
		return
	}
	C.sparkPlayerWrite((*C.int16_t)(unsafe.Pointer(&s[0])), C.size_t(len(s)))
}

func (p darwinPlayer) Drain(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		C.sparkPlayerDrain()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		p.Stop() // releases the drain goroutine
		<-done
		return ctx.Err()
	}
}

func (darwinPlayer) Stop() { C.sparkPlayerStop() }
