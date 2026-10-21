//go:build darwin

package overlay

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

static NSWindow *sparkWindow = nil;
// Last application that was active before Spark, to hand focus back.
static NSRunningApplication *sparkPreviousApp = nil;

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

		NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
		if (![front isEqual:[NSRunningApplication currentApplication]]) {
			sparkPreviousApp = front;
		}
		// Track the last other app that was activated, whether Spark got the
		// focus through a hotkey or a mouse click.
		[[[NSWorkspace sharedWorkspace] notificationCenter]
			addObserverForName:NSWorkspaceDidActivateApplicationNotification
			            object:nil
			             queue:[NSOperationQueue mainQueue]
			        usingBlock:^(NSNotification *n) {
				NSRunningApplication *app = n.userInfo[NSWorkspaceApplicationKey];
				if (![app isEqual:[NSRunningApplication currentApplication]]) {
					sparkPreviousApp = app;
				}
			}];
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

// Permission dialogs of macOS sit below the status-bar level the overlay
// uses: step down to a normal window while one is shown.
static BOOL sparkYielding = NO;

static void sparkYield(int yield) {
	dispatch_async(dispatch_get_main_queue(), ^{
		sparkYielding = yield ? YES : NO;
		[sparkWindow setLevel:(yield ? NSNormalWindowLevel : NSStatusWindowLevel)];
		if (!yield) {
			[sparkWindow orderFrontRegardless];
		}
	});
}

// Self-healing: put the overlay back on top if anything (permission
// dialogs, Spaces, full-screen apps, activation changes) demoted or hid it.
// Returns a bit mask of what was repaired: 1 level, 2 ordering.
static int sparkEnsureFront(void) {
	__block int fixed = 0;
	sparkOnMain(^{
		if (sparkWindow == nil || sparkYielding) {
			return;
		}
		if (sparkWindow.level != NSStatusWindowLevel) {
			[sparkWindow setLevel:NSStatusWindowLevel];
			fixed |= 1;
		}
		if (!sparkWindow.isVisible || !(sparkWindow.occlusionState & NSWindowOcclusionStateVisible)) {
			[sparkWindow orderFrontRegardless];
			fixed |= 2;
		}
	});
	return fixed;
}

static void sparkFocus(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
		[NSApp activateIgnoringOtherApps:YES];
#pragma clang diagnostic pop
		[sparkWindow makeKeyAndOrderFront:nil];
	});
}

static void sparkRestoreFocus(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (![NSApp isActive] || sparkPreviousApp == nil || [sparkPreviousApp isTerminated]) {
			return;
		}
		[sparkPreviousApp activateWithOptions:0];
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

func (darwinPlatform) focus()        { C.sparkFocus() }
func (darwinPlatform) restoreFocus() { C.sparkRestoreFocus() }

func (darwinPlatform) yield(y bool) {
	v := C.int(0)
	if y {
		v = 1
	}
	C.sparkYield(v)
}

func (darwinPlatform) ensureFront() int { return int(C.sparkEnsureFront()) }
