#import <Cocoa/Cocoa.h>
#include "tray_darwin.h"
#include "_cgo_export.h"

@interface SparkTrayTarget : NSObject
- (void)fire:(NSMenuItem *)item;
@end

@implementation SparkTrayTarget
- (void)fire:(NSMenuItem *)item {
	sparkTrayFired((int)item.tag);
}
@end

static NSStatusItem *statusItem = nil;
static SparkTrayTarget *target = nil;

void sparkTrayStart(const char *tooltip, const char **titles, int count) {
	// Copy now: the C strings are freed by Go as soon as we return.
	NSString *tip = [NSString stringWithUTF8String:tooltip];
	NSMutableArray<NSString *> *labels = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		[labels addObject:[NSString stringWithUTF8String:titles[i]]];
	}
	dispatch_async(dispatch_get_main_queue(), ^{
		if (statusItem != nil) {
			return;
		}
		target = [SparkTrayTarget new];
		statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
		NSImage *icon = nil;
		if (@available(macOS 11.0, *)) {
			icon = [NSImage imageWithSystemSymbolName:@"sparkles" accessibilityDescription:tip];
		}
		if (icon != nil) {
			icon.template = YES; // follows the light/dark menu bar
			statusItem.button.image = icon;
		} else {
			statusItem.button.title = @"✦";
		}
		statusItem.button.toolTip = tip;

		NSMenu *menu = [NSMenu new];
		menu.autoenablesItems = NO;
		for (NSUInteger i = 0; i < labels.count; i++) {
			if (labels[i].length == 0) {
				[menu addItem:[NSMenuItem separatorItem]];
				continue;
			}
			NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:labels[i] action:@selector(fire:) keyEquivalent:@""];
			item.target = target;
			item.tag = (NSInteger)i;
			[menu addItem:item];
		}
		statusItem.menu = menu;
	});
}
