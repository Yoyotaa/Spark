//go:build darwin

package audio

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework Speech -framework AppKit -framework Foundation
#include <stdlib.h>
#include "mic_darwin.h"
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

// macOS: AVAudioEngine + SFSpeechRecognizer, on-device when available.
// Recognition is local and streamed, so any AI provider gets plain text.

var (
	eventsMu  sync.RWMutex
	events    InputEvents
	permMu    sync.Mutex
	permWaits = map[int][]func(bool){}
)

type darwinInput struct{}

// NewInput returns the native input. cloud is unused on macOS.
func NewInput(ev InputEvents, _ Transcriber) Input {
	eventsMu.Lock()
	events = ev
	eventsMu.Unlock()
	return darwinInput{}
}

func current() InputEvents {
	eventsMu.RLock()
	defer eventsMu.RUnlock()
	return events
}

//export sparkMicLevel
func sparkMicLevel(rms C.float) {
	if f := current().Level; f != nil {
		f(float32(rms))
	}
}

//export sparkSpeechResult
func sparkSpeechResult(text *C.char, final C.int) {
	ev := current()
	s := C.GoString(text)
	if final != 0 {
		if ev.Final != nil {
			ev.Final(s)
		}
	} else if ev.Partial != nil {
		ev.Partial(s)
	}
}

//export sparkSpeechError
func sparkSpeechError(msg *C.char) {
	if f := current().Error; f != nil {
		f(errors.New(C.GoString(msg)))
	}
}

//export sparkPermissionResult
func sparkPermissionResult(kind C.int, granted C.int) {
	permMu.Lock()
	waits := permWaits[int(kind)]
	delete(permWaits, int(kind))
	permMu.Unlock()
	for _, f := range waits {
		go f(granted != 0)
	}
}

func cerr(buf []byte) error {
	return errors.New(C.GoString((*C.char)(unsafe.Pointer(&buf[0]))))
}

func (darwinInput) StartMic() error {
	buf := make([]byte, 256)
	if C.sparkMicStart((*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))) != 0 {
		return cerr(buf)
	}
	return nil
}

func (darwinInput) StopMic() { C.sparkMicStop() }

func (darwinInput) Begin(lang string) error {
	loc := C.CString(locale(lang))
	defer C.free(unsafe.Pointer(loc))
	buf := make([]byte, 256)
	if C.sparkSpeechBegin(loc, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))) != 0 {
		return cerr(buf)
	}
	return nil
}

func (darwinInput) End()       { C.sparkSpeechEnd() }
func (darwinInput) Cancel()    { C.sparkSpeechCancel() }
func (darwinInput) Live() bool { return true }

// MicPermission and SpeechPermission report the OS permission states.
func MicPermission() Permission    { return Permission(C.sparkMicPermission()) }
func SpeechPermission() Permission { return Permission(C.sparkSpeechPermission()) }

// RequestPermissions asks for the microphone then speech recognition, as
// needed, and reports whether both are granted.
func RequestPermissions(done func(ok bool)) {
	ask := func(kind int, request func(), next func(bool)) {
		permMu.Lock()
		permWaits[kind] = append(permWaits[kind], next)
		permMu.Unlock()
		request()
	}
	afterMic := func(ok bool) {
		if !ok {
			done(false)
			return
		}
		if SpeechPermission() == PermissionGranted {
			done(true)
			return
		}
		ask(1, func() { C.sparkRequestSpeechPermission() }, done)
	}
	if MicPermission() == PermissionGranted {
		afterMic(true)
		return
	}
	ask(0, func() { C.sparkRequestMicPermission() }, afterMic)
}

// PlayCue plays a short system sound when listening starts or stops.
func PlayCue(start bool) {
	v := C.int(0)
	if start {
		v = 1
	}
	C.sparkPlayCue(v)
}
