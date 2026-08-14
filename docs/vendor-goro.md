# Vendored Goro changes

ompphp vendors Goro because the component must build reproducibly and because
the pinned revision needs a small portability layer for Windows.

The local changes replace direct Unix `syscall` assumptions in the standard and
SPL extensions with build-tagged Unix and Windows implementations. Windows uses
Win32 file-time and disk-space APIs where equivalents exist. Unsupported Unix
operations such as syslog and process priority return the same kind of failure a
PHP caller would receive for an unavailable platform feature.

The complete delta is recorded in `third_party/goro-windows.patch`. After
changing the Goro version, refresh the vendor tree and reapply it from the
repository root:

```bash
go mod vendor
patch -p1 < third_party/goro-windows.patch
gofmt -w vendor/github.com/KarpelesLab/goro/ext/standard \
  vendor/github.com/KarpelesLab/goro/ext/spl
task check
task component
task component:windows
```

If the patch no longer applies, port each change to the new revision and
regenerate the patch against an untouched copy from the Go module cache. Do not
run `go mod vendor` without reapplying or updating this patch: that command
replaces the modified files.
