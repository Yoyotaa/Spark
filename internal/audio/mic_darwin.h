#include <stddef.h>

// Permissions: 0 undetermined, 1 denied/restricted, 2 granted.
int sparkMicPermission(void);
int sparkSpeechPermission(void);
// Asynchronous: the answer arrives through sparkPermissionResult(kind, granted),
// kind 0 = microphone, 1 = speech recognition.
void sparkRequestMicPermission(void);
void sparkRequestSpeechPermission(void);

// Starts/stops the audio engine. While running, sparkMicLevel(rms) is called
// ~20 times per second.
int sparkMicStart(char *err, size_t errLen);
void sparkMicStop(void);

// Recognition of one utterance, fed by the running engine. Results arrive
// through sparkSpeechResult(text, final). Errors through sparkSpeechError(msg).
int sparkSpeechBegin(const char *locale, char *err, size_t errLen);
void sparkSpeechEnd(void);
void sparkSpeechCancel(void);

void sparkPlayCue(int start);
