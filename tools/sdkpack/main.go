package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	version := flag.String("version", "0.1.0-dev", "SDK package version")
	outDir := flag.String("out", "build", "output directory")
	flag.Parse()
	if err := packageSDK(*version, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func packageSDK(version, outDir string) error {
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("SDK version must not be empty")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(outDir, "ompphp-sdk_"+version+".zip")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(file)
	failed := func(err error) error {
		_ = archive.Close()
		_ = file.Close()
		return err
	}

	composer, err := os.ReadFile("sdk/composer.json")
	if err != nil {
		return failed(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(composer, &metadata); err != nil {
		return failed(err)
	}
	metadata["version"] = version
	delete(metadata, "scripts")
	composer, err = json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return failed(err)
	}
	composer = append(composer, '\n')
	if err := writeArchiveFile(archive, "composer.json", composer); err != nil {
		return failed(err)
	}

	for _, source := range []string{"sdk/README.md", "LICENSE"} {
		data, err := os.ReadFile(source)
		if err != nil {
			return failed(err)
		}
		if err := writeArchiveFile(archive, filepath.Base(source), data); err != nil {
			return failed(err)
		}
	}

	var sources []string
	if err := filepath.WalkDir("sdk/src", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			sources = append(sources, path)
		}
		return nil
	}); err != nil {
		return failed(err)
	}
	sort.Strings(sources)
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			return failed(err)
		}
		name, err := sdkArchivePath(source)
		if err != nil {
			return failed(err)
		}
		if err := writeArchiveFile(archive, name, data); err != nil {
			return failed(err)
		}
	}
	if err := archive.Close(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

func sdkArchivePath(source string) (string, error) {
	relative, err := filepath.Rel("sdk", source)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}

func writeArchiveFile(archive *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o644)
	header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}
