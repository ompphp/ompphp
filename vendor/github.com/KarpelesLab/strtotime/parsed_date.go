package strtotime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// ParsedDate holds the intermediate result of parsing a strtotime-style string.
// It is the return type of DateParse and the internal working state of StrToTime.
// Its JSON form is byte-compatible with PHP's date_parse() output: unset scalar
// fields marshal as false, field order matches PHP, and timezone metadata only
// appears when a timezone was explicitly present in the input.
type ParsedDate struct {
	Year     OptInt
	Month    OptInt
	Day      OptInt
	Hour     OptInt
	Minute   OptInt
	Second   OptInt
	Fraction OptFloat

	WarningCount int
	Warnings     map[int]string
	ErrorCount   int
	Errors       map[int]string

	IsLocaltime bool
	ZoneType    int // 1 = UTC offset, 2 = abbreviation, 3 = timezone identifier
	Zone        int // offset in seconds (zone_type 1 and 2)
	IsDST       bool
	TzAbbr      string
	TzID        string

	Relative *Relative

	// Internal materialization helpers. Not exported via JSON.
	sourceLoc       *time.Location // explicit timezone parsed from input
	materialized    time.Time      // when a parser couldn't cleanly decompose
	hasMaterialized bool
	// relativeApplied records that the parser already folded the Relative
	// block into the materialized time. Materialize will then skip
	// re-applying Relative (avoids double-counting). Used by the token-based
	// parser which mutates time.Time directly as it parses.
	relativeApplied bool
	// fractionDefaultsZero flags that the parser recognised a colon-style
	// time portion. PHP emits fraction=0 (instead of false) in that case,
	// even when the input has no "." fractional separator. Bare HHMM inputs
	// leave fraction=false.
	fractionDefaultsZero bool
}

// Relative captures the relative-time portion of a parsed expression.
// Year/Month/Day/Hour/Minute/Second are always emitted in JSON (zero when absent).
// Weekday is only emitted when a named weekday offset was present
// (e.g. "next monday"). Weekdays is emitted for compound weekday expressions.
type Relative struct {
	Year     int
	Month    int
	Day      int
	Hour     int
	Minute   int
	Second   int
	Weekday  OptInt
	Weekdays OptInt

	// Internal flag: "first day of" / "last day of" semantics.
	// 0 = none, 1 = first day, 2 = last day.
	firstLastDayMode int
}

// OptInt is an integer that distinguishes "unset" from zero. When unset it
// marshals as JSON `false` (PHP date_parse compatibility).
type OptInt struct {
	V   int
	Set bool
}

// Int returns the value, or 0 when unset.
func (o OptInt) Int() int { return o.V }

// MarshalJSON emits the integer value or false when unset.
func (o OptInt) MarshalJSON() ([]byte, error) {
	if !o.Set {
		return []byte("false"), nil
	}
	return []byte(strconv.Itoa(o.V)), nil
}

// OptFloat is a float64 that distinguishes "unset" from zero.
type OptFloat struct {
	V   float64
	Set bool
}

// Float returns the value, or 0 when unset.
func (o OptFloat) Float() float64 { return o.V }

// MarshalJSON emits the float value or false when unset.
func (o OptFloat) MarshalJSON() ([]byte, error) {
	if !o.Set {
		return []byte("false"), nil
	}
	return json.Marshal(o.V)
}

// newParsedDate returns an initialized ParsedDate with empty warning/error maps.
func newParsedDate() *ParsedDate {
	return &ParsedDate{
		Warnings: map[int]string{},
		Errors:   map[int]string{},
	}
}

// SetDate records year/month/day from a parsed absolute date.
func (pd *ParsedDate) SetDate(year, month, day int) {
	pd.Year = OptInt{V: year, Set: true}
	pd.Month = OptInt{V: month, Set: true}
	pd.Day = OptInt{V: day, Set: true}
}

// SetYear records only the year.
func (pd *ParsedDate) SetYear(year int) { pd.Year = OptInt{V: year, Set: true} }

// SetMonth records only the month.
func (pd *ParsedDate) SetMonth(month int) { pd.Month = OptInt{V: month, Set: true} }

// SetDay records only the day.
func (pd *ParsedDate) SetDay(day int) { pd.Day = OptInt{V: day, Set: true} }

// SetTime records hour/minute/second from a parsed clock time. It also
// flags fractionDefaultsZero so that fraction marshals as 0 (not false)
// when no fractional part was explicitly present — this matches PHP for
// every clock-time form except bare HHMM (which must clear the flag).
func (pd *ParsedDate) SetTime(hour, minute, second int) {
	pd.Hour = OptInt{V: hour, Set: true}
	pd.Minute = OptInt{V: minute, Set: true}
	pd.Second = OptInt{V: second, Set: true}
	pd.fractionDefaultsZero = true
}

// SetHour records only the hour.
func (pd *ParsedDate) SetHour(hour int) { pd.Hour = OptInt{V: hour, Set: true} }

// SetMinute records only the minute.
func (pd *ParsedDate) SetMinute(minute int) { pd.Minute = OptInt{V: minute, Set: true} }

// SetSecond records only the second.
func (pd *ParsedDate) SetSecond(second int) { pd.Second = OptInt{V: second, Set: true} }

// SetFraction records a fractional-second component. PHP truncates the
// fraction to microsecond precision (6 digits), so we match that behavior.
func (pd *ParsedDate) SetFraction(f float64) {
	// Truncate to microseconds.
	f = float64(int64(f*1e6)) / 1e6
	pd.Fraction = OptFloat{V: f, Set: true}
}

// SetTZOffset records a numeric UTC offset (zone_type 1).
// offsetSeconds is the signed number of seconds east of UTC.
func (pd *ParsedDate) SetTZOffset(loc *time.Location, offsetSeconds int) {
	pd.IsLocaltime = true
	pd.ZoneType = 1
	pd.Zone = offsetSeconds
	pd.IsDST = false
	pd.TzAbbr = ""
	pd.TzID = ""
	pd.sourceLoc = loc
}

// SetTZAbbreviation records a timezone abbreviation (zone_type 2) like "EST".
func (pd *ParsedDate) SetTZAbbreviation(loc *time.Location, abbr string, offsetSeconds int, isDST bool) {
	pd.IsLocaltime = true
	pd.ZoneType = 2
	pd.Zone = offsetSeconds
	pd.IsDST = isDST
	pd.TzAbbr = abbr
	pd.TzID = ""
	pd.sourceLoc = loc
}

// SetTZIdentifier records an IANA timezone identifier (zone_type 3) like
// "Asia/Tokyo". Matches PHP: zone_type 3 does not emit zone / is_dst.
func (pd *ParsedDate) SetTZIdentifier(loc *time.Location, id string) {
	pd.IsLocaltime = true
	pd.ZoneType = 3
	pd.Zone = 0
	pd.IsDST = false
	pd.TzAbbr = ""
	pd.TzID = id
	pd.sourceLoc = loc
}

// relative returns the Relative block, creating it on first use.
func (pd *ParsedDate) relative() *Relative {
	if pd.Relative == nil {
		pd.Relative = &Relative{}
	}
	return pd.Relative
}

// AddRelative adds (amount, unit) to the relative block. Unit must be a
// canonical unit name from const.go (UnitYear/UnitMonth/UnitDay/...).
func (pd *ParsedDate) AddRelative(unit string, amount int) {
	r := pd.relative()
	switch normalizeTimeUnit(unit) {
	case UnitYear:
		r.Year += amount
	case UnitMonth:
		r.Month += amount
	case UnitWeek:
		r.Day += amount * 7
	case UnitDay:
		r.Day += amount
	case UnitHour:
		r.Hour += amount
	case UnitMinute:
		r.Minute += amount
	case UnitSecond:
		r.Second += amount
	case UnitWeekDay:
		r.Weekdays = OptInt{V: r.Weekdays.V + amount, Set: true}
	}
}

// SetRelativeWeekday records a "next monday" / "last friday" style weekday
// offset. weekday is 0=Sunday..6=Saturday. PHP uses 0=Sun semantics.
func (pd *ParsedDate) SetRelativeWeekday(weekday int) {
	r := pd.relative()
	r.Weekday = OptInt{V: weekday, Set: true}
}

// SetFirstLastDayOf marks a "first day of" (mode=1) or "last day of" (mode=2)
// expression. The caller still records any month/year offset in Relative.
func (pd *ParsedDate) SetFirstLastDayOf(mode int) {
	pd.relative().firstLastDayMode = mode
}

// AddWarning records a warning at the given character position.
func (pd *ParsedDate) AddWarning(pos int, msg string) {
	if pd.Warnings == nil {
		pd.Warnings = map[int]string{}
	}
	pd.Warnings[pos] = msg
	pd.WarningCount++
}

// AddError records an error at the given character position.
func (pd *ParsedDate) AddError(pos int, msg string) {
	if pd.Errors == nil {
		pd.Errors = map[int]string{}
	}
	pd.Errors[pos] = msg
	pd.ErrorCount++
}

// setMaterialized stashes a fully-built time.Time for parsers that cannot
// cleanly decompose their output (e.g. compound expressions). Materialize
// will prefer this over the component fields.
func (pd *ParsedDate) setMaterialized(t time.Time) {
	pd.materialized = t
	pd.hasMaterialized = true
}

// Materialize renders the ParsedDate into a time.Time. Unset components fall
// back to the corresponding field of now. Relative offsets are applied after
// the base date is assembled. Returns an error if the extracted date or time
// components are themselves invalid (e.g. month 13).
func (pd *ParsedDate) Materialize(now time.Time, loc *time.Location) (time.Time, error) {
	if pd.hasMaterialized && (pd.Relative == nil || pd.relativeApplied) {
		return pd.materialized, nil
	}

	effectiveLoc := loc
	if pd.sourceLoc != nil {
		effectiveLoc = pd.sourceLoc
	}

	var t time.Time
	if pd.hasMaterialized {
		t = pd.materialized
		if pd.relativeApplied {
			return t, nil
		}
	} else {
		baseNow := now
		if effectiveLoc != nil && baseNow.Location() != effectiveLoc {
			baseNow = baseNow.In(effectiveLoc)
		}

		year := baseNow.Year()
		month := int(baseNow.Month())
		day := baseNow.Day()
		hour := 0
		minute := 0
		second := 0
		nsec := 0

		if pd.Year.Set {
			year = pd.Year.V
		}
		if pd.Month.Set {
			month = pd.Month.V
		}
		if pd.Day.Set {
			day = pd.Day.V
		}
		if pd.Hour.Set {
			hour = pd.Hour.V
		}
		if pd.Minute.Set {
			minute = pd.Minute.V
		}
		if pd.Second.Set {
			second = pd.Second.V
		}
		if pd.Fraction.Set {
			nsec = int(pd.Fraction.V * 1e9)
		}

		if !IsValidDate(year, month, day) {
			return time.Time{}, NewInvalidDateError(year, month, day)
		}
		if !IsValidTime(hour, minute, second) {
			return time.Time{}, NewInvalidTimeError(hour, minute, second)
		}

		t = time.Date(year, time.Month(month), day, hour, minute, second, nsec, effectiveLoc)
	}

	if pd.Relative != nil {
		t = applyRelative(t, pd.Relative, effectiveLoc)
	}

	return t, nil
}

// applyRelative applies a Relative block to a base time.
func applyRelative(t time.Time, r *Relative, loc *time.Location) time.Time {
	// Order matches PHP timelib: firstLastDayOf first, then relative units,
	// then weekday snap.
	if r.firstLastDayMode != 0 {
		// Adjust year/month first, then snap to first/last day.
		year := t.Year() + r.Year
		month := int(t.Month()) + r.Month
		firstOfTarget := time.Date(year, time.Month(month), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
		var day int
		if r.firstLastDayMode == 1 {
			day = 1
		} else {
			day = daysInMonth(firstOfTarget.Year(), firstOfTarget.Month())
		}
		t = time.Date(firstOfTarget.Year(), firstOfTarget.Month(), day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
		// Non-month/year units still apply below via the remaining unit offsets.
		if r.Day != 0 {
			t = applyTimeOffset(t, r.Day, UnitDay)
		}
		if r.Hour != 0 {
			t = applyTimeOffset(t, r.Hour, UnitHour)
		}
		if r.Minute != 0 {
			t = applyTimeOffset(t, r.Minute, UnitMinute)
		}
		if r.Second != 0 {
			t = applyTimeOffset(t, r.Second, UnitSecond)
		}
	} else {
		if r.Year != 0 {
			t = applyTimeOffset(t, r.Year, UnitYear)
		}
		if r.Month != 0 {
			t = applyTimeOffset(t, r.Month, UnitMonth)
		}
		if r.Day != 0 {
			t = applyTimeOffset(t, r.Day, UnitDay)
		}
		if r.Hour != 0 {
			t = applyTimeOffset(t, r.Hour, UnitHour)
		}
		if r.Minute != 0 {
			t = applyTimeOffset(t, r.Minute, UnitMinute)
		}
		if r.Second != 0 {
			t = applyTimeOffset(t, r.Second, UnitSecond)
		}
	}

	if r.Weekdays.Set && r.Weekdays.V != 0 {
		t = addWeekdays(t, r.Weekdays.V)
	}
	if r.Weekday.Set {
		cur := int(t.Weekday())
		delta := (r.Weekday.V - cur + 7) % 7
		t = t.AddDate(0, 0, delta)
	}

	return t
}

// MarshalJSON produces PHP date_parse-compatible JSON. Field order matches
// PHP's insertion order. Timezone fields are conditionally emitted based on
// zone_type. The relative block is only emitted when present.
func (pd *ParsedDate) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	writeField := func(name string, raw []byte, first *bool) {
		if !*first {
			buf.WriteByte(',')
		}
		*first = false
		buf.WriteByte('"')
		buf.WriteString(name)
		buf.WriteString(`":`)
		buf.Write(raw)
	}

	marshal := func(v interface{}) []byte {
		b, _ := json.Marshal(v)
		return b
	}

	first := true
	writeField("year", marshalJSONOrDefault(pd.Year), &first)
	writeField("month", marshalJSONOrDefault(pd.Month), &first)
	writeField("day", marshalJSONOrDefault(pd.Day), &first)
	writeField("hour", marshalJSONOrDefault(pd.Hour), &first)
	writeField("minute", marshalJSONOrDefault(pd.Minute), &first)
	writeField("second", marshalJSONOrDefault(pd.Second), &first)
	fraction := pd.Fraction
	if !fraction.Set && pd.fractionDefaultsZero {
		fraction = OptFloat{V: 0, Set: true}
	}
	writeField("fraction", marshalJSONOrDefault(fraction), &first)

	writeField("warning_count", marshal(pd.WarningCount), &first)
	writeField("warnings", marshalMapIntString(pd.Warnings), &first)
	writeField("error_count", marshal(pd.ErrorCount), &first)
	writeField("errors", marshalMapIntString(pd.Errors), &first)

	writeField("is_localtime", marshal(pd.IsLocaltime), &first)

	if pd.IsLocaltime {
		writeField("zone_type", marshal(pd.ZoneType), &first)
		switch pd.ZoneType {
		case 1:
			writeField("zone", marshal(pd.Zone), &first)
			writeField("is_dst", marshal(pd.IsDST), &first)
		case 2:
			writeField("zone", marshal(pd.Zone), &first)
			writeField("is_dst", marshal(pd.IsDST), &first)
			writeField("tz_abbr", marshal(pd.TzAbbr), &first)
		case 3:
			// UTC is the only tz where PHP emits both tz_abbr and tz_id.
			if pd.TzAbbr != "" {
				writeField("tz_abbr", marshal(pd.TzAbbr), &first)
			}
			writeField("tz_id", marshal(pd.TzID), &first)
		}
	}

	if pd.Relative != nil {
		rb, err := json.Marshal(pd.Relative)
		if err != nil {
			return nil, err
		}
		writeField("relative", rb, &first)
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// marshalJSONOrDefault calls MarshalJSON on a json.Marshaler, falling back
// to a simple encoding for plain values.
func marshalJSONOrDefault(v json.Marshaler) []byte {
	b, err := v.MarshalJSON()
	if err != nil {
		return []byte("null")
	}
	return b
}

// marshalMapIntString emits an int-keyed map the way PHP's json_encode
// does: a JSON array when the keys are a contiguous sequence starting at 0,
// a JSON object otherwise.
func marshalMapIntString(m map[int]string) []byte {
	if len(m) == 0 {
		return []byte("[]")
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	// Sequential 0..N-1 → JSON array (PHP behaviour for "list" arrays).
	isList := true
	for i, k := range keys {
		if k != i {
			isList = false
			break
		}
	}

	var buf bytes.Buffer
	if isList {
		buf.WriteByte('[')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			vb, _ := json.Marshal(m[k])
			buf.Write(vb)
		}
		buf.WriteByte(']')
		return buf.Bytes()
	}
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%q:", strconv.Itoa(k))
		vb, _ := json.Marshal(m[k])
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// MarshalJSON for Relative always emits the six integer fields and
// conditionally includes weekday, weekdays, first_day_of_month,
// last_day_of_month — matching PHP's date_parse relative layout.
func (r *Relative) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	fmt.Fprintf(&buf, `"year":%d,"month":%d,"day":%d,"hour":%d,"minute":%d,"second":%d`,
		r.Year, r.Month, r.Day, r.Hour, r.Minute, r.Second)
	if r.Weekday.Set {
		fmt.Fprintf(&buf, `,"weekday":%d`, r.Weekday.V)
	}
	if r.Weekdays.Set {
		fmt.Fprintf(&buf, `,"weekdays":%d`, r.Weekdays.V)
	}
	if r.firstLastDayMode == 1 {
		buf.WriteString(`,"first_day_of_month":true`)
	} else if r.firstLastDayMode == 2 {
		buf.WriteString(`,"last_day_of_month":true`)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Time renders the ParsedDate into a time.Time using the current wall-clock
// time for unset components and the caller-supplied location for unset
// timezone. It is a convenience wrapper around Materialize.
func (pd *ParsedDate) Time(loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	return pd.Materialize(time.Now().In(loc), loc)
}

// firstError returns the first recorded error as a Go error, suitable for
// returning from StrToTime when DateParse found problems.
func (pd *ParsedDate) firstError() error {
	if pd.ErrorCount == 0 {
		return nil
	}
	keys := make([]int, 0, len(pd.Errors))
	for k := range pd.Errors {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	if len(keys) == 0 {
		return fmt.Errorf("parse error")
	}
	return fmt.Errorf("%s", pd.Errors[keys[0]])
}
