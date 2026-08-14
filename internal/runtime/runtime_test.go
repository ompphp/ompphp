package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ompphp/ompphp/internal/native"
)

func script(t *testing.T, source string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "main.php")
	if err := os.WriteFile(name, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestPersistentStateAndReturn(t *testing.T) {
	r := New(context.Background(), native.Func(func(string, []any) (any, error) { return true, nil }), nil)
	t.Cleanup(r.Close)
	err := r.Load(script(t, `<?php namespace Omp\Internal; $GLOBALS['calls']=0; function dispatch($event,...$args){ $GLOBALS['calls']++; return $GLOBALS['calls'] < 2; }`))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Dispatch("PlayerConnect", 7) {
		t.Fatal("first dispatch should return true")
	}
	if r.Dispatch("PlayerConnect", 7) {
		t.Fatal("global state did not persist")
	}
}

func TestLoadFailuresAreReturned(t *testing.T) {
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	if err := r.Load(filepath.Join(t.TempDir(), "missing.php")); err == nil || !strings.Contains(err.Error(), "missing.php") {
		t.Fatalf("missing file error = %v", err)
	}
	if err := r.Load(script(t, `<?php function broken(`)); err == nil {
		t.Fatal("syntax error was accepted")
	}
}

func TestGamemodeCannotLoadTwice(t *testing.T) {
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; function dispatch($event,$args){ return true; }`)); err != nil {
		t.Fatal(err)
	}
	if err := r.Load("another.php"); err == nil || !strings.Contains(err.Error(), "already loaded") {
		t.Fatalf("second load error = %v", err)
	}
}

func TestDispatchIsSerialized(t *testing.T) {
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; function dispatch($event,...$args){ return true; }`)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !r.Dispatch("Tick") {
				t.Error("false")
			}
		}()
	}
	wg.Wait()
}

func TestLongLivedDispatch(t *testing.T) {
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; $GLOBALS['calls']=0; function dispatch($event,$args){ return ++$GLOBALS['calls'] <= 10000; }`)); err != nil {
		t.Fatal(err)
	}
	before := goruntime.NumGoroutine()
	for i := 0; i < 10000; i++ {
		if !r.Dispatch("Tick", i) {
			t.Fatalf("dispatch %d failed", i)
		}
	}
	if after := goruntime.NumGoroutine(); after > before+2 {
		t.Fatalf("goroutines grew from %d to %d", before, after)
	}
}

func TestLongLivedDispatchMemoryStabilizes(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; function dispatch($event,$args){ $value = new \stdClass(); $value->id = $args[0]; return $value->id >= 0; }`)); err != nil {
		t.Fatal(err)
	}
	for i := range 2000 {
		r.Dispatch("Tick", i)
	}
	debug.FreeOSMemory()
	var before goruntime.MemStats
	goruntime.ReadMemStats(&before)
	for i := range 50000 {
		if !r.Dispatch("Tick", i) {
			t.Fatalf("dispatch %d failed", i)
		}
	}
	debug.FreeOSMemory()
	var after goruntime.MemStats
	goruntime.ReadMemStats(&after)
	const allowedGrowth = 32 << 20
	if after.HeapAlloc > before.HeapAlloc+allowedGrowth {
		t.Fatalf("live heap grew from %d to %d bytes", before.HeapAlloc, after.HeapAlloc)
	}
}

func TestShutdownSerializesWithPendingDispatch(t *testing.T) {
	r := New(context.Background(), nil, nil)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; function dispatch($event,$args){ $value = 0; for ($i = 0; $i < 100; $i++) $value += $i; return $value === 4950; }`)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r.Dispatch("Tick") }()
	}
	r.Close()
	wg.Wait()
	r.Close()
	if r.DispatchDefault("Tick", false) {
		t.Fatal("closed runtime executed a callback")
	}
}

func TestExceptionDoesNotCorruptRuntime(t *testing.T) {
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; $GLOBALS['calls']=0; function handler(){ $GLOBALS['calls']++; if ($GLOBALS['calls'] === 1) throw new \RuntimeException('broken handler'); return false; } function dispatch($event,...$args){ try { return handler(); } catch (\Throwable $error) { error_log($error->getMessage()); return true; } }`)); err != nil {
		t.Fatal(err)
	}
	if !r.Dispatch("PlayerConnect") {
		t.Fatal("failed handlers must use the safe default")
	}
	if r.Dispatch("PlayerConnect") {
		t.Fatal("a later handler did not execute")
	}
}

type recordingLogger struct {
	messages []string
}

func (l *recordingLogger) Printf(format string, arguments ...any) {
	l.messages = append(l.messages, fmt.Sprintf(format, arguments...))
}

func TestDispatchStatsAndSlowCallbackLogging(t *testing.T) {
	logger := &recordingLogger{}
	r := New(context.Background(), native.Func(func(string, []any) (any, error) {
		time.Sleep(5 * time.Millisecond)
		return true, nil
	}), logger)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; function dispatch($event, $args){ return \ompphp_native_call('Delay'); }`)); err != nil {
		t.Fatal(err)
	}
	r.slow = time.Millisecond
	if !r.Dispatch("PlayerConnect", 7) {
		t.Fatal("dispatch failed")
	}
	stats := r.Stats()
	if stats.Dispatches != 1 || stats.Failures != 0 || stats.TotalTime <= 0 || stats.MaxTime <= 0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if len(logger.messages) != 1 || !strings.Contains(logger.messages[0], "slow PHP callback PlayerConnect") {
		t.Fatalf("unexpected log messages: %#v", logger.messages)
	}
}

func TestSlowCallbackThresholdFromEnvironment(t *testing.T) {
	t.Setenv("OMPPHP_SLOW_CALLBACK_MS", "12.5")
	if got := slowCallbackThreshold(); got != 12500*time.Microsecond {
		t.Fatalf("threshold = %s", got)
	}
	t.Setenv("OMPPHP_SLOW_CALLBACK_MS", "invalid")
	if got := slowCallbackThreshold(); got != 0 {
		t.Fatalf("invalid threshold = %s", got)
	}
}

func TestGeneratedSDKLoadsAndCallsNative(t *testing.T) {
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	sdk := filepath.Join(filepath.Dir(file), "..", "..", "sdk", "src")
	called := false
	gateway := native.Func(func(name string, arguments []any) (any, error) {
		called = true
		if name != "Player_SetHealth" {
			t.Fatalf("native name = %q", name)
		}
		if len(arguments) != 2 || arguments[0] != int64(7) || arguments[1] != float64(95) {
			t.Fatalf("arguments = %#v", arguments)
		}
		return true, nil
	})
	r := New(context.Background(), gateway, nil)
	t.Cleanup(r.Close)
	entry := fmt.Sprintf(`<?php
require %q;
require %q;
require %q;
require %q;
require %q;
$GLOBALS['nativeResult'] = (new \Omp\Player(7))->setHealth(95.0);
`, filepath.Join(sdk, "Runtime.php"), filepath.Join(sdk, "Server.php"), filepath.Join(sdk, "Player.php"), filepath.Join(sdk, "Internal", "functions.php"), filepath.Join(sdk, "Internal", "api_generated.php"))
	if err := r.Load(script(t, entry)); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("native gateway was not called")
	}
}

func TestRuntimesKeepIndependentNativeGateways(t *testing.T) {
	firstCalls := 0
	secondCalls := 0
	first := New(context.Background(), native.Func(func(name string, arguments []any) (any, error) {
		firstCalls++
		return name, nil
	}), nil)
	t.Cleanup(first.Close)
	second := New(context.Background(), native.Func(func(name string, arguments []any) (any, error) {
		secondCalls++
		return name, nil
	}), nil)
	t.Cleanup(second.Close)

	if err := first.Load(script(t, `<?php $GLOBALS['result'] = ompphp_native_call('First');`)); err != nil {
		t.Fatal(err)
	}
	if err := second.Load(script(t, `<?php $GLOBALS['result'] = ompphp_native_call('Second');`)); err != nil {
		t.Fatal(err)
	}
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("native calls routed to wrong runtime: first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestSDKEventDispatcherRunsInGoro(t *testing.T) {
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	sdk := filepath.Join(filepath.Dir(file), "..", "..", "sdk", "src")
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	entry := fmt.Sprintf(`<?php
require %q;
require %q;
require %q;
\Omp\Server::on('PlayerConnect', static fn(int $id): bool => $id === 42);
\Omp\Server::on('NoOpinion', static function (): void {});
`, filepath.Join(sdk, "Server.php"), filepath.Join(sdk, "Internal", "functions.php"), filepath.Join(sdk, "Event", "Events.php"))
	if err := r.Load(script(t, entry)); err != nil {
		t.Fatal(err)
	}
	if !r.Dispatch("PlayerConnect", 42) {
		t.Fatal("registered handler result was not returned")
	}
	if r.Dispatch("PlayerConnect", 7) {
		t.Fatal("handler arguments were not delivered")
	}
	if r.DispatchDefault("Unregistered", false) {
		t.Fatal("unregistered event ignored metadata default")
	}
	if r.DispatchDefault("NoOpinion", false) {
		t.Fatal("non-boolean handler ignored metadata default")
	}
}

func TestGrandLarcenyExampleLoadsAndDispatches(t *testing.T) {
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	entry := filepath.Join(filepath.Dir(file), "..", "..", "examples", "grandlarc", "gamemode.php")
	autoload := filepath.Join(filepath.Dir(entry), "vendor", "autoload.php")
	if _, err := os.Stat(autoload); err != nil {
		t.Skip("run composer install in examples/grandlarc to enable example smoke test")
	}
	calls := 0
	gateway := native.Func(func(name string, arguments []any) (any, error) {
		calls++
		switch name {
		case "TextDraw_Create", "Class_Add", "Vehicle_AddStaticEx":
			return int64(calls), nil
		case "Player_IsNPC":
			return false, nil
		case "Player_GetState":
			return int64(9), nil
		case "Player_GetKeys":
			return []any{int64(0), int64(0), int64(1)}, nil
		case "Player_GetMoney":
			return int64(100), nil
		default:
			return true, nil
		}
	})
	r := New(context.Background(), gateway, nil)
	t.Cleanup(r.Close)
	if err := r.Load(entry); err != nil {
		t.Fatal(err)
	}
	if !r.Dispatch("PlayerConnect", 7) {
		t.Fatal("connect callback failed")
	}
	if r.Dispatch("PlayerRequestClass", 7, 0) {
		t.Fatal("first class request should remain in city selection")
	}
	if !r.Dispatch("PlayerUpdate", 7) {
		t.Fatal("city-selection update failed")
	}
	if !r.Dispatch("PlayerSpawn", 7) {
		t.Fatal("spawn callback failed")
	}
	if calls < 20 {
		t.Fatalf("only %d native calls were made", calls)
	}
}

func TestArrayCrossesNativeBoundary(t *testing.T) {
	called := false
	r := New(context.Background(), native.Func(func(name string, arguments []any) (any, error) {
		called = true
		if name != "ArrayEcho" || len(arguments) != 1 {
			t.Fatalf("call = %s %#v", name, arguments)
		}
		values, ok := arguments[0].([]any)
		if !ok || len(values) != 3 || values[0] != int64(1) || values[1] != true || values[2] != "three" {
			t.Fatalf("array = %#v", arguments[0])
		}
		return []any{float64(1.5), float64(2.5), float64(3.5)}, nil
	}), nil)
	t.Cleanup(r.Close)
	entry := script(t, `<?php namespace Omp\Internal; $GLOBALS['result'] = \ompphp_native_call('ArrayEcho', [1, true, 'three']); function dispatch($event, $args){ return $GLOBALS['result'][2] === 3.5; }`)
	if err := r.Load(entry); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("gateway was not called")
	}
	if !r.Dispatch("Check") {
		t.Fatal("returned array did not reach PHP")
	}
}

func TestNativeBoundaryRejectsUnsupportedValues(t *testing.T) {
	t.Run("native name", func(t *testing.T) {
		called := false
		r := New(context.Background(), native.Func(func(string, []any) (any, error) {
			called = true
			return true, nil
		}), nil)
		t.Cleanup(r.Close)
		err := r.Load(script(t, `<?php ompphp_native_call(123);`))
		if err == nil || !strings.Contains(err.Error(), "native function name must be a string, got int") {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("gateway was called with an invalid native name")
		}
	})

	t.Run("PHP argument", func(t *testing.T) {
		called := false
		r := New(context.Background(), native.Func(func(string, []any) (any, error) {
			called = true
			return true, nil
		}), nil)
		t.Cleanup(r.Close)
		err := r.Load(script(t, `<?php ompphp_native_call('ObjectArgument', new \stdClass());`))
		if err == nil || !strings.Contains(err.Error(), "ObjectArgument argument 1: unsupported PHP value type object") {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("gateway was called with an unsupported PHP value")
		}
	})

	t.Run("Go result", func(t *testing.T) {
		r := New(context.Background(), native.Func(func(string, []any) (any, error) {
			return struct{}{}, nil
		}), nil)
		t.Cleanup(r.Close)
		err := r.Load(script(t, `<?php ompphp_native_call('InvalidResult');`))
		if err == nil || !strings.Contains(err.Error(), "InvalidResult result: unsupported Go value type struct {}") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDispatchRejectsUnsupportedEventArguments(t *testing.T) {
	logger := &recordingLogger{}
	r := New(context.Background(), nil, logger)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; function dispatch($event, $args, $default) { return !$default; }`)); err != nil {
		t.Fatal(err)
	}
	if !r.DispatchDefault("Invalid", true, struct{}{}) {
		t.Fatal("unsupported event argument did not use the event default")
	}
	stats := r.Stats()
	if stats.Dispatches != 1 || stats.Failures != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if len(logger.messages) != 1 || !strings.Contains(logger.messages[0], "unsupported Go value type struct {}") {
		t.Fatalf("unexpected log messages: %#v", logger.messages)
	}
}

func TestComposerCompatibilityMatrix(t *testing.T) {
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	autoload := filepath.Join(filepath.Dir(file), "..", "..", "sdk", "vendor", "autoload.php")
	if _, err := os.Stat(autoload); err != nil {
		t.Skip("run composer install in sdk to enable compatibility test")
	}
	r := New(context.Background(), nil, nil)
	t.Cleanup(r.Close)
	entry := fmt.Sprintf(`<?php
require %q;
trait CounterTrait { public function next(): int { return 1; } }
final class MatrixLogger extends \Psr\Log\AbstractLogger { use CounterTrait; public array $messages = []; public function log($level, string|\Stringable $message, array $context = []): void { $this->messages[] = [$level, (string) $message, $context]; } }
$logger = new MatrixLogger();
$logger->info((new \Symfony\Component\String\UnicodeString('open.mp'))->upper());
$iterator = new \ArrayIterator([20, 22]);
$sum = 0; foreach ($iterator as $value) { $sum += $value; }
\Omp\Server::on('ComposerMatrix', static function () use ($logger, $sum): bool { return $logger->next() > 0 && $logger->messages[0][1] === 'OPEN.MP' && $sum === 42; });
`, autoload)
	if err := r.Load(script(t, entry)); err != nil {
		t.Fatal(err)
	}
	if !r.DispatchDefault("ComposerMatrix", false) {
		t.Fatal("Composer package matrix failed in Goro")
	}
}

func BenchmarkDispatch(b *testing.B) {
	r := New(context.Background(), nil, nil)
	defer r.Close()
	entry := filepath.Join(b.TempDir(), "main.php")
	if err := os.WriteFile(entry, []byte(`<?php namespace Omp\Internal; function dispatch($event,...$args){ return true; }`), 0o600); err != nil {
		b.Fatal(err)
	}
	if err := r.Load(entry); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		r.Dispatch("Tick", 16)
	}
}
