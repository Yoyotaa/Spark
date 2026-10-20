#import <AVFoundation/AVFoundation.h>
#include "player_darwin.h"

// AVAudioEngine + AVAudioPlayerNode: buffers are scheduled as they arrive,
// so speech starts with the first chunk of the network stream.

static AVAudioEngine *engine = nil;
static AVAudioPlayerNode *player = nil;
static AVAudioFormat *format = nil;
static dispatch_group_t pending = nil;
static NSLock *lock = nil;

int sparkPlayerStart(double sampleRate, char *err, size_t errLen) {
	static dispatch_once_t once;
	dispatch_once(&once, ^{ lock = [NSLock new]; });
	[lock lock];
	[player stop];
	[engine stop];
	engine = [AVAudioEngine new];
	player = [AVAudioPlayerNode new];
	format = [[AVAudioFormat alloc] initWithCommonFormat:AVAudioPCMFormatFloat32
	                                          sampleRate:sampleRate
	                                            channels:1
	                                         interleaved:NO];
	[engine attachNode:player];
	[engine connect:player to:engine.mainMixerNode format:format];
	NSError *e = nil;
	if (![engine startAndReturnError:&e]) {
		strlcpy(err, (e.localizedDescription ?: @"audio output unavailable").UTF8String, errLen);
		engine = nil;
		player = nil;
		[lock unlock];
		return -1;
	}
	pending = dispatch_group_create();
	[player play];
	[lock unlock];
	return 0;
}

void sparkPlayerWrite(const int16_t *samples, size_t count) {
	[lock lock];
	if (player == nil || count == 0) {
		[lock unlock];
		return;
	}
	AVAudioPCMBuffer *buf = [[AVAudioPCMBuffer alloc] initWithPCMFormat:format frameCapacity:(AVAudioFrameCount)count];
	buf.frameLength = (AVAudioFrameCount)count;
	float *out = buf.floatChannelData[0];
	for (size_t i = 0; i < count; i++) {
		out[i] = samples[i] / 32768.0f;
	}
	dispatch_group_t group = pending;
	dispatch_group_enter(group);
	[player scheduleBuffer:buf completionCallbackType:AVAudioPlayerNodeCompletionDataPlayedBack
	     completionHandler:^(AVAudioPlayerNodeCompletionCallbackType type) {
		dispatch_group_leave(group);
	}];
	[lock unlock];
}

void sparkPlayerDrain(void) {
	[lock lock];
	dispatch_group_t group = pending;
	[lock unlock];
	if (group != nil) {
		dispatch_group_wait(group, DISPATCH_TIME_FOREVER);
	}
}

void sparkPlayerStop(void) {
	[lock lock];
	// Stopping the node fires the completion handlers of every scheduled
	// buffer, which releases Drain.
	[player stop];
	[engine stop];
	player = nil;
	engine = nil;
	[lock unlock];
}
