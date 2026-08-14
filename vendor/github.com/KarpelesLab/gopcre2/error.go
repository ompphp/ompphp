package gopcre2

import (
	"errors"
	"fmt"
)

// CompileError describes a failure to compile a PCRE2 pattern.
type CompileError struct {
	Pattern string // the pattern that failed
	Offset  int    // byte offset in pattern where the error was detected
	Message string // description of the error
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("gopcre2: compile error at offset %d: %s", e.Offset, e.Message)
}

// MatchError describes a failure during matching.
type MatchError struct {
	Message string
}

func (e *MatchError) Error() string {
	return fmt.Sprintf("gopcre2: match error: %s", e.Message)
}

var (
	// ErrMatchLimit is returned when the match step limit is exceeded.
	ErrMatchLimit = errors.New("gopcre2: match limit exceeded")

	// ErrDepthLimit is returned when the recursion depth limit is exceeded.
	ErrDepthLimit = errors.New("gopcre2: depth limit exceeded")

	// ErrHeapLimit is returned when the heap memory limit is exceeded.
	ErrHeapLimit = errors.New("gopcre2: heap limit exceeded")

	// ErrNoMatch is returned when no match is found (internal use).
	errNoMatch = errors.New("gopcre2: no match")
)
