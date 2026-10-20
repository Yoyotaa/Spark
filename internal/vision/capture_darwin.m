#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ImageIO/ImageIO.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#include <unistd.h>
#include "capture_darwin.h"

// Wails compiles for macOS 10.13: guard every newer API to keep the build
// warning-free (the app itself requires 12.3, see Info.plist).
int sparkHasScreenAccess(void) {
	if (@available(macOS 10.15, *)) {
		return CGPreflightScreenCaptureAccess() ? 1 : 0;
	}
	return 1;
}

void sparkRequestScreenAccess(void) {
	if (@available(macOS 10.15, *)) {
		CGRequestScreenCaptureAccess();
	}
}

static NSData *encodeJPEG(CGImageRef image, double quality) {
	NSMutableData *out = [NSMutableData data];
	CGImageDestinationRef dst = CGImageDestinationCreateWithData(
		(__bridge CFMutableDataRef)out, CFSTR("public.jpeg"), 1, NULL);
	if (dst == NULL) {
		return nil;
	}
	NSDictionary *props = @{(__bridge NSString *)kCGImageDestinationLossyCompressionQuality : @(quality)};
	CGImageDestinationAddImage(dst, image, (__bridge CFDictionaryRef)props);
	BOOL ok = CGImageDestinationFinalize(dst);
	CFRelease(dst);
	return ok ? out : nil;
}

// Captures the main display without Spark's own windows, downscaled by
// ScreenCaptureKit (GPU) so that the longest side is at most maxSide.
// Blocks the calling (non-main) thread until done or timed out.
int sparkCaptureScreen(int maxSide, double quality, int showCursor, double timeoutSec, sparkFrame *out) {
	if (@available(macOS 14.0, *)) {
		// __block storage outlives this function if we time out: the
		// completion handlers keep it alive, so late writes are harmless.
		__block NSData *jpeg = nil;
		__block NSString *errMsg = nil;
		__block int width = 0, height = 0, denied = 0;
		dispatch_semaphore_t done = dispatch_semaphore_create(0);

		[SCShareableContent getShareableContentExcludingDesktopWindows:NO
		                                           onScreenWindowsOnly:YES
		                                             completionHandler:^(SCShareableContent *content, NSError *error) {
			if (error != nil || content.displays.count == 0) {
				errMsg = error.localizedDescription ?: @"no display found";
				denied = (error != nil && error.code == SCStreamErrorUserDeclined) ? 1 : 0;
				dispatch_semaphore_signal(done);
				return;
			}

			CGDirectDisplayID mainID = CGMainDisplayID();
			SCDisplay *display = content.displays.firstObject;
			for (SCDisplay *d in content.displays) {
				if (d.displayID == mainID) {
					display = d;
				}
			}

			NSMutableArray<SCRunningApplication *> *self_ = [NSMutableArray array];
			pid_t me = getpid();
			for (SCRunningApplication *app in content.applications) {
				if (app.processID == me) {
					[self_ addObject:app];
				}
			}

			SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display
			                                             excludingApplications:self_
			                                                  exceptingWindows:@[]];
			double scale = filter.pointPixelScale;
			double w = display.width * scale, h = display.height * scale;
			double k = MIN(1.0, (double)maxSide / MAX(w, h));

			SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
			cfg.width = (size_t)llround(w * k);
			cfg.height = (size_t)llround(h * k);
			cfg.showsCursor = showCursor ? YES : NO;

			[SCScreenshotManager captureImageWithFilter:filter
			                              configuration:cfg
			                          completionHandler:^(CGImageRef image, NSError *err) {
				if (err != nil || image == NULL) {
					errMsg = err.localizedDescription ?: @"capture failed";
					denied = (err != nil && err.code == SCStreamErrorUserDeclined) ? 1 : 0;
				} else {
					width = (int)CGImageGetWidth(image);
					height = (int)CGImageGetHeight(image);
					jpeg = encodeJPEG(image, quality);
					if (jpeg == nil) {
						errMsg = @"JPEG encoding failed";
					}
				}
				dispatch_semaphore_signal(done);
			}];
		}];

		long timedOut = dispatch_semaphore_wait(
			done, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(timeoutSec * NSEC_PER_SEC)));
		if (timedOut) {
			strlcpy(out->err, "capture timed out", sizeof out->err);
			return -1;
		}
		if (jpeg == nil) {
			out->denied = denied;
			strlcpy(out->err, (errMsg ?: @"unknown error").UTF8String, sizeof out->err);
			return -1;
		}
		out->len = jpeg.length;
		out->data = malloc(out->len);
		memcpy(out->data, jpeg.bytes, out->len);
		out->width = width;
		out->height = height;
		return 0;
	}
	strlcpy(out->err, "screen capture requires macOS 14 or later", sizeof out->err);
	return -1;
}
