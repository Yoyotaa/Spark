#import <Carbon/Carbon.h>
#import <Foundation/Foundation.h>
#include "hotkey_darwin.h"
#include "_cgo_export.h"

#define SPARK_MAX_HOTKEYS 32

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

void sparkUnregisterHotkeys(void) {
	// Async: called during shutdown, the main thread must never be waited on.
	dispatch_async(dispatch_get_main_queue(), ^{
		for (int i = 0; i < SPARK_MAX_HOTKEYS; i++) {
			if (refs[i] != NULL) {
				UnregisterEventHotKey(refs[i]);
				refs[i] = NULL;
			}
		}
	});
}
