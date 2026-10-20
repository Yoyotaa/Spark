#import <AVFoundation/AVFoundation.h>
#import <AppKit/AppKit.h>
#import <Speech/Speech.h>
#include <math.h>
#include "mic_darwin.h"
#include "_cgo_export.h"

// One engine for the app. The tap computes the input level and, while an
// utterance is being recognized, feeds the recognition request. Everything
// below is serialized on `queue`, except the tap (audio thread), which only
// touches `request` under a lock.

static AVAudioEngine *engine = nil;
static SFSpeechRecognizer *recognizer = nil;
static SFSpeechAudioBufferRecognitionRequest *request = nil;
static SFSpeechRecognitionTask *task = nil;
static NSLock *requestLock = nil;
// Last ~0.5 s of audio, replayed into a new request: voice detection fires
// slightly after speech starts and would otherwise clip the wake word.
static NSMutableArray<AVAudioPCMBuffer *> *preroll = nil;
static const NSUInteger prerollMax = 24; // × 1024 frames ≈ 0.5 s at 48 kHz
static dispatch_queue_t queue = nil;
static CFAbsoluteTime lastLevel = 0;

static void setup(void) {
	static dispatch_once_t once;
	dispatch_once(&once, ^{
		queue = dispatch_queue_create("spark.mic", DISPATCH_QUEUE_SERIAL);
		requestLock = [NSLock new];
		preroll = [NSMutableArray array];
	});
}

static void copyErr(NSString *msg, char *err, size_t errLen) {
	if (err != NULL && errLen > 0) {
		strlcpy(err, (msg ?: @"unknown error").UTF8String, errLen);
	}
}

int sparkMicPermission(void) {
	switch ([AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio]) {
	case AVAuthorizationStatusAuthorized: return 2;
	case AVAuthorizationStatusNotDetermined: return 0;
	default: return 1;
	}
}

int sparkSpeechPermission(void) {
	switch ([SFSpeechRecognizer authorizationStatus]) {
	case SFSpeechRecognizerAuthorizationStatusAuthorized: return 2;
	case SFSpeechRecognizerAuthorizationStatusNotDetermined: return 0;
	default: return 1;
	}
}

void sparkRequestMicPermission(void) {
	[AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {
		sparkPermissionResult(0, granted ? 1 : 0);
	}];
}

void sparkRequestSpeechPermission(void) {
	[SFSpeechRecognizer requestAuthorization:^(SFSpeechRecognizerAuthorizationStatus status) {
		sparkPermissionResult(1, status == SFSpeechRecognizerAuthorizationStatusAuthorized ? 1 : 0);
	}];
}

int sparkMicStart(char *err, size_t errLen) {
	setup();
	__block int rc = 0;
	dispatch_sync(queue, ^{
		if (engine != nil && engine.isRunning) {
			return;
		}
		engine = [AVAudioEngine new];
		AVAudioInputNode *input = engine.inputNode;
		AVAudioFormat *format = [input outputFormatForBus:0];
		if (format.sampleRate == 0) {
			copyErr(@"aucun micro disponible", err, errLen);
			engine = nil;
			rc = -1;
			return;
		}
		[input installTapOnBus:0 bufferSize:1024 format:format block:^(AVAudioPCMBuffer *buffer, AVAudioTime *when) {
			[requestLock lock];
			if (request != nil) {
				[request appendAudioPCMBuffer:buffer];
			} else {
				// The tap's buffer is reused by the engine: keep a copy.
				AVAudioPCMBuffer *copy = [[AVAudioPCMBuffer alloc] initWithPCMFormat:buffer.format frameCapacity:buffer.frameLength];
				copy.frameLength = buffer.frameLength;
				for (AVAudioChannelCount ch = 0; ch < buffer.format.channelCount && buffer.floatChannelData; ch++) {
					memcpy(copy.floatChannelData[ch], buffer.floatChannelData[ch], buffer.frameLength * sizeof(float));
				}
				[preroll addObject:copy];
				if (preroll.count > prerollMax) {
					[preroll removeObjectAtIndex:0];
				}
			}
			[requestLock unlock];

			// RMS of the first channel, reported at ~20 Hz.
			CFAbsoluteTime now = CFAbsoluteTimeGetCurrent();
			if (now - lastLevel < 0.05 || buffer.floatChannelData == NULL) {
				return;
			}
			lastLevel = now;
			const float *samples = buffer.floatChannelData[0];
			AVAudioFrameCount n = buffer.frameLength;
			double sum = 0;
			for (AVAudioFrameCount i = 0; i < n; i++) {
				sum += samples[i] * samples[i];
			}
			sparkMicLevel(n > 0 ? (float)sqrt(sum / n) : 0);
		}];
		[engine prepare];
		NSError *e = nil;
		if (![engine startAndReturnError:&e]) {
			copyErr(e.localizedDescription, err, errLen);
			[input removeTapOnBus:0];
			engine = nil;
			rc = -1;
		}
	});
	return rc;
}

void sparkMicStop(void) {
	setup();
	dispatch_sync(queue, ^{
		if (engine == nil) {
			return;
		}
		[engine.inputNode removeTapOnBus:0];
		[engine stop];
		engine = nil;
		[requestLock lock];
		[preroll removeAllObjects];
		[requestLock unlock];
	});
}

int sparkSpeechBegin(const char *locale, char *err, size_t errLen) {
	setup();
	NSString *loc = [NSString stringWithUTF8String:locale];
	__block int rc = 0;
	dispatch_sync(queue, ^{
		[task cancel];
		task = nil;

		if (recognizer == nil || ![recognizer.locale.localeIdentifier isEqualToString:loc]) {
			recognizer = [[SFSpeechRecognizer alloc] initWithLocale:[NSLocale localeWithLocaleIdentifier:loc]];
		}
		if (recognizer == nil || !recognizer.isAvailable) {
			copyErr(@"reconnaissance vocale indisponible pour cette langue", err, errLen);
			rc = -1;
			return;
		}

		SFSpeechAudioBufferRecognitionRequest *req = [SFSpeechAudioBufferRecognitionRequest new];
		req.shouldReportPartialResults = YES;
		// Private and fast when the model is installed (most languages on
		// Apple silicon); otherwise Apple's servers are used.
		if (recognizer.supportsOnDeviceRecognition) {
			req.requiresOnDeviceRecognition = YES;
		}
		if (@available(macOS 13.0, *)) {
			req.addsPunctuation = YES;
		}

		task = [recognizer recognitionTaskWithRequest:req resultHandler:^(SFSpeechRecognitionResult *result, NSError *error) {
			if (result != nil) {
				sparkSpeechResult((char *)result.bestTranscription.formattedString.UTF8String, result.isFinal ? 1 : 0);
			}
			if (error != nil && result == nil) {
				// 216: cancelled; 1110: no speech detected. Both end quietly.
				NSInteger code = error.code;
				if (code == 216 || code == 1110 || code == 301) {
					sparkSpeechResult("", 1);
				} else {
					sparkSpeechError((char *)error.localizedDescription.UTF8String);
				}
			}
		}];

		[requestLock lock];
		for (AVAudioPCMBuffer *b in preroll) {
			[req appendAudioPCMBuffer:b];
		}
		[preroll removeAllObjects];
		request = req;
		[requestLock unlock];
	});
	return rc;
}

void sparkSpeechEnd(void) {
	setup();
	dispatch_sync(queue, ^{
		[requestLock lock];
		[request endAudio];
		request = nil;
		[requestLock unlock];
	});
}

void sparkSpeechCancel(void) {
	setup();
	dispatch_sync(queue, ^{
		[requestLock lock];
		request = nil;
		[requestLock unlock];
		[task cancel];
		task = nil;
	});
}

void sparkPlayCue(int start) {
	NSSound *s = [NSSound soundNamed:(start ? @"Tink" : @"Pop")];
	s.volume = 0.35;
	[s play];
}
