package runtime

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/ompphp/ompphp/internal/native"
)

func registerExtension() {
	phpctx.RegisterExt(&phpctx.Ext{Name: "ompphp", Version: Version, Functions: map[string]*phpctx.ExtFunction{
		"ompphp_native_call":     {Func: nativeCall, MinArgs: 1, MaxArgs: -1},
		"ompphp_runtime_version": {Func: runtimeVersion, ZeroArgs: true},
		"ompphp_api_version":     {Func: apiVersion, ZeroArgs: true},
	}})
}

func runtimeVersion(phpv.Context, []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString(Version).ZVal(), nil
}
func apiVersion(phpv.Context, []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(APIVersion).ZVal(), nil
}

func nativeCall(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	name := string(args[0].AsString(ctx))
	values := make([]any, 0, len(args)-1)
	for _, arg := range args[1:] {
		values = append(values, fromPHP(ctx, arg))
	}
	gateway, ok := ctx.Global().State(gatewayStateKey).(native.Gateway)
	if !ok || gateway == nil {
		return nil, fmt.Errorf("%s: %w", name, native.ErrUnavailable)
	}
	result, err := gateway.Call(name, values)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return toPHP(result), nil
}
