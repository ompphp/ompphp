package main

import (
	"strings"
	"testing"
)

func TestGatewayRejectsUnknownNative(t *testing.T) {
	_, err := (capiGateway{}).Call("Player_NotReal", nil)
	if err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGatewayValidatesArgumentsBeforeCAPI(t *testing.T) {
	tests := []struct {
		name string
		args []any
	}{
		{"Player_SetHealth", []any{int64(1)}},
		{"Player_SetArmor", []any{"one", float64(50)}},
		{"Player_SetScore", []any{int64(1), float64(5)}},
		{"Player_SendClientMessage", []any{int64(1), int64(-1)}},
		{"Player_SetPos", []any{int64(1), float64(1), "two", float64(3)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (capiGateway{}).Call(test.name, test.args); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestIntegerConversions(t *testing.T) {
	if value, ok := asInt(int64(42)); !ok || value != 42 {
		t.Fatalf("got %d, %v", value, ok)
	}
	if _, ok := asInt(float64(42)); ok {
		t.Fatal("float unexpectedly accepted as integer")
	}
}

func TestComponentVersionParts(t *testing.T) {
	for part, want := range []int{1, 12, 3} {
		if got := componentVersionPart("v1.12.3-beta.1", part); got != want {
			t.Fatalf("part %d = %d, want %d", part, got, want)
		}
	}
	if got := componentVersionPart("dev", 0); got != 0 {
		t.Fatalf("invalid version part = %d", got)
	}
}

func TestGeneratedGatewayCoverageWithoutCAPI(t *testing.T) {
	result, handled, err := callGenerated("Core_MaxPlayers", nil)
	if err != nil || !handled || result != int64(0) {
		t.Fatalf("result=%#v handled=%v err=%v", result, handled, err)
	}
	_, handled, err = callGenerated("Actor_SetHealth", []any{int64(1), "high"})
	if !handled || err == nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	result, handled, err = callGenerated("Actor_SetHealth", []any{int64(1), float64(50)})
	if err != nil || !handled || result != false {
		t.Fatalf("result=%#v handled=%v err=%v", result, handled, err)
	}
	_, handled, err = callGenerated("Not_A_Function", nil)
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
}

func TestGeneratedArrayBounds(t *testing.T) {
	_, handled, err := callGenerated("NPC_GetAll", []any{int64(10001)})
	if !handled || err == nil || !strings.Contains(err.Error(), "between 0 and 10000") {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	result, handled, err := callGenerated("NPC_GetAll", []any{int64(0)})
	if err != nil || !handled || len(result.([]any)) != 0 {
		t.Fatalf("result=%#v handled=%v err=%v", result, handled, err)
	}
}
