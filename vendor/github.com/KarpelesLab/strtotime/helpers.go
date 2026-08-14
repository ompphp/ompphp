package strtotime

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// fixedZone creates a time.FixedZone with a PHP-style "+HH:MM" / "-HH:MM" name
// derived from the offset in seconds.
func fixedZone(offsetSeconds int) *time.Location {
	sign := "+"
	abs := offsetSeconds
	if abs < 0 {
		sign = "-"
		abs = -abs
	}
	h := abs / 3600
	m := (abs % 3600) / 60
	name := fmt.Sprintf("%s%02d:%02d", sign, h, m)
	return time.FixedZone(name, offsetSeconds)
}

// applyTimeOffset applies a time unit offset to the given time.
// It normalizes the unit string and handles all PHP-compatible time arithmetic
// including DST-aware day/hour operations.
func applyTimeOffset(t time.Time, amount int, unit string) time.Time {
	canonical := normalizeTimeUnit(unit)

	switch canonical {
	case UnitDay:
		return addDaysPHP(t, amount)
	case UnitWeek:
		return addDaysPHP(t, amount*7)
	case UnitWeekDay:
		return addWeekdays(t, amount)
	case UnitMonth:
		return t.AddDate(0, amount, 0)
	case UnitYear:
		return t.AddDate(amount, 0, 0)
	case UnitHour:
		// PHP uses wall-clock arithmetic for hours (important for DST transitions)
		y, m, d := t.Date()
		h, mi, s := t.Clock()
		return time.Date(y, m, d, h+amount, mi, s, t.Nanosecond(), t.Location())
	case UnitMinute:
		y, m, d := t.Date()
		h, mi, s := t.Clock()
		return time.Date(y, m, d, h, mi+amount, s, t.Nanosecond(), t.Location())
	case UnitSecond:
		y, m, d := t.Date()
		h, mi, s := t.Clock()
		return time.Date(y, m, d, h, mi, s+amount, t.Nanosecond(), t.Location())
	}
	return t
}

// fixDSTGap adjusts a time that fell into a DST spring-forward gap.
// When time.Date produces a result on the wrong day (Go falls backward),
// this shifts forward to match PHP's behavior (which falls forward).
func fixDSTGap(t time.Time, wantYear int, wantMonth time.Month, wantDay int) time.Time {
	if t.Year() == wantYear && t.Month() == wantMonth && t.Day() == wantDay {
		return t
	}
	// DST gap: shift forward by 1 hour until we land on the right day
	for i := 0; i < 4; i++ {
		t = t.Add(time.Hour)
		if t.Day() == wantDay && t.Month() == wantMonth && t.Year() == wantYear {
			return t
		}
	}
	return t // give up after 4 hours
}

// addDaysPHP adds N calendar days using PHP-compatible DST handling.
// It preserves wall-clock time (like Go's AddDate) but when the result
// falls in a DST gap (non-existent local time), it shifts forward past
// the gap instead of backward (which is what Go's AddDate does).
func addDaysPHP(t time.Time, n int) time.Time {
	result := t.AddDate(0, 0, n)
	// Compute the expected date in UTC to avoid DST-related day shifts
	wantUTC := time.Date(t.Year(), t.Month(), t.Day()+n, 12, 0, 0, 0, time.UTC)
	if result.Year() != wantUTC.Year() || result.Month() != wantUTC.Month() || result.Day() != wantUTC.Day() {
		// DST gap: AddDate landed on wrong day. Use duration-based add instead.
		return t.Add(time.Duration(n) * 24 * time.Hour)
	}
	// Check if DST gap shifted the wall-clock time (Go falls backward, PHP falls forward).
	// Example: 02:30 EST +1 day across spring-forward → Go gives 01:30 EST, PHP gives 03:30 EDT.
	// Fix: use duration-based add which crosses the gap correctly.
	origH, origM, origS := t.Clock()
	resH, resM, resS := result.Clock()
	if origH != resH || origM != resM || origS != resS {
		return t.Add(time.Duration(n) * 24 * time.Hour)
	}
	return result
}

// addWeekdays adds N business days (Mon-Fri) to the given time.
// PHP behavior: if starting on Sat/Sun, snap to Monday first (counts as 1 weekday),
// then continue adding remaining weekdays.
func addWeekdays(t time.Time, n int) time.Time {
	if n == 0 {
		// PHP behavior: 0 weekdays from a weekend snaps to next Monday
		if t.Weekday() == time.Saturday {
			return t.AddDate(0, 0, 2)
		}
		if t.Weekday() == time.Sunday {
			return t.AddDate(0, 0, 1)
		}
		return t
	}

	step := 1
	if n < 0 {
		step = -1
		n = -n
	}

	result := t
	for i := 0; i < n; i++ {
		result = result.AddDate(0, 0, step)
		// Skip weekends
		for result.Weekday() == time.Saturday || result.Weekday() == time.Sunday {
			result = result.AddDate(0, 0, step)
		}
	}
	return result
}

// parseOrdinalPrefix parses a numeric or word ordinal from the beginning of a fields slice.
// Returns the ordinal value (-1 for "last"), whether it's a word ordinal (affects PHP semantics),
// the next field index to process, and whether parsing succeeded.
func parseOrdinalPrefix(fields []string, idx int) (ordinal int, isWord bool, nextIdx int, ok bool) {
	if idx >= len(fields) {
		return 0, false, idx, false
	}

	// Try numeric ordinal
	if n, err := strconv.Atoi(fields[idx]); err == nil {
		if n <= 0 || n > 53 {
			return 0, false, idx, false
		}
		return n, false, idx + 1, true
	}

	// Try word ordinal
	switch strings.ToLower(fields[idx]) {
	case "first", "1st":
		return 1, true, idx + 1, true
	case "second", "2nd":
		return 2, true, idx + 1, true
	case "third", "3rd":
		return 3, true, idx + 1, true
	case "fourth", "4th":
		return 4, true, idx + 1, true
	case "fifth", "5th":
		return 5, true, idx + 1, true
	case "sixth", "6th":
		return 6, true, idx + 1, true
	case "seventh", "7th":
		return 7, true, idx + 1, true
	case "eighth", "8th":
		return 8, true, idx + 1, true
	case "ninth", "9th":
		return 9, true, idx + 1, true
	case "tenth", "10th":
		return 10, true, idx + 1, true
	case "eleventh", "11th":
		return 11, true, idx + 1, true
	case "twelfth", "12th":
		return 12, true, idx + 1, true
	case "last":
		return -1, false, idx + 1, true
	}

	// Bare weekday name implies ordinal=1
	if getDayOfWeek(fields[idx]) >= 0 {
		return 1, false, idx, true // don't advance — weekday parsed next
	}

	return 0, false, idx, false
}

// stripWeekdayPrefix strips a leading weekday name from a string.
// Returns the remaining string, the day number (0=Sunday), and whether a weekday was found.
func stripWeekdayPrefix(s string) (rest string, dayNum int, stripped bool) {
	// Try full weekday names first (must check before 3-letter to avoid partial match)
	for _, name := range []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"} {
		if strings.HasPrefix(s, name) && len(s) > len(name) {
			r := strings.TrimLeft(s[len(name):], ", ")
			if len(r) > 0 {
				return r, getDayOfWeek(name), true
			}
		}
	}
	// Try 3-letter abbreviations
	if len(s) > 3 {
		prefix := s[:3]
		dn := getDayOfWeek(prefix)
		if dn >= 0 {
			r := strings.TrimLeft(s[3:], ", ")
			if len(r) > 0 {
				return r, dn, true
			}
		}
	}
	return s, -1, false
}

// phpEpochDays computes the number of days from the Unix epoch (1970-01-01)
// using PHP's timelib algorithm (Hinnant's civil_from_days). This uses int64
// arithmetic which may overflow for extreme years, matching PHP's behavior.
func phpEpochDays(year, month, day int64) int64 {
	const (
		yearsPerEra       = int64(400)
		daysPerYear       = int64(365)
		daysPerEra        = int64(146097)
		hinnantEpochShift = int64(719468)
	)

	y := year
	if month <= 2 {
		y--
	}

	var era int64
	if y >= 0 {
		era = y / yearsPerEra
	} else {
		era = (y - 399) / yearsPerEra
	}

	yearOfEra := y - era*yearsPerEra

	var m int64
	if month > 2 {
		m = month - 3
	} else {
		m = month + 9
	}
	dayOfYear := (153*m+2)/5 + day - 1
	dayOfEra := yearOfEra*daysPerYear + yearOfEra/4 - yearOfEra/100 + dayOfYear

	return era*daysPerEra + dayOfEra - hinnantEpochShift
}

// phpUnixTimestamp computes a Unix timestamp using PHP-compatible int64 arithmetic.
// For extreme years this will overflow the same way PHP does, producing matching values.
func phpUnixTimestamp(year int, month, day, hour, minute, second int) int64 {
	days := phpEpochDays(int64(year), int64(month), int64(day))
	return days*86400 + int64(hour)*3600 + int64(minute)*60 + int64(second)
}

// fixOverflowedTime checks if a time.Date result overflowed Go's internal representation
// (detected by the year changing) and if so, recomputes the timestamp using PHP's int64
// overflow arithmetic to match PHP's behavior for extreme years.
func fixOverflowedTime(t time.Time, wantYear, month, day, hour, minute, second int, loc *time.Location) time.Time {
	if t.Year() == wantYear {
		return t // no overflow
	}
	// Go's time.Date overflowed internally — compute Unix timestamp using PHP's algorithm
	unix := phpUnixTimestamp(wantYear, month, day, hour, minute, second)
	return time.Unix(unix, 0).In(loc)
}
