#ifndef OMPPHP_WINDOWS_COMPAT_H
#define OMPPHP_WINDOWS_COMPAT_H

// ompcapi.h includes <Windows.h>. On a case-sensitive cross-build host this
// shim fixes that spelling, but on Windows <windows.h> would resolve straight
// back to this file. Skip the current include directory and load MinGW's real
// system header instead.
#include_next <windows.h>

#endif
