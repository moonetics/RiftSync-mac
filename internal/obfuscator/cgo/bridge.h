#ifndef LUAU_BRIDGE_H
#define LUAU_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

char* LuauParseToJSON(const char* source);
void LuauFreeString(char* str);

#ifdef __cplusplus
}
#endif

#endif // LUAU_BRIDGE_H
