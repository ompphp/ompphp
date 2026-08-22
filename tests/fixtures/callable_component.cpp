#include <sdk.hpp>
#include <ompcapi.h>

#include <cstdint>

namespace
{
constexpr uint64_t ComponentUID = 0x4F4D505043414C4CULL;
OMPAPI_t api {};

OMPCallableStringView text(const char* value, uint32_t length)
{
	return { OMP_CALLABLE_ABI_VERSION, sizeof(OMPCallableStringView), value, length };
}

OMPCallableValue emptyValue()
{
	OMPCallableValue value {};
	value.abi_version = OMP_CALLABLE_ABI_VERSION;
	value.struct_size = sizeof(value);
	return value;
}

bool add(OMPCallableContext* context, void*)
{
	context->result->type = OMPCallableValueType_Int64;
	context->result->value.int64_value = context->arguments[0].value.int64_value + context->arguments[1].value.int64_value;
	return true;
}

bool maximumUnsigned(OMPCallableContext* context, void*)
{
	context->result->type = OMPCallableValueType_UInt64;
	context->result->value.uint64_value = UINT64_MAX;
	return true;
}

class CallableFixture final : public IComponent
{
public:
	PROVIDE_UID(ComponentUID)
	StringView componentName() const override { return "ompphp callable fixture"; }
	SemanticVersion componentVersion() const override { return { 1, 0, 0, 0 }; }
	void onLoad(ICore*) override {}
	void onInit(IComponentList*) override
	{
		omp_initialize_capi(&api);
	}
	void onReady() override
	{
		if (!api.ComponentInterop.RegisterCallable) return;
		static OMPCallableParameter addParameters[2] {
			{ OMP_CALLABLE_ABI_VERSION, sizeof(OMPCallableParameter), text("left", 4), OMPCallableValueType_Int64, 0, emptyValue() },
			{ OMP_CALLABLE_ABI_VERSION, sizeof(OMPCallableParameter), text("right", 5), OMPCallableValueType_Int64, 0, emptyValue() },
		};
		static OMPCallableDescriptor addDescriptor { OMP_CALLABLE_ABI_VERSION, sizeof(OMPCallableDescriptor), text("add", 3), text("Adds two signed 64-bit integers.", 32), 2, addParameters, OMPCallableValueType_Int64, OMPCallableFlag_MainThreadOnly };
		static OMPCallableDescriptor maximumDescriptor { OMP_CALLABLE_ABI_VERSION, sizeof(OMPCallableDescriptor), text("maximumUnsigned", 15), text("Returns UINT64_MAX.", 19), 0, nullptr, OMPCallableValueType_UInt64, OMPCallableFlag_MainThreadOnly };
		api.ComponentInterop.RegisterCallable(ComponentUID, &addDescriptor, add, nullptr);
		api.ComponentInterop.RegisterCallable(ComponentUID, &maximumDescriptor, maximumUnsigned, nullptr);
	}
	void onFree(IComponent*) override {}
	void free() override { delete this; }
	void reset() override {}
};
}

COMPONENT_ENTRY_POINT()
{
	return new CallableFixture();
}
