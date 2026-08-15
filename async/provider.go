// Package async lets Go extensions register work that ompphp runs outside the
// main PHP runtime. PHP arrays arrive as Map values. Providers may return null,
// booleans, numbers, strings, slices, string-keyed maps, or Map values.
package async

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Provider func(context.Context, any) (any, error)

// Map preserves PHP array keys and insertion order across the runtime boundary.
type Map []Entry

type Entry struct {
	Key   Key
	Value any
}

type Key struct {
	String  string
	Integer int64
	IsInt   bool
}

var registry = struct {
	sync.RWMutex
	providers map[string]Provider
}{providers: make(map[string]Provider)}

func Register(name string, provider Provider) error {
	if name == "" || provider == nil {
		return errors.New("async provider needs a name and function")
	}
	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.providers[name]; exists {
		return fmt.Errorf("async provider %q is already registered", name)
	}
	registry.providers[name] = provider
	return nil
}

func Providers() map[string]Provider {
	registry.RLock()
	defer registry.RUnlock()
	result := make(map[string]Provider, len(registry.providers))
	for name, provider := range registry.providers {
		result[name] = provider
	}
	return result
}
