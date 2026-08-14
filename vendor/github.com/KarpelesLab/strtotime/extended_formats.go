package strtotime

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// isAllDigits checks if a string contains only ASCII digits
func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// isAlpha checks if a string contains only ASCII letters
func isAlpha(s string) bool {
	for _, c := range s {
		if !unicode.IsLetter(c) {
			return false
		}
	}
	return len(s) > 0
}

// parseCompactTimestamp parses compact timestamp formats:
// - "19970523091528" (YYYYMMDDhhmmss, exactly 14 digits)
// - "20050620091407 GMT" (14 digits + timezone)
// - "20101212" (YYYYMMDD, exactly 8 digits)
func parseCompactTimestamp(str string, loc *time.Location) (time.Time, bool) {
	// Split on space to handle optional timezone suffix
	parts := strings.SplitN(str, " ", 2)
	digits := parts[0]

	// 8-digit YYYYMMDD format
	if len(digits) == 8 && isAllDigits(digits) {
		year, _ := strconv.Atoi(digits[0:4])
		month, _ := strconv.Atoi(digits[4:6])
		day, _ := strconv.Atoi(digits[6:8])
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc), true
		}
		return time.Time{}, false
	}

	// 14-digit YYYYMMDDhhmmss format (with optional timezone)
	if len(digits) != 14 || !isAllDigits(digits) {
		return time.Time{}, false
	}

	year, _ := strconv.Atoi(digits[0:4])
	month, _ := strconv.Atoi(digits[4:6])
	day, _ := strconv.Atoi(digits[6:8])
	hour, _ := strconv.Atoi(digits[8:10])
	minute, _ := strconv.Atoi(digits[10:12])
	second, _ := strconv.Atoi(digits[12:14])

	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return time.Time{}, false
	}

	tzLoc := loc
	if len(parts) > 1 && parts[1] != "" {
		if parsed, found := tryParseTimezone(strings.TrimSpace(parts[1])); found {
			tzLoc = parsed
		}
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, tzLoc), true
}

// parseCompactTimeFormats handles PHP-specific compact time/date formats:
// - "022233": 6-digit hhmmss time-only
// - "2006167": 7-digit year+day-of-year (pgydotd)
// - "t0222": 't' prefix + hhmm time
// - "22.49.12.42GMT": dotted time with optional fractional seconds and timezone
func parseCompactTimeFormats(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	// "t" prefix + 4 digits = tHHMM time format
	if len(str) >= 5 && str[0] == 't' && isAllDigits(str[1:5]) {
		hour, _ := strconv.Atoi(str[1:3])
		minute, _ := strconv.Atoi(str[3:5])
		if IsValidTime(hour, minute, 0) {
			y, m, d := now.Date()
			return time.Date(y, m, d, hour, minute, 0, 0, loc), true
		}
	}

	// Dotted time format: "HH.MM.SS[.frac][TZ]"
	if len(str) >= 8 && str[2] == '.' && str[5] == '.' {
		if isAllDigits(str[0:2]) && isAllDigits(str[3:5]) && str[6] >= '0' && str[6] <= '9' {
			hour, _ := strconv.Atoi(str[0:2])
			minute, _ := strconv.Atoi(str[3:5])
			// Parse seconds (may be followed by .frac or tz)
			pos := 6
			for pos < len(str) && str[pos] >= '0' && str[pos] <= '9' {
				pos++
			}
			second, _ := strconv.Atoi(str[6:pos])
			if !IsValidTime(hour, minute, second) {
				return time.Time{}, false
			}
			// Skip optional fractional part .NN
			if pos < len(str) && str[pos] == '.' {
				pos++
				for pos < len(str) && str[pos] >= '0' && str[pos] <= '9' {
					pos++
				}
			}
			// Parse optional timezone suffix (no space)
			tzLoc := loc
			if pos < len(str) {
				tzStr := str[pos:]
				// Strip leading space if present
				tzStr = strings.TrimSpace(tzStr)
				if parsed, found := tryParseTimezone(tzStr); found {
					tzLoc = parsed
				} else {
					return time.Time{}, false
				}
			}
			y, m, d := now.Date()
			return time.Date(y, m, d, hour, minute, second, 0, tzLoc), true
		}
	}

	// All-digit formats
	if !isAllDigits(str) {
		return time.Time{}, false
	}

	// 6-digit hhmmss: compact time-only format
	if len(str) == 6 {
		hour, _ := strconv.Atoi(str[0:2])
		minute, _ := strconv.Atoi(str[2:4])
		second, _ := strconv.Atoi(str[4:6])
		if IsValidTime(hour, minute, second) {
			y, m, d := now.Date()
			return time.Date(y, m, d, hour, minute, second, 0, loc), true
		}
	}

	// 7-digit pgydotd: year + day-of-year (e.g., "2006167" = June 16, 2006)
	if len(str) == 7 {
		year, _ := strconv.Atoi(str[0:4])
		doy, _ := strconv.Atoi(str[4:7])
		if year >= 1 && doy >= 1 && doy <= 366 {
			t := time.Date(year, 1, 1, 0, 0, 0, 0, loc).AddDate(0, 0, doy-1)
			if t.Year() == year { // ensure doy didn't overflow into next year
				return t, true
			}
		}
	}

	return time.Time{}, false
}

// parseMonthNameFormat parses formats like "Jan-15-2006" or "2006-Jan-15"
func parseMonthNameFormat(str string, loc *time.Location) (time.Time, bool) {
	parts := strings.Split(str, "-")
	if len(parts) != 3 {
		return time.Time{}, false
	}

	// Try "Jan-15-2006" format: alpha-digits-4digits
	if isAlpha(parts[0]) && len(parts[0]) >= 3 {
		day, dayErr := strconv.Atoi(parts[1])
		year, yearErr := strconv.Atoi(parts[2])

		if dayErr == nil && yearErr == nil {
			month, ok := getMonthByName(parts[0])
			if ok && IsValidDate(year, int(month), day) {
				return time.Date(year, month, day, 0, 0, 0, 0, loc), true
			}
		}
	}

	// Try "2006-Jan-15" format: 4digits-alpha-digits
	if len(parts[0]) == 4 && isAlpha(parts[1]) && len(parts[1]) >= 3 {
		year, yearErr := strconv.Atoi(parts[0])
		day, dayErr := strconv.Atoi(parts[2])

		if yearErr == nil && dayErr == nil {
			month, ok := getMonthByName(parts[1])
			if ok && IsValidDate(year, int(month), day) {
				return time.Date(year, month, day, 0, 0, 0, 0, loc), true
			}
		}
	}

	// Try "15-Jan-2006" format: digits-alpha-4digits (DD-Mon-YYYY)
	if isAllDigits(parts[0]) && isAlpha(parts[1]) && len(parts[1]) >= 3 && isAllDigits(parts[2]) {
		day, dayErr := strconv.Atoi(parts[0])
		year, yearErr := strconv.Atoi(parts[2])

		if dayErr == nil && yearErr == nil {
			if year < 100 {
				year = parseTwoDigitYear(year)
			}
			month, ok := getMonthByName(parts[1])
			if ok && IsValidDate(year, int(month), day) {
				return time.Date(year, month, day, 0, 0, 0, 0, loc), true
			}
		}
	}

	return time.Time{}, false
}

// parseHTTPLogFormat parses formats like "10/Oct/2000:13:55:36 +0100"
func parseHTTPLogFormat(str string, loc *time.Location) (time.Time, bool) {
	// Find the space separating datetime from timezone offset
	spaceIdx := strings.IndexByte(str, ' ')
	if spaceIdx < 0 {
		return time.Time{}, false
	}

	datePart := str[:spaceIdx]
	tzOffset := strings.TrimSpace(str[spaceIdx+1:])

	// Parse date part: DD/Mon/YYYY:HH:MM:SS
	slash1 := strings.IndexByte(datePart, '/')
	if slash1 < 0 || slash1 < 1 || slash1 > 2 {
		return time.Time{}, false
	}

	slash2 := strings.IndexByte(datePart[slash1+1:], '/')
	if slash2 < 0 {
		return time.Time{}, false
	}
	slash2 += slash1 + 1

	// Month part must be exactly 3 letters
	monthStr := datePart[slash1+1 : slash2]
	if len(monthStr) != 3 || !isAlpha(monthStr) {
		return time.Time{}, false
	}

	// After second slash: YYYY:HH:MM:SS
	rest := datePart[slash2+1:]
	colon1 := strings.IndexByte(rest, ':')
	if colon1 < 0 {
		return time.Time{}, false
	}

	yearStr := rest[:colon1]
	if len(yearStr) != 4 {
		return time.Time{}, false
	}

	timeStr := rest[colon1+1:]
	timeParts := strings.Split(timeStr, ":")
	if len(timeParts) != 3 {
		return time.Time{}, false
	}

	// Validate all parts are digits with correct lengths
	if len(timeParts[0]) != 2 || len(timeParts[1]) != 2 || len(timeParts[2]) != 2 {
		return time.Time{}, false
	}

	day, dayErr := strconv.Atoi(datePart[:slash1])
	year, yearErr := strconv.Atoi(yearStr)
	hour, hourErr := strconv.Atoi(timeParts[0])
	minute, minErr := strconv.Atoi(timeParts[1])
	second, secErr := strconv.Atoi(timeParts[2])

	if dayErr != nil || yearErr != nil || hourErr != nil || minErr != nil || secErr != nil {
		return time.Time{}, false
	}

	month, ok := getMonthByName(monthStr)
	if !ok {
		return time.Time{}, false
	}

	if !IsValidDate(year, int(month), day) {
		return time.Time{}, false
	}

	if !IsValidTime(hour, minute, second) {
		return time.Time{}, false
	}

	// Parse the timezone offset (format: "+0100" or "-0500")
	if len(tzOffset) != 5 || (tzOffset[0] != '+' && tzOffset[0] != '-') {
		return time.Time{}, false
	}

	tzHour, tzHourErr := strconv.Atoi(tzOffset[1:3])
	tzMin, tzMinErr := strconv.Atoi(tzOffset[3:5])

	if tzHourErr != nil || tzMinErr != nil || tzHour < 0 || tzHour > 23 || tzMin < 0 || tzMin > 59 {
		return time.Time{}, false
	}

	tzOffsetSeconds := tzHour*3600 + tzMin*60
	if tzOffset[0] == '-' {
		tzOffsetSeconds = -tzOffsetSeconds
	}

	tz := fixedZone(tzOffsetSeconds)
	return time.Date(year, month, day, hour, minute, second, 0, tz), true
}

// stripOrdinalSuffix removes ordinal suffixes: "26th" → "26", "1st" → "1"
func stripOrdinalSuffix(s string) string {
	lower := strings.ToLower(s)
	for _, suffix := range []string{"st", "nd", "rd", "th"} {
		if strings.HasSuffix(lower, suffix) && len(s) > len(suffix) {
			prefix := s[:len(s)-len(suffix)]
			if isAllDigits(prefix) {
				return prefix
			}
		}
	}
	return s
}

// stripMonthPeriod removes trailing period from month abbreviations: "dec." → "dec"
func stripMonthPeriod(s string) string {
	return strings.TrimSuffix(s, ".")
}

// getMonthByNameFlex tries to match a month name, handling trailing periods
func getMonthByNameFlex(name string) (time.Month, bool) {
	m, ok := getMonthByName(name)
	if ok {
		return m, true
	}
	return getMonthByName(stripMonthPeriod(name))
}

// parseDayMonthYear parses formats like "DD Mon YYYY", "DD-Mon-YYYY", "DDMonYYYY"
// with optional time and timezone. Also handles day-of-week prefix and ordinal suffix.
// Examples: "11 Oct 2005", "11-MAY-1988 12:00:00AM", "11Oct2005",
//
//	"Sat 26th Nov 2005 18:18", "Thu, 20 Nov 2003 16:20:42 +0000"
func parseDayMonthYear(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	// Skip leading day-of-week like "Sat " or "Thursday, "
	s, prefixDayNum, _ := stripWeekdayPrefix(str)
	if prefixDayNum < 0 {
		s = str // no weekday found, use original
	}

	// Try to extract day, month, year from the remaining string
	fields := strings.Fields(s)
	if len(fields) < 2 {
		// Try compact DDMonYYYY (no spaces)
		return parseDayMonthYearCompact(s, now, loc)
	}

	idx := 0

	// Handle hyphen-separated date: "24-Jan-2019" or "24-Jan-19"
	// (used by DATE_COOKIE and DATE_RFC850 formats)
	if strings.Contains(fields[0], "-") {
		dateParts := strings.SplitN(fields[0], "-", 3)
		if len(dateParts) == 3 {
			// Replace the single hyphenated field with 3 separate fields
			rest := fields[1:]
			fields = append(dateParts, rest...)
		}
	}

	// Parse day (may include ordinal suffix)
	dayStr := stripOrdinalSuffix(fields[idx])
	day, err := strconv.Atoi(dayStr)
	if err != nil || day < 1 || day > 31 {
		// Day might be fused with month: "11Oct" in "11Oct 2005"
		return parseDayMonthYearCompact(s, now, loc)
	}
	idx++

	// Parse month name
	if idx >= len(fields) {
		return time.Time{}, false
	}
	month, ok := getMonthByNameFlex(fields[idx])
	if !ok {
		return time.Time{}, false
	}
	idx++

	// Parse year
	if idx >= len(fields) {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(fields[idx])
	if err != nil || year < 0 {
		return time.Time{}, false
	}
	if year < 100 {
		year = parseTwoDigitYear(year)
	}
	idx++

	hour, minute, second, nanos := 0, 0, 0, 0

	// Parse optional time
	if idx < len(fields) {
		timeStr := fields[idx]
		if strings.Contains(timeStr, ":") {
			h, m, sec, consumed, ok := parseFlexTime(timeStr)
			if ok {
				hour, minute, second = h, m, sec
				idx++
				// Check for AM/PM attached to time or as next field
				remaining := timeStr[consumed:]
				if ampm := strings.ToLower(remaining); ampm == "am" || ampm == "pm" {
					hour = applyAMPM(hour, ampm)
				} else if idx < len(fields) {
					ampm = strings.ToLower(fields[idx])
					if ampm == "am" || ampm == "pm" {
						hour = applyAMPM(hour, ampm)
						idx++
					}
				}
				// Handle fractional seconds
				if consumed < len(timeStr) && timeStr[consumed] == '.' {
					fracStart := consumed + 1
					fracEnd := fracStart
					for fracEnd < len(timeStr) && timeStr[fracEnd] >= '0' && timeStr[fracEnd] <= '9' {
						fracEnd++
					}
					if fracEnd > fracStart {
						fracStr := timeStr[fracStart:fracEnd]
						for len(fracStr) < 9 {
							fracStr += "0"
						}
						if len(fracStr) > 9 {
							fracStr = fracStr[:9]
						}
						nanos, _ = strconv.Atoi(fracStr)
					}
				}
			}
		}
	}

	// Parse optional timezone
	tzLoc := loc
	if idx < len(fields) {
		tzStr := strings.Join(fields[idx:], " ")
		if parsed, _, ok := parseNumericTimezoneOffset(tzStr); ok {
			tzLoc = parsed
		} else if parsed, found := tryParseTimezone(tzStr); found {
			tzLoc = parsed
		}
	}

	if month > 0 && day > 0 {
		result := time.Date(year, month, day, hour, minute, second, nanos, tzLoc)
		// PHP: if a weekday prefix was present and doesn't match the parsed date,
		// advance to the next occurrence of that weekday
		if prefixDayNum >= 0 && int(result.Weekday()) != prefixDayNum {
			daysUntil := (prefixDayNum - int(result.Weekday()) + 7) % 7
			if daysUntil == 0 {
				daysUntil = 7
			}
			result = time.Date(result.Year(), result.Month(), result.Day()+daysUntil, hour, minute, second, nanos, tzLoc)
		}
		return result, true
	}
	return time.Time{}, false
}

// parseDayMonthYearCompact parses "DDMonYYYY", "DDMon YYYY", or "DDMon" (no year = base year)
func parseDayMonthYearCompact(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	s := strings.TrimSpace(str)
	if len(s) < 4 { // at least DMon
		return time.Time{}, false
	}

	// Extract leading digits as day
	dayEnd := 0
	for dayEnd < len(s) && s[dayEnd] >= '0' && s[dayEnd] <= '9' {
		dayEnd++
	}
	if dayEnd == 0 || dayEnd > 2 {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(s[:dayEnd])

	// Extract month name (3+ letters)
	monthStart := dayEnd
	monthEnd := monthStart
	for monthEnd < len(s) && ((s[monthEnd] >= 'a' && s[monthEnd] <= 'z') || (s[monthEnd] >= 'A' && s[monthEnd] <= 'Z')) {
		monthEnd++
	}
	if monthEnd-monthStart < 3 {
		return time.Time{}, false
	}
	month, ok := getMonthByNameFlex(s[monthStart:monthEnd])
	if !ok {
		return time.Time{}, false
	}

	// Rest may have optional space then year, then optional time
	rest := strings.TrimSpace(s[monthEnd:])
	if rest == "" {
		// No year — default to base year (e.g., "11Oct")
		if day < 1 || day > 31 {
			return time.Time{}, false
		}
		return time.Date(now.Year(), month, day, 0, 0, 0, 0, loc), true
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return time.Time{}, false
	}

	year, err := strconv.Atoi(fields[0])
	if err != nil {
		return time.Time{}, false
	}
	if year < 100 {
		year = parseTwoDigitYear(year)
	}

	hour, minute, second, nanos := 0, 0, 0, 0
	fidx := 1

	// Parse optional time
	if fidx < len(fields) && strings.Contains(fields[fidx], ":") {
		h, m, sec, consumed, ok := parseFlexTime(fields[fidx])
		if ok {
			hour, minute, second = h, m, sec
			// Check for AM/PM
			remaining := fields[fidx][consumed:]
			if ampm := strings.ToLower(remaining); ampm == "am" || ampm == "pm" {
				hour = applyAMPM(hour, ampm)
			} else if fidx+1 < len(fields) {
				ampm = strings.ToLower(fields[fidx+1])
				if ampm == "am" || ampm == "pm" {
					hour = applyAMPM(hour, ampm)
				}
			}
			// Handle fractional seconds
			if consumed < len(fields[fidx]) && fields[fidx][consumed] == '.' {
				fracStart := consumed + 1
				fracEnd := fracStart
				for fracEnd < len(fields[fidx]) && fields[fidx][fracEnd] >= '0' && fields[fidx][fracEnd] <= '9' {
					fracEnd++
				}
				if fracEnd > fracStart {
					fracStr := fields[fidx][fracStart:fracEnd]
					for len(fracStr) < 9 {
						fracStr += "0"
					}
					if len(fracStr) > 9 {
						fracStr = fracStr[:9]
					}
					nanos, _ = strconv.Atoi(fracStr)
				}
			}
		}
	}

	if day < 1 || day > 31 {
		return time.Time{}, false
	}

	return time.Date(year, month, day, hour, minute, second, nanos, loc), true
}

// applyAMPM converts 12-hour time to 24-hour format

// parseMonthYearOnly parses "Oct 2001" or "2001 Oct" (month + year, day defaults to 1)
// with optional trailing time: "october 2010 23:00", "2010 october 11:30 pm"
func parseMonthYearOnly(str string, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) < 2 {
		return time.Time{}, false
	}

	var month time.Month
	var year int
	idx := 0

	// Try "Month Year" — year must be >= 100 or have 4+ digits (e.g., "0099") to avoid
	// "April 4" being treated as year 4
	if m, ok := getMonthByNameFlex(fields[0]); ok {
		if y, err := strconv.Atoi(fields[1]); err == nil && (y >= 100 || len(fields[1]) >= 4) {
			month, year = m, y
			idx = 2
		}
	}

	// Try "Year Month"
	if idx == 0 {
		if y, err := strconv.Atoi(fields[0]); err == nil && (y >= 100 || len(fields[0]) >= 4) {
			if m, ok := getMonthByNameFlex(fields[1]); ok {
				month, year = m, y
				idx = 2
			}
		}
	}

	if idx == 0 {
		return time.Time{}, false
	}

	// Parse optional trailing time
	hour, minute, second := 0, 0, 0
	if idx < len(fields) {
		timeStr := fields[idx]
		if strings.Contains(timeStr, ":") {
			h, m, s, consumed, ok := parseFlexTime(timeStr)
			if !ok {
				return time.Time{}, false
			}
			hour, minute, second = h, m, s
			idx++
			// Check for AM/PM attached to time or as next field
			remaining := timeStr[consumed:]
			if ampm := strings.ToLower(remaining); ampm == "am" || ampm == "pm" {
				hour = applyAMPM(hour, ampm)
			} else if idx < len(fields) {
				ampm = strings.ToLower(fields[idx])
				if ampm == "am" || ampm == "pm" {
					hour = applyAMPM(hour, ampm)
					idx++
				}
			}
		} else {
			// Try bare AM/PM time: "11pm", "3am"
			f := strings.ToLower(timeStr)
			var ampm string
			if strings.HasSuffix(f, "pm") {
				ampm = "pm"
				f = f[:len(f)-2]
			} else if strings.HasSuffix(f, "am") {
				ampm = "am"
				f = f[:len(f)-2]
			}
			if ampm != "" {
				if h, err := strconv.Atoi(f); err == nil && h >= 1 && h <= 12 {
					hour = applyAMPM(h, ampm)
					idx++
				}
			}
		}
	}

	// All fields must be consumed
	if idx != len(fields) {
		return time.Time{}, false
	}

	return time.Date(year, month, 1, hour, minute, second, 0, loc), true
}

// parseTimeBeforeDate parses formats where time precedes the date:
// "19:30 Dec 17 2005", "17:00 2004-01-01", "1pm Aug 1 GMT 2007"
func parseTimeBeforeDate(str string, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) < 2 {
		return time.Time{}, false
	}

	var hour, minute, second int

	// Try colon-based time (19:30, 17:00:00, 10:00:00 AM, etc.)
	timeFieldEnd := 1 // index after the time fields
	if strings.Contains(fields[0], ":") {
		var ok bool
		hour, minute, second, _, ok = parseFlexTime(fields[0])
		if !ok || !IsValidTime(hour, minute, second) {
			return time.Time{}, false
		}
		// Check for AM/PM after the colon time
		if len(fields) > 1 {
			ampm := strings.ToLower(fields[1])
			if ampm == "am" || ampm == "pm" || ampm == "a.m." || ampm == "p.m." {
				ampm = strings.TrimSuffix(strings.Replace(ampm, ".", "", -1), "")
				if strings.HasPrefix(ampm, "a") {
					hour = applyAMPM(hour, "am")
				} else {
					hour = applyAMPM(hour, "pm")
				}
				timeFieldEnd = 2
			}
		}
	} else {
		// Try bare AM/PM time: "1pm", "12am", "3am"
		f := strings.ToLower(fields[0])
		var ampm string
		if strings.HasSuffix(f, "pm") {
			ampm = "pm"
			f = f[:len(f)-2]
		} else if strings.HasSuffix(f, "am") {
			ampm = "am"
			f = f[:len(f)-2]
		}
		if ampm == "" {
			return time.Time{}, false
		}
		h, err := strconv.Atoi(f)
		if err != nil || h < 1 || h > 12 {
			return time.Time{}, false
		}
		hour = applyAMPM(h, ampm)
	}

	// Try to parse the rest as a date
	dateStr := strings.Join(fields[timeFieldEnd:], " ")

	// Try ISO date
	if t, ok := parseISOFormat(dateStr, loc); ok {
		return time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, 0, loc), true
	}

	// Try month name date with optional timezone: "Aug 1 2007", "Aug 1 GMT 2007"
	dateFields := strings.Fields(dateStr)
	if len(dateFields) >= 2 {
		if month, ok := getMonthByNameFlex(dateFields[0]); ok {
			dayStr := stripOrdinalSuffix(strings.TrimSuffix(dateFields[1], ","))
			day, err := strconv.Atoi(dayStr)
			if err == nil && day >= 1 && day <= 31 {
				year := time.Now().Year()
				tzLoc := loc
				fidx := 2

				// Parse optional timezone and year from remaining fields
				for fidx < len(dateFields) {
					// Try as year
					if y, err := strconv.Atoi(dateFields[fidx]); err == nil && y > 0 {
						year = y
						fidx++
						continue
					}
					// Try as timezone
					if tz, found := tryParseTimezone(dateFields[fidx]); found {
						tzLoc = tz
						fidx++
						continue
					}
					break
				}

				return time.Date(year, month, day, hour, minute, second, 0, tzLoc), true
			}
		}
	}

	return time.Time{}, false
}

// parseUSDateWithTime parses "MM/DD/YYYY H:MM AM" format (US date with 12-hour time)
func parseUSDateWithTime(str string, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) < 2 {
		return time.Time{}, false
	}

	// First field should be the date
	t, ok := parseUSFormat(fields[0], loc)
	if !ok {
		return time.Time{}, false
	}

	// Parse time
	if len(fields) >= 2 && strings.Contains(fields[1], ":") {
		hour, minute, second, consumed, ok := parseFlexTime(fields[1])
		if !ok {
			return time.Time{}, false
		}
		// Check for AM/PM
		remaining := fields[1][consumed:]
		if ampm := strings.ToLower(remaining); ampm == "am" || ampm == "pm" {
			hour = applyAMPM(hour, ampm)
		} else if len(fields) >= 3 {
			ampm = strings.ToLower(fields[2])
			if ampm == "am" || ampm == "pm" {
				hour = applyAMPM(hour, ampm)
			}
		}
		return time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, 0, loc), true
	}

	return time.Time{}, false
}

// parseFirstLastDayOfDate parses "first day of YYYY-MM" and "last day of YYYY-MM" patterns
func parseFirstLastDayOfDate(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	lower := strings.ToLower(strings.TrimSpace(str))

	var isFirst, isLast bool
	var rest string

	if strings.HasPrefix(lower, "first day of ") {
		isFirst = true
		rest = strings.TrimSpace(lower[13:])
	} else if strings.HasPrefix(lower, "last day of ") {
		isLast = true
		rest = strings.TrimSpace(lower[12:])
	} else {
		return time.Time{}, false
	}

	// Try to parse rest as a relative expression: "+1 month", "-2 months"
	if len(rest) > 0 && (rest[0] == '+' || rest[0] == '-') {
		fields := strings.Fields(rest)
		if len(fields) == 2 {
			amount, err := strconv.Atoi(fields[0])
			if err == nil {
				unit := normalizeTimeUnit(fields[1])
				var refTime time.Time
				switch unit {
				case UnitMonth:
					// Use day=1 as reference to avoid overflow (e.g., Jan 31
					// + 1 month would produce March 2 via time.AddDate, but
					// "first day of +1 month" must land in Feb).
					firstOfCurrent := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
					refTime = firstOfCurrent.AddDate(0, amount, 0)
				case UnitYear:
					refTime = now.AddDate(amount, 0, 0)
				default:
					return time.Time{}, false
				}
				year, month, _ := refTime.Date()
				if isFirst {
					return time.Date(year, month, 1, now.Hour(), now.Minute(), now.Second(), 0, loc), true
				}
				return time.Date(year, month, daysInMonth(year, month), now.Hour(), now.Minute(), now.Second(), 0, loc), true
			}
		}
	}

	// Try to parse rest as YYYY-MM or YYYY-M
	if t, ok := parseYearMonthFormat(rest, loc); ok {
		year, month, _ := t.Date()
		if isFirst {
			return time.Date(year, month, 1, 0, 0, 0, 0, loc), true
		}
		if isLast {
			return time.Date(year, month, daysInMonth(year, month), 0, 0, 0, 0, loc), true
		}
	}

	// Try to parse rest as a month name with optional year and optional trailing time
	fields := strings.Fields(rest)
	if len(fields) >= 1 {
		if month, ok := getMonthByNameFlex(fields[0]); ok {
			idx := 1
			year := now.Year()
			if idx < len(fields) {
				if y, err := strconv.Atoi(fields[idx]); err == nil {
					year = y
					idx++
				}
			}
			hour, minute, second := 0, 0, 0
			if idx < len(fields) {
				h, m, s, consumed, ok := parseFlexTime(fields[idx])
				if ok && consumed == len(fields[idx]) {
					hour, minute, second = h, m, s
					idx++
				}
			}
			if idx != len(fields) {
				return time.Time{}, false
			}
			if isFirst {
				return time.Date(year, month, 1, hour, minute, second, 0, loc), true
			}
			return time.Date(year, month, daysInMonth(year, month), hour, minute, second, 0, loc), true
		}
	}

	return time.Time{}, false
}

// parseOrdinalDate parses "26th Nov" or "December 4th, 2005" etc.
// with optional time. Handles month name followed by ordinal day or ordinal day followed by month.
func parseOrdinalDate(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) < 2 {
		return time.Time{}, false
	}

	// Try "DDth Mon [YYYY] [time]" format
	dayStr := stripOrdinalSuffix(fields[0])
	if day, err := strconv.Atoi(dayStr); err == nil && day >= 1 && day <= 31 {
		if month, ok := getMonthByNameFlex(fields[1]); ok {
			year := now.Year()
			fidx := 2
			if fidx < len(fields) {
				if y, err := strconv.Atoi(fields[fidx]); err == nil {
					year = y
					fidx++
				}
			}
			hour, minute, second := 0, 0, 0
			if fidx < len(fields) && strings.Contains(fields[fidx], ":") {
				h, m, s, _, ok := parseFlexTime(fields[fidx])
				if ok {
					hour, minute, second = h, m, s
				}
			}
			return time.Date(year, month, day, hour, minute, second, 0, loc), true
		}
	}

	return time.Time{}, false
}

// parseMonthDayTimeYear parses "Dec 17 19:30 2005" (month day time year)
func parseMonthDayTimeYear(str string, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) != 4 {
		return time.Time{}, false
	}

	month, ok := getMonthByNameFlex(fields[0])
	if !ok {
		return time.Time{}, false
	}

	dayStr := stripOrdinalSuffix(strings.TrimSuffix(fields[1], ","))
	day, err := strconv.Atoi(dayStr)
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}

	// Third field should be time
	if !strings.Contains(fields[2], ":") {
		return time.Time{}, false
	}
	hour, minute, second, _, ok := parseFlexTime(fields[2])
	if !ok {
		return time.Time{}, false
	}

	// Fourth field should be year
	year, err := strconv.Atoi(fields[3])
	if err != nil {
		return time.Time{}, false
	}

	return time.Date(year, month, day, hour, minute, second, 0, loc), true
}

// parseDateTimeTZRelative parses "YYYY-MM-DD TZ +N unit" or "YYYY-MM-DDThh:mm:ss+ZZZZ +N unit"
// Handles date with timezone followed by one or more relative time adjustments.
// Examples: "2004-10-31 EDT +1 hour", "2004-04-07 00:00:00 -10 day +2 hours"
func parseDateTimeTZRelative(str string, loc *time.Location) (time.Time, bool) {
	// Strategy: collect relative expressions from the end, then parse the remaining date.
	// Each relative expression is " +N unit" or " -N unit" (space + sign + number + space + unit).
	type relExpr struct {
		amount int
		unit   string
	}
	var rels []relExpr
	remaining := str

	// Strip relative expressions from the end
	for {
		remaining = strings.TrimRight(remaining, " ")
		if len(remaining) == 0 {
			break
		}

		// Find the last space-preceded +/- sign
		found := false
		for i := len(remaining) - 1; i > 0; i-- {
			if (remaining[i] == '+' || remaining[i] == '-') && remaining[i-1] == ' ' {
				relPart := remaining[i:]
				relFields := strings.Fields(relPart)
				if len(relFields) != 2 {
					continue
				}
				amount, err := strconv.Atoi(relFields[0])
				if err != nil {
					continue
				}
				unit := normalizeTimeUnit(relFields[1])
				switch unit {
				case UnitDay, UnitWeek, UnitWeekDay, UnitMonth, UnitYear,
					UnitHour, UnitMinute, UnitSecond:
					// Valid relative expression — strip it and continue
					rels = append(rels, relExpr{amount, unit})
					remaining = strings.TrimSpace(remaining[:i])
					found = true
				default:
					continue
				}
				break
			}
		}
		if !found {
			break
		}
	}

	if len(rels) == 0 {
		return time.Time{}, false
	}

	// Try to parse the remaining date part
	datePart := remaining
	var t time.Time
	var ok bool

	if t, ok = parseISO8601(datePart, loc); !ok {
		if t, ok = parseDateTimeFormat(datePart, loc); !ok {
			if t, ok = parseISODateTimeWithTimezone(datePart, loc); !ok {
				if t, ok = parseDateWithTZ(datePart, loc); !ok {
					if t, ok = parseISOFormat(datePart, loc); !ok {
						if t, ok = parseDayMonthYear(datePart, time.Now(), loc); !ok {
							return time.Time{}, false
						}
					}
				}
			}
		}
	}

	// Apply relative expressions in reverse order (innermost first)
	for i := len(rels) - 1; i >= 0; i-- {
		t = applyTimeOffset(t, rels[i].amount, rels[i].unit)
	}

	return t, true
}

// parseDateWithTZ parses "YYYY-MM-DD TZname" (date + timezone without time)
func parseDateWithTZ(str string, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) != 2 {
		return time.Time{}, false
	}
	t, ok := parseISOFormat(fields[0], loc)
	if !ok {
		return time.Time{}, false
	}
	tzLoc, found := tryParseTimezone(fields[1])
	if !found {
		return time.Time{}, false
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tzLoc), true
}

// parseFrontBackOf parses Scottish time expressions:
// "front of 7" = 6:45 (15 minutes before the hour)
// "back of 7" = 7:15 (15 minutes after the hour)
// Also supports AM/PM: "front of 12pm" = 11:45, "back of 3am" = 3:15
func parseFrontBackOf(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	lower := strings.ToLower(strings.TrimSpace(str))

	var isFront bool
	var rest string

	if strings.HasPrefix(lower, "front of ") {
		isFront = true
		rest = strings.TrimSpace(lower[9:])
	} else if strings.HasPrefix(lower, "back of ") {
		isFront = false
		rest = strings.TrimSpace(lower[8:])
	} else {
		return time.Time{}, false
	}

	// Parse the hour, possibly with am/pm suffix
	ampm := ""
	if strings.HasSuffix(rest, "am") {
		ampm = "am"
		rest = strings.TrimSpace(rest[:len(rest)-2])
	} else if strings.HasSuffix(rest, "pm") {
		ampm = "pm"
		rest = strings.TrimSpace(rest[:len(rest)-2])
	}

	hour, err := strconv.Atoi(rest)
	if err != nil || hour < 0 || hour > 24 {
		return time.Time{}, false
	}

	year, month, day := now.Date()
	if isFront {
		// PHP: "front of" with pm adds 12 directly (12pm→24, 1pm→13)
		if ampm == "pm" {
			hour += 12
		}
		// "front of N" = (N-1):45
		return time.Date(year, month, day, hour-1, 45, 0, 0, loc), true
	}
	// "back of" uses standard AM/PM conversion
	if ampm != "" {
		hour = applyAMPM(hour, ampm)
	}
	// "back of N" = N:15
	return time.Date(year, month, day, hour, 15, 0, 0, loc), true
}

// romanNumeralMonths maps Roman numerals to month numbers
var romanNumeralMonths = map[string]time.Month{
	"i": time.January, "ii": time.February, "iii": time.March,
	"iv": time.April, "v": time.May, "vi": time.June,
	"vii": time.July, "viii": time.August, "ix": time.September,
	"x": time.October, "xi": time.November, "xii": time.December,
}

// parseRomanNumeralDate parses dates with Roman numeral months: "20 VI. 2005", "1 III 2010"
func parseRomanNumeralDate(str string, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) < 3 {
		return time.Time{}, false
	}

	// Parse day
	day, err := strconv.Atoi(fields[0])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}

	// Parse Roman numeral month (strip trailing period)
	monthStr := strings.ToLower(strings.TrimSuffix(fields[1], "."))
	month, ok := romanNumeralMonths[monthStr]
	if !ok {
		return time.Time{}, false
	}

	// Parse year
	year, err := strconv.Atoi(fields[2])
	if err != nil {
		return time.Time{}, false
	}

	if !IsValidDate(year, int(month), day) {
		return time.Time{}, false
	}

	return time.Date(year, month, day, 0, 0, 0, 0, loc), true
}

// parseNumberedWeekday parses formats like "1 Monday December 2008", "second Monday December 2008"
// It handles formats like "first Monday of December 2008" or "3rd Friday of January"
// Also handles "+N week Thursday Nov 2007" (week offset + weekday + month + year)
// Also handles "3 tuesday" or "third tuesday" (no month context — count forward from now)
// Also handles "third tuesday of this month" (calendar month context)
func parseNumberedWeekday(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	fields := strings.Fields(str)
	if len(fields) < 2 {
		return time.Time{}, false
	}

	idx := 0

	// Parse the ordinal (numeric, word, or implicit from bare weekday)
	ordinal, isWordOrdinal, idx, ok := parseOrdinalPrefix(fields, idx)
	if !ok {
		return time.Time{}, false
	}

	// Handle "+N week(s) Weekday Month Year" — skip "week(s)" after numeric ordinal
	if idx < len(fields) {
		unit := normalizeTimeUnit(fields[idx])
		if unit == UnitWeek {
			isWordOrdinal = true // week-based ordinals use same semantics as word ordinals
			idx++
		}
	}

	// Parse the day of week or "day" keyword
	if idx >= len(fields) {
		return time.Time{}, false
	}
	isDayOfMonth := false
	dayOfWeek := getDayOfWeek(fields[idx])
	if dayOfWeek < 0 {
		if strings.ToLower(fields[idx]) == "day" {
			// PHP only supports "first day of" and "last day of"
			if ordinal != 1 && ordinal != -1 {
				return time.Time{}, false
			}
			isDayOfMonth = true
		} else {
			return time.Time{}, false
		}
	}
	idx++

	// Check for optional "of" — its presence changes ordinal semantics
	hasOf := false
	if idx < len(fields) && strings.ToLower(fields[idx]) == "of" {
		hasOf = true
		idx++
	}

	// No month context: "3 tuesday", "third tuesday" — count forward from base date
	// Exclude ordinal=-1 ("last weekday") — that's handled by the token parser.
	if idx >= len(fields) && !hasOf && !isDayOfMonth && ordinal > 0 {
		currentDay := int(now.Weekday())
		daysUntil := (dayOfWeek - currentDay + 7) % 7
		if isWordOrdinal && daysUntil == 0 {
			daysUntil = 7
		}
		totalDays := daysUntil + (ordinal-1)*7
		h, mi, s := now.Hour(), now.Minute(), now.Second()
		if isWordOrdinal {
			h, mi, s = 0, 0, 0
		}
		return time.Date(now.Year(), now.Month(), now.Day()+totalDays, h, mi, s, 0, loc), true
	}

	// Parse the month context: either a literal month name or "this/next/last month/year"
	if idx >= len(fields) {
		return time.Time{}, false
	}

	var month time.Month
	var year int
	relativeYears := 0 // PHP applies year offset AFTER weekday adjustment

	direction := strings.ToLower(fields[idx])
	if direction == DirectionNext || direction == DirectionLast || direction == "this" {
		// Relative month/year: "next month", "last year", etc.
		idx++
		if idx >= len(fields) {
			return time.Time{}, false
		}
		unit := normalizeTimeUnit(fields[idx])
		idx++

		switch unit {
		case UnitMonth:
			// Use day=1 as reference to avoid overflow when base day doesn't
			// exist in the target month (e.g., Jan 31 → "next month" must
			// resolve within Feb, not overflow into March).
			firstOfCurrent := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
			if direction == DirectionNext {
				ref := firstOfCurrent.AddDate(0, 1, 0)
				month = ref.Month()
				year = ref.Year()
			} else if direction == DirectionLast {
				ref := firstOfCurrent.AddDate(0, -1, 0)
				month = ref.Month()
				year = ref.Year()
			} else {
				// "this month" — current month/year unchanged
				month = now.Month()
				year = now.Year()
			}
		case UnitYear:
			// PHP: weekday is resolved in the BASE year, then the year offset is
			// applied afterward. This means the day-of-week may shift.
			month = now.Month()
			year = now.Year()
			if direction == DirectionNext {
				relativeYears = 1
			} else if direction == DirectionLast {
				relativeYears = -1
			}
			// "this year" — no offset needed
		default:
			return time.Time{}, false
		}
	} else {
		// Literal month name
		var ok bool
		month, ok = getMonthByName(fields[idx])
		if !ok {
			return time.Time{}, false
		}
		idx++

		// Parse the optional year
		year = now.Year()
		if idx < len(fields) {
			var err error
			year, err = strconv.Atoi(fields[idx])
			if err != nil || year < 1 || year > 9999 {
				return time.Time{}, false
			}
			idx++
		}
	}

	// Parse optional trailing time expression (HH:MM[:SS])
	trailingHour, trailingMinute, trailingSecond := 0, 0, 0
	hasTrailingTime := false
	if idx < len(fields) {
		h, m, s, consumed, ok := parseFlexTime(fields[idx])
		if ok && consumed == len(fields[idx]) {
			trailingHour, trailingMinute, trailingSecond = h, m, s
			hasTrailingTime = true
			idx++
		}
	}

	// Make sure we consumed all fields
	if idx != len(fields) {
		return time.Time{}, false
	}

	var resultDay int

	if isDayOfMonth {
		// "first/last day of" — use ordinal directly as the day number
		lastDay := daysInMonth(year, month)
		if ordinal > 0 {
			resultDay = ordinal
			if resultDay > lastDay {
				return time.Time{}, false
			}
		} else if ordinal == -1 {
			resultDay = lastDay
		} else {
			return time.Time{}, false
		}
	} else {
		// Weekday-based: find the nth occurrence of the specified day of week.
		// PHP pipeline: weekday is resolved in current year/month, THEN
		// relative year offset is applied (which may shift the day-of-week).
		if ordinal > 0 {
			// PHP algorithm for positive ordinals:
			// 1. Set d=1 in current month
			// 2. Find next occurrence of target weekday (with skip for word ordinals)
			// 3. Add (ordinal-1)*7 days
			// 4. Later: add relative years
			firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
			firstDayOfWeek := int(firstOfMonth.Weekday())
			daysUntilFirst := (dayOfWeek - firstDayOfWeek + 7) % 7

			if isWordOrdinal && !hasOf && daysUntilFirst == 0 {
				// Word ordinals without "of" skip when target weekday matches the 1st
				resultDay = 1 + daysUntilFirst + ordinal*7
			} else {
				resultDay = 1 + daysUntilFirst + (ordinal-1)*7
			}

			// When a relative year offset is involved, allow overflow
			// (day-of-week may shift when the year changes).
			// Also allow overflow for all ordinal expressions —
			// PHP permits e.g. "sixth Monday of January" which overflows into February.
			// Go's time.Date naturally handles day overflow.
		} else if ordinal == -1 {
			if hasOf {
				// "last Thursday of November" — PHP: go to 1st of NEXT month,
				// find target weekday, subtract 7 days.
				// For relative year: the weekday search is in the base year.
				nextMonth := month + 1
				nextMonthYear := year
				if nextMonth > 12 {
					nextMonth = 1
					nextMonthYear++
				}
				firstOfNext := time.Date(nextMonthYear, nextMonth, 1, 0, 0, 0, 0, loc)
				firstDayOfWeek := int(firstOfNext.Weekday())
				daysUntilTarget := (dayOfWeek - firstDayOfWeek + 7) % 7
				// Go to first occurrence in next month, then back 7 days
				resultDay = daysInMonth(year, month) + 1 + daysUntilTarget - 7
			} else {
				// "last Thursday November" — last occurrence BEFORE the 1st of the month
				firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
				firstDayOfWeek := int(firstOfMonth.Weekday())
				daysBack := (firstDayOfWeek - dayOfWeek + 7) % 7
				if daysBack == 0 {
					daysBack = 7
				}
				resultDay = 1 - daysBack
				return time.Date(year, month, resultDay, 0, 0, 0, 0, loc), true
			}
		} else {
			return time.Time{}, false
		}
	}

	// PHP: "first/last day of" preserves the base time's hour/minute/second;
	// weekday expressions reset to midnight. An explicit trailing time overrides both.
	h, mi, s := 0, 0, 0
	if isDayOfMonth {
		h, mi, s = now.Hour(), now.Minute(), now.Second()
	}
	if hasTrailingTime {
		h, mi, s = trailingHour, trailingMinute, trailingSecond
	}
	// Apply relative year offset AFTER weekday resolution (PHP pipeline order).
	// This means the day-of-week may shift when the year changes.
	return time.Date(year+relativeYears, month, resultDay, h, mi, s, 0, loc), true
}
