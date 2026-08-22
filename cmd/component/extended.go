package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/omp-capi/lib/open.mp-capi/include
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "ompcapi.h"

extern int OMPPHPDispatchNetwork(uintptr_t, void*, int32_t, struct OMPNetBuffer*);
extern void OMPPHPDispatchComponentInvalidated(uintptr_t);
extern struct OMPAPI_t ompphp_api;

static enum OMPNetResult ompphp_network_callback(void* player, int32_t id, struct OMPNetBuffer* buffer, void* userdata) {
	return (enum OMPNetResult)OMPPHPDispatchNetwork((uintptr_t)userdata, player, id, buffer);
}
static struct OMPNetSubscription* ompphp_network_subscribe(int32_t direction, int32_t id, int8_t priority, bool all, uintptr_t token) {
	if (all) return ompphp_api.Network.SubscribeAll((enum OMPNetDirection)direction, priority, ompphp_network_callback, (void*)token);
	return ompphp_api.Network.Subscribe((enum OMPNetDirection)direction, id, priority, ompphp_network_callback, (void*)token);
}
static int ompphp_network_player_id(void* player) {
	return player && ompphp_api.Player.GetID ? ompphp_api.Player.GetID(player) : -1;
}
static bool ompphp_network_apply(struct OMPNetBuffer* buffer, const char* data, uint32_t bytes, uint32_t bits) {
	if (bits > bytes * 8 || !ompphp_api.Network.BufferResize(buffer, bits)) return false;
	if (bytes) memcpy(buffer->data, data, bytes);
	return true;
}
static struct OMPComponentHandle* ompphp_component_find(uint64_t uid) { return ompphp_api.ComponentInterop.Find(uid); }
static bool ompphp_component_valid(struct OMPComponentHandle* handle) { return ompphp_api.ComponentInterop.IsValid(handle); }
static int ompphp_component_name(struct OMPComponentHandle* handle, struct CAPIStringBuffer* output) { return ompphp_api.ComponentInterop.GetName(handle, output); }
static bool ompphp_component_version(struct OMPComponentHandle* handle, struct ComponentVersion* output) { return ompphp_api.ComponentInterop.GetVersion(handle, output); }
static int32_t ompphp_component_type(struct OMPComponentHandle* handle) { return ompphp_api.ComponentInterop.GetType(handle); }
static bool ompphp_component_supports(struct OMPComponentHandle* handle, uint64_t uid, uint32_t version, uint32_t size) {
	const void* table = ompphp_api.ComponentInterop.QueryAPI(handle, uid, version, size);
	return table && ompphp_api.ComponentInterop.APIIsValid(handle, table);
}
static void ompphp_component_invalidated(struct OMPComponentHandle* handle, void* userdata) { (void)handle; OMPPHPDispatchComponentInvalidated((uintptr_t)userdata); }
static struct OMPComponentWatch* ompphp_component_watch(struct OMPComponentHandle* handle, uintptr_t token) { return ompphp_api.ComponentInterop.Watch(handle, ompphp_component_invalidated, (void*)token); }
static bool ompphp_component_unwatch(struct OMPComponentWatch* watch) { return ompphp_api.ComponentInterop.Unwatch(watch); }
static struct OMPCallableRegistration* ompphp_callable_find(struct OMPComponentHandle* component, const char* name, uint32_t length) {
	struct CAPIStringView view = { length, name };
	return ompphp_api.ComponentInterop.FindCallable(component, view);
}
static uint32_t ompphp_callable_count(struct OMPComponentHandle* component) { return ompphp_api.ComponentInterop.GetCallableCount(component); }
static struct OMPCallableRegistration* ompphp_callable_at(struct OMPComponentHandle* component, uint32_t index) { return ompphp_api.ComponentInterop.GetCallableAt(component, index); }
static const struct OMPCallableDescriptor* ompphp_callable_descriptor(struct OMPCallableRegistration* callable) { return ompphp_api.ComponentInterop.GetCallableDescriptor(callable); }
static void ompphp_callable_value_init(struct OMPCallableValue* value, uint32_t type) {
	memset(value, 0, sizeof(*value)); value->abi_version = OMP_CALLABLE_ABI_VERSION;
	value->struct_size = sizeof(*value); value->type = type;
}
static void ompphp_callable_set_bool(struct OMPCallableValue* value, bool input) { value->value.boolean = input ? 1 : 0; }
static void ompphp_callable_set_i32(struct OMPCallableValue* value, int32_t input) { value->value.int32_value = input; }
static void ompphp_callable_set_u32(struct OMPCallableValue* value, uint32_t input) { value->value.uint32_value = input; }
static void ompphp_callable_set_i64(struct OMPCallableValue* value, int64_t input) { value->value.int64_value = input; }
static void ompphp_callable_set_u64(struct OMPCallableValue* value, uint64_t input) { value->value.uint64_value = input; }
static void ompphp_callable_set_float(struct OMPCallableValue* value, float input) { value->value.float_value = input; }
static void ompphp_callable_set_double(struct OMPCallableValue* value, double input) { value->value.double_value = input; }
static void ompphp_callable_set_string(struct OMPCallableValue* value, const void* data, uint32_t length, bool bytes) {
	if (bytes) value->value.bytes_value = (struct OMPCallableBytesView){ OMP_CALLABLE_ABI_VERSION, sizeof(struct OMPCallableBytesView), data, length };
	else value->value.string_value = (struct OMPCallableStringView){ OMP_CALLABLE_ABI_VERSION, sizeof(struct OMPCallableStringView), data, length };
}
static void ompphp_callable_set_entity(struct OMPCallableValue* value, uint32_t type, uint64_t id) {
	value->value.entity_value = (struct OMPCallableEntityValue){ OMP_CALLABLE_ABI_VERSION, sizeof(struct OMPCallableEntityValue), type, 0, id };
}
static bool ompphp_callable_invoke(struct OMPCallableRegistration* callable, const struct OMPCallableValue* args,
	uint32_t count, struct OMPCallableValue* result, struct OMPCallableOutputBuffer* output, struct OMPCallableError* error) {
	return ompphp_api.ComponentInterop.InvokeCallable(callable, args, count, result, output, error, 0);
}
static uint8_t ompphp_callable_get_bool(const struct OMPCallableValue* value) { return value->value.boolean; }
static int32_t ompphp_callable_get_i32(const struct OMPCallableValue* value) { return value->value.int32_value; }
static uint32_t ompphp_callable_get_u32(const struct OMPCallableValue* value) { return value->value.uint32_value; }
static int64_t ompphp_callable_get_i64(const struct OMPCallableValue* value) { return value->value.int64_value; }
static uint64_t ompphp_callable_get_u64(const struct OMPCallableValue* value) { return value->value.uint64_value; }
static float ompphp_callable_get_float(const struct OMPCallableValue* value) { return value->value.float_value; }
static double ompphp_callable_get_double(const struct OMPCallableValue* value) { return value->value.double_value; }
static const void* ompphp_callable_get_data(const struct OMPCallableValue* value) { return value->type == OMPCallableValueType_String ? (const void*)value->value.string_value.data : value->value.bytes_value.data; }
static uint32_t ompphp_callable_get_length(const struct OMPCallableValue* value) { return value->type == OMPCallableValueType_String ? value->value.string_value.length : value->value.bytes_value.length; }
static uint32_t ompphp_callable_get_entity_type(const struct OMPCallableValue* value) { return value->value.entity_value.entity_type; }
static uint64_t ompphp_callable_get_entity_id(const struct OMPCallableValue* value) { return value->value.entity_value.id; }
static bool ompphp_network_unsubscribe(struct OMPNetSubscription* subscription) { return ompphp_api.Network.Unsubscribe(subscription); }
static void* ompphp_player_from_id(int32_t id) { return ompphp_api.Player.FromID(id); }
static bool ompphp_network_send_packet(void* player, const uint8_t* data, uint32_t bits, int32_t channel, bool dispatch) { return ompphp_api.Network.SendPacket(player, data, bits, channel, dispatch); }
static bool ompphp_network_send_rpc(void* player, int32_t id, const uint8_t* data, uint32_t bits, int32_t channel, bool dispatch) { return ompphp_api.Network.SendRPC(player, id, data, bits, channel, dispatch); }
static uint32_t ompphp_network_broadcast_packet(const uint8_t* data, uint32_t bits, int32_t channel, bool dispatch) { return ompphp_api.Network.BroadcastPacket(-1, NULL, data, bits, channel, dispatch); }
static uint32_t ompphp_network_broadcast_rpc(int32_t id, const uint8_t* data, uint32_t bits, int32_t channel, bool dispatch) { return ompphp_api.Network.BroadcastRPC(-1, NULL, id, data, bits, channel, dispatch); }
static uint32_t ompphp_network_count(void) { return ompphp_api.Network.Count(); }
static int32_t ompphp_network_type(uint32_t index) { return ompphp_api.Network.Type(index); }
*/
import "C"

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/ompphp/ompphp/internal/native"
)

var extendedState = struct {
	sync.Mutex
	next              uint64
	subscriptions     map[uint64]*C.struct_OMPNetSubscription
	watches           map[uint64]*C.struct_OMPComponentWatch
	watchUIDs         map[uint64]uint64
	dispatch          func(native.NetworkMessage) native.NetworkResponse
	componentDispatch func(uint64, uint64)
	callbacks         int64
	dropped           int64
	rejected          int64
	callbackNS        int64
	depth             int
	maxBytes          int
	maxSubscriptions  int
}{next: 1, subscriptions: make(map[uint64]*C.struct_OMPNetSubscription), watches: make(map[uint64]*C.struct_OMPComponentWatch), watchUIDs: make(map[uint64]uint64)}

func configuredLimit(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func validateNetworkSubscription(direction, id int32, all bool) error {
	if direction < 0 || direction > 3 {
		return fmt.Errorf("network direction %d is invalid", direction)
	}
	if !all && id < 0 {
		return fmt.Errorf("network message ID must not be negative")
	}
	return nil
}

func validateNetworkPayload(data string, bits uint32, maxBytes int) error {
	if len(data) > maxBytes {
		return fmt.Errorf("network payload is %d bytes; maximum is %d", len(data), maxBytes)
	}
	if uint64(bits) > uint64(len(data))*8 {
		return fmt.Errorf("bit length %d exceeds payload capacity", bits)
	}
	return nil
}

func (capiGateway) Component(uid uint64) (native.ComponentInfo, bool) {
	handle := C.ompphp_component_find(C.uint64_t(uid))
	if handle == nil || !bool(C.ompphp_component_valid(handle)) {
		return native.ComponentInfo{}, false
	}
	var version C.struct_ComponentVersion
	if !bool(C.ompphp_component_version(handle, &version)) {
		return native.ComponentInfo{}, false
	}
	length := int(C.ompphp_component_name(handle, nil))
	name := ""
	if length > 0 {
		data := C.malloc(C.size_t(length + 1))
		defer C.free(data)
		buffer := C.struct_CAPIStringBuffer{data: (*C.char)(data), capacity: C.uint(length + 1)}
		C.ompphp_component_name(handle, &buffer)
		name = C.GoStringN((*C.char)(data), C.int(length))
	}
	return native.ComponentInfo{UID: uid, Name: name, Major: int64(version.major), Minor: int64(version.minor), Patch: int64(version.patch), PreRel: int64(version.prerel), Type: int64(C.ompphp_component_type(handle))}, true
}

func (capiGateway) ComponentSupports(componentUID, interfaceUID uint64, version, size uint32) bool {
	handle := C.ompphp_component_find(C.uint64_t(componentUID))
	if handle == nil {
		return false
	}
	return bool(C.ompphp_component_supports(handle, C.uint64_t(interfaceUID), C.uint32_t(version), C.uint32_t(size)))
}

func callableString(value C.struct_OMPCallableStringView) string {
	if value.data == nil || value.length == 0 {
		return ""
	}
	return C.GoStringN(value.data, C.int(value.length))
}

func callableDescriptor(value *C.struct_OMPCallableDescriptor) native.CallableDescriptor {
	parameters := make([]native.CallableParameter, 0, uint32(value.parameter_count))
	for index := uint32(0); index < uint32(value.parameter_count); index++ {
		parameter := (*C.struct_OMPCallableParameter)(unsafe.Add(unsafe.Pointer(value.parameters), uintptr(index)*C.sizeof_struct_OMPCallableParameter))
		hasDefault := uint32(parameter.flags)&uint32(C.OMPCallableParameterFlag_HasDefault) != 0
		var defaultValue any
		if hasDefault {
			defaultValue, _ = decodeCallableValue(&parameter.default_value)
		}
		parameters = append(parameters, native.CallableParameter{Name: callableString(parameter.name), Type: native.CallableType(parameter._type), Optional: uint32(parameter.flags)&uint32(C.OMPCallableParameterFlag_Optional) != 0, HasDefault: hasDefault, Default: defaultValue})
	}
	return native.CallableDescriptor{Name: callableString(value.name), Documentation: callableString(value.documentation), Parameters: parameters, ReturnType: native.CallableType(value.return_type), Deprecated: uint32(value.flags)&uint32(C.OMPCallableFlag_Deprecated) != 0, MayCallback: uint32(value.flags)&uint32(C.OMPCallableFlag_MayCallback) != 0}
}

func (capiGateway) ComponentCallables(uid uint64) ([]native.CallableDescriptor, error) {
	handle := C.ompphp_component_find(C.uint64_t(uid))
	if handle == nil {
		return nil, fmt.Errorf("component %016x is unavailable", uid)
	}
	count := uint32(C.ompphp_callable_count(handle))
	result := make([]native.CallableDescriptor, 0, count)
	for index := uint32(0); index < count; index++ {
		callable := C.ompphp_callable_at(handle, C.uint32_t(index))
		descriptor := C.ompphp_callable_descriptor(callable)
		if descriptor != nil {
			result = append(result, callableDescriptor(descriptor))
		}
	}
	return result, nil
}

func callableInteger(value any, unsigned bool, bits int) (uint64, error) {
	switch input := value.(type) {
	case int64:
		if unsigned {
			if input < 0 {
				return 0, fmt.Errorf("negative value is invalid for uint%d", bits)
			}
			return uint64(input), nil
		}
		if bits == 32 && (input < -1<<31 || input > 1<<31-1) {
			return 0, fmt.Errorf("value is outside int32 range")
		}
		return uint64(input), nil
	case string:
		if !unsigned {
			return 0, fmt.Errorf("decimal strings are only accepted for unsigned integers")
		}
		parsed, err := strconv.ParseUint(input, 10, bits)
		if err != nil {
			return 0, fmt.Errorf("invalid uint%d value %q", bits, input)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("expected an integer, got %T", value)
	}
}

func callableEntity(value any) (uint32, uint64, error) {
	items, ok := value.([]any)
	if !ok || len(items) != 2 {
		return 0, 0, fmt.Errorf("entity must contain its type and ID")
	}
	typeValue, err := callableInteger(items[0], true, 32)
	if err != nil || typeValue == 0 {
		return 0, 0, fmt.Errorf("entity type is invalid")
	}
	id, err := callableInteger(items[1], true, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("entity ID: %w", err)
	}
	return uint32(typeValue), id, nil
}

func setCallableArgument(target *C.struct_OMPCallableValue, kind native.CallableType, value any) (unsafe.Pointer, error) {
	C.ompphp_callable_value_init(target, C.uint32_t(kind))
	switch kind {
	case native.CallableNull:
		if value != nil {
			return nil, fmt.Errorf("expected null, got %T", value)
		}
	case native.CallableBool:
		input, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", value)
		}
		C.ompphp_callable_set_bool(target, C.bool(input))
	case native.CallableInt32:
		input, err := callableInteger(value, false, 32)
		if err != nil {
			return nil, err
		}
		C.ompphp_callable_set_i32(target, C.int32_t(int64(input)))
	case native.CallableUInt32:
		input, err := callableInteger(value, true, 32)
		if err != nil {
			return nil, err
		}
		C.ompphp_callable_set_u32(target, C.uint32_t(input))
	case native.CallableInt64:
		input, err := callableInteger(value, false, 64)
		if err != nil {
			return nil, err
		}
		C.ompphp_callable_set_i64(target, C.int64_t(input))
	case native.CallableUInt64:
		input, err := callableInteger(value, true, 64)
		if err != nil {
			return nil, err
		}
		C.ompphp_callable_set_u64(target, C.uint64_t(input))
	case native.CallableFloat, native.CallableDouble:
		input, ok := value.(float64)
		if !ok {
			if integer, valid := value.(int64); valid {
				input, ok = float64(integer), true
			}
		}
		if !ok {
			return nil, fmt.Errorf("expected float, got %T", value)
		}
		if kind == native.CallableFloat {
			C.ompphp_callable_set_float(target, C.float(input))
		} else {
			C.ompphp_callable_set_double(target, C.double(input))
		}
	case native.CallableString, native.CallableBytes:
		input, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}
		storage := C.CBytes([]byte(input))
		C.ompphp_callable_set_string(target, storage, C.uint32_t(len(input)), C.bool(kind == native.CallableBytes))
		return storage, nil
	case native.CallableEntity:
		entityType, id, err := callableEntity(value)
		if err != nil {
			return nil, err
		}
		C.ompphp_callable_set_entity(target, C.uint32_t(entityType), C.uint64_t(id))
	default:
		return nil, fmt.Errorf("unsupported callable type %d", kind)
	}
	return nil, nil
}

func decodeCallableValue(value *C.struct_OMPCallableValue) (any, error) {
	switch native.CallableType(value._type) {
	case native.CallableNull:
		return nil, nil
	case native.CallableBool:
		return bool(C.ompphp_callable_get_bool(value) != 0), nil
	case native.CallableInt32:
		return int64(C.ompphp_callable_get_i32(value)), nil
	case native.CallableUInt32:
		return int64(C.ompphp_callable_get_u32(value)), nil
	case native.CallableInt64:
		return int64(C.ompphp_callable_get_i64(value)), nil
	case native.CallableUInt64:
		return native.CallableUInt64Value(strconv.FormatUint(uint64(C.ompphp_callable_get_u64(value)), 10)), nil
	case native.CallableFloat:
		return float64(C.ompphp_callable_get_float(value)), nil
	case native.CallableDouble:
		return float64(C.ompphp_callable_get_double(value)), nil
	case native.CallableString, native.CallableBytes:
		return C.GoStringN((*C.char)(C.ompphp_callable_get_data(value)), C.int(C.ompphp_callable_get_length(value))), nil
	case native.CallableEntity:
		return native.CallableEntityValue{Type: uint32(C.ompphp_callable_get_entity_type(value)), ID: strconv.FormatUint(uint64(C.ompphp_callable_get_entity_id(value)), 10)}, nil
	default:
		return nil, fmt.Errorf("unsupported callable result type %d", value._type)
	}
}

func (capiGateway) ComponentInvoke(uid uint64, name string, arguments []any) (any, error) {
	handle := C.ompphp_component_find(C.uint64_t(uid))
	if handle == nil {
		return nil, fmt.Errorf("component %016x is unavailable", uid)
	}
	nameData := C.CBytes([]byte(name))
	defer C.free(nameData)
	callable := C.ompphp_callable_find(handle, (*C.char)(nameData), C.uint32_t(len(name)))
	if callable == nil {
		return nil, &native.CallableError{Code: int64(C.OMPCallableError_NotFound), Message: fmt.Sprintf("callable %q is unavailable", name)}
	}
	descriptor := C.ompphp_callable_descriptor(callable)
	if descriptor == nil {
		return nil, &native.CallableError{Code: int64(C.OMPCallableError_InvalidHandle), Message: "callable is no longer valid"}
	}
	if len(arguments) > int(descriptor.parameter_count) {
		return nil, &native.CallableError{Code: int64(C.OMPCallableError_ArgumentCount), Message: "too many callable arguments"}
	}
	var args *C.struct_OMPCallableValue
	if len(arguments) != 0 {
		args = (*C.struct_OMPCallableValue)(C.calloc(C.size_t(len(arguments)), C.sizeof_struct_OMPCallableValue))
		if args == nil {
			return nil, fmt.Errorf("allocate callable arguments")
		}
		defer C.free(unsafe.Pointer(args))
	}
	var allocations []unsafe.Pointer
	for index, argument := range arguments {
		parameter := (*C.struct_OMPCallableParameter)(unsafe.Add(unsafe.Pointer(descriptor.parameters), uintptr(index)*C.sizeof_struct_OMPCallableParameter))
		target := (*C.struct_OMPCallableValue)(unsafe.Add(unsafe.Pointer(args), uintptr(index)*C.sizeof_struct_OMPCallableValue))
		storage, err := setCallableArgument(target, native.CallableType(parameter._type), argument)
		if err != nil {
			for _, item := range allocations {
				C.free(item)
			}
			return nil, fmt.Errorf("callable %s argument %d (%s): %w", name, index+1, callableString(parameter.name), err)
		}
		if storage != nil {
			allocations = append(allocations, storage)
		}
	}
	defer func() {
		for _, item := range allocations {
			C.free(item)
		}
	}()
	maxOutput := configuredLimit("OMPPHP_CALLABLE_MAX_OUTPUT_BYTES", 1<<20)
	outputData := C.malloc(C.size_t(maxOutput))
	if outputData == nil {
		return nil, fmt.Errorf("allocate callable output")
	}
	defer C.free(outputData)
	output := C.struct_OMPCallableOutputBuffer{abi_version: C.OMP_CALLABLE_ABI_VERSION, struct_size: C.sizeof_struct_OMPCallableOutputBuffer, data: (*C.uint8_t)(outputData), capacity: C.uint32_t(maxOutput)}
	errorData := C.malloc(1024)
	if errorData == nil {
		return nil, fmt.Errorf("allocate callable error")
	}
	defer C.free(errorData)
	callError := C.struct_OMPCallableError{abi_version: C.OMP_CALLABLE_ABI_VERSION, struct_size: C.sizeof_struct_OMPCallableError, message: C.struct_CAPIStringBuffer{data: (*C.char)(errorData), capacity: 1024}}
	var result C.struct_OMPCallableValue
	C.ompphp_callable_value_init(&result, C.uint32_t(descriptor.return_type))
	if !bool(C.ompphp_callable_invoke(callable, args, C.uint32_t(len(arguments)), &result, &output, &callError)) {
		messageLength := min(uint32(callError.message.len), uint32(callError.message.capacity))
		message := C.GoStringN((*C.char)(errorData), C.int(messageLength))
		if message == "" {
			message = "callable invocation failed"
		}
		return nil, &native.CallableError{Code: int64(callError.code), Message: message}
	}
	if native.CallableType(result._type) == native.CallableString || native.CallableType(result._type) == native.CallableBytes {
		return C.GoStringN((*C.char)(outputData), C.int(output.length)), nil
	}
	return decodeCallableValue(&result)
}

func (capiGateway) ComponentWatch(uid uint64) (uint64, error) {
	extendedState.Lock()
	defer extendedState.Unlock()
	handle := C.ompphp_component_find(C.uint64_t(uid))
	if handle == nil {
		return 0, fmt.Errorf("component %016x is unavailable", uid)
	}
	token := extendedState.next
	extendedState.next++
	watch := C.ompphp_component_watch(handle, C.uintptr_t(token))
	if watch == nil {
		return 0, fmt.Errorf("component watch was rejected")
	}
	extendedState.watches[token], extendedState.watchUIDs[token] = watch, uid
	return token, nil
}

func (capiGateway) ComponentUnwatch(token uint64) bool {
	extendedState.Lock()
	defer extendedState.Unlock()
	watch := extendedState.watches[token]
	if watch == nil {
		return false
	}
	delete(extendedState.watches, token)
	delete(extendedState.watchUIDs, token)
	return bool(C.ompphp_component_unwatch(watch))
}

func (capiGateway) NetworkSubscribe(direction, id int32, priority int8, all bool) (uint64, error) {
	extendedState.Lock()
	defer extendedState.Unlock()
	if err := validateNetworkSubscription(direction, id, all); err != nil {
		return 0, err
	}
	if len(extendedState.subscriptions) >= extendedState.maxSubscriptions {
		return 0, fmt.Errorf("network subscription limit of %d reached", extendedState.maxSubscriptions)
	}
	token := extendedState.next
	extendedState.next++
	subscription := C.ompphp_network_subscribe(C.int32_t(direction), C.int32_t(id), C.int8_t(priority), C.bool(all), C.uintptr_t(token))
	if subscription == nil {
		return 0, fmt.Errorf("network subscription was rejected")
	}
	extendedState.subscriptions[token] = subscription
	return token, nil
}

func (capiGateway) NetworkUnsubscribe(id uint64) bool {
	extendedState.Lock()
	defer extendedState.Unlock()
	subscription := extendedState.subscriptions[id]
	if subscription == nil {
		return false
	}
	delete(extendedState.subscriptions, id)
	return bool(C.ompphp_network_unsubscribe(subscription))
}

func (capiGateway) NetworkSend(request native.NetworkSendRequest) (uint32, error) {
	extendedState.Lock()
	maxBytes := extendedState.maxBytes
	extendedState.Unlock()
	if err := validateNetworkPayload(request.Data, request.BitLength, maxBytes); err != nil {
		return 0, err
	}
	if request.PlayerID < -1 {
		return 0, fmt.Errorf("player ID must be -1 or greater")
	}
	if request.Channel < 0 {
		return 0, fmt.Errorf("network channel must not be negative")
	}
	if request.RPC && request.MessageID < 0 {
		return 0, fmt.Errorf("RPC ID must not be negative")
	}
	bytes := unsafe.StringData(request.Data)
	if request.PlayerID < 0 {
		if request.RPC {
			return uint32(C.ompphp_network_broadcast_rpc(C.int32_t(request.MessageID), (*C.uint8_t)(bytes), C.uint32_t(request.BitLength), C.int32_t(request.Channel), C.bool(request.DispatchEvents))), nil
		}
		return uint32(C.ompphp_network_broadcast_packet((*C.uint8_t)(bytes), C.uint32_t(request.BitLength), C.int32_t(request.Channel), C.bool(request.DispatchEvents))), nil
	}
	player := C.ompphp_player_from_id(C.int32_t(request.PlayerID))
	if player == nil {
		return 0, fmt.Errorf("player %d is unavailable", request.PlayerID)
	}
	var sent bool
	if request.RPC {
		sent = bool(C.ompphp_network_send_rpc(player, C.int32_t(request.MessageID), (*C.uint8_t)(bytes), C.uint32_t(request.BitLength), C.int32_t(request.Channel), C.bool(request.DispatchEvents)))
	} else {
		sent = bool(C.ompphp_network_send_packet(player, (*C.uint8_t)(bytes), C.uint32_t(request.BitLength), C.int32_t(request.Channel), C.bool(request.DispatchEvents)))
	}
	if sent {
		return 1, nil
	}
	return 0, nil
}

func (capiGateway) NetworkTypes() []int64 {
	count := uint32(C.ompphp_network_count())
	result := make([]int64, 0, count)
	for index := uint32(0); index < count; index++ {
		result = append(result, int64(C.ompphp_network_type(C.uint32_t(index))))
	}
	return result
}

func (capiGateway) NetworkStats() native.NetworkStats {
	extendedState.Lock()
	defer extendedState.Unlock()
	return native.NetworkStats{Subscriptions: int64(len(extendedState.subscriptions)), Callbacks: extendedState.callbacks, Dropped: extendedState.dropped, Rejected: extendedState.rejected, CallbackNS: extendedState.callbackNS}
}

func (capiGateway) SetNetworkDispatcher(dispatch func(native.NetworkMessage) native.NetworkResponse) {
	extendedState.Lock()
	extendedState.maxBytes = configuredLimit("OMPPHP_NETWORK_MAX_BYTES", 1<<20)
	extendedState.maxSubscriptions = configuredLimit("OMPPHP_NETWORK_MAX_SUBSCRIPTIONS", 1024)
	extendedState.dispatch = dispatch
	extendedState.Unlock()
}
func (capiGateway) SetComponentDispatcher(dispatch func(uint64, uint64)) {
	extendedState.Lock()
	extendedState.componentDispatch = dispatch
	extendedState.Unlock()
}
func (capiGateway) CloseExtended() {
	extendedState.Lock()
	defer extendedState.Unlock()
	for id, subscription := range extendedState.subscriptions {
		C.ompphp_network_unsubscribe(subscription)
		delete(extendedState.subscriptions, id)
	}
	for id, watch := range extendedState.watches {
		C.ompphp_component_unwatch(watch)
		delete(extendedState.watches, id)
		delete(extendedState.watchUIDs, id)
	}
	extendedState.dispatch = nil
	extendedState.componentDispatch = nil
	extendedState.depth = 0
}

//export OMPPHPDispatchComponentInvalidated
func OMPPHPDispatchComponentInvalidated(rawToken C.uintptr_t) {
	token := uint64(rawToken)
	extendedState.Lock()
	dispatch, uid := extendedState.componentDispatch, extendedState.watchUIDs[token]
	delete(extendedState.watches, token)
	delete(extendedState.watchUIDs, token)
	extendedState.Unlock()
	if dispatch != nil {
		dispatch(token, uid)
	}
}

//export OMPPHPDispatchNetwork
func OMPPHPDispatchNetwork(token C.uintptr_t, player unsafe.Pointer, messageID C.int32_t, buffer *C.struct_OMPNetBuffer) C.int {
	extendedState.Lock()
	dispatch := extendedState.dispatch
	if dispatch == nil || buffer == nil {
		extendedState.rejected++
		extendedState.Unlock()
		return C.int(C.OMPNetResult_Continue)
	}
	bytes := (uint32(buffer.bit_length) + 7) / 8
	if int(bytes) > extendedState.maxBytes || extendedState.depth != 0 {
		extendedState.rejected++
		extendedState.Unlock()
		return C.int(C.OMPNetResult_Continue)
	}
	extendedState.depth++
	extendedState.Unlock()
	started := time.Now()
	defer func() {
		extendedState.Lock()
		extendedState.depth--
		extendedState.callbacks++
		extendedState.callbackNS += time.Since(started).Nanoseconds()
		extendedState.Unlock()
	}()
	message := native.NetworkMessage{SubscriptionID: uint64(token), PlayerID: int64(C.ompphp_network_player_id(player)), MessageID: int64(messageID), Data: C.GoStringN((*C.char)(unsafe.Pointer(buffer.data)), C.int(bytes)), BitLength: int64(buffer.bit_length), ReadOffsetBits: int64(buffer.read_offset_bits)}
	response := dispatch(message)
	if response.BitLength < 0 || response.BitLength > int64(len(response.Data))*8 {
		return C.int(C.OMPNetResult_Continue)
	}
	if !bool(C.ompphp_network_apply(buffer, (*C.char)(unsafe.Pointer(unsafe.StringData(response.Data))), C.uint32_t(len(response.Data)), C.uint32_t(response.BitLength))) {
		extendedState.Lock()
		extendedState.rejected++
		extendedState.Unlock()
		return C.int(C.OMPNetResult_Continue)
	}
	if response.Drop {
		extendedState.Lock()
		extendedState.dropped++
		extendedState.Unlock()
		return C.int(C.OMPNetResult_Drop)
	}
	return C.int(C.OMPNetResult_Continue)
}
