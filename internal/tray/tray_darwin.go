//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

var (
	mu      sync.Mutex
	actions []func()
)

//export sparkTrayFired
func sparkTrayFired(index C.int) {
	mu.Lock()
	var fn func()
	if i := int(index); i >= 0 && i < len(actions) {
		fn = actions[i]
	}
	mu.Unlock()
	if fn != nil {
		go fn() // main thread: hand off at once
	}
}

func start(tooltip string, items []Item) error {
	mu.Lock()
	actions = make([]func(), len(items))
	for i, it := range items {
		actions[i] = it.Action
	}
	mu.Unlock()

	tip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(tip))
	titles := make([]*C.char, len(items))
	for i, it := range items {
		titles[i] = C.CString(it.Title)
		defer C.free(unsafe.Pointer(titles[i]))
	}
	var first **C.char
	if len(titles) > 0 {
		first = &titles[0]
	}
	C.sparkTrayStart(tip, first, C.int(len(items)))
	return nil
}
