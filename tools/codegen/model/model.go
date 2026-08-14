package model

type Model struct {
	Functions []Function
	Events    []Event
}

type Function struct {
	Group      string
	Name       string
	ReturnType string
	Parameters []Parameter
}

type Event struct {
	Group      string
	Name       string
	ReturnType string
	Parameters []Parameter
}

type Parameter struct {
	Name     string
	Type     string
	Nullable bool
	Output   bool
}
