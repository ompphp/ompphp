package async

import (
	"context"
	"testing"
)

func TestRegister(t *testing.T) {
	name := t.Name()
	provider := func(context.Context, any) (any, error) { return int64(1), nil }
	if err := Register(name, provider); err != nil {
		t.Fatal(err)
	}
	if Providers()[name] == nil {
		t.Fatal("provider was not registered")
	}
	if err := Register(name, provider); err == nil {
		t.Fatal("duplicate provider was accepted")
	}
}
