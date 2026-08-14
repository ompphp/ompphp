package strtotime

import (
	"strings"
	"time"
)

// DateParse parses a date/time string the same way StrToTime does and returns
// a ParsedDate describing which components were present in the input.
//
// The returned value marshals to JSON in a form equivalent to PHP's
// date_parse() output: missing scalar components appear as false, the
// "relative" sub-object is emitted when the input contains relative offsets,
// and timezone metadata is emitted only when the input contains an explicit
// timezone.
//
// Unlike StrToTime, DateParse does not apply any relative offsets — they are
// reported verbatim in the "relative" block.
func DateParse(str string) *ParsedDate {
	pd := newParsedDate()
	// PHP's timelib emits "Empty string" only for a literal zero-length input.
	// Whitespace-only inputs are trimmed and then parsed further, which means
	// they return a structurally-empty result with no error (bug35499).
	if str == "" {
		pd.AddError(0, "Empty string")
		return pd
	}
	str = strings.ToLower(strings.TrimSpace(str))
	if str == "" {
		return pd
	}

	// Use a stable zero base time so parsers that consult `now` (e.g. for
	// "today"-relative cases) produce deterministic results. Components
	// derived purely from the input string are independent of this base.
	var zeroNow time.Time
	loc := time.UTC

	dispatchStrToTime(str, zeroNow, loc, nil, pd)
	return pd
}
