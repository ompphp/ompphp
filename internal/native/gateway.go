package native

import (
	"errors"
	"sync"
)

var ErrUnavailable = errors.New("open.mp native API is not available")

// Gateway is the narrow boundary implemented by the generated C API bridge.
// Arguments and results may contain nil, bool, int, int32, int64, float32,
// float64, string, and recursively nested []any values.
type Gateway interface {
	Call(name string, arguments []any) (any, error)
}

type Func func(name string, arguments []any) (any, error)

func (f Func) Call(name string, arguments []any) (any, error) { return f(name, arguments) }

type Registry struct {
	mu      sync.RWMutex
	gateway Gateway
}

func (r *Registry) Set(g Gateway) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gateway = g
}

func (r *Registry) Call(name string, arguments []any) (any, error) {
	r.mu.RLock()
	g := r.gateway
	r.mu.RUnlock()
	if g == nil {
		return nil, ErrUnavailable
	}
	return g.Call(name, arguments)
}
