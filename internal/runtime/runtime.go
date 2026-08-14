package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
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

	"github.com/ompphp/ompphp/internal/native"
)

const APIVersion = 1

var Version = "0.1.0-dev"

var registerOnce sync.Once
var gatewayStateKey = phpv.NewStateKey("ompphp native gateway")

type Logger interface{ Printf(string, ...any) }

type Runtime struct {
	mu      sync.Mutex
	global  *phpctx.Global
	process *phpctx.Process
	logger  Logger
	closed  bool
	loaded  bool
	stats   Stats
	slow    time.Duration
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
	return &Runtime{global: global, process: p, logger: logger, slow: slowCallbackThreshold()}
}

func (r *Runtime) Load(entry string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("PHP runtime is shut down")
	}
	if r.loaded {
		return errors.New("PHP gamemode is already loaded")
	}
	quoted := strings.ReplaceAll(strings.ReplaceAll(entry, "\\", "\\\\"), "'", "\\'")
	if _, err := r.global.DoString(r.global, phpv.ZString("require '"+quoted+"';")); err != nil {
		return fmt.Errorf("load PHP gamemode %q: %w", entry, err)
	}
	r.loaded = true
	return nil
}

// Dispatch executes on the caller's goroutine. The mutex is the single entry
// point into Goro and preserves synchronous open.mp callback semantics.
func (r *Runtime) Dispatch(event string, arguments ...any) (result bool) {
	return r.DispatchDefault(event, true, arguments...)
}

func (r *Runtime) DispatchDefault(event string, defaultResult bool, arguments ...any) (result bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		r.stats.Dispatches++
		r.stats.TotalTime += elapsed
		if elapsed > r.stats.MaxTime {
			r.stats.MaxTime = elapsed
		}
		if r.slow > 0 && elapsed >= r.slow && r.logger != nil {
			r.logger.Printf("slow PHP callback %s took %s", event, elapsed.Round(time.Microsecond))
		}
	}()
	if r.closed {
		return defaultResult
	}
	fn, err := r.global.GetFunction(r.global, phpv.ZString("Omp\\Internal\\dispatch"))
	if err != nil {
		return defaultResult
	}
	values := phpv.NewZArray()
	for _, argument := range arguments {
		_ = values.OffsetSet(r.global, nil, toPHP(argument))
	}
	args := []*phpv.ZVal{phpv.ZString(event).ZVal(), values.ZVal(), phpv.ZBool(defaultResult).ZVal()}
	value, err := fn.Call(r.global, args)
	if err != nil {
		r.stats.Failures++
		if r.logger != nil {
			r.logger.Printf("PHP handler for %s failed: %v", event, err)
		}
		return defaultResult
	}
	if value == nil || value.GetType() == phpv.ZtNull {
		return defaultResult
	}
	return bool(value.AsBool(r.global))
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.global.RunShutdownFunctions()
	r.closed = true
}

func toPHP(value any) *phpv.ZVal {
	switch v := value.(type) {
	case nil:
		return phpv.ZNULL.ZVal()
	case bool:
		return phpv.ZBool(v).ZVal()
	case int:
		return phpv.ZInt(v).ZVal()
	case int32:
		return phpv.ZInt(v).ZVal()
	case int64:
		return phpv.ZInt(v).ZVal()
	case float32:
		return phpv.ZFloat(v).ZVal()
	case float64:
		return phpv.ZFloat(v).ZVal()
	case string:
		return phpv.ZString(v).ZVal()
	case []any:
		array := phpv.NewZArray()
		for _, item := range v {
			_ = array.OffsetSet(nil, nil, toPHP(item))
		}
		return array.ZVal()
	default:
		return phpv.ZString(fmt.Sprint(v)).ZVal()
	}
}

func fromPHP(ctx phpv.Context, value *phpv.ZVal) any {
	switch value.GetType() {
	case phpv.ZtNull:
		return nil
	case phpv.ZtBool:
		return bool(value.AsBool(ctx))
	case phpv.ZtInt:
		return int64(value.AsInt(ctx))
	case phpv.ZtFloat:
		return float64(value.AsFloat(ctx))
	case phpv.ZtString:
		return string(value.AsString(ctx))
	case phpv.ZtArray:
		array := value.AsArray(ctx)
		result := make([]any, 0, int(array.Count(ctx)))
		for _, item := range array.Iterate(ctx) {
			result = append(result, fromPHP(ctx, item))
		}
		return result
	default:
		return string(value.AsString(ctx))
	}
}
