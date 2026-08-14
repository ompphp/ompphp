package strtotime

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// parseWithTimezone tries to parse dates with timezone information
// Examples: "January 1 2023 PST", "June 1 1985 16:30:00 Europe/Paris", "2005-07-14 22:30:41 GMT"
func parseWithTimezone(str string, loc *time.Location) (time.Time, bool) {
	// First try the full date + time + timezone format
	if t, ok := parseFullDateTimeWithTimezone(str, loc); ok {
		return t, ok
	}

	// Try to parse ISO format date + time + timezone
	// Format: YYYY-M-D H:M:S timezone
	if t, ok := parseISODateTimeWithTimezone(str, loc); ok {
		return t, ok
	}

	// Try just time + timezone (e.g., "22:30:41 GMT")
	if t, ok := parseTimeOnlyWithTimezone(str, loc); ok {
		return t, ok
	}

	return time.Time{}, false
}

// parseISODateTimeWithTimezone parses "YYYY-MM-DD HH:MM:SS timezone"
func parseISODateTimeWithTimezone(str string, loc *time.Location) (time.Time, bool) {
	// Need at least "Y-M-D H:M:S T" which is quite short
	// Find the date part (ends at first space)
	spaceIdx := strings.IndexByte(str, ' ')
	if spaceIdx < 0 {
		return time.Time{}, false
	}

	datePart := str[:spaceIdx]

	// Validate date part looks like YYYY-M-D
	if strings.Count(datePart, "-") != 2 {
		return time.Time{}, false
	}

	// Remaining part: "HH:MM:SS timezone"
	rest := strings.TrimSpace(str[spaceIdx+1:])

	// Find the space between time and timezone
	timeSpaceIdx := strings.IndexByte(rest, ' ')
	if timeSpaceIdx < 0 {
		return time.Time{}, false
	}

	timePart := rest[:timeSpaceIdx]
	tzString := strings.TrimSpace(rest[timeSpaceIdx+1:])

	// Parse time part H:M:S
	timeParts := strings.Split(timePart, ":")
	if len(timeParts) != 3 {
		return time.Time{}, false
	}

	hour, errH := strconv.Atoi(timeParts[0])
	minute, errM := strconv.Atoi(timeParts[1])
	second, errS := strconv.Atoi(timeParts[2])

	if errH != nil || hour < 0 || hour > 23 ||
		errM != nil || minute < 0 || minute > 59 ||
		errS != nil || second < 0 || second > 59 {
		return time.Time{}, false
	}

	// Validate timezone contains only valid characters (alphanumeric, /, _, .)
	for _, c := range tzString {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '/' && c != '_' && c != '.' {
			return time.Time{}, false
		}
	}

	// Parse the date
	t, ok := parseISOFormat(datePart, loc)
	if !ok {
		return time.Time{}, false
	}

	// Add the time components
	t = time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, 0, t.Location())

	// Parse timezone
	tzLoc, found := tryParseTimezone(tzString)
	if !found {
		return time.Time{}, false
	}

	// Create the time directly in the target timezone (preserving wall clock time)
	return time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, 0, tzLoc), true
}

// parseTimeOnlyWithTimezone parses "HH:MM[:SS] timezone"
func parseTimeOnlyWithTimezone(str string, loc *time.Location) (time.Time, bool) {
	// Find the last space (separating time from timezone)
	spaceIdx := strings.LastIndexByte(str, ' ')
	if spaceIdx < 0 {
		return time.Time{}, false
	}

	timePart := strings.TrimSpace(str[:spaceIdx])
	tzString := strings.TrimSpace(str[spaceIdx+1:])

	// Parse time part
	timeParts := strings.Split(timePart, ":")
	if len(timeParts) < 2 || len(timeParts) > 3 {
		return time.Time{}, false
	}

	hour, errH := strconv.Atoi(timeParts[0])
	minute, errM := strconv.Atoi(timeParts[1])
	second := 0
	var errS error
	if len(timeParts) == 3 {
		second, errS = strconv.Atoi(timeParts[2])
	}

	if errH != nil || hour < 0 || hour > 23 ||
		errM != nil || minute < 0 || minute > 59 ||
		(len(timeParts) == 3 && (errS != nil || second < 0 || second > 59)) {
		return time.Time{}, false
	}

	// Validate timezone contains only valid characters
	for _, c := range tzString {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '/' && c != '_' && c != '.' {
			return time.Time{}, false
		}
	}

	// Parse timezone
	tzLoc, found := tryParseTimezone(tzString)
	if !found {
		return time.Time{}, false
	}

	now := time.Now().In(tzLoc)
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, tzLoc), true
}

// parseFullDateTimeWithTimezone parses the month name + day + year + time + timezone format
func parseFullDateTimeWithTimezone(str string, loc *time.Location) (time.Time, bool) {
	// Format: "MonthName Day Year [HH:MM[:SS]] Timezone"
	// Examples: "January 1 2023 PST", "June 1 1985 16:30:00 Europe/Paris"
	fields := strings.Fields(str)
	if len(fields) < 4 {
		return time.Time{}, false
	}

	idx := 0

	// Parse month name
	month, ok := getMonthByName(fields[idx])
	if !ok {
		return time.Time{}, false
	}
	idx++

	// Parse day (may have ordinal suffix like "1st", "2nd", "3rd", "4th")
	dayStr := fields[idx]
	// Strip ordinal suffixes
	for _, suffix := range []string{"st", "nd", "rd", "th"} {
		if strings.HasSuffix(dayStr, suffix) {
			dayStr = dayStr[:len(dayStr)-len(suffix)]
			break
		}
	}
	day, err := strconv.Atoi(dayStr)
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	idx++

	// Parse year
	if idx >= len(fields) {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(fields[idx])
	if err != nil || year < 1 || year > 9999 {
		return time.Time{}, false
	}
	idx++

	// Check if date is valid
	maxDays := 31
	switch month {
	case time.April, time.June, time.September, time.November:
		maxDays = 30
	case time.February:
		if IsLeapYear(year) {
			maxDays = 29
		} else {
			maxDays = 28
		}
	}
	if day > maxDays {
		return time.Time{}, false
	}

	// Default time components
	hour, minute, second := 0, 0, 0

	// The remaining fields could be: time timezone, or just timezone
	if idx >= len(fields) {
		return time.Time{}, false
	}

	// Check if next field looks like a time (contains ':')
	if strings.Contains(fields[idx], ":") {
		timeParts := strings.Split(fields[idx], ":")
		if len(timeParts) < 2 || len(timeParts) > 3 {
			return time.Time{}, false
		}

		hourVal, err := strconv.Atoi(timeParts[0])
		if err != nil || hourVal < 0 || hourVal > 23 {
			return time.Time{}, false
		}
		hour = hourVal

		minuteVal, err := strconv.Atoi(timeParts[1])
		if err != nil || minuteVal < 0 || minuteVal > 59 {
			return time.Time{}, false
		}
		minute = minuteVal

		if len(timeParts) == 3 {
			secondVal, err := strconv.Atoi(timeParts[2])
			if err != nil || secondVal < 0 || secondVal > 59 {
				return time.Time{}, false
			}
			second = secondVal
		}

		idx++
	}

	// Parse timezone (must be the last field)
	if idx >= len(fields) {
		return time.Time{}, false
	}

	// Remaining fields form the timezone (could be "America/New_York" which is one field,
	// or multi-word but typically one field since Fields splits on spaces)
	tzString := strings.Join(fields[idx:], " ")

	tzLoc, found := tryParseTimezone(tzString)
	if !found {
		return time.Time{}, false
	}

	return time.Date(year, month, day, hour, minute, second, 0, tzLoc), true
}
