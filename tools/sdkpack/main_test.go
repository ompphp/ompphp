package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageSDK(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(workingDirectory, "..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })
	outDir := t.TempDir()
	if err := packageSDK("1.2.3", outDir); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(filepath.Join(outDir, "ompphp-sdk_1.2.3.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	foundAPI := false
	for _, file := range archive.File {
		switch file.Name {
		case "src/Api/Player.php":
			foundAPI = true
		case "composer.json":
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			var metadata map[string]any
			if err := json.NewDecoder(reader).Decode(&metadata); err != nil {
				_ = reader.Close()
				t.Fatal(err)
			}
			_ = reader.Close()
			if metadata["version"] != "1.2.3" {
				t.Fatalf("package version = %#v", metadata["version"])
			}
			if _, exists := metadata["scripts"]; exists {
				t.Fatal("release package contains development scripts")
			}
		}
	}
	if !foundAPI {
		t.Fatal("public API missing from SDK archive")
	}
	if _, err := os.Stat(filepath.Join(outDir, "ompphp-sdk_1.2.3.zip")); err != nil {
		t.Fatal(err)
	}
}
