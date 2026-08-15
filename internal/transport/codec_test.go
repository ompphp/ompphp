package transport

import (
	"context"
	"errors"
	"testing"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
)

func testContext(t *testing.T) *phpctx.Global {
	t.Helper()
	global := phpctx.NewGlobal(context.Background(), phpctx.NewProcess("test"), ini.New())
	t.Cleanup(func() { _ = global.Close() })
	return global
}

func TestRoundTrip(t *testing.T) {
	ctx := testContext(t)
	input := Map{
		{Key: Key{String: "name"}, Value: "café"},
		{Key: Key{String: "items"}, Value: Map{{Key: Key{Integer: 0, IsInt: true}, Value: int64(7)}, {Key: Key{Integer: 1, IsInt: true}, Value: true}}},
	}
	php, err := ToPHP(input, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	output, err := FromPHP(ctx, php, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(output.(Map)) != 2 {
		t.Fatalf("round trip = %#v", output)
	}
}

func TestGoMapKeysAreDeterministic(t *testing.T) {
	ctx := testContext(t)
	php, err := ToPHP(map[string]any{"z": int64(2), "a": int64(1)}, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	value, err := FromPHP(ctx, php, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	entries := value.(Map)
	if entries[0].Key.String != "a" || entries[1].Key.String != "z" {
		t.Fatalf("map order = %#v", entries)
	}
}

func TestLimitsAndUnsupportedValues(t *testing.T) {
	ctx := testContext(t)
	if _, err := FromPHP(ctx, phpv.ZString("large").ZVal(), Limits{MaxDepth: 2, MaxBytes: 2}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("size error = %v", err)
	}
	if _, err := ToPHP(make(chan int), DefaultLimits()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported error = %v", err)
	}
	nested := any(nil)
	for range 5 {
		nested = Map{{Key: Key{Integer: 0, IsInt: true}, Value: nested}}
	}
	if _, err := ToPHP(nested, Limits{MaxDepth: 2, MaxBytes: 1024}); !errors.Is(err, ErrTooDeep) {
		t.Fatalf("depth error = %v", err)
	}
}

func TestArrayCycleIsRejected(t *testing.T) {
	ctx := testContext(t)
	array := phpv.NewZArray()
	value := array.ZVal()
	_ = array.OffsetSet(ctx, nil, value.Ref())
	if _, err := FromPHP(ctx, array.ZVal(), DefaultLimits()); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestGoCycleIsRejected(t *testing.T) {
	value := map[string]any{}
	value["self"] = value
	if _, err := ToPHP(value, DefaultLimits()); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func FuzzToPHP(f *testing.F) {
	f.Add("value", int64(4), true)
	f.Fuzz(func(t *testing.T, text string, number int64, flag bool) {
		value := Map{{Key: Key{String: text}, Value: Map{{Key: Key{Integer: number, IsInt: true}, Value: flag}}}}
		_, _ = ToPHP(value, Limits{MaxDepth: 8, MaxBytes: 4096})
	})
}
