package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDKArchivePath(t *testing.T) {
	path, err := sdkArchivePath(filepath.Join("sdk", "src", "Internal", "functions.php"))
	if err != nil {
		t.Fatal(err)
	}
	if path != "src/Internal/functions.php" {
		t.Fatalf("archive path = %q", path)
	}
}

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
	required := map[string]bool{
		"src/Api/Player.php":             false,
		"src/Internal/api_generated.php": false,
		"src/Internal/functions.php":     false,
	}
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "sdk/") {
			t.Fatalf("SDK archive contains an unexpected sdk/ prefix: %q", file.Name)
		}
		if _, exists := required[file.Name]; exists {
			required[file.Name] = true
		}
		switch file.Name {
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
	for name, found := range required {
		if !found {
			t.Errorf("%s missing from SDK archive", name)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "ompphp-sdk_1.2.3.zip")); err != nil {
		t.Fatal(err)
	}
}
