# Vendored Goro changes

ompphp vendors Goro for reproducible component builds. The pinned revision also needs the Windows portability changes in `third_party/goro-windows.patch`.

## Local changes

The patch:

- replaces direct Unix `syscall` assumptions with build-tagged Unix and Windows implementations in the standard and SPL extensions
- keeps Goro virtual paths separate from Windows drive-qualified paths, allowing relative includes to resolve from the server's working directory
- uses Win32 file-time and disk-space APIs where equivalents exist
- returns platform-feature failures to PHP for unsupported Unix operations such as syslog and process priority
- avoids mutating shared scalar values during dereferencing so isolated Goro runtimes can execute concurrently

## Update Goro

`go mod vendor` replaces the patched files. Reapply or update `third_party/goro-windows.patch` immediately after refreshing the vendor tree.

After changing the Goro version, run these commands from the repository root:

```sh
go mod vendor
patch -p1 < third_party/goro-windows.patch
gofmt -w vendor/github.com/KarpelesLab/goro/core/phpv \
  vendor/github.com/KarpelesLab/goro/core/stream \
  vendor/github.com/KarpelesLab/goro/ext/standard \
  vendor/github.com/KarpelesLab/goro/ext/spl
task check
task component
task component:windows
```

If the patch no longer applies, port each change to the new revision. Regenerate the patch against an untouched copy from the Go module cache.
