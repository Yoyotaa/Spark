//go:build darwin

package overlay

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

static NSWindow *sparkWindow = nil;

// AppKit is not thread-safe: every window access is marshalled to the main
// thread. dispatch_sync is fine here because the main thread never waits on
// the polling goroutine.
static void sparkOnMain(dispatch_block_t block) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_sync(dispatch_get_main_queue(), block);
	}
}

static int sparkAttach(void) {
	__block int ok = 0;
	sparkOnMain(^{
		NSWindow *w = [NSApp mainWindow];
		if (w == nil) {
			w = [[NSApp windows] firstObject];
		}
		if (w == nil) {
			return;
		}
		sparkWindow = w;

		// Accessory app: no Dock icon, and allowed to float above other
		// apps' full-screen spaces (games, videos).
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

		[w setOpaque:NO];
		[w setBackgroundColor:[NSColor clearColor]];
		[w setHasShadow:NO];
		[w setLevel:NSStatusWindowLevel];
		[w setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces |
		                         NSWindowCollectionBehaviorStationary |
		                         NSWindowCollectionBehaviorFullScreenAuxiliary |
		                         NSWindowCollectionBehaviorIgnoresCycle];
		// Keep A.R.I.A out of her own screenshots.
		[w setSharingType:NSWindowSharingNone];

		NSScreen *screen = [w screen] ?: [NSScreen mainScreen];
		[w setFrame:[screen frame] display:YES];
		[w orderFrontRegardless];
		ok = 1;
	});
	return ok;
}

static int sparkCursor(double *x, double *y) {
	__block int ok = 0;
	sparkOnMain(^{
		if (sparkWindow == nil) {
			return;
		}
		// Window coordinates, bottom-left origin, in points (= CSS px).
		NSPoint p = [sparkWindow mouseLocationOutsideOfEventStream];
		NSRect b = [[sparkWindow contentView] bounds];
		*x = p.x;
		*y = b.size.height - p.y;
		ok = 1;
	});
	return ok;
}

static void sparkSetIgnoreMouse(int ignore) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[sparkWindow setIgnoresMouseEvents:(ignore ? YES : NO)];
	});
}
*/
import "C"

import "errors"

type darwinPlatform struct{}

func newPlatform() platform { return darwinPlatform{} }

func (darwinPlatform) attach() error {
	if C.sparkAttach() == 0 {
		return errors.New("overlay: NSWindow not found")
	}
	return nil
}

func (darwinPlatform) cursor() (float64, float64, bool) {
	var x, y C.double
	if C.sparkCursor(&x, &y) == 0 {
		return 0, 0, false
	}
	return float64(x), float64(y), true
}

func (darwinPlatform) setIgnoreMouse(ignore bool) {
	v := C.int(0)
	if ignore {
		v = 1
	}
	C.sparkSetIgnoreMouse(v)
}
