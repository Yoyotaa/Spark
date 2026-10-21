#include <stdint.h>

int sparkKeyCodeForChar(uint16_t ch);
int sparkCharForKeyCode(uint16_t code);
int sparkRegisterHotkey(uint32_t id, uint32_t keyCode, uint32_t modifiers);
void sparkUnregisterHotkey(uint32_t id);
void sparkRunMainLoop(void);
