package runtime

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/ompphp/ompphp/internal/native"
	"github.com/ompphp/ompphp/internal/transport"
)

func registerExtension() {
	phpctx.RegisterExt(&phpctx.Ext{Name: "ompphp", Version: Version, Functions: map[string]*phpctx.ExtFunction{
		"ompphp_native_call":         {Func: nativeCall, MinArgs: 1, MaxArgs: -1},
		"ompphp_runtime_version":     {Func: runtimeVersion, ZeroArgs: true},
		"ompphp_api_version":         {Func: apiVersion, ZeroArgs: true},
		"ompphp_runtime_context":     {Func: runtimeContext, ZeroArgs: true},
		"ompphp_async_run":           {Func: asyncRun, MinArgs: 2, MaxArgs: 2},
		"ompphp_async_native":        {Func: asyncNative, MinArgs: 2, MaxArgs: 2},
		"ompphp_future_cancel":       {Func: futureCancel, MinArgs: 1, MaxArgs: 1},
		"ompphp_future_timeout":      {Func: futureTimeout, MinArgs: 2, MaxArgs: 2},
		"ompphp_actor_spawn":         {Func: actorSpawn, MinArgs: 2, MaxArgs: 2},
		"ompphp_actor_call":          {Func: actorCall, MinArgs: 3, MaxArgs: 3},
		"ompphp_actor_stop":          {Func: actorStop, MinArgs: 1, MaxArgs: 1},
		"ompphp_actor_pool_spawn":    {Func: actorPoolSpawn, MinArgs: 3, MaxArgs: 3},
		"ompphp_actor_pool_stop":     {Func: actorPoolStop, MinArgs: 1, MaxArgs: 1},
		"ompphp_timer_start":         {Func: timerStart, MinArgs: 2, MaxArgs: 2},
		"ompphp_timer_cancel":        {Func: timerCancel, MinArgs: 1, MaxArgs: 1},
		"ompphp_concurrency_stats":   {Func: concurrencyStats, ZeroArgs: true},
		"ompphp_component_get":       {Func: componentGet, MinArgs: 1, MaxArgs: 1},
		"ompphp_component_supports":  {Func: componentSupports, MinArgs: 4, MaxArgs: 4},
		"ompphp_component_watch":     {Func: componentWatch, MinArgs: 1, MaxArgs: 1},
		"ompphp_component_unwatch":   {Func: componentUnwatch, MinArgs: 1, MaxArgs: 1},
		"ompphp_component_callables": {Func: componentCallables, MinArgs: 1, MaxArgs: 1},
		"ompphp_component_invoke":    {Func: componentInvoke, MinArgs: 3, MaxArgs: 3},
		"ompphp_network_subscribe":   {Func: networkSubscribe, MinArgs: 4, MaxArgs: 4},
		"ompphp_network_unsubscribe": {Func: networkUnsubscribe, MinArgs: 1, MaxArgs: 1},
		"ompphp_network_send":        {Func: networkSend, MinArgs: 7, MaxArgs: 7},
		"ompphp_network_types":       {Func: networkTypes, ZeroArgs: true},
		"ompphp_network_stats":       {Func: networkStats, ZeroArgs: true},
	}})
}

func extendedGateway(ctx phpv.Context) (native.ExtendedGateway, error) {
	if currentContext(ctx) != contextMain {
		return nil, fmt.Errorf("extended open.mp APIs are unavailable in a %s runtime", currentContext(ctx))
	}
	gateway, ok := ctx.Global().State(gatewayStateKey).(native.ExtendedGateway)
	if !ok || gateway == nil {
		return nil, native.ErrUnavailable
	}
	return gateway, nil
}

func parseUID(ctx phpv.Context, value *phpv.ZVal) (uint64, error) {
	text := string(value.AsString(ctx))
	uid, err := strconv.ParseUint(strings.TrimPrefix(text, "0x"), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hexadecimal UID %q", text)
	}
	return uid, nil
}

func componentGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	uid, err := parseUID(ctx, args[0])
	if err != nil {
		return nil, err
	}
	info, found := gateway.Component(uid)
	if !found {
		return phpv.ZNULL.ZVal(), nil
	}
	return toPHP([]any{fmt.Sprintf("%016x", info.UID), info.Name, info.Major, info.Minor, info.Patch, info.PreRel, info.Type})
}

func componentSupports(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	componentUID, err := parseUID(ctx, args[0])
	if err != nil {
		return nil, err
	}
	interfaceUID, err := parseUID(ctx, args[1])
	if err != nil {
		return nil, err
	}
	ok := gateway.ComponentSupports(componentUID, interfaceUID, uint32(args[2].AsInt(ctx)), uint32(args[3].AsInt(ctx)))
	return phpv.ZBool(ok).ZVal(), nil
}

func componentWatch(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	uid, err := parseUID(ctx, args[0])
	if err != nil {
		return nil, err
	}
	token, err := gateway.ComponentWatch(uid)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(token).ZVal(), nil
}

func componentUnwatch(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gateway.ComponentUnwatch(uint64(args[0].AsInt(ctx)))).ZVal(), nil
}

func callableDescriptorValue(descriptor native.CallableDescriptor) []any {
	parameters := make([]any, 0, len(descriptor.Parameters))
	for _, parameter := range descriptor.Parameters {
		parameters = append(parameters, []any{parameter.Name, int64(parameter.Type), parameter.Optional, parameter.HasDefault, parameter.Default})
	}
	return []any{descriptor.Name, descriptor.Documentation, parameters, int64(descriptor.ReturnType), descriptor.Deprecated, descriptor.MayCallback}
}

func componentCallables(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	uid, err := parseUID(ctx, args[0])
	if err != nil {
		return nil, err
	}
	descriptors, err := gateway.ComponentCallables(uid)
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, len(descriptors))
	for _, descriptor := range descriptors {
		result = append(result, callableDescriptorValue(descriptor))
	}
	return toPHP(result)
}

func componentInvoke(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	uid, err := parseUID(ctx, args[0])
	if err != nil {
		return nil, err
	}
	name := string(args[1].AsString(ctx))
	converted, err := fromPHP(ctx, args[2])
	if err != nil {
		return nil, fmt.Errorf("callable %s arguments: %w", name, err)
	}
	arguments, ok := converted.([]any)
	if !ok {
		return nil, fmt.Errorf("callable arguments must be an array")
	}
	result, err := gateway.ComponentInvoke(uid, name, arguments)
	if err != nil {
		var callableError *native.CallableError
		if errors.As(err, &callableError) {
			return toPHP([]any{false, callableError.Code, callableError.Message, nil})
		}
		return nil, fmt.Errorf("invoke %s: %w", name, err)
	}
	return toPHP([]any{true, int64(0), "", result})
}

func networkSubscribe(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	id, err := gateway.NetworkSubscribe(int32(args[0].AsInt(ctx)), int32(args[1].AsInt(ctx)), int8(args[2].AsInt(ctx)), bool(args[3].AsBool(ctx)))
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(id).ZVal(), nil
}

func networkUnsubscribe(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gateway.NetworkUnsubscribe(uint64(args[0].AsInt(ctx)))).ZVal(), nil
}

func networkSend(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	request := native.NetworkSendRequest{RPC: bool(args[0].AsBool(ctx)), PlayerID: int32(args[1].AsInt(ctx)), MessageID: int32(args[2].AsInt(ctx)), Data: string(args[3].AsString(ctx)), BitLength: uint32(args[4].AsInt(ctx)), Channel: int32(args[5].AsInt(ctx)), DispatchEvents: bool(args[6].AsBool(ctx))}
	count, err := gateway.NetworkSend(request)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(count).ZVal(), nil
}

func networkTypes(ctx phpv.Context, _ []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	values := gateway.NetworkTypes()
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return toPHP(result)
}

func networkStats(ctx phpv.Context, _ []*phpv.ZVal) (*phpv.ZVal, error) {
	gateway, err := extendedGateway(ctx)
	if err != nil {
		return nil, err
	}
	stats := gateway.NetworkStats()
	return toPHP([]any{stats.Subscriptions, stats.Callbacks, stats.Dropped, stats.Rejected, stats.CallbackNS})
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
