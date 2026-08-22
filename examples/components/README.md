# Component API example

This example discovers `ompphp` by its 64-bit component UID and registers an invalidation handler. UIDs are strings so PHP can represent the complete unsigned 64-bit range on every supported platform.

`supports()` negotiates a native interface UID, ABI version, and minimum table size. Raw function pointers are never exposed to PHP.

Components expose scripting functions by registering CAPI callables. Use `callables()` for discovery or `Component::call()` for direct invocation. Component-specific Composer packages can wrap those operations in a typed API, serving the same purpose as Pawn include packages.
