package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ompphp/ompphp/tools/codegen/model"
)

func TestDirectEventCategories(t *testing.T) {
	for _, test := range []struct {
		event model.Event
		want  bool
	}{
		{model.Event{Name: "onTick", Parameters: []model.Parameter{{Name: "elapsed", Type: "int"}}}, true},
		{model.Event{Name: "onPlayerText", Parameters: []model.Parameter{{Name: "player", Type: "void*"}, {Name: "text", Type: "CAPIStringView"}}}, true},
		{model.Event{Name: "onVehicleSpawn", Parameters: []model.Parameter{{Name: "vehicle", Type: "void*"}}}, true},
		{model.Event{Name: "onPlayerDeath", Parameters: []model.Parameter{{Name: "player", Type: "void*"}, {Name: "killer", Type: "void*"}}}, true},
		{model.Event{Name: "onNPCShotPlayerObject", Parameters: []model.Parameter{{Name: "npc", Type: "void*"}, {Name: "playerObject", Type: "void*"}}}, true},
	} {
		if got := directEvent(test.event); got != test.want {
			t.Errorf("%s: got %v", test.event.Name, got)
		}
	}
}

func TestNPCShotPlayerObjectUsesNPCPlayerHandle(t *testing.T) {
	dir := t.TempDir()
	header := filepath.Join(dir, "events.h")
	source := filepath.Join(dir, "events.go")
	event := model.Event{Name: "onNPCShotPlayerObject", ReturnType: "false", Parameters: []model.Parameter{
		{Name: "npc", Type: "void*"},
		{Name: "playerObject", Type: "void*"},
		{Name: "weapon", Type: "int"},
	}}
	count, err := generateEventBridge(header, source, model.Model{Events: []model.Event{event}})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	generated, err := os.ReadFile(header)
	if err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	if !strings.Contains(text, "ompphp_api.NPC.GetPlayer(*event->list[0].npc)") {
		t.Fatalf("NPC player conversion missing:\n%s", text)
	}
	if !strings.Contains(text, "ompphp_api.PlayerObject.GetID(npcPlayer1, *event->list[0].playerObject)") {
		t.Fatalf("player-object ID conversion missing:\n%s", text)
	}
}

func TestEventEntityMapping(t *testing.T) {
	if got := eventEntity(model.Event{Name: "onVehicleSpawn"}, model.Parameter{Name: "vehicle"}, 0); got != "Vehicle" {
		t.Fatalf("got %q", got)
	}
	if got := eventEntity(model.Event{Name: "onPlayerObjectMove"}, model.Parameter{Name: "object"}, 1); got != "" {
		t.Fatalf("ambiguous player object mapped as %q", got)
	}
	event := model.Event{Name: "onPlayerObjectMove", Parameters: []model.Parameter{{Name: "player"}, {Name: "object"}}}
	if got := eventEntity(event, model.Parameter{Name: "object"}, 1); got != "PlayerObject" {
		t.Fatalf("got %q", got)
	}
}
