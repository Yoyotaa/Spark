#include <stddef.h>
#include <stdint.h>

// Streaming playback of 16-bit mono PCM. One stream at a time.
int sparkPlayerStart(double sampleRate, char *err, size_t errLen);
void sparkPlayerWrite(const int16_t *samples, size_t count);
// Blocks until everything written has been played (or Stop was called).
void sparkPlayerDrain(void);
void sparkPlayerStop(void);
