package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ompphp/ompphp/tools/codegen/model"
)

func TestGeneratePHPIsDeterministic(t *testing.T) {
	m := model.Model{Functions: []model.Function{{Name: "Player_SetHealth", Parameters: []model.Parameter{{Name: "player", Type: "void*"}, {Name: "health", Type: "float"}}}}}
	path := filepath.Join(t.TempDir(), "api.php")
	if err := generatePHP(path, m); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := generatePHP(path, m); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("generation is not deterministic")
	}
	if !strings.Contains(string(first), "function player_set_health(int $player, float $health): mixed") {
		t.Fatalf("unexpected output:\n%s", first)
	}
}

func TestSnakeInitialism(t *testing.T) {
	if got := snake("Core_GetHTTPStatus"); got != "core_get_http_status" {
		t.Fatalf("got %q", got)
	}
}

func TestLowerCamel(t *testing.T) {
	if got := lowerCamel("GameMode_SetText"); got != "gameModeSetText" {
		t.Fatalf("got %q", got)
	}
	if got := lowerCamel("GetHTTPStatus"); got != "getHttpStatus" {
		t.Fatalf("got %q", got)
	}
}

func TestGenerateEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Events.php")
	m := model.Model{Events: []model.Event{{Name: "onPlayerConnect"}}}
	if err := generateEvents(path, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "PLAYER_CONNECT = 'PlayerConnect'") {
		t.Fatalf("unexpected output:\n%s", data)
	}
}

func TestGenerateEventHandlers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Handlers.php")
	m := model.Model{Events: []model.Event{{
		Name:       "onPlayerConnect",
		Parameters: []model.Parameter{{Name: "player", Type: "void*"}},
	}}}
	if err := generateEventHandlers(path, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"@param callable(int): mixed $handler",
		"function playerConnect(callable $handler): void",
		"Server::on(Events::PLAYER_CONNECT, $handler)",
	} {
		if !strings.Contains(string(data), fragment) {
			t.Fatalf("generated handlers do not contain %q:\n%s", fragment, data)
		}
	}
}

func TestGeneratePHPExcludesOutputParameters(t *testing.T) {
	m := model.Model{Functions: []model.Function{{Name: "Player_GetPos", Parameters: []model.Parameter{
		{Name: "player", Type: "void*"}, {Name: "x", Type: "float*", Output: true}, {Name: "y", Type: "float*", Output: true}, {Name: "z", Type: "float*", Output: true},
	}}}}
	path := filepath.Join(t.TempDir(), "api.php")
	if err := generatePHP(path, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "function player_get_pos(int $player): mixed") {
		t.Fatalf("unexpected output:\n%s", data)
	}
}

func TestGeneratePublicAPI(t *testing.T) {
	m := model.Model{Functions: []model.Function{
		{Group: "Player", Name: "Player_SetHealth", ReturnType: "bool", Parameters: []model.Parameter{{Name: "player", Type: "void*"}, {Name: "health", Type: "float"}}},
		{Group: "Object", Name: "Object_GetPos", ReturnType: "bool", Parameters: []model.Parameter{{Name: "object", Type: "void*"}, {Name: "x", Type: "float*", Output: true}, {Name: "y", Type: "float*", Output: true}, {Name: "z", Type: "float*", Output: true}}},
	}}
	outDir := t.TempDir()
	count, err := generatePublicAPI(outDir, m)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("generated %d methods, want 2", count)
	}
	player, err := os.ReadFile(filepath.Join(outDir, "Player.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(player), "public static function setHealth(int $player, float $health): bool") || !strings.Contains(string(player), "return (bool) \\Omp\\Internal\\player_set_health($player, $health);") {
		t.Fatalf("unexpected Player API:\n%s", player)
	}
	object, err := os.ReadFile(filepath.Join(outDir, "GameObject.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(object), "@return array{float, float, float}") || !strings.Contains(string(object), "public static function getPos(int $object): array") {
		t.Fatalf("unexpected Object API:\n%s", object)
	}
}
