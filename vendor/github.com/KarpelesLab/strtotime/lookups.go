package strtotime

import (
	"strings"
	"time"
)

// monthNames maps month name strings (lowercase) to time.Month values.
var monthNames = map[string]time.Month{
	"january":   time.January,
	"jan":       time.January,
	"february":  time.February,
	"feb":       time.February,
	"march":     time.March,
	"mar":       time.March,
	"april":     time.April,
	"apr":       time.April,
	"may":       time.May,
	"june":      time.June,
	"jun":       time.June,
	"july":      time.July,
	"jul":       time.July,
	"august":    time.August,
	"aug":       time.August,
	"september": time.September,
	"sep":       time.September,
	"october":   time.October,
	"oct":       time.October,
	"november":  time.November,
	"nov":       time.November,
	"december":  time.December,
	"dec":       time.December,
}

// getMonthByName returns the time.Month for a month name.
// Input should be lowercase (StrToTime lowercases before calling parsers).
// Handles trailing periods (e.g., "dec." → December).
func getMonthByName(name string) (time.Month, bool) {
	month, ok := monthNames[name]
	if ok {
		return month, true
	}
	month, ok = monthNames[strings.TrimSuffix(name, ".")]
	return month, ok
}

// getDayOfWeek converts a day name to day number (0 = Sunday, 6 = Saturday).
// Input should be lowercase. Returns -1 if the name is not recognized.
func getDayOfWeek(day string) int {
	switch day {
	case "sunday", "sun":
		return 0
	case "monday", "mon":
		return 1
	case "tuesday", "tue":
		return 2
	case "wednesday", "wed":
		return 3
	case "thursday", "thu":
		return 4
	case "friday", "fri":
		return 5
	case "saturday", "sat":
		return 6
	default:
		return -1
	}
}

// unitMap maps unit name strings to canonical unit constants.
var unitMap = map[string]string{
	// Day variations
	"d": UnitDay, "day": UnitDay, "days": UnitDay, "days.": UnitDay,
	// Week variations
	"w": UnitWeek, "wk": UnitWeek, "wks": UnitWeek, "wks.": UnitWeek,
	"week": UnitWeek, "weeks": UnitWeek,
	// Weekday (business day) variations
	"weekday": UnitWeekDay, "weekdays": UnitWeekDay,
	// Month variations
	"m": UnitMonth, "mon": UnitMonth, "mons": UnitMonth, "mons.": UnitMonth,
	"month": UnitMonth, "months": UnitMonth,
	// Year variations
	"y": UnitYear, "yr": UnitYear, "yrs": UnitYear, "yrs.": UnitYear,
	"year": UnitYear, "years": UnitYear,
	// Hour variations
	"h": UnitHour, "hr": UnitHour, "hrs": UnitHour, "hrs.": UnitHour,
	"hour": UnitHour, "hours": UnitHour, "hourss": UnitHour,
	// Minute variations
	"min": UnitMinute, "mins": UnitMinute, "mins.": UnitMinute,
	"minute": UnitMinute, "minutes": UnitMinute,
	// Second variations
	"sec": UnitSecond, "secs": UnitSecond, "secs.": UnitSecond,
	"second": UnitSecond, "seconds": UnitSecond,
}

// normalizeTimeUnit converts various time unit notations to a canonical form.
// Input should be lowercase.
func normalizeTimeUnit(unit string) string {
	if canonical, found := unitMap[unit]; found {
		return canonical
	}

	trimmed := strings.TrimSuffix(unit, "s")
	if canonical, found := unitMap[trimmed]; found {
		return canonical
	}

	if strings.HasPrefix(unit, "day") {
		return UnitDay
	} else if strings.HasPrefix(unit, "weekday") {
		return UnitWeekDay
	} else if strings.HasPrefix(unit, "week") {
		return UnitWeek
	} else if strings.HasPrefix(unit, "month") {
		return UnitMonth
	} else if strings.HasPrefix(unit, "year") {
		return UnitYear
	} else if strings.HasPrefix(unit, "hour") || strings.HasPrefix(unit, "hr") {
		return UnitHour
	} else if strings.HasPrefix(unit, "min") {
		return UnitMinute
	} else if strings.HasPrefix(unit, "sec") {
		return UnitSecond
	}

	return unit
}

// ordinalWordToNumber converts ordinal words ("first", "second", ..., "twelfth") to numbers.
// Returns 0 for unrecognized words.
func ordinalWordToNumber(word string) int {
	switch strings.ToLower(word) {
	case "first":
		return 1
	case "second":
		return 2
	case "third":
		return 3
	case "fourth":
		return 4
	case "fifth":
		return 5
	case "sixth":
		return 6
	case "seventh":
		return 7
	case "eighth":
		return 8
	case "ninth":
		return 9
	case "tenth":
		return 10
	case "eleventh":
		return 11
	case "twelfth":
		return 12
	}
	return 0
}

// parseTwoDigitYear converts a 2-digit year to a 4-digit year.
// 00-69 → 2000-2069, 70-99 → 1970-1999.
func parseTwoDigitYear(year int) int {
	if year < 100 {
		if year < 70 {
			return year + 2000
		}
		return year + 1900
	}
	return year
}

// applyAMPM converts an hour to 24-hour format based on AM/PM indicator.
func applyAMPM(hour int, ampm string) int {
	if ampm == "am" {
		if hour == 12 {
			return 0
		}
		return hour
	}
	if hour == 12 {
		return 12
	}
	return hour + 12
}

// daysInMonth returns the number of days in the given month/year.
func daysInMonth(year int, month time.Month) int {
	nextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	lastDay := nextMonth.AddDate(0, 0, -1)
	return lastDay.Day()
}
