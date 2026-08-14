package strtotime

import (
	"strconv"
	"strings"
	"time"
)

// parseISO8601 parses ISO 8601 formats including:
// - DateTime with T separator: 2023-01-15T14:30:00, 20060212T231223
// - With timezone offset: 2023-01-15T14:30:00Z, 2023-01-15T14:30:00+05:30
// - Week dates: 2023-W03, 2023-W03-1, 2023W03, 2023W031
func parseISO8601(str string, loc *time.Location) (time.Time, bool) {
	// Try week dates first
	if t, ok := parseISOWeekDate(str, loc); ok {
		return t, true
	}

	// Try datetime with T separator
	if t, ok := parseISO8601DateTime(str, loc); ok {
		return t, true
	}

	return time.Time{}, false
}

// parseISO8601DateTime parses ISO 8601 datetime formats with T separator
func parseISO8601DateTime(str string, loc *time.Location) (time.Time, bool) {
	// Find T separator between digits (lowercased to 't')
	tIdx := -1
	for i := 1; i < len(str)-1; i++ {
		if str[i] == 't' && str[i-1] >= '0' && str[i-1] <= '9' && str[i+1] >= '0' && str[i+1] <= '9' {
			tIdx = i
			break
		}
	}
	if tIdx < 0 {
		return time.Time{}, false
	}

	datePart := str[:tIdx]
	rest := str[tIdx+1:]

	// Parse date part
	var year, month, day int

	if strings.Contains(datePart, "-") {
		// YYYY-MM-DD (or other dash formats)
		t, ok := parseISOFormat(datePart, loc)
		if !ok {
			return time.Time{}, false
		}
		year = t.Year()
		month = int(t.Month())
		day = t.Day()
	} else if len(datePart) >= 8 && isAllDigits(datePart) {
		// YYYYMMDD compact format
		year, _ = strconv.Atoi(datePart[:len(datePart)-4])
		month, _ = strconv.Atoi(datePart[len(datePart)-4 : len(datePart)-2])
		day, _ = strconv.Atoi(datePart[len(datePart)-2:])
		if !IsValidDate(year, month, day) {
			return time.Time{}, false
		}
	} else {
		return time.Time{}, false
	}

	// Parse time part (may include fractional seconds and timezone)
	hour, minute, second, nanos, timeConsumed, ok := parseISO8601Time(rest)
	if !ok {
		return time.Time{}, false
	}

	// Parse timezone suffix from remaining string
	tzRest := rest[timeConsumed:]
	tzLoc := loc

	if len(tzRest) > 0 {
		// Skip optional space before timezone
		tzStr := strings.TrimLeft(tzRest, " ")

		if parsed, consumed, ok := parseNumericTimezoneOffset(tzStr); ok {
			// Ensure no trailing content after the timezone
			remaining := strings.TrimSpace(tzStr[consumed:])
			if len(remaining) > 0 {
				return time.Time{}, false
			}
			tzLoc = parsed
		} else if len(tzStr) > 0 {
			// Try named timezone
			if parsed, found := tryParseTimezone(tzStr); found {
				tzLoc = parsed
			} else {
				return time.Time{}, false
			}
		}
	}

	// Handle 24:00:00 as midnight of next day
	if hour == 24 {
		return time.Date(year, time.Month(month), day+1, 0, minute, second, nanos, tzLoc), true
	}
	return time.Date(year, time.Month(month), day, hour, minute, second, nanos, tzLoc), true
}

// parseISO8601Time parses time components from an ISO 8601 time string.
// Returns hour, minute, second, nanos, characters consumed, and success.
func parseISO8601Time(s string) (int, int, int, int, int, bool) {
	if len(s) < 1 {
		return 0, 0, 0, 0, 0, false
	}

	var hour, minute, second, nanos, consumed int

	// Try flexible H:M:S / HH:MM:SS parsing (supports single-digit components)
	if flexH, flexM, flexS, flexN, ok := parseFlexTime(s); ok {
		hour, minute, second = flexH, flexM, flexS
		consumed = flexN
	} else if len(s) >= 6 && isAllDigits(s[:6]) {
		// HHMMSS
		hour, _ = strconv.Atoi(s[:2])
		minute, _ = strconv.Atoi(s[2:4])
		second, _ = strconv.Atoi(s[4:6])
		consumed = 6
	} else if len(s) >= 4 && isAllDigits(s[:4]) {
		// HHMM
		hour, _ = strconv.Atoi(s[:2])
		minute, _ = strconv.Atoi(s[2:4])
		consumed = 4
	} else if len(s) >= 2 && isAllDigits(s[:2]) && (len(s) == 2 || (s[2] < '0' || s[2] > '9')) {
		// HH (hour only, no minute - e.g., "2012-02-02T10")
		hour, _ = strconv.Atoi(s[:2])
		consumed = 2
	} else if len(s) >= 1 && s[0] >= '0' && s[0] <= '9' && (len(s) == 1 || (s[1] < '0' || s[1] > '9')) {
		// H (single-digit hour)
		hour, _ = strconv.Atoi(s[:1])
		consumed = 1
	} else {
		return 0, 0, 0, 0, 0, false
	}

	// Allow hour 24 as a special case (wraps to next day, handled by caller)
	if hour == 24 {
		// Valid, will be handled by caller
	} else if !IsValidTime(hour, minute, second) {
		return 0, 0, 0, 0, 0, false
	}

	// Handle fractional seconds
	if consumed < len(s) && s[consumed] == '.' {
		consumed++
		fracStart := consumed
		for consumed < len(s) && s[consumed] >= '0' && s[consumed] <= '9' {
			consumed++
		}
		if consumed > fracStart {
			fracStr := s[fracStart:consumed]
			// Pad or truncate to 9 digits (nanoseconds)
			for len(fracStr) < 9 {
				fracStr += "0"
			}
			if len(fracStr) > 9 {
				fracStr = fracStr[:9]
			}
			nanos, _ = strconv.Atoi(fracStr)
		}
	}

	return hour, minute, second, nanos, consumed, true
}

// parseFlexTime parses time in flexible format H:M:S or H:M (supports 1 or 2 digit components)
// Returns hour, minute, second, consumed, ok
func parseFlexTime(s string) (int, int, int, int, bool) {
	pos := 0
	// Parse hour (1-2 digits)
	hStart := pos
	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		pos++
	}
	if pos == hStart || pos-hStart > 2 {
		return 0, 0, 0, 0, false
	}
	if pos >= len(s) || s[pos] != ':' {
		return 0, 0, 0, 0, false
	}
	hour, _ := strconv.Atoi(s[hStart:pos])
	pos++ // skip ':'

	// Parse minute (1-2 digits)
	mStart := pos
	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		pos++
	}
	if pos == mStart || pos-mStart > 2 {
		return 0, 0, 0, 0, false
	}
	minute, _ := strconv.Atoi(s[mStart:pos])

	second := 0
	// Optional seconds
	if pos < len(s) && s[pos] == ':' {
		pos++ // skip ':'
		sStart := pos
		for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
			pos++
		}
		if pos == sStart || pos-sStart > 2 {
			return 0, 0, 0, 0, false
		}
		second, _ = strconv.Atoi(s[sStart:pos])
	}

	return hour, minute, second, pos, true
}

// parseNumericTimezoneOffset parses numeric timezone offsets:
// Z, +HH:MM, -HH:MM, +HHMM, -HHMM, +HH, -HH
// Returns the location, number of characters consumed, and success.
func parseNumericTimezoneOffset(s string) (*time.Location, int, bool) {
	if len(s) == 0 {
		return nil, 0, false
	}

	if s[0] == 'z' || s[0] == 'Z' {
		// Check there's nothing else after Z (or only whitespace)
		if len(s) == 1 || s[1] == ' ' {
			return time.UTC, 1, true
		}
		return nil, 0, false
	}

	if s[0] != '+' && s[0] != '-' {
		return nil, 0, false
	}

	sign := 1
	if s[0] == '-' {
		sign = -1
	}
	rest := s[1:]

	// Try +HH:MM or -HH:MM
	if len(rest) >= 5 && rest[2] == ':' && isAllDigits(rest[:2]) && isAllDigits(rest[3:5]) {
		h, _ := strconv.Atoi(rest[:2])
		m, _ := strconv.Atoi(rest[3:5])
		if h <= 14 && m <= 59 {
			offset := sign * (h*3600 + m*60)
			return fixedZone(offset), 6, true
		}
	}

	// Try +H:MM or -H:MM (single-digit hour).
	if len(rest) >= 4 && rest[1] == ':' && rest[0] >= '0' && rest[0] <= '9' && isAllDigits(rest[2:4]) {
		h := int(rest[0] - '0')
		m, _ := strconv.Atoi(rest[2:4])
		if h <= 9 && m <= 59 {
			offset := sign * (h*3600 + m*60)
			return fixedZone(offset), 5, true
		}
	}

	// Try +HH:M or -HH:M (shortened single-digit minute, bug74173)
	if len(rest) >= 4 && rest[2] == ':' && isAllDigits(rest[:2]) && rest[3] >= '0' && rest[3] <= '9' &&
		(len(rest) == 4 || rest[4] < '0' || rest[4] > '9') {
		h, _ := strconv.Atoi(rest[:2])
		m := int(rest[3] - '0')
		if h <= 14 && m <= 59 {
			offset := sign * (h*3600 + m*60)
			return fixedZone(offset), 5, true
		}
	}

	// Try +HHMM or -HHMM
	if len(rest) >= 4 && isAllDigits(rest[:4]) {
		h, _ := strconv.Atoi(rest[:2])
		m, _ := strconv.Atoi(rest[2:4])
		if h <= 14 && m <= 59 {
			offset := sign * (h*3600 + m*60)
			return fixedZone(offset), 5, true
		}
	}

	// Try +HH or -HH
	if len(rest) >= 2 && isAllDigits(rest[:2]) {
		// Make sure there's nothing else after (or only non-digit)
		if len(rest) == 2 || rest[2] < '0' || rest[2] > '9' {
			h, _ := strconv.Atoi(rest[:2])
			if h <= 14 {
				offset := sign * h * 3600
				return fixedZone(offset), 3, true
			}
		}
	}

	// Try flexible +H:M or -H:M format (single-digit components)
	if len(rest) >= 1 && rest[0] >= '0' && rest[0] <= '9' {
		h, m, _, consumed, ok := parseFlexTime(rest)
		if ok && h <= 14 && m <= 59 {
			offset := sign * (h*3600 + m*60)
			return fixedZone(offset), consumed + 1, true
		}
	}

	return nil, 0, false
}

// parseISOWeekDate parses ISO 8601 week dates:
// 2023-W03, 2023-W03-1, 2023W03, 2023W031
// After lowercasing, W becomes w.
func parseISOWeekDate(str string, loc *time.Location) (time.Time, bool) {
	// Find 'w' preceded by a digit (possibly with a dash before w)
	wIdx := -1
	for i := 1; i < len(str); i++ {
		if str[i] == 'w' {
			prev := str[i-1]
			if prev >= '0' && prev <= '9' {
				wIdx = i
				break
			}
			if prev == '-' && i >= 2 && str[i-2] >= '0' && str[i-2] <= '9' {
				wIdx = i
				break
			}
		}
	}
	if wIdx < 0 {
		return time.Time{}, false
	}

	// Extract year part (strip trailing dash)
	yearPart := strings.TrimSuffix(str[:wIdx], "-")
	if !isAllDigits(yearPart) || len(yearPart) == 0 {
		return time.Time{}, false
	}

	year, err := strconv.Atoi(yearPart)
	if err != nil || year < 1 {
		return time.Time{}, false
	}

	// Parse week number after 'w'
	// In compact form (no dash before W), week is always 2 digits
	compact := wIdx > 0 && str[wIdx-1] != '-'
	rest := str[wIdx+1:]
	weekStr := ""
	i := 0
	maxWeekDigits := 2
	if !compact {
		maxWeekDigits = 2 // extended form also allows 1-2 digits
	}
	for i < len(rest) && i < maxWeekDigits && rest[i] >= '0' && rest[i] <= '9' {
		weekStr += string(rest[i])
		i++
	}
	if len(weekStr) == 0 || len(weekStr) < 2 {
		// PHP requires 2-digit week numbers
		return time.Time{}, false
	}
	week, _ := strconv.Atoi(weekStr)
	if week < 1 || week > 53 {
		return time.Time{}, false
	}

	// Parse optional day of week (0=Sunday before, 1=Monday, 7=Sunday)
	day := 1 // Default to Monday
	rest = rest[i:]
	if len(rest) > 0 {
		if rest[0] == '-' {
			rest = rest[1:]
		}
		if len(rest) >= 1 && rest[0] >= '0' && rest[0] <= '7' {
			day = int(rest[0] - '0')
			rest = rest[1:]
		} else if len(rest) >= 1 && rest[0] >= '0' && rest[0] <= '9' {
			// Day >= 8: PHP treats as timezone offset (e.g., "-8" → UTC-8)
			h := int(rest[0] - '0')
			rest = rest[1:]
			if len(rest) > 0 {
				return time.Time{}, false
			}
			// Compute the Monday of the target week using UTC for calendar math
			jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
			isoWeekday := int(jan4.Weekday())
			if isoWeekday == 0 {
				isoWeekday = 7
			}
			week1Monday := jan4.AddDate(0, 0, -(isoWeekday - 1))
			target := week1Monday.AddDate(0, 0, (week-1)*7)
			// Create midnight in the offset timezone (not just relabel)
			tzLoc := fixedZone(-h * 3600)
			return time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, tzLoc), true
		}
		// Reject any remaining content
		if len(rest) > 0 {
			return time.Time{}, false
		}
	}

	// Convert ISO week date to calendar date
	// ISO week 1 contains the year's first Thursday.
	// Find January 4th (always in week 1), then find that week's Monday.
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, loc)
	isoWeekday := int(jan4.Weekday())
	if isoWeekday == 0 {
		isoWeekday = 7 // Sunday = 7 in ISO
	}
	// Monday of week 1
	week1Monday := jan4.AddDate(0, 0, -(isoWeekday - 1))

	// Target date: week1Monday + (week-1)*7 + (day-1)
	target := week1Monday.AddDate(0, 0, (week-1)*7+(day-1))

	return target, true
}
