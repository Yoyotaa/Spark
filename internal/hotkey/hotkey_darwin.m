#import <Carbon/Carbon.h>
#import <Foundation/Foundation.h>
#include <wctype.h>
#include "hotkey_darwin.h"
#include "_cgo_export.h"

#define SPARK_MAX_HOTKEYS 256

static EventHotKeyRef refs[SPARK_MAX_HOTKEYS];
static BOOL handlerInstalled = NO;

static OSStatus onHotkey(EventHandlerCallRef next, EventRef event, void *data) {
	EventHotKeyID hk;
	GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID, NULL,
	                  sizeof(hk), NULL, &hk);
	sparkHotkeyFired(hk.id);
	return noErr;
}

static void onMain(dispatch_block_t block) {
	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_sync(dispatch_get_main_queue(), block);
	}
}

// Finds the virtual key code that types `ch` with the current keyboard
// layout (AZERTY, QWERTZ...): Carbon hotkeys are bound to physical keys.
// Returns -1 if no key produces it.
int sparkKeyCodeForChar(uint16_t ch) {
	__block int result = -1;
	onMain(^{ // Text Input Sources must be queried on the main thread.
		TISInputSourceRef src = TISCopyCurrentKeyboardLayoutInputSource();
		CFDataRef data = src ? TISGetInputSourceProperty(src, kTISPropertyUnicodeKeyLayoutData) : NULL;
		if (data == NULL) { // e.g. a Chinese/Japanese input method is active
			if (src) CFRelease(src);
			src = TISCopyCurrentASCIICapableKeyboardLayoutInputSource();
			data = src ? TISGetInputSourceProperty(src, kTISPropertyUnicodeKeyLayoutData) : NULL;
		}
		if (data != NULL) {
			const UCKeyboardLayout *layout = (const UCKeyboardLayout *)CFDataGetBytePtr(data);
			UniChar want = (UniChar)towlower(ch);
			for (UInt16 code = 0; code < 128 && result < 0; code++) {
				UInt32 dead = 0;
				UniChar chars[4];
				UniCharCount len = 0;
				OSStatus st = UCKeyTranslate(layout, code, kUCKeyActionDown, 0, LMGetKbdType(),
				                             kUCKeyTranslateNoDeadKeysBit, &dead, 4, &len, chars);
				if (st == noErr && len == 1 && towlower(chars[0]) == want) {
					result = code;
				}
			}
		}
		if (src) CFRelease(src);
	});
	return result;
}

// Inverse of sparkKeyCodeForChar: the character a physical key types with
// the current layout and no modifier, or -1.
int sparkCharForKeyCode(uint16_t code) {
	__block int result = -1;
	onMain(^{
		TISInputSourceRef src = TISCopyCurrentKeyboardLayoutInputSource();
		CFDataRef data = src ? TISGetInputSourceProperty(src, kTISPropertyUnicodeKeyLayoutData) : NULL;
		if (data == NULL) {
			if (src) CFRelease(src);
			src = TISCopyCurrentASCIICapableKeyboardLayoutInputSource();
			data = src ? TISGetInputSourceProperty(src, kTISPropertyUnicodeKeyLayoutData) : NULL;
		}
		if (data != NULL) {
			const UCKeyboardLayout *layout = (const UCKeyboardLayout *)CFDataGetBytePtr(data);
			UInt32 dead = 0;
			UniChar chars[4];
			UniCharCount len = 0;
			OSStatus st = UCKeyTranslate(layout, code, kUCKeyActionDown, 0, LMGetKbdType(),
			                             kUCKeyTranslateNoDeadKeysBit, &dead, 4, &len, chars);
			if (st == noErr && len == 1) {
				result = chars[0];
			}
		}
		if (src) CFRelease(src);
	});
	return result;
}

int sparkRegisterHotkey(uint32_t id, uint32_t keyCode, uint32_t modifiers) {
	if (id >= SPARK_MAX_HOTKEYS) {
		return -1;
	}
	__block OSStatus status = noErr;
	onMain(^{
		if (!handlerInstalled) {
			EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
			InstallApplicationEventHandler(&onHotkey, 1, &spec, NULL, NULL);
			handlerInstalled = YES;
		}
		EventHotKeyID hk = {'SPRK', id};
		status = RegisterEventHotKey(keyCode, modifiers, hk,
		                             GetApplicationEventTarget(), 0, &refs[id]);
	});
	return (int)status;
}

void sparkUnregisterHotkey(uint32_t id) {
	if (id >= SPARK_MAX_HOTKEYS) {
		return;
	}
	onMain(^{
		if (refs[id] != NULL) {
			UnregisterEventHotKey(refs[id]);
			refs[id] = NULL;
		}
	});
}
