//go:build windows

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestComposerStyleRequireUsesWindowsAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	composerDir := filepath.Join(root, "vendor", "composer")
	sdkDir := filepath.Join(root, "vendor", "ompphp", "sdk")
	if err := os.MkdirAll(composerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sdkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdkDir, "functions.php"), []byte("<?php $GLOBALS['composerLoaded'] = true;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(composerDir, "autoload_real.php"), []byte("<?php require __DIR__ . '/..' . '/ompphp/sdk/functions.php';"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(root, "gamemode.php")
	source := `<?php
namespace Omp\Internal;
require __DIR__ . '/vendor/composer/autoload_real.php';
function dispatch($event, $arguments, $default) { return !empty($GLOBALS['composerLoaded']); }
`
	if err := os.WriteFile(entry, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	if err := r.Load(entry); err != nil {
		t.Fatal(err)
	}
	if !r.Dispatch("Loaded") {
		t.Fatal("Composer-style require did not load the SDK file")
	}
}
