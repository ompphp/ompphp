package runtime

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/KarpelesLab/goro/core/phpctx"
)

var errRuntimeClosed = errors.New("PHP runtime is shut down")

// mainExecutor is the only entry point into the authoritative PHP Global.
// Worker runtimes never use it and never receive a pointer to the main Global.
type mainExecutor struct {
	mu        sync.Mutex
	global    *phpctx.Global
	closed    bool
	active    atomic.Int32
	maxActive atomic.Int32
}

func newMainExecutor(global *phpctx.Global) *mainExecutor {
	return &mainExecutor{global: global}
}

func (e *mainExecutor) run(fn func(*phpctx.Global) error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errRuntimeClosed
	}
	active := e.active.Add(1)
	defer e.active.Add(-1)
	e.global.ResetDeadline()
	for current := e.maxActive.Load(); active > current && !e.maxActive.CompareAndSwap(current, active); current = e.maxActive.Load() {
	}
	return fn(e.global)
}

func (e *mainExecutor) close(fn func(*phpctx.Global)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	fn(e.global)
	e.closed = true
}

func (e *mainExecutor) maxConcurrent() int32 { return e.maxActive.Load() }
