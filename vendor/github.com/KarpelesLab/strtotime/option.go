package strtotime

import "time"

type Option interface {
	isOption() bool
}

// Rel represents a relative time to use as base
type Rel time.Time

func (r Rel) isOption() bool {
	return true
}

// InTZ sets a timezone to use for parsing
func InTZ(loc *time.Location) Option {
	return tzOption{loc: loc}
}

// tzOption is an internal type for timezone options
type tzOption struct {
	loc *time.Location
}

func (t tzOption) isOption() bool {
	return true
}
