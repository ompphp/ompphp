package runtime

import (
	"context"
	"fmt"

	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/ompphp/ompphp/internal/native"
	"github.com/ompphp/ompphp/internal/transport"
)

func registerExtension() {
	phpctx.RegisterExt(&phpctx.Ext{Name: "ompphp", Version: Version, Functions: map[string]*phpctx.ExtFunction{
		"ompphp_native_call":       {Func: nativeCall, MinArgs: 1, MaxArgs: -1},
		"ompphp_runtime_version":   {Func: runtimeVersion, ZeroArgs: true},
		"ompphp_api_version":       {Func: apiVersion, ZeroArgs: true},
		"ompphp_runtime_context":   {Func: runtimeContext, ZeroArgs: true},
		"ompphp_async_run":         {Func: asyncRun, MinArgs: 2, MaxArgs: 2},
		"ompphp_async_native":      {Func: asyncNative, MinArgs: 2, MaxArgs: 2},
		"ompphp_future_cancel":     {Func: futureCancel, MinArgs: 1, MaxArgs: 1},
		"ompphp_future_timeout":    {Func: futureTimeout, MinArgs: 2, MaxArgs: 2},
		"ompphp_actor_spawn":       {Func: actorSpawn, MinArgs: 2, MaxArgs: 2},
		"ompphp_actor_call":        {Func: actorCall, MinArgs: 3, MaxArgs: 3},
		"ompphp_actor_stop":        {Func: actorStop, MinArgs: 1, MaxArgs: 1},
		"ompphp_actor_pool_spawn":  {Func: actorPoolSpawn, MinArgs: 3, MaxArgs: 3},
		"ompphp_actor_pool_stop":   {Func: actorPoolStop, MinArgs: 1, MaxArgs: 1},
		"ompphp_timer_start":       {Func: timerStart, MinArgs: 2, MaxArgs: 2},
		"ompphp_timer_cancel":      {Func: timerCancel, MinArgs: 1, MaxArgs: 1},
		"ompphp_concurrency_stats": {Func: concurrencyStats, ZeroArgs: true},
	}})
}

func runtimeVersion(phpv.Context, []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString(Version).ZVal(), nil
}
func apiVersion(phpv.Context, []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(APIVersion).ZVal(), nil
}

func nativeCall(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if args[0].GetType() != phpv.ZtString {
		return nil, fmt.Errorf("native function name must be a string, got %s", args[0].GetType().TypeName())
	}
	name := string(args[0].AsString(ctx))
	if currentContext(ctx) != contextMain {
		return nil, fmt.Errorf("%s cannot call open.mp from a %s runtime", name, currentContext(ctx))
	}
	values := make([]any, 0, len(args)-1)
	for index, arg := range args[1:] {
		value, err := fromPHP(ctx, arg)
		if err != nil {
			return nil, fmt.Errorf("%s argument %d: %w", name, index+1, err)
		}
		values = append(values, value)
	}
	gateway, ok := ctx.Global().State(gatewayStateKey).(native.Gateway)
	if !ok || gateway == nil {
		return nil, fmt.Errorf("%s: %w", name, native.ErrUnavailable)
	}
	result, err := gateway.Call(name, values)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	converted, err := toPHP(result)
	if err != nil {
		return nil, fmt.Errorf("%s result: %w", name, err)
	}
	return converted, nil
}

func currentContext(ctx phpv.Context) executionContext {
	value, _ := ctx.Global().State(contextStateKey).(executionContext)
	if value == "" {
		return contextMain
	}
	return value
}

func runtimeContext(ctx phpv.Context, _ []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString(currentContext(ctx)).ZVal(), nil
}

func schedulerFor(ctx phpv.Context) (*Runtime, error) {
	runtime, ok := ctx.Global().State(runtimeStateKey).(*Runtime)
	if !ok || runtime == nil {
		return nil, fmt.Errorf("concurrency is only available in the main runtime")
	}
	return runtime, nil
}

func transferFromPHP(ctx phpv.Context, runtime *Runtime, value *phpv.ZVal) (any, error) {
	return transport.FromPHP(ctx, value, runtime.transferLimits)
}

func asyncRun(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if args[0].GetType() != phpv.ZtString {
		return nil, fmt.Errorf("async task class must be a string")
	}
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := transferFromPHP(ctx, runtime, args[1])
	if err != nil {
		return nil, fmt.Errorf("async task payload: %w", err)
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	id, err := scheduler.Submit(string(args[0].AsString(ctx)), payload)
	if err != nil {
		return nil, fmt.Errorf("submit async task: %w", err)
	}
	return phpv.ZInt(id).ZVal(), nil
}

func asyncNative(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if args[0].GetType() != phpv.ZtString {
		return nil, fmt.Errorf("native async provider name must be a string")
	}
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := transferFromPHP(ctx, runtime, args[1])
	if err != nil {
		return nil, fmt.Errorf("native async payload: %w", err)
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	id, err := scheduler.SubmitNative(string(args[0].AsString(ctx)), payload)
	if err != nil {
		return nil, fmt.Errorf("submit native async task: %w", err)
	}
	return phpv.ZInt(id).ZVal(), nil
}

func futureCancel(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(scheduler.Cancel(uint64(args[0].AsInt(ctx)))).ZVal(), nil
}

func futureTimeout(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	duration, err := durationMilliseconds(int64(args[1].AsInt(ctx)))
	if err != nil {
		return nil, err
	}
	if err := scheduler.Timeout(uint64(args[0].AsInt(ctx)), duration); err != nil {
		return nil, err
	}
	return phpv.ZNULL.ZVal(), nil
}

func actorSpawn(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if args[0].GetType() != phpv.ZtString {
		return nil, fmt.Errorf("actor class must be a string")
	}
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := transferFromPHP(ctx, runtime, args[1])
	if err != nil {
		return nil, fmt.Errorf("actor constructor data: %w", err)
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	actorID, futureID, err := scheduler.SpawnActor(string(args[0].AsString(ctx)), payload)
	if err != nil {
		return nil, fmt.Errorf("spawn actor: %w", err)
	}
	result := phpv.NewZArray()
	_ = result.OffsetSet(ctx, nil, phpv.ZInt(actorID).ZVal())
	_ = result.OffsetSet(ctx, nil, phpv.ZInt(futureID).ZVal())
	return result.ZVal(), nil
}

func actorCall(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if args[1].GetType() != phpv.ZtString {
		return nil, fmt.Errorf("actor method must be a string")
	}
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := transferFromPHP(ctx, runtime, args[2])
	if err != nil {
		return nil, fmt.Errorf("actor message: %w", err)
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	id, err := scheduler.CallActor(uint64(args[0].AsInt(ctx)), string(args[1].AsString(ctx)), payload)
	if err != nil {
		return nil, fmt.Errorf("call actor: %w", err)
	}
	return phpv.ZInt(id).ZVal(), nil
}

func actorStop(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	id, err := scheduler.StopActor(uint64(args[0].AsInt(ctx)))
	if err != nil {
		return nil, fmt.Errorf("stop actor: %w", err)
	}
	return phpv.ZInt(id).ZVal(), nil
}

func actorPoolSpawn(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if args[0].GetType() != phpv.ZtString {
		return nil, fmt.Errorf("actor pool class must be a string")
	}
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := transferFromPHP(ctx, runtime, args[2])
	if err != nil {
		return nil, fmt.Errorf("actor pool constructor data: %w", err)
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	poolID, handles, err := scheduler.SpawnActorPool(string(args[0].AsString(ctx)), int(args[1].AsInt(ctx)), payload)
	if err != nil {
		return nil, fmt.Errorf("spawn actor pool: %w", err)
	}
	result := phpv.NewZArray()
	_ = result.OffsetSet(ctx, nil, phpv.ZInt(poolID).ZVal())
	actors := phpv.NewZArray()
	for _, handle := range handles {
		pair := phpv.NewZArray()
		_ = pair.OffsetSet(ctx, nil, phpv.ZInt(handle.ActorID).ZVal())
		_ = pair.OffsetSet(ctx, nil, phpv.ZInt(handle.FutureID).ZVal())
		_ = actors.OffsetSet(ctx, nil, pair.ZVal())
	}
	_ = result.OffsetSet(ctx, nil, actors.ZVal())
	return result.ZVal(), nil
}

func actorPoolStop(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	ids, err := scheduler.StopActorPool(uint64(args[0].AsInt(ctx)))
	if err != nil {
		return nil, fmt.Errorf("stop actor pool: %w", err)
	}
	result := phpv.NewZArray()
	for _, id := range ids {
		_ = result.OffsetSet(ctx, nil, phpv.ZInt(id).ZVal())
	}
	return result.ZVal(), nil
}

func timerStart(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	duration, err := durationMilliseconds(int64(args[0].AsInt(ctx)))
	if err != nil {
		return nil, err
	}
	id, err := scheduler.TimerAfter(duration, bool(args[1].AsBool(ctx)))
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(id).ZVal(), nil
}

func timerCancel(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(scheduler.CancelTimer(uint64(args[0].AsInt(ctx)))).ZVal(), nil
}

func concurrencyStats(ctx phpv.Context, _ []*phpv.ZVal) (*phpv.ZVal, error) {
	runtime, err := schedulerFor(ctx)
	if err != nil {
		return nil, err
	}
	scheduler, err := runtime.ensureScheduler()
	if err != nil {
		return nil, err
	}
	stats := scheduler.Stats()
	return transport.ToPHP(transport.Map{
		{Key: transport.Key{String: "workers"}, Value: int64(stats.Workers)}, {Key: transport.Key{String: "busyWorkers"}, Value: stats.BusyWorkers},
		{Key: transport.Key{String: "queuedTasks"}, Value: int64(stats.QueuedTasks)}, {Key: transport.Key{String: "runningTasks"}, Value: int64(stats.RunningTasks)},
		{Key: transport.Key{String: "phpWorkers"}, Value: int64(stats.PHPWorkers)}, {Key: transport.Key{String: "busyPHPWorkers"}, Value: stats.BusyPHPWorkers},
		{Key: transport.Key{String: "queuedPHPTasks"}, Value: int64(stats.QueuedPHPTasks)}, {Key: transport.Key{String: "nativeWorkers"}, Value: int64(stats.NativeWorkers)},
		{Key: transport.Key{String: "busyNativeWorkers"}, Value: stats.BusyNativeWorkers}, {Key: transport.Key{String: "queuedNativeTasks"}, Value: int64(stats.QueuedNativeTasks)},
		{Key: transport.Key{String: "completedTasks"}, Value: int64(stats.CompletedTasks)}, {Key: transport.Key{String: "failedTasks"}, Value: int64(stats.FailedTasks)},
		{Key: transport.Key{String: "cancelledTasks"}, Value: int64(stats.CancelledTasks)}, {Key: transport.Key{String: "timedOutTasks"}, Value: int64(stats.TimedOutTasks)},
		{Key: transport.Key{String: "actors"}, Value: int64(stats.Actors)}, {Key: transport.Key{String: "actorPools"}, Value: int64(stats.ActorPools)},
		{Key: transport.Key{String: "queuedActorMessages"}, Value: int64(stats.QueuedActorMessages)}, {Key: transport.Key{String: "pendingFutures"}, Value: int64(stats.PendingFutures)},
		{Key: transport.Key{String: "timers"}, Value: int64(stats.Timers)}, {Key: transport.Key{String: "completionQueue"}, Value: int64(stats.CompletionQueue)},
	}, runtime.transferLimits)
}

func (r *Runtime) RegisterAsyncProvider(name string, provider func(context.Context, any) (any, error)) error {
	scheduler, err := r.ensureScheduler()
	if err != nil {
		return err
	}
	return scheduler.RegisterNative(name, provider)
}
