#include <stdint.h>

int sparkKeyCodeForChar(uint16_t ch);
int sparkRegisterHotkey(uint32_t id, uint32_t keyCode, uint32_t modifiers);
void sparkUnregisterHotkeys(void);
