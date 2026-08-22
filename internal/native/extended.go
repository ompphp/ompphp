package native

type ComponentInfo struct {
	UID    uint64
	Name   string
	Major  int64
	Minor  int64
	Patch  int64
	PreRel int64
	Type   int64
}

type CallableType int64

const (
	CallableNull CallableType = iota
	CallableBool
	CallableInt32
	CallableUInt32
	CallableInt64
	CallableUInt64
	CallableFloat
	CallableDouble
	CallableString
	CallableBytes
	CallableEntity
)

type CallableParameter struct {
	Name       string
	Type       CallableType
	Optional   bool
	HasDefault bool
	Default    any
}

type CallableDescriptor struct {
	Name          string
	Documentation string
	Parameters    []CallableParameter
	ReturnType    CallableType
	Deprecated    bool
	MayCallback   bool
}

type CallableEntityValue struct {
	Type uint32
	ID   string
}

type CallableUInt64Value string

type CallableError struct {
	Code    int64
	Message string
}

func (e *CallableError) Error() string { return e.Message }

type NetworkMessage struct {
	SubscriptionID uint64
	PlayerID       int64
	MessageID      int64
	Data           string
	BitLength      int64
	ReadOffsetBits int64
}

type NetworkResponse struct {
	Drop           bool
	Data           string
	BitLength      int64
	ReadOffsetBits int64
}

type NetworkSendRequest struct {
	RPC            bool
	PlayerID       int32
	MessageID      int32
	Data           string
	BitLength      uint32
	Channel        int32
	DispatchEvents bool
}

type NetworkStats struct {
	Subscriptions int64
	Callbacks     int64
	Dropped       int64
	Rejected      int64
	CallbackNS    int64
}

type ExtendedGateway interface {
	Component(uint64) (ComponentInfo, bool)
	ComponentSupports(uint64, uint64, uint32, uint32) bool
	ComponentCallables(uint64) ([]CallableDescriptor, error)
	ComponentInvoke(uint64, string, []any) (any, error)
	ComponentWatch(uint64) (uint64, error)
	ComponentUnwatch(uint64) bool
	NetworkSubscribe(int32, int32, int8, bool) (uint64, error)
	NetworkUnsubscribe(uint64) bool
	NetworkSend(NetworkSendRequest) (uint32, error)
	NetworkTypes() []int64
	NetworkStats() NetworkStats
	SetNetworkDispatcher(func(NetworkMessage) NetworkResponse)
	SetComponentDispatcher(func(uint64, uint64))
	CloseExtended()
}
