#include <stddef.h>

typedef struct {
	void *data; // malloc'd JPEG, freed by the caller
	size_t len;
	int width;
	int height;
	int denied; // 1 if Screen Recording permission is missing
	char err[256];
} sparkFrame;

int sparkCaptureScreen(int maxSide, double quality, double timeoutSec, sparkFrame *out);
int sparkHasScreenAccess(void);
void sparkRequestScreenAccess(void);
