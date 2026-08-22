package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNormalizesAndSorts(t *testing.T) {
	d := t.TempDir()
	api := filepath.Join(d, "api.json")
	events := filepath.Join(d, "events.json")
	if err := os.WriteFile(api, []byte(`{"Player":[{"name":"Zed","ret":"bool"},{"name":"Alpha","ret":"int","params":[{"name":"id","type":"int"}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(events, []byte(`{"Player":[{"name":"OnConnect","badret":"false","args":[]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := load(api, events)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Functions) != 2 || m.Functions[0].Name != "Alpha" {
		t.Fatalf("unexpected model: %#v", m)
	}
	if len(m.Events) != 1 || m.Events[0].Name != "OnConnect" {
		t.Fatalf("unexpected events: %#v", m.Events)
	}
}

func TestParametersClassifyCOutputPointers(t *testing.T) {
	got := parameters([]rawParameter{{Name: "player", Type: "void*"}, {Name: "x", Type: "float*"}, {Name: "text", Type: "const char*"}})
	if got[0].Output || !got[1].Output || got[2].Output {
		t.Fatalf("unexpected output classification: %#v", got)
	}
}

func TestLoadOmitsLowLevelExtensionAPIs(t *testing.T) {
	d := t.TempDir()
	api := filepath.Join(d, "api.json")
	events := filepath.Join(d, "events.json")
	data := `{
		"Core":[{"name":"Core_Log","ret":"bool"}],
		"ComponentInterop":[{"name":"Component_Find","ret":"struct OMPComponentHandle*"}],
		"Network":[{"name":"Network_Subscribe","ret":"struct OMPNetSubscription*"}]
	}`
	if err := os.WriteFile(api, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(events, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := load(api, events)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Functions) != 1 || m.Functions[0].Name != "Core_Log" {
		t.Fatalf("unexpected functions: %#v", m.Functions)
	}
}
