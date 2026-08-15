package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	publicasync "github.com/ompphp/ompphp/async"
	oconcurrency "github.com/ompphp/ompphp/internal/concurrency"
	"github.com/ompphp/ompphp/internal/transport"
)

type executionContext string

const (
	contextMain   executionContext = "main"
	contextWorker executionContext = "worker"
	contextActor  executionContext = "actor"
)

var (
	runtimeStateKey = phpv.NewStateKey("ompphp runtime")
	contextStateKey = phpv.NewStateKey("ompphp execution context")
)

type phpWorker struct {
	id      int
	global  *phpctx.Global
	process *phpctx.Process
	mu      sync.Mutex
	start   <-chan struct{}
	limits  transport.Limits
}

const workerBootstrap = `
namespace Omp\Internal;
final class WorkerActorStore { public static array $actors = []; }
function worker_error(\Throwable $error): array {
    return ['ok' => false, 'error' => [
        'class' => get_class($error), 'message' => $error->getMessage(),
        'file' => $error->getFile(), 'line' => $error->getLine(), 'trace' => $error->getTraceAsString(),
    ]];
}
function worker_run(string $class, mixed $payload): array {
    try { $task = new $class(); return ['ok' => true, 'value' => $task($payload)]; }
    catch (\Throwable $error) { return worker_error($error); }
}
function worker_actor_spawn(int $id, string $class, mixed $payload): array {
    try { WorkerActorStore::$actors[$id] = new $class($payload); return ['ok' => true, 'value' => null]; }
    catch (\Throwable $error) { return worker_error($error); }
}
function worker_actor_call(int $id, string $method, mixed $payload): array {
    try {
        if (!isset(WorkerActorStore::$actors[$id])) throw new \RuntimeException("Actor {$id} is not running.");
        return ['ok' => true, 'value' => WorkerActorStore::$actors[$id]->{$method}($payload)];
    } catch (\Throwable $error) { return worker_error($error); }
}
function worker_actor_stop(int $id): array {
    unset(WorkerActorStore::$actors[$id]); return ['ok' => true, 'value' => null];
}`

func newPHPWorker(parent context.Context, id int, autoload string) (*phpWorker, error) {
	process := phpctx.NewProcess("ompphp-worker")
	global := phpctx.NewGlobal(parent, process, ini.New())
	global.SetState(contextStateKey, contextWorker)
	if autoload != "" {
		quoted := phpQuote(autoload)
		if _, err := global.DoString(global, phpv.ZString("if (is_file('"+quoted+"')) require_once '"+quoted+"';")); err != nil {
			_ = global.Close()
			return nil, fmt.Errorf("load Composer autoloader: %w", err)
		}
	}
	if _, err := global.DoString(global, phpv.ZString(workerBootstrap)); err != nil {
		_ = global.Close()
		return nil, fmt.Errorf("load worker bootstrap: %w", err)
	}
	return &phpWorker{id: id, global: global, process: process, limits: transport.DefaultLimits()}, nil
}

func (w *phpWorker) Run(class string, payload any) (any, *oconcurrency.RemoteError) {
	return w.call(contextWorker, "Omp\\Internal\\worker_run", phpv.ZString(class).ZVal(), payload)
}

func (w *phpWorker) SpawnActor(id uint64, class string, payload any) *oconcurrency.RemoteError {
	_, remote := w.call(contextActor, "Omp\\Internal\\worker_actor_spawn", phpv.ZInt(id).ZVal(), class, payload)
	return remote
}

func (w *phpWorker) CallActor(id uint64, method string, payload any) (any, *oconcurrency.RemoteError) {
	return w.call(contextActor, "Omp\\Internal\\worker_actor_call", phpv.ZInt(id).ZVal(), method, payload)
}

func (w *phpWorker) StopActor(id uint64) *oconcurrency.RemoteError {
	_, remote := w.call(contextActor, "Omp\\Internal\\worker_actor_stop", phpv.ZInt(id).ZVal())
	return remote
}

func (w *phpWorker) call(kind executionContext, name string, fixed *phpv.ZVal, values ...any) (any, *oconcurrency.RemoteError) {
	if w.start != nil {
		<-w.start
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.global.ResetDeadline()
	w.global.SetState(contextStateKey, kind)
	defer w.global.SetState(contextStateKey, contextWorker)
	fn, err := w.global.GetFunction(w.global, phpv.ZString(name))
	if err != nil {
		return nil, &oconcurrency.RemoteError{Class: "WorkerBootstrapError", Message: err.Error()}
	}
	args := []*phpv.ZVal{fixed}
	for _, value := range values {
		if text, ok := value.(string); ok {
			args = append(args, phpv.ZString(text).ZVal())
			continue
		}
		converted, err := transport.ToPHP(value, w.limits)
		if err != nil {
			return nil, &oconcurrency.RemoteError{Class: "TransferError", Message: err.Error()}
		}
		args = append(args, converted)
	}
	result, err := fn.Call(w.global, args)
	if err != nil {
		return nil, &oconcurrency.RemoteError{Class: "WorkerRuntimeError", Message: err.Error()}
	}
	decoded, err := transport.FromPHP(w.global, result, w.limits)
	if err != nil {
		return nil, &oconcurrency.RemoteError{Class: "TransferError", Message: err.Error()}
	}
	return decodeWorkerResult(decoded)
}

func decodeWorkerResult(value any) (any, *oconcurrency.RemoteError) {
	root, ok := mapStrings(value)
	if !ok {
		return nil, &oconcurrency.RemoteError{Class: "WorkerProtocolError", Message: "worker returned an invalid response"}
	}
	if success, _ := root["ok"].(bool); success {
		return root["value"], nil
	}
	data, _ := mapStrings(root["error"])
	line, _ := data["line"].(int64)
	return nil, &oconcurrency.RemoteError{
		Class: stringValue(data["class"]), Message: stringValue(data["message"]),
		File: stringValue(data["file"]), Line: line, Trace: stringValue(data["trace"]),
	}
}

func mapStrings(value any) (map[string]any, bool) {
	entries, ok := value.(transport.Map)
	if !ok {
		return nil, false
	}
	result := make(map[string]any, len(entries))
	for _, entry := range entries {
		if entry.Key.IsInt {
			return nil, false
		}
		result[entry.Key.String] = entry.Value
	}
	return result, true
}

func stringValue(value any) string { text, _ := value.(string); return text }

func (w *phpWorker) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.global.RunShutdownFunctions()
	_ = w.global.Close()
}

func phpQuote(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "\\'")
}

func concurrencyConfig() oconcurrency.Config {
	config := oconcurrency.DefaultConfig()
	config.Workers = envPositiveInt("OMPPHP_WORKERS", config.Workers)
	config.TaskQueue = envPositiveInt("OMPPHP_TASK_QUEUE", config.TaskQueue)
	config.NativeWorkers = envPositiveInt("OMPPHP_NATIVE_WORKERS", config.NativeWorkers)
	config.NativeQueue = envPositiveInt("OMPPHP_NATIVE_QUEUE", config.NativeQueue)
	config.CompletionQueue = envPositiveInt("OMPPHP_COMPLETION_QUEUE", config.CompletionQueue)
	config.ActorMailbox = envPositiveInt("OMPPHP_ACTOR_MAILBOX", config.ActorMailbox)
	return config
}

func envPositiveInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (r *Runtime) ensureScheduler() (*oconcurrency.Scheduler, error) {
	if r.closing.Load() {
		return nil, errRuntimeClosed
	}
	r.schedulerMu.Lock()
	defer r.schedulerMu.Unlock()
	if r.closing.Load() {
		return nil, errRuntimeClosed
	}
	if r.scheduler != nil {
		return r.scheduler, nil
	}
	autoload := os.Getenv("OMPPHP_WORKER_BOOTSTRAP")
	if autoload == "" {
		autoload = filepath.Join(r.entryDir, "vendor", "autoload.php")
	}
	scheduler, err := oconcurrency.New(concurrencyConfig(), func(id int) (oconcurrency.Runner, error) {
		worker, err := newPHPWorker(r.parent, id, autoload)
		if worker != nil {
			worker.start = r.workerStart
			worker.limits = r.transferLimits
		}
		return worker, err
	})
	if err != nil {
		return nil, err
	}
	for name, provider := range publicasync.Providers() {
		if err := scheduler.RegisterNative(name, oconcurrency.NativeFunc(provider)); err != nil {
			scheduler.Close()
			return nil, err
		}
	}
	r.scheduler = scheduler
	return scheduler, nil
}

func durationMilliseconds(value int64) (time.Duration, error) {
	if value <= 0 {
		return 0, errors.New("duration must be greater than zero")
	}
	return time.Duration(value) * time.Millisecond, nil
}
