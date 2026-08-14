package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndGenerateConstants(t *testing.T) {
	manifest, err := loadConstants("data/gamemode_constants.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Groups) != 41 {
		t.Fatalf("got %d constant groups, want 41", len(manifest.Groups))
	}
	outDir := t.TempDir()
	if err := generateConstants(outDir, manifest); err != nil {
		t.Fatal(err)
	}
	tests := map[string][]string{
		"WeaponID.php": {"final class WeaponID", "public const M4 = 31;"},
		"Keys.php":     {"final class Keys", "public const FIRE = 4;", "public const AIM = 128;"},
	}
	for filename, fragments := range tests {
		data, err := os.ReadFile(filepath.Join(outDir, filename))
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(data), fragment) {
				t.Fatalf("%s does not contain %q:\n%s", filename, fragment, data)
			}
		}
	}
}

func TestLoadConstantsRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "constants.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"groups":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConstants(path); err == nil || !strings.Contains(err.Error(), "version 99") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommonConstantPrefix(t *testing.T) {
	items := []constantItem{{Name: "KeyFire"}, {Name: "KeyAim"}}
	if got := commonConstantPrefix(items); got != "KEY" {
		t.Fatalf("got %q, want KEY", got)
	}
}
