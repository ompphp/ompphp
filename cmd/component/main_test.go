package main

import (
	"math"
	"strings"
	"testing"
)

func TestCallableIntegerConversion(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		unsigned bool
		bits     int
		want     uint64
		wantErr  bool
	}{
		{name: "signed", value: int64(-42), bits: 64, want: uint64(math.MaxUint64 - 41)},
		{name: "unsigned integer", value: int64(42), unsigned: true, bits: 64, want: 42},
		{name: "maximum unsigned string", value: "18446744073709551615", unsigned: true, bits: 64, want: math.MaxUint64},
		{name: "negative unsigned", value: int64(-1), unsigned: true, bits: 64, wantErr: true},
		{name: "int32 overflow", value: int64(math.MaxInt32) + 1, bits: 32, wantErr: true},
		{name: "invalid string", value: "not-a-number", unsigned: true, bits: 64, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := callableInteger(test.value, test.unsigned, test.bits)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr = %t", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("value = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCallableEntityConversion(t *testing.T) {
	kind, id, err := callableEntity([]any{int64(7), "18446744073709551615"})
	if err != nil {
		t.Fatal(err)
	}
	if kind != 7 || id != math.MaxUint64 {
		t.Fatalf("entity = (%d, %d)", kind, id)
	}
	if _, _, err := callableEntity([]any{int64(0), int64(1)}); err == nil {
		t.Fatal("zero entity type was accepted")
	}
}

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

func TestNetworkSubscriptionValidation(t *testing.T) {
	tests := []struct {
		direction, id int32
		all           bool
		valid         bool
	}{
		{0, 0, false, true}, {3, 255, false, true}, {0, -1, true, true},
		{-1, 0, false, false}, {4, 0, false, false}, {0, -1, false, false},
	}
	for _, test := range tests {
		if got := validateNetworkSubscription(test.direction, test.id, test.all); (got == nil) != test.valid {
			t.Fatalf("direction=%d id=%d all=%v error=%v", test.direction, test.id, test.all, got)
		}
	}
}

func TestNetworkPayloadValidation(t *testing.T) {
	if err := validateNetworkPayload("x", 9, 1024); err == nil {
		t.Fatal("accepted bit length beyond payload")
	}
	if err := validateNetworkPayload("xx", 16, 1); err == nil {
		t.Fatal("accepted payload beyond configured maximum")
	}
	if err := validateNetworkPayload("x", 7, 1); err != nil {
		t.Fatalf("valid payload: %v", err)
	}
}
