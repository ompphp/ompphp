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
	"sync/atomic"
	"time"

	"github.com/KarpelesLab/goro/core/ini"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	_ "github.com/KarpelesLab/goro/core/vm/vmcompiler"
	_ "github.com/KarpelesLab/goro/ext/ctype"
	_ "github.com/KarpelesLab/goro/ext/date"
	_ "github.com/KarpelesLab/goro/ext/hash"
	_ "github.com/KarpelesLab/goro/ext/json"
	_ "github.com/KarpelesLab/goro/ext/mbstring"
	_ "github.com/KarpelesLab/goro/ext/pcre"
	_ "github.com/KarpelesLab/goro/ext/reflection"
	_ "github.com/KarpelesLab/goro/ext/spl"
	_ "github.com/KarpelesLab/goro/ext/standard"

	oconcurrency "github.com/ompphp/ompphp/internal/concurrency"
	"github.com/ompphp/ompphp/internal/native"
	"github.com/ompphp/ompphp/internal/transport"
)

const APIVersion = 5

var Version = "0.1.0-dev"

var registerOnce sync.Once
var gatewayStateKey = phpv.NewStateKey("ompphp native gateway")

type Logger interface{ Printf(string, ...any) }

type Runtime struct {
	parent         context.Context
	mu             sync.Mutex
	global         *phpctx.Global
	process        *phpctx.Process
	executor       *mainExecutor
	logger         Logger
	closed         bool
	closing        atomic.Bool
	loaded         bool
	stats          Stats
	slow           time.Duration
	entryDir       string
	schedulerMu    sync.Mutex
	scheduler      *oconcurrency.Scheduler
	workerStart    chan struct{}
	startOnce      sync.Once
	transferLimits transport.Limits
}

type Stats struct {
	Dispatches uint64
	Failures   uint64
	TotalTime  time.Duration
	MaxTime    time.Duration
}

func New(parent context.Context, gateway native.Gateway, logger Logger) *Runtime {
	registerOnce.Do(registerExtension)
	p := phpctx.NewProcess("ompphp")
	global := phpctx.NewGlobal(parent, p, ini.New())
	global.SetState(gatewayStateKey, gateway)
	r := &Runtime{
		parent: parent, global: global, process: p, logger: logger, slow: slowCallbackThreshold(), workerStart: make(chan struct{}),
		transferLimits: transport.Limits{
			MaxDepth: envPositiveInt("OMPPHP_TRANSFER_MAX_DEPTH", transport.DefaultMaxDepth),
			MaxBytes: envPositiveInt("OMPPHP_TRANSFER_MAX_BYTES", transport.DefaultMaxBytes),
		},
	}
	r.executor = newMainExecutor(global)
	global.SetState(runtimeStateKey, r)
	global.SetState(contextStateKey, contextMain)
	if extended, ok := gateway.(native.ExtendedGateway); ok {
		extended.SetNetworkDispatcher(r.dispatchNetwork)
		extended.SetComponentDispatcher(r.dispatchComponentInvalidated)
	}
	return r
}

func (r *Runtime) dispatchComponentInvalidated(token, uid uint64) {
	_ = r.executor.run(func(global *phpctx.Global) error {
		fn, err := global.GetFunction(global, phpv.ZString("Omp\\Internal\\dispatch_component_invalidated"))
		if err != nil {
			return nil
		}
		_, err = fn.Call(global, []*phpv.ZVal{phpv.ZInt(token).ZVal(), phpv.ZString(fmt.Sprintf("%016x", uid)).ZVal()})
		if err != nil && r.logger != nil {
			r.logger.Printf("PHP component invalidation handler %d failed: %v", token, err)
		}
		return nil
	})
}

func (r *Runtime) Load(entry string) error {
	defer r.startOnce.Do(func() { close(r.workerStart) })
	return r.executor.run(func(global *phpctx.Global) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.closed {
			return errRuntimeClosed
		}
		if r.loaded {
			return errors.New("PHP gamemode is already loaded")
		}
		absolute, err := filepath.Abs(entry)
		if err != nil {
			return fmt.Errorf("resolve PHP gamemode %q: %w", entry, err)
		}
		r.entryDir = filepath.Dir(absolute)
		quoted := strings.ReplaceAll(strings.ReplaceAll(entry, "\\", "\\\\"), "'", "\\'")
		if _, err := global.DoString(global, phpv.ZString("require '"+quoted+"';")); err != nil {
			return fmt.Errorf("load PHP gamemode %q: %w", entry, err)
		}
		r.loaded = true
		return nil
	})
}

// Dispatch executes on the caller's goroutine. The mutex is the single entry
// point into Goro and preserves synchronous open.mp callback semantics.
func (r *Runtime) Dispatch(event string, arguments ...any) (result bool) {
	return r.DispatchDefault(event, true, arguments...)
}

func (r *Runtime) DispatchDefault(event string, defaultResult bool, arguments ...any) (result bool) {
	result = defaultResult
	_ = r.executor.run(func(global *phpctx.Global) error {
		r.pumpGlobal(global, 256)
		started := time.Now()
		defer func() {
			elapsed := time.Since(started)
			r.mu.Lock()
			defer r.mu.Unlock()
			r.stats.Dispatches++
			r.stats.TotalTime += elapsed
			if elapsed > r.stats.MaxTime {
				r.stats.MaxTime = elapsed
			}
			if r.slow > 0 && elapsed >= r.slow && r.logger != nil {
				r.logger.Printf("slow PHP callback %s took %s", event, elapsed.Round(time.Microsecond))
			}
		}()
		fn, err := global.GetFunction(global, phpv.ZString("Omp\\Internal\\dispatch"))
		if err != nil {
			return nil
		}
		values := phpv.NewZArray()
		for index, argument := range arguments {
			converted, err := toPHP(argument)
			if err != nil {
				r.mu.Lock()
				r.stats.Failures++
				r.mu.Unlock()
				if r.logger != nil {
					r.logger.Printf("PHP event %s argument %d: %v", event, index+1, err)
				}
				return nil
			}
			_ = values.OffsetSet(global, nil, converted)
		}
		args := []*phpv.ZVal{phpv.ZString(event).ZVal(), values.ZVal(), phpv.ZBool(defaultResult).ZVal()}
		value, err := fn.Call(global, args)
		if err != nil {
			r.mu.Lock()
			r.stats.Failures++
			r.mu.Unlock()
			if r.logger != nil {
				r.logger.Printf("PHP handler for %s failed: %v", event, err)
			}
			return nil
		}
		if value != nil && value.GetType() != phpv.ZtNull {
			result = bool(value.AsBool(global))
		}
		return nil
	})
	return result
}

func (r *Runtime) dispatchNetwork(message native.NetworkMessage) (response native.NetworkResponse) {
	response = native.NetworkResponse{Data: message.Data, BitLength: message.BitLength, ReadOffsetBits: message.ReadOffsetBits}
	_ = r.executor.run(func(global *phpctx.Global) error {
		fn, err := global.GetFunction(global, phpv.ZString("Omp\\Internal\\dispatch_network"))
		if err != nil {
			return nil
		}
		value, err := fn.Call(global, []*phpv.ZVal{
			phpv.ZInt(message.SubscriptionID).ZVal(), phpv.ZInt(message.PlayerID).ZVal(),
			phpv.ZInt(message.MessageID).ZVal(), phpv.ZString(message.Data).ZVal(),
			phpv.ZInt(message.BitLength).ZVal(), phpv.ZInt(message.ReadOffsetBits).ZVal(),
		})
		if err != nil {
			if r.logger != nil {
				r.logger.Printf("PHP network handler %d failed: %v", message.SubscriptionID, err)
			}
			return nil
		}
		converted, err := fromPHP(global, value)
		if err != nil {
			return nil
		}
		items, ok := converted.([]any)
		if !ok || len(items) != 4 {
			return nil
		}
		response.Drop, _ = items[0].(bool)
		response.Data, _ = items[1].(string)
		response.BitLength, _ = items[2].(int64)
		response.ReadOffsetBits, _ = items[3].(int64)
		return nil
	})
	return response
}

func (r *Runtime) Stats() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stats
}

func slowCallbackThreshold() time.Duration {
	value := strings.TrimSpace(os.Getenv("OMPPHP_SLOW_CALLBACK_MS"))
	if value == "" {
		return 0
	}
	milliseconds, err := strconv.ParseFloat(value, 64)
	if err != nil || milliseconds <= 0 {
		return 0
	}
	return time.Duration(milliseconds * float64(time.Millisecond))
}

func (r *Runtime) Close() {
	if !r.closing.CompareAndSwap(false, true) {
		return
	}
	r.schedulerMu.Lock()
	if r.scheduler != nil {
		r.scheduler.Close()
		r.scheduler = nil
	}
	r.schedulerMu.Unlock()
	extended, _ := r.global.State(gatewayStateKey).(native.ExtendedGateway)
	r.executor.close(func(global *phpctx.Global) {
		global.RunShutdownFunctions()
		_ = global.Close()
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()
	})
	if extended != nil {
		extended.CloseExtended()
	}
}

func (r *Runtime) Pump(limit int) {
	_ = r.executor.run(func(global *phpctx.Global) error {
		r.pumpGlobal(global, limit)
		return nil
	})
}

func (r *Runtime) pumpGlobal(global *phpctx.Global, limit int) {
	r.schedulerMu.Lock()
	scheduler := r.scheduler
	r.schedulerMu.Unlock()
	if scheduler == nil {
		return
	}
	deadline := time.Now().Add(2 * time.Millisecond)
	processed := 0
	for processed < limit {
		batch := limit - processed
		if batch > 8 {
			batch = 8
		}
		completions := scheduler.Drain(batch)
		if len(completions) == 0 {
			return
		}
		for _, completion := range completions {
			processed++
			if completion.Kind == oconcurrency.CompletionTimer {
				fn, err := global.GetFunction(global, phpv.ZString("Omp\\Internal\\fire_timer"))
				if err == nil {
					_, err = fn.Call(global, []*phpv.ZVal{phpv.ZInt(completion.ID).ZVal()})
				}
				if err != nil && r.logger != nil {
					r.logger.Printf("timer %d failed: %v", completion.ID, err)
				}
				continue
			}
			r.callCompletion(global, "Omp\\Internal\\complete_future", completion.ID, completion.Value, completion.Error, completion.Cancelled)
			scheduler.Acknowledge(completion.ID)
		}
		if time.Now().After(deadline) {
			return
		}
	}
}

func (r *Runtime) callCompletion(global *phpctx.Global, name string, id uint64, value any, remote *oconcurrency.RemoteError, cancelled bool) {
	fn, err := global.GetFunction(global, phpv.ZString(name))
	if err != nil {
		return
	}
	converted, err := transport.ToPHP(value, r.transferLimits)
	if err != nil {
		converted = phpv.ZNULL.ZVal()
		remote = &oconcurrency.RemoteError{Class: "TransferError", Message: err.Error(), TaskID: id}
	}
	errorValue := any(nil)
	if remote != nil {
		errorValue = transport.Map{
			{Key: transport.Key{String: "class"}, Value: remote.Class},
			{Key: transport.Key{String: "message"}, Value: remote.Message},
			{Key: transport.Key{String: "file"}, Value: remote.File},
			{Key: transport.Key{String: "line"}, Value: remote.Line},
			{Key: transport.Key{String: "trace"}, Value: remote.Trace},
			{Key: transport.Key{String: "worker"}, Value: int64(remote.WorkerID)},
			{Key: transport.Key{String: "task"}, Value: int64(remote.TaskID)},
		}
	}
	convertedError, _ := transport.ToPHP(errorValue, r.transferLimits)
	_, err = fn.Call(global, []*phpv.ZVal{phpv.ZInt(id).ZVal(), converted, convertedError, phpv.ZBool(cancelled).ZVal()})
	if err != nil && r.logger != nil {
		r.logger.Printf("concurrency completion %d failed: %v", id, err)
	}
}

func toPHP(value any) (*phpv.ZVal, error) {
	switch v := value.(type) {
	case nil:
		return phpv.ZNULL.ZVal(), nil
	case bool:
		return phpv.ZBool(v).ZVal(), nil
	case int:
		return phpv.ZInt(v).ZVal(), nil
	case int32:
		return phpv.ZInt(v).ZVal(), nil
	case int64:
		return phpv.ZInt(v).ZVal(), nil
	case float32:
		return phpv.ZFloat(v).ZVal(), nil
	case float64:
		return phpv.ZFloat(v).ZVal(), nil
	case string:
		return phpv.ZString(v).ZVal(), nil
	case native.CallableUInt64Value:
		return phpv.ZString(v).ZVal(), nil
	case native.CallableEntityValue:
		return toPHP([]any{int64(v.Type), v.ID})
	case []any:
		array := phpv.NewZArray()
		for index, item := range v {
			converted, err := toPHP(item)
			if err != nil {
				return nil, fmt.Errorf("array item %d: %w", index, err)
			}
			_ = array.OffsetSet(nil, nil, converted)
		}
		return array.ZVal(), nil
	default:
		return nil, fmt.Errorf("unsupported Go value type %T", value)
	}
}

func fromPHP(ctx phpv.Context, value *phpv.ZVal) (any, error) {
	switch value.GetType() {
	case phpv.ZtNull:
		return nil, nil
	case phpv.ZtBool:
		return bool(value.AsBool(ctx)), nil
	case phpv.ZtInt:
		return int64(value.AsInt(ctx)), nil
	case phpv.ZtFloat:
		return float64(value.AsFloat(ctx)), nil
	case phpv.ZtString:
		return string(value.AsString(ctx)), nil
	case phpv.ZtArray:
		array := value.AsArray(ctx)
		result := make([]any, 0, int(array.Count(ctx)))
		index := 0
		for _, item := range array.Iterate(ctx) {
			converted, err := fromPHP(ctx, item)
			if err != nil {
				return nil, fmt.Errorf("array item %d: %w", index, err)
			}
			result = append(result, converted)
			index++
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported PHP value type %s", value.GetType().TypeName())
	}
}
