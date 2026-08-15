package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	publicasync "github.com/ompphp/ompphp/async"
	"github.com/ompphp/ompphp/internal/native"
)

var providerSequence atomic.Uint64

func TestAsyncActorsTimersAndWorkerGuard(t *testing.T) {
	root := t.TempDir()
	sdk, err := filepath.Abs(filepath.Join("..", "..", "sdk", "vendor", "autoload.php"))
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := filepath.Join(root, "autoload.php")
	workerCode := fmt.Sprintf(`<?php
require %q;
final class DoubleTask { public function __invoke(mixed $value): int { return $value * 2; } }
final class GuardTask { public function __invoke(mixed $value): mixed { return \Omp\Internal\native_call('Core_TickCount'); } }
final class CounterActor {
    public function __construct(private int $value) {}
    public function add(mixed $value): int { return $this->value += $value; }
}
`, sdk)
	if err := os.WriteFile(bootstrap, []byte(workerCode), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OMPPHP_WORKER_BOOTSTRAP", bootstrap)
	t.Setenv("OMPPHP_WORKERS", "2")
	providerName := fmt.Sprintf("runtime-test-%d", providerSequence.Add(1))
	if err := publicasync.Register(providerName, func(_ context.Context, value any) (any, error) {
		return value.(int64) + 1, nil
	}); err != nil {
		t.Fatal(err)
	}

	main := filepath.Join(root, "gamemode.php")
	mainCode := fmt.Sprintf(`<?php
require %q;
use Omp\Async;
use Omp\Concurrency\Actor;
use Omp\Timer;
$GLOBALS['async'] = 0; $GLOBALS['nativeAsync'] = 0; $GLOBALS['actor'] = 0; $GLOBALS['timer'] = false; $GLOBALS['guarded'] = false;
Async::run(DoubleTask::class, 21)->then(function (mixed $value): void { $GLOBALS['async'] = $value; });
Async::native(%q, 8)->then(function (mixed $value): void { $GLOBALS['nativeAsync'] = $value; });
$actor = Actor::spawn(CounterActor::class, 10);
$actor->call('add', 1)->then(function (mixed $value): void { $GLOBALS['actor'] = $value; });
$actor->call('add', 2)->then(function (mixed $value): void { $GLOBALS['actor'] = $value; });
Async::run(GuardTask::class, null)->catch(function (\Throwable $error): void {
    $GLOBALS['guarded'] = str_contains($error->getMessage(), 'cannot call open.mp from a worker runtime');
});
Timer::after(1, function (): void { $GLOBALS['timer'] = true; });
\Omp\Server::on('Tick', function (): bool {
    return $GLOBALS['async'] === 42 && $GLOBALS['nativeAsync'] === 9 && $GLOBALS['actor'] === 13 && $GLOBALS['timer'] && $GLOBALS['guarded'];
});
`, bootstrap, providerName)
	if err := os.WriteFile(main, []byte(mainCode), 0o600); err != nil {
		t.Fatal(err)
	}

	nativeCalls := 0
	r := New(context.Background(), native.Func(func(string, []any) (any, error) { nativeCalls++; return nil, nil }), nil)
	t.Cleanup(r.Close)
	if err := r.Load(main); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.DispatchDefault("Tick", false) {
			if nativeCalls != 0 {
				t.Fatalf("worker reached open.mp gateway %d times", nativeCalls)
			}
			if maximum := r.executor.maxConcurrent(); maximum != 1 {
				t.Fatalf("maximum concurrent main entries = %d", maximum)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("concurrency operations did not complete")
}

func TestMainExecutorNeverEntersGlobalConcurrently(t *testing.T) {
	r := New(context.Background(), native.Func(func(string, []any) (any, error) { return nil, nil }), nil)
	t.Cleanup(r.Close)
	if err := r.Load(script(t, `<?php namespace Omp\Internal; function dispatch($event, $args, $default = true) { usleep(100); return true; }`)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{}, 32)
	for range 32 {
		go func() { r.Dispatch("Tick"); done <- struct{}{} }()
	}
	for range 32 {
		<-done
	}
	if maximum := r.executor.maxConcurrent(); maximum != 1 {
		t.Fatalf("maximum concurrent main entries = %d", maximum)
	}
}
