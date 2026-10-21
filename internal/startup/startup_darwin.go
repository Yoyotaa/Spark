//go:build darwin

package startup

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework ServiceManagement -framework Foundation
#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>
#include <string.h>

// SMAppService (macOS 13+): the app itself as a login item, listed in
// System Settings › General › Login Items.
static int sparkLoginEnabled(void) {
	if (@available(macOS 13.0, *)) {
		return SMAppService.mainAppService.status == SMAppServiceStatusEnabled ? 1 : 0;
	}
	return 0;
}

static int sparkLoginSet(int on, char *err, size_t errLen) {
	if (@available(macOS 13.0, *)) {
		NSError *e = nil;
		BOOL ok = on ? [SMAppService.mainAppService registerAndReturnError:&e]
		             : [SMAppService.mainAppService unregisterAndReturnError:&e];
		if (!ok) {
			strlcpy(err, (e.localizedDescription ?: @"échec").UTF8String, errLen);
			return -1;
		}
		return 0;
	}
	strlcpy(err, "nécessite macOS 13 ou plus récent", errLen);
	return -1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func enabled() bool { return C.sparkLoginEnabled() == 1 }

func set(on bool) error {
	v := C.int(0)
	if on {
		v = 1
	}
	buf := make([]byte, 256)
	if C.sparkLoginSet(v, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))) != 0 {
		return errors.New(C.GoString((*C.char)(unsafe.Pointer(&buf[0]))))
	}
	return nil
}
