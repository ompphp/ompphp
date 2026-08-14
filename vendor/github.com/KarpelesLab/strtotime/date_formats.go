package strtotime

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// parseISOFormat tries to parse a ISO format date (YYYY-MM-DD or D-M-YYYY)
func parseISOFormat(str string, loc *time.Location) (time.Time, bool) {
	if strings.Count(str, "-") != 2 {
		return time.Time{}, false
	}

	parts := strings.Split(str, "-")
	if len(parts) != 3 {
		return time.Time{}, false
	}

	// All parts must be numeric
	for _, p := range parts {
		if !isAllDigits(p) || len(p) == 0 {
			return time.Time{}, false
		}
	}

	first, _ := strconv.Atoi(parts[0])
	second, _ := strconv.Atoi(parts[1])
	third, _ := strconv.Atoi(parts[2])

	var year, month, day int

	if len(parts[0]) >= 4 {
		// YYYY-MM-DD (ISO format)
		year, month, day = first, second, third
		// PHP doesn't support years > 9999 in YYYY-MM-DD format;
		// it reinterprets the digits differently (e.g., as compact time).
		if year > 9999 {
			return time.Time{}, false
		}
	} else if len(parts[2]) >= 4 {
		// D-M-YYYY (European style with dashes)
		day, month, year = first, second, third
	} else {
		// Short year: try as YYYY-MM-DD with small year
		year, month, day = first, second, third
		// Handle 2-digit years
		if year < 100 {
			year = parseTwoDigitYear(year)
		}
	}

	if !IsValidDate(year, month, day) {
		return time.Time{}, false
	}

	result := time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
	return fixDSTGap(result, year, time.Month(month), day), true
}

// parseSlashFormat tries to parse a slash format date (YYYY/MM/DD)
func parseSlashFormat(str string, loc *time.Location) (time.Time, bool) {
	if strings.Count(str, "/") != 2 {
		return time.Time{}, false
	}

	parts := strings.Split(str, "/")
	if len(parts) != 3 || len(parts[0]) < 4 {
		return time.Time{}, false
	}

	// All parts must be numeric
	for _, p := range parts {
		if !isAllDigits(p) || len(p) == 0 {
			return time.Time{}, false
		}
	}

	year, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	day, _ := strconv.Atoi(parts[2])

	if !IsValidDate(year, month, day) {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc), true
}

// parseUSFormat tries to parse a US format date (MM/DD/YYYY)
func parseUSFormat(str string, loc *time.Location) (time.Time, bool) {
	if strings.Count(str, "/") != 2 {
		return time.Time{}, false
	}

	parts := strings.Split(str, "/")
	if len(parts) != 3 || len(parts[2]) < 4 {
		return time.Time{}, false
	}

	// All parts must be numeric
	for _, p := range parts {
		if !isAllDigits(p) || len(p) == 0 {
			return time.Time{}, false
		}
	}

	month, _ := strconv.Atoi(parts[0])
	day, _ := strconv.Atoi(parts[1])
	year, _ := strconv.Atoi(parts[2])

	if !IsValidDate(year, month, day) {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc), true
}

// parseEuropeanFormat tries to parse a European format date (DD.MM.YY or DD.MM.YYYY)
func parseEuropeanFormat(str string, loc *time.Location) (time.Time, bool) {
	if strings.Count(str, ".") == 2 {
		parts := strings.Split(str, ".")
		if len(parts) == 3 {
			// Validate each part contains only digits
			for _, part := range parts {
				for _, char := range part {
					if !unicode.IsDigit(char) {
						return time.Time{}, false
					}
				}
			}

			// Parse the components
			day, dayErr := strconv.Atoi(parts[0])
			month, monthErr := strconv.Atoi(parts[1])
			year, yearErr := strconv.Atoi(parts[2])

			// Check for parsing errors
			if yearErr != nil || monthErr != nil || dayErr != nil {
				return time.Time{}, false
			}

			// Handle 2-digit years
			if year < 100 {
				year = parseTwoDigitYear(year)
			}

			// Validate date components
			if !IsValidDate(year, month, day) {
				return time.Time{}, false
			}

			// Valid European format date
			return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc), true
		}
	}
	return time.Time{}, false
}

// parseTwoDigitYear normalizes 2-digit years according to standard practice

// parseDateTimeFormat parses "YYYY-MM-DD HH:MM:SS" and optionally a timezone offset
func parseDateTimeFormat(str string, loc *time.Location) (time.Time, bool) {
	// Find the space separating date from time
	spaceIdx := strings.IndexByte(str, ' ')
	if spaceIdx < 0 {
		return time.Time{}, false
	}

	datePart := str[:spaceIdx]
	rest := strings.TrimSpace(str[spaceIdx+1:])

	// Handle AM/PM — check if rest ends with AM or PM (possibly attached to time)
	ampm := ""
	restLower := strings.ToLower(rest)
	if strings.HasSuffix(restLower, "a.m.") || strings.HasSuffix(restLower, "p.m.") {
		// Dot notation: "a.m." or "p.m."
		ampm = string(restLower[len(restLower)-4]) + "m" // extract "a" or "p" + "m"
		rest = strings.TrimSpace(rest[:len(rest)-4])
	} else if strings.HasSuffix(restLower, "am") || strings.HasSuffix(restLower, "pm") {
		ampm = restLower[len(restLower)-2:]
		rest = strings.TrimSpace(rest[:len(rest)-2])
	} else {
		// Check for " AM" or " PM" as separate word
		upperRest := strings.ToUpper(rest)
		if strings.HasSuffix(upperRest, " AM") || strings.HasSuffix(upperRest, " PM") {
			ampm = strings.ToLower(upperRest[len(upperRest)-2:])
			rest = strings.TrimSpace(rest[:len(rest)-3])
		}
	}

	// Parse time using the ISO 8601 time parser (handles HH:MM:SS and fractional seconds)
	hour, minute, second, nanos, consumed, ok := parseISO8601Time(rest)
	if !ok {
		return time.Time{}, false
	}

	// Apply AM/PM
	if ampm != "" {
		hour = applyAMPM(hour, ampm)
	}

	// Parse the date — try ISO format first, then month-name format
	t, dateOk := parseISOFormat(datePart, loc)
	if !dateOk {
		t, dateOk = parseMonthNameFormat(datePart, loc)
		if !dateOk {
			return time.Time{}, false
		}
	}

	// Check for timezone offset after the time
	tzLoc := loc
	tzRest := rest[consumed:]
	if len(tzRest) > 0 {
		tzStr := strings.TrimSpace(tzRest)
		if parsed, tzConsumed, ok := parseNumericTimezoneOffset(tzStr); ok {
			// Only accept if the entire remaining string is the timezone
			if strings.TrimSpace(tzStr[tzConsumed:]) != "" {
				return time.Time{}, false
			}
			tzLoc = parsed
		} else if len(tzStr) > 0 {
			// Try named timezone (abbreviation or full name)
			if parsed, found := tryParseTimezone(tzStr); found {
				tzLoc = parsed
			} else {
				return time.Time{}, false
			}
		}
	}

	return time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, nanos, tzLoc), true
}

// parseYearMonthFormat parses "YYYY-MM" or "YYYY-M" as year-month (day defaults to 1)
func parseYearMonthFormat(str string, loc *time.Location) (time.Time, bool) {
	if strings.Count(str, "-") != 1 {
		return time.Time{}, false
	}

	parts := strings.SplitN(str, "-", 2)
	if len(parts) != 2 || !isAllDigits(parts[0]) || !isAllDigits(parts[1]) {
		return time.Time{}, false
	}
	if len(parts[0]) < 4 {
		return time.Time{}, false
	}

	year, _ := strconv.Atoi(parts[0])
	dayOrMonth, _ := strconv.Atoi(parts[1])

	// ISO ordinal date: YYYY-DDD (3 digits, day of year 001-366)
	if len(parts[1]) == 3 && dayOrMonth >= 1 && dayOrMonth <= 366 {
		t := time.Date(year, 1, 1, 0, 0, 0, 0, loc).AddDate(0, 0, dayOrMonth-1)
		if t.Year() == year { // ensure doy didn't overflow into next year
			return t, true
		}
		return time.Time{}, false
	}

	month := dayOrMonth
	if month < 1 || month > 12 {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc), true
}

// parseSignedYear parses "-YYYY-MM-DD [HH:MM:SS [TZ]]" or "+YYYY-MM-DD[T][HH:MM:SS [TZ]]" format.
func parseSignedYear(str string, loc *time.Location) (time.Time, bool) {
	if len(str) < 2 {
		return time.Time{}, false
	}
	var sign int
	switch str[0] {
	case '-':
		sign = -1
	case '+':
		sign = 1
	default:
		return time.Time{}, false
	}
	rest := str[1:]

	// Split off the date portion from optional time/tz
	datePart := rest
	timeTzPart := ""
	if spaceIdx := strings.IndexByte(rest, ' '); spaceIdx >= 0 {
		datePart = rest[:spaceIdx]
		timeTzPart = strings.TrimSpace(rest[spaceIdx+1:])
	} else if sign > 0 {
		// ISO 8601 expanded format uses T separator (only for positive)
		if tIdx := strings.IndexByte(rest, 't'); tIdx >= 0 {
			datePart = rest[:tIdx]
			timeTzPart = rest[tIdx+1:]
		}
	}

	if strings.Count(datePart, "-") != 2 {
		return time.Time{}, false
	}
	parts := strings.Split(datePart, "-")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	if !isAllDigits(parts[0]) || !isAllDigits(parts[1]) || !isAllDigits(parts[2]) {
		return time.Time{}, false
	}
	// Positive years must have >= 4 digits to differentiate from "+1 week" etc.
	if sign > 0 && len(parts[0]) < 4 {
		return time.Time{}, false
	}

	year, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	day, _ := strconv.Atoi(parts[2])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}

	hour, minute, second, nanos := 0, 0, 0, 0
	tzLoc := loc

	if timeTzPart != "" {
		hour, minute, second, nanos, tzLoc = parseTimeTzSuffix(timeTzPart, loc)
	}

	result := time.Date(sign*year, time.Month(month), day, hour, minute, second, nanos, tzLoc)
	result = fixOverflowedTime(result, sign*year, month, day, hour, minute, second, tzLoc)
	return result, true
}

// parseTimeTzSuffix parses "HH:MM:SS[.frac] [TZ]" from the suffix of a date string
func parseTimeTzSuffix(s string, loc *time.Location) (int, int, int, int, *time.Location) {
	hour, minute, second, nanos := 0, 0, 0, 0
	tzLoc := loc

	h, m, sec, consumed, ok := parseFlexTime(s)
	if !ok {
		return hour, minute, second, nanos, tzLoc
	}
	hour, minute, second = h, m, sec
	remaining := s[consumed:]

	// Handle fractional seconds
	if len(remaining) > 0 && remaining[0] == '.' {
		fracStart := 1
		fracEnd := fracStart
		for fracEnd < len(remaining) && remaining[fracEnd] >= '0' && remaining[fracEnd] <= '9' {
			fracEnd++
		}
		if fracEnd > fracStart {
			fracStr := remaining[fracStart:fracEnd]
			for len(fracStr) < 9 {
				fracStr += "0"
			}
			if len(fracStr) > 9 {
				fracStr = fracStr[:9]
			}
			nanos, _ = strconv.Atoi(fracStr)
		}
		remaining = remaining[fracEnd:]
	}

	remaining = strings.TrimSpace(remaining)
	if remaining != "" {
		if parsed, _, ok := parseNumericTimezoneOffset(remaining); ok {
			tzLoc = parsed
		} else if parsed, found := tryParseTimezone(remaining); found {
			tzLoc = parsed
		}
	}

	return hour, minute, second, nanos, tzLoc
}

// parseShortYearUSDateWithMilitaryTime parses "MM/DD/YY HHMM" format
func parseShortYearUSDateWithMilitaryTime(str string, loc *time.Location) (time.Time, bool) {
	parts := strings.SplitN(str, " ", 2)
	if len(parts) != 2 {
		return time.Time{}, false
	}

	datePart := parts[0]
	timePart := strings.TrimSpace(parts[1])

	// Date must have exactly 2 slashes
	if strings.Count(datePart, "/") != 2 {
		return time.Time{}, false
	}

	dateParts := strings.Split(datePart, "/")
	if len(dateParts) != 3 {
		return time.Time{}, false
	}
	for _, p := range dateParts {
		if !isAllDigits(p) || len(p) == 0 {
			return time.Time{}, false
		}
	}

	// Only handle short year (1-2 digits)
	if len(dateParts[2]) > 2 {
		return time.Time{}, false
	}

	month, _ := strconv.Atoi(dateParts[0])
	day, _ := strconv.Atoi(dateParts[1])
	year, _ := strconv.Atoi(dateParts[2])
	year = parseTwoDigitYear(year)

	// Time must be exactly 4 digits (military time HHMM)
	if len(timePart) != 4 || !isAllDigits(timePart) {
		return time.Time{}, false
	}

	hour, _ := strconv.Atoi(timePart[:2])
	minute, _ := strconv.Atoi(timePart[2:4])

	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	if !IsValidTime(hour, minute, 0) {
		return time.Time{}, false
	}

	return time.Date(year, time.Month(month), day, hour, minute, 0, 0, loc), true
}

// parseZeroDate handles the special case "0000-00-00 ..." which PHP maps to -0001-11-30.
// PHP treats month 0 as December of previous year minus 1, and day 0 as last day of
// previous month, resulting in year=-1, month=11(November), day=30.
func parseZeroDate(str string, loc *time.Location) (time.Time, bool) {
	trimmed := strings.TrimSpace(str)
	if !strings.HasPrefix(trimmed, "0000-00-00") {
		return time.Time{}, false
	}
	// PHP: year 0, month 0, day 0 → year -1, month 11 (Nov), day 30
	return time.Date(0, 0, 0, 0, 0, 0, 0, loc), true
}

// splitDateAndRest splits a string into a date portion and the rest after whitespace.
// It validates the date portion looks like a recognized date format.
func splitDateAndRest(str string) (string, string, bool) {
	// Find the first whitespace
	spaceIdx := strings.IndexByte(str, ' ')
	if spaceIdx < 0 {
		return "", "", false
	}

	datePart := str[:spaceIdx]
	rest := strings.TrimSpace(str[spaceIdx+1:])
	if rest == "" {
		return "", "", false
	}

	// Validate the date part looks like one of our recognized formats
	if looksLikeDateFormat(datePart) {
		return datePart, rest, true
	}

	return "", "", false
}

// looksLikeDateFormat checks if a string looks like a date format we recognize
func looksLikeDateFormat(s string) bool {
	// YYYY-M-D
	if strings.Count(s, "-") == 2 {
		parts := strings.Split(s, "-")
		if len(parts) == 3 && isAllDigits(parts[0]) && isAllDigits(parts[1]) && isAllDigits(parts[2]) {
			return true
		}
	}

	// YYYY/M/D or M/D/YYYY
	if strings.Count(s, "/") == 2 {
		parts := strings.Split(s, "/")
		if len(parts) == 3 && isAllDigits(parts[0]) && isAllDigits(parts[1]) && isAllDigits(parts[2]) {
			return true
		}
	}

	// DD.MM.YY or DD.MM.YYYY
	if strings.Count(s, ".") == 2 {
		parts := strings.Split(s, ".")
		if len(parts) == 3 && isAllDigits(parts[0]) && isAllDigits(parts[1]) && isAllDigits(parts[2]) {
			return true
		}
	}

	return false
}

// parseLargeYearAsTime handles numbers with 5+ digits that look like years > 9999
// but PHP interprets differently. PHP treats the digits as compact time (HHMMS or HHMMSS)
// and the subsequent -MM-DD as month and day, defaulting the year to "now".
//
// Examples:
//   - "10000-01-01" → hour=10, min=00, sec=0, month=01, day=01 (5-digit: HHMMS)
//   - "20000-06-15" → hour=20, min=00, sec=0, month=06, day=15 (5-digit: HHMMS)
//   - "100000-12-31" → hour=10, min=00, sec=00, month=12, day=31 (6-digit: HHMMSS)
func parseLargeYearAsTime(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	if strings.Count(str, "-") != 2 {
		return time.Time{}, false
	}

	parts := strings.SplitN(str, "-", 3)
	if len(parts) != 3 {
		return time.Time{}, false
	}

	digits := parts[0]
	if !isAllDigits(digits) || len(digits) < 5 || len(digits) > 6 {
		return time.Time{}, false
	}

	// Parse the remaining parts as month and day
	if !isAllDigits(parts[1]) || !isAllDigits(parts[2]) {
		return time.Time{}, false
	}
	month, _ := strconv.Atoi(parts[1])
	day, _ := strconv.Atoi(parts[2])

	if len(digits) == 5 {
		// HHMMS format (5 digits): time + month-day
		hour, _ := strconv.Atoi(digits[0:2])
		minute, _ := strconv.Atoi(digits[2:4])
		second := int(digits[4] - '0')

		if !IsValidTime(hour, minute, second) {
			return time.Time{}, false
		}
		if month < 1 || month > 12 || day < 1 || day > 31 {
			return time.Time{}, false
		}

		year := now.Year()
		return time.Date(year, time.Month(month), day, hour, minute, second, 0, loc), true
	}

	// HHMMSS format (6 digits): PHP treats subsequent -XX as timezone offset,
	// keeping today's date. The day portion (-DD) is consumed but ignored by PHP.
	hour, _ := strconv.Atoi(digits[0:2])
	minute, _ := strconv.Atoi(digits[2:4])
	second, _ := strconv.Atoi(digits[4:6])

	if !IsValidTime(hour, minute, second) {
		return time.Time{}, false
	}

	// The first dash-number after the 6 digits is a timezone offset
	tzOffset, _ := strconv.Atoi(parts[1])
	if tzOffset < 0 || tzOffset > 14 {
		return time.Time{}, false
	}

	y, m, d := now.Date()
	tzLoc := fixedZone(-tzOffset * 3600)
	return time.Date(y, m, d, hour, minute, second, 0, tzLoc), true
}
