package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ompphp/ompphp/tools/codegen/model"
)

func TestDirectNativeCategories(t *testing.T) {
	fromID := map[string]bool{"Player": true}
	tests := []struct {
		function model.Function
		want     bool
	}{
		{model.Function{Group: "Core", Name: "Core_MaxPlayers", ReturnType: "int"}, true},
		{model.Function{Group: "Player", Name: "Player_SetHealth", ReturnType: "bool", Parameters: []model.Parameter{{Type: "void*"}, {Type: "float"}}}, true},
		{model.Function{Group: "Player", Name: "Player_GetPos", ReturnType: "bool", Parameters: []model.Parameter{{Type: "void*"}, {Type: "float*", Output: true}}}, true},
		{model.Function{Group: "Vehicle", Name: "Vehicle_Link", ReturnType: "bool", Parameters: []model.Parameter{{Type: "void*"}, {Type: "void*"}}}, false},
		{model.Function{Group: "Event", Name: "Event_RemoveAllHandlers", ReturnType: "void"}, false},
	}
	for _, test := range tests {
		_, got := directNative(test.function, fromID)
		if got != test.want {
			t.Errorf("%s: got %v", test.function.Name, got)
		}
	}
}

func TestGeneratedNativeIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	header := filepath.Join(dir, "native.h")
	source := filepath.Join(dir, "native.go")
	m := model.Model{Functions: []model.Function{{Group: "Core", Name: "Core_MaxPlayers", ReturnType: "int"}}}
	count, err := generateNative(header, source, m)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	first, _ := os.ReadFile(source)
	if _, err := generateNative(header, source, m); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(source)
	if string(first) != string(second) {
		t.Fatal("generation is not deterministic")
	}
	if !strings.Contains(string(first), `case "Core_MaxPlayers"`) {
		t.Fatalf("missing dispatch: %s", first)
	}
}
