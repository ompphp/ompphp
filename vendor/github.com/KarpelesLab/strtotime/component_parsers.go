package strtotime

import (
	"strconv"
	"strings"
	"time"
)

// A componentParser is the Into variant of a format parser: it populates a
// ParsedDate with the components it extracted from the input, and returns
// true if the input matched its format.
type componentParser func(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool

// formatParsers is the ordered pipeline shared by StrToTime and DateParse.
// Each entry is a wrapper around one of the parse* functions in
// date_formats.go / extended_formats.go / iso8601.go / date_with_timezone.go,
// with explicit knowledge of which ParsedDate fields that parser populates.
var formatParsers = []componentParser{
	guardDigit(wrapDateOnly(parseEuropeanFormat)),
	guardPrefix("front of ", "back of ")(parseFrontBackOfInto),
	guardDigit(wrapDateOnly(parseRomanNumeralDate)),
	guardPrefix("0000-00-00")(parseZeroDateInto),
	guardByte('-', '+')(parseSignedYearInto),
	guardByte('-', '+')(parseBareNumericOffsetInto),
	parseISO8601Into,
	parseDateTimeFormatInto,
	parseTimeWithNumericOffsetInto,
	parseTimeWithNamedTZInto,
	parseWithTimezoneInto,
	wrapDateOnly(parseISOFormat),
	guardDigit(parseInvalidISOFormatInto),
	guardDigit(parseInvalidDottedDateInto),
	parseInvalidMonthNameDateInto,
	guardDigit(parseLargeYearAsTimeInto),
	parseYearMonthFormatInto,
	guardDigit(wrapDateOnly(parseSlashFormat)),
	guardDigit(wrapDateOnly(parseUSFormat)),
	guardDigit(parseUSDateWithTimeInto),
	guardDigit(parseShortYearUSDateWithMilitaryTimeInto),
	guardDigit(parseCompactDateWithTimeInto),
	guardDigit(parseCompactTimestampInto),
	parseCompactTimeFormatsInto,
	parseMonthNameFormatInto,
	guardDigit(parseHTTPLogFormatInto),
	parseDateTimeTZRelativeInto,
	parseDateWithTZInto,
	parseDayMonthYearInto,
	parseMonthYearOnlyInto,
	guardDigit(parseTimeBeforeDateInto),
	parseMonthDayTimeYearInto,
	parseFirstLastDayOfDateInto,
	parseNumberedWeekdayInto,
	guardDigit(parseOrdinalOfMonthYearInto),
	parseBareTimezoneInto,
	guardDigit(parseBareDigitsFallbackInto),
}

// --- guards (componentParser flavor) ---

func guardDigit(fn componentParser) componentParser {
	return func(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
		if len(str) == 0 || str[0] < '0' || str[0] > '9' {
			return false
		}
		return fn(str, now, loc, opts, pd)
	}
}

func guardByte(bytes ...byte) func(componentParser) componentParser {
	return func(fn componentParser) componentParser {
		return func(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
			if len(str) == 0 {
				return false
			}
			for _, b := range bytes {
				if str[0] == b {
					return fn(str, now, loc, opts, pd)
				}
			}
			return false
		}
	}
}

func guardPrefix(prefixes ...string) func(componentParser) componentParser {
	return func(fn componentParser) componentParser {
		return func(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
			for _, p := range prefixes {
				if strings.HasPrefix(str, p) {
					return fn(str, now, loc, opts, pd)
				}
			}
			return false
		}
	}
}

// wrapDateOnly wraps a legacy date-only parser (yields only y/m/d with time = 00:00:00)
// as a componentParser that records SetDate only.
func wrapDateOnly(fn func(string, *time.Location) (time.Time, bool)) componentParser {
	return func(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
		t, ok := fn(str, loc)
		if !ok {
			return false
		}
		pd.SetDate(t.Year(), int(t.Month()), t.Day())
		pd.setMaterialized(t)
		return true
	}
}

// parseInvalidMonthNameDateInto matches "<MonthName> DD YYYY" where DD is
// out of range for that month. PHP still reports the components plus a
// warning.
func parseInvalidMonthNameDateInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	fields := strings.Fields(str)
	if len(fields) != 3 {
		return false
	}
	m, isMonth := getMonthByNameFlex(fields[0])
	if !isMonth {
		return false
	}
	day, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(fields[1], "st"), "nd"), "rd"), "th"))
	if err != nil || day < 1 || day > 31 {
		return false
	}
	year, err := strconv.Atoi(fields[2])
	if err != nil || year < 1 || year > 9999 {
		return false
	}
	if IsValidDate(year, int(m), day) {
		return false
	}
	pd.SetDate(year, int(m), day)
	pd.AddWarning(len(str)+1, "The parsed date was invalid")
	pd.setMaterialized(time.Date(year, m, day, 0, 0, 0, 0, loc))
	return true
}

// parseInvalidISOFormatInto catches ISO "YYYY-MM-DD" inputs whose components
// are out of range (month 0, day 29 in Feb of non-leap, day > daysInMonth).
// PHP still reports the year/month/day it saw plus a warning "The parsed
// date was invalid" at position == len(str).
func parseInvalidISOFormatInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	if strings.Count(str, "-") != 2 {
		return false
	}
	parts := strings.Split(str, "-")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if !isAllDigits(p) || len(p) == 0 {
			return false
		}
	}
	// Only match 4-digit year form (YYYY-MM-DD).
	if len(parts[0]) < 4 {
		return false
	}
	y, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	d, _ := strconv.Atoi(parts[2])
	if IsValidDate(y, m, d) {
		return false
	}
	// Only accept "gently wrong" dates — month 0-12 and day within a wide
	// range — so obviously-invalid inputs like 2023-99-99 still fail.
	if m < 0 || m > 12 || d < 0 || d > 31 {
		return false
	}
	pd.SetDate(y, m, d)
	// PHP reports the warning position as len(str)+1 (one past the end of
	// the parsed date portion).
	pd.AddWarning(len(str)+1, "The parsed date was invalid")
	// Materialize via Go's tolerant date arithmetic so StrToTime still
	// returns a time.Time (matches the original behaviour before this
	// refactor when validation was not enforced).
	pd.setMaterialized(time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc))
	return true
}

// parseInvalidDottedDateInto catches "DD.MM.YYYY" inputs whose components are
// out of range (e.g. "00.00.0000"), optionally followed by " [- ]HH:MM:SS".
// PHP's date_parse reports the literal components, a warning "The parsed date
// was invalid" at the end, and an "Unexpected character" error for the "-"
// separator when one is present (bug41709).
func parseInvalidDottedDateInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	// Find the "DD.MM.YYYY" prefix.
	datePart, rest := str, ""
	if sp := strings.IndexByte(str, ' '); sp >= 0 {
		datePart = str[:sp]
		rest = str[sp:]
	}
	if strings.Count(datePart, ".") != 2 {
		return false
	}
	parts := strings.Split(datePart, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if !isAllDigits(p) || len(p) == 0 {
			return false
		}
	}
	if len(parts[2]) < 4 {
		return false
	}
	d, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	y, _ := strconv.Atoi(parts[2])
	if IsValidDate(y, m, d) {
		return false
	}
	if m < 0 || m > 12 || d < 0 || d > 31 {
		return false
	}

	pd.SetDate(y, m, d)

	// Optional trailing time, possibly with a "-" separator.
	hasTime := false
	var hour, minute, second, nanos int
	if trimmed := strings.TrimLeft(rest, " \t"); trimmed != "" {
		leading := len(rest) - len(trimmed)
		// Record "Unexpected character" error when a '-' separates date and time.
		if len(trimmed) > 0 && trimmed[0] == '-' {
			dashPos := len(datePart) + leading
			pd.AddError(dashPos, "Unexpected character")
			trimmed = strings.TrimLeft(trimmed[1:], " \t")
		}
		if trimmed != "" {
			h, mm, s, n, consumed, ok := parseISO8601Time(trimmed)
			if !ok || strings.TrimSpace(trimmed[consumed:]) != "" {
				return false
			}
			hour, minute, second, nanos = h, mm, s, n
			hasTime = true
		}
	}

	if hasTime {
		pd.SetTime(hour, minute, second)
		if nanos != 0 {
			pd.SetFraction(float64(nanos) / 1e9)
		}
	}
	pd.AddWarning(len(str)+1, "The parsed date was invalid")
	pd.setMaterialized(time.Date(y, time.Month(m), d, hour, minute, second, nanos, loc))
	return true
}

// parseTimeWithNamedTZInto matches "HH:MM[:SS][.frac] <TZname>" where
// <TZname> is an abbreviation, IANA identifier, or "Z". Emits time plus TZ
// metadata, no date.
func parseTimeWithNamedTZInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	fields := strings.Fields(str)
	if len(fields) != 2 {
		return false
	}
	timePart := fields[0]
	tzStr := fields[1]
	if !strings.Contains(timePart, ":") {
		return false
	}
	if len(tzStr) == 0 || tzStr[0] == '+' || tzStr[0] == '-' {
		return false
	}
	if _, found := tryParseTimezone(tzStr); !found && !strings.EqualFold(tzStr, "Z") {
		return false
	}

	h, m, s, consumed, ok := parseFlexTime(timePart)
	if !ok {
		return false
	}
	hasFrac := false
	var frac float64
	if consumed < len(timePart) && timePart[consumed] == '.' {
		fs := timePart[consumed+1:]
		if f, err := strconv.ParseFloat("0."+fs, 64); err == nil {
			frac = f
			hasFrac = true
		}
	}

	pd.SetTime(h, m, s)
	if hasFrac {
		pd.SetFraction(frac)
	}
	if strings.EqualFold(tzStr, "Z") {
		pd.SetTZAbbreviation(time.UTC, "Z", 0, false)
	} else if resolved, found := tryParseTimezone(tzStr); found {
		setTZFromName(pd, tzStr, resolved)
	}
	pd.setMaterialized(time.Date(now.Year(), now.Month(), now.Day(), h, m, s, int(frac*1e9), loc))
	return true
}

// parseCompoundRelativeInto handles compound purely-relative inputs like
// "-1 week +2 days" / "+1 year -2 months" / "-3 hours +10 minutes". The
// result is reported as a relative-only ParsedDate with no absolute date.
func parseCompoundRelativeInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	normalizer := strings.NewReplacer(" + ", " +", " - ", " -", "+ ", "+", "- ", "-")
	s := strings.TrimSpace(normalizer.Replace(str))

	var parts []string
	current := ""
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c == '+' || c == '-') && i > 0 && current != "" {
			parts = append(parts, strings.TrimSpace(current))
			current = string(c)
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, strings.TrimSpace(current))
	}
	if len(parts) < 2 {
		return false
	}

	// Each part must be a pure relative expression: "[+-]N unit".
	type relPart struct {
		amount int
		unit   string
	}
	var rels []relPart
	for _, p := range parts {
		sign := 1
		body := p
		if len(body) == 0 {
			return false
		}
		switch body[0] {
		case '+':
			body = body[1:]
		case '-':
			sign = -1
			body = body[1:]
		}
		body = strings.TrimSpace(body)
		fields := strings.Fields(body)
		if len(fields) != 2 {
			return false
		}
		amount, err := strconv.Atoi(fields[0])
		if err != nil {
			return false
		}
		unit := normalizeTimeUnit(fields[1])
		switch unit {
		case UnitYear, UnitMonth, UnitWeek, UnitDay, UnitHour, UnitMinute, UnitSecond:
		default:
			return false
		}
		rels = append(rels, relPart{sign * amount, unit})
	}

	for _, r := range rels {
		pd.AddRelative(r.unit, r.amount)
	}
	// Materialize for StrToTime so it still returns a meaningful time.Time.
	t := now
	for _, r := range rels {
		t = applyTimeOffset(t, r.amount, r.unit)
	}
	pd.setMaterialized(t)
	pd.relativeApplied = true
	return true
}

// parseBareTimezoneInto matches a standalone timezone string like "UTC",
// "EST", "Z", "Asia/Tokyo". PHP reports is_localtime=true plus the
// appropriate zone_type, and no date or time components. Unknown alphabetic
// tokens (e.g. "YYYY", "foo") are reported with is_localtime=true,
// zone_type=0, and an error "The timezone could not be found in the database".
func parseBareTimezoneInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	s := strings.TrimSpace(str)
	if strings.ContainsAny(s, " \t") {
		return false
	}
	// Reject obvious non-TZ forms (digits, punctuation, etc.).
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '/' || c == '_' || c == '-') {
			return false
		}
	}
	// Special-case "Z".
	if strings.EqualFold(s, "Z") {
		pd.SetTZAbbreviation(time.UTC, "Z", 0, false)
		pd.setMaterialized(now.In(time.UTC))
		return true
	}
	if resolved, found := tryParseTimezone(s); found {
		setTZFromName(pd, s, resolved)
		pd.setMaterialized(now.In(resolved))
		return true
	}
	// Unknown — PHP emits a zone_type 0 placeholder plus a TZ lookup error.
	// Only match if the input isn't a recognized month, weekday, or known
	// keyword — those have their own handlers.
	lower := strings.ToLower(s)
	if _, ok := getMonthByNameFlex(lower); ok {
		return false
	}
	if getDayOfWeek(lower) >= 0 {
		return false
	}
	switch lower {
	case "noon", "midnight", "tomorrow", "yesterday", "today", "now",
		"am", "pm", "next", "last", "this", "first", "third",
		"fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth",
		"ago", "of", "day", "week", "month", "year", "hour", "minute",
		"second", "days", "weeks", "months", "years", "hours", "minutes",
		"seconds":
		return false
	}
	pd.IsLocaltime = true
	pd.ZoneType = 0
	pd.AddError(0, "The timezone could not be found in the database")
	return true
}

// parseOrdinalOfMonthYearInto matches PHP's "Nth of <Month> <Year>" quirk.
// PHP ignores the ordinal portion (reports day=1), emits a "Double timezone
// specification" warning at the position of the space before "of", and
// produces sequential-array errors for the unexpected tail.
func parseOrdinalOfMonthYearInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	fields := strings.Fields(str)
	if len(fields) != 4 {
		return false
	}
	// First field: ordinal digits + suffix.
	ordPart := fields[0]
	suffix := ""
	digits := ordPart
	for _, sfx := range []string{"st", "nd", "rd", "th"} {
		if strings.HasSuffix(strings.ToLower(ordPart), sfx) {
			suffix = sfx
			digits = ordPart[:len(ordPart)-len(sfx)]
			break
		}
	}
	if suffix == "" || digits == "" || !isAllDigits(digits) {
		return false
	}
	if _, err := strconv.Atoi(digits); err != nil {
		return false
	}
	if strings.ToLower(fields[1]) != "of" {
		return false
	}
	month, isMonth := getMonthByNameFlex(fields[2])
	if !isMonth {
		return false
	}
	year, err := strconv.Atoi(fields[3])
	if err != nil || year < 1 {
		return false
	}

	pd.SetDate(year, int(month), 1)
	// Warning at position 1 past the ordinal token.
	pd.AddWarning(len(digits)+len(suffix)+1, "Double timezone specification")
	// Sequential-key errors: one "Unexpected character" per ordinal digit,
	// followed by a "The timezone could not be found in the database".
	pos := 0
	for range digits {
		pd.AddError(pos, "Unexpected character")
		pos++
	}
	pd.AddError(pos, "The timezone could not be found in the database")
	pd.IsLocaltime = true
	pd.ZoneType = 0
	pd.setMaterialized(time.Date(year, month, 1, 0, 0, 0, 0, loc))
	return true
}

// parseBareDigitsFallbackInto handles bare pure-digit inputs that aren't a
// valid year and aren't a 4-digit HHMM. PHP:
//   - 1..3 digits: one "Unexpected character" per digit (ignored as invalid)
//   - 5+ digits: parse the first 4 as HHMM when valid, then one
//     "Unexpected character" per trailing digit.
func parseBareDigitsFallbackInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	s := strings.TrimSpace(str)
	if len(s) == 0 || !isAllDigits(s) {
		return false
	}
	// 4-digit: handled by tryParseYearOnly (year / HHMM).
	if len(s) == 4 {
		return false
	}
	if len(s) >= 5 {
		hh, _ := strconv.Atoi(s[:2])
		mm, _ := strconv.Atoi(s[2:4])
		if hh <= 24 && mm <= 59 {
			pd.SetTime(hh, mm, 0)
			pd.fractionDefaultsZero = false
			for i := 4; i < len(s); i++ {
				pd.AddError(i, "Unexpected character")
			}
			return true
		}
	}
	for i := range s {
		pd.AddError(i, "Unexpected character")
	}
	return true
}

// parseCompactDateWithTimeInto matches "YYYYMMDD HH:MM[:SS]" — the 8-digit
// compact date followed by a colon-style time.
func parseCompactDateWithTimeInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	fields := strings.Fields(str)
	if len(fields) != 2 {
		return false
	}
	if len(fields[0]) != 8 || !isAllDigits(fields[0]) {
		return false
	}
	if !strings.Contains(fields[1], ":") {
		return false
	}
	year, _ := strconv.Atoi(fields[0][:4])
	month, _ := strconv.Atoi(fields[0][4:6])
	day, _ := strconv.Atoi(fields[0][6:8])
	if !IsValidDate(year, month, day) {
		return false
	}
	h, m, s, _, ok := parseFlexTime(fields[1])
	if !ok {
		return false
	}
	pd.SetDate(year, month, day)
	pd.SetTime(h, m, s)
	pd.setMaterialized(time.Date(year, time.Month(month), day, h, m, s, 0, loc))
	return true
}

// parseBareNumericOffsetInto matches a standalone numeric TZ like "+0900",
// "-12:00", "-00:05:00", or a minimal "+0" / "-0". PHP reports
// is_localtime=true, zone_type=1 with the offset, and no date or time
// components set.
func parseBareNumericOffsetInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	s := strings.TrimSpace(str)
	if len(s) < 2 || (s[0] != '+' && s[0] != '-') {
		return false
	}
	// Single-digit offset forms like "+0" / "-0" / "+5": PHP accepts them.
	if len(s) >= 2 && isAllDigits(s[1:]) && len(s) <= 3 {
		h, _ := strconv.Atoi(s[1:])
		if h <= 14 {
			sign := 1
			if s[0] == '-' {
				sign = -1
			}
			offset := sign * h * 3600
			pd.SetTZOffset(fixedZone(offset), offset)
			pd.setMaterialized(now.In(fixedZone(offset)))
			return true
		}
	}
	// Try an extended "±HH:MM:SS" form that PHP accepts.
	if offset, ok := parseHMSOffset(s); ok {
		pd.SetTZOffset(fixedZone(offset), offset)
		pd.setMaterialized(now.In(fixedZone(offset)))
		return true
	}
	tzLoc, consumed, ok := parseNumericTimezoneOffset(s)
	if !ok || consumed != len(s) {
		return false
	}
	pd.SetTZOffset(tzLoc, computeOffsetSeconds(s))
	pd.setMaterialized(now.In(tzLoc))
	return true
}

// parseHMSOffset parses "±HH:MM:SS" and returns the total offset in seconds.
func parseHMSOffset(s string) (int, bool) {
	if len(s) != 9 || (s[0] != '+' && s[0] != '-') || s[3] != ':' || s[6] != ':' {
		return 0, false
	}
	sign := 1
	if s[0] == '-' {
		sign = -1
	}
	if !isAllDigits(s[1:3]) || !isAllDigits(s[4:6]) || !isAllDigits(s[7:9]) {
		return 0, false
	}
	h, _ := strconv.Atoi(s[1:3])
	m, _ := strconv.Atoi(s[4:6])
	sec, _ := strconv.Atoi(s[7:9])
	return sign * (h*3600 + m*60 + sec), true
}

// parseTimeWithNumericOffsetInto matches "HH:MM[:SS][.frac](+HHMM|+HH:MM|-HHMM)"
// — optionally with a space separating the time and TZ. PHP emits hour/minute/
// second and a zone_type 1 numeric offset for this shape.
func parseTimeWithNumericOffsetInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	timePart, tzPart, ok := splitTimeAndNumericTZ(str)
	if !ok {
		return false
	}
	h, m, s, consumed, ok := parseFlexTime(timePart)
	if !ok {
		return false
	}
	hasFrac := false
	var frac float64
	if consumed < len(timePart) && timePart[consumed] == '.' {
		fs := timePart[consumed+1:]
		if f, err := strconv.ParseFloat("0."+fs, 64); err == nil {
			frac = f
			hasFrac = true
		}
	}

	pd.SetTime(h, m, s)
	if hasFrac {
		pd.SetFraction(frac)
	}
	pd.SetTZOffset(loc, computeOffsetSeconds(tzPart))
	pd.setMaterialized(time.Date(now.Year(), now.Month(), now.Day(), h, m, s, int(frac*1e9), loc))
	return true
}

// splitTimeAndNumericTZ splits "HH:MM[...]+HHMM" or "HH:MM[...] +HHMM" into
// the time portion and the numeric offset portion. Either the time is
// immediately followed by the sign (no whitespace) or the two are separated
// by a single space. The entire tail after the sign must be the numeric
// offset — otherwise we reject to avoid misparsing something like
// "17:00 2004-01-01" where "-01" is part of a date.
func splitTimeAndNumericTZ(str string) (timePart, tzPart string, ok bool) {
	s := strings.TrimSpace(str)
	if !strings.Contains(s, ":") {
		return "", "", false
	}
	// Try space-separated form first: exactly two fields with TZ on right.
	if idx := strings.IndexByte(s, ' '); idx > 0 {
		left := s[:idx]
		right := strings.TrimSpace(s[idx+1:])
		if len(right) >= 3 && (right[0] == '+' || right[0] == '-') {
			if loc, consumed, ok := parseNumericTimezoneOffset(right); ok && consumed == len(right) {
				_ = loc
				return left, right, true
			}
		}
		return "", "", false
	}
	// No space: find the first '+' or '-' that appears after the colon.
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return "", "", false
	}
	for i := colon + 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			candidate := s[i:]
			if _, consumed, ok := parseNumericTimezoneOffset(candidate); ok && consumed == len(candidate) {
				return s[:i], candidate, true
			}
			break
		}
	}
	return "", "", false
}

// --- Into adapters (each wraps a specific legacy parser) ---

func parseZeroDateInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	if !strings.HasPrefix(str, "0000-00-00") {
		return false
	}
	rest := strings.TrimSpace(str[len("0000-00-00"):])
	hasTime := false
	var hour, minute, second, nanos int
	if rest != "" {
		h, m, s, n, consumed, ok := parseISO8601Time(rest)
		if !ok || strings.TrimSpace(rest[consumed:]) != "" {
			return false
		}
		hour, minute, second, nanos = h, m, s, n
		hasTime = true
	}
	// PHP's date_parse reports the literal components (year=0, month=0,
	// day=0) plus a warning "The parsed date was invalid" at len(str)+1.
	// StrToTime still needs a materialized time: Go normalizes year/month/day
	// 0 to -0001-11-30, matching PHP's underlying behaviour.
	pd.SetDate(0, 0, 0)
	if hasTime {
		pd.SetTime(hour, minute, second)
		if nanos != 0 {
			pd.SetFraction(float64(nanos) / 1e9)
		}
	}
	pd.AddWarning(len(str)+1, "The parsed date was invalid")
	pd.setMaterialized(time.Date(0, 0, 0, hour, minute, second, nanos, loc))
	return true
}

func parseFrontBackOfInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseFrontBackOf(str, now, loc)
	if !ok {
		return false
	}
	pd.setMaterialized(t)
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	return true
}

func parseSignedYearInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseSignedYear(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	if t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0 {
		pd.SetTime(t.Hour(), t.Minute(), t.Second())
	}
	if t.Location() != loc {
		setTZFromLocation(pd, t.Location(), t)
	}
	pd.setMaterialized(t)
	return true
}

func parseISO8601Into(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	// Week date: YYYY-Www[-D] — PHP represents this as year-Jan-1 plus a
	// relative day offset derived from the ISO week calendar.
	if t, ok := parseISOWeekDate(str, loc); ok {
		year, week, dayOfWeek := extractWeekDateParts(str)
		if year > 0 {
			pd.SetDate(year, 1, 1)
			days := isoWeekDayOffset(year, week, dayOfWeek)
			if days != 0 {
				pd.AddRelative(UnitDay, days)
			}
			pd.setMaterialized(t)
			pd.relativeApplied = true
			return true
		}
		pd.SetDate(t.Year(), int(t.Month()), t.Day())
		pd.setMaterialized(t)
		return true
	}
	// Date-time with T separator
	if t, ok := parseISO8601DateTime(str, loc); ok {
		// Populate components via the decomposing variant, but also
		// remember the full materialized time to preserve legacy semantics
		// (24:00 rollover, DST gap handling).
		parseISO8601DateTimeInto(str, loc, pd)
		pd.setMaterialized(t)
		return true
	}
	return false
}

// parseISO8601DateTimeInto reparses an ISO 8601 date+time string, populating
// pd with exactly the components that were present in the input.
func parseISO8601DateTimeInto(str string, loc *time.Location, pd *ParsedDate) bool {
	tIdx := -1
	for i := 1; i < len(str)-1; i++ {
		if str[i] == 't' && str[i-1] >= '0' && str[i-1] <= '9' && str[i+1] >= '0' && str[i+1] <= '9' {
			tIdx = i
			break
		}
	}
	if tIdx < 0 {
		return false
	}

	datePart := str[:tIdx]
	rest := str[tIdx+1:]

	var year, month, day int
	if strings.Contains(datePart, "-") {
		t, ok := parseISOFormat(datePart, loc)
		if !ok {
			return false
		}
		year = t.Year()
		month = int(t.Month())
		day = t.Day()
	} else if len(datePart) >= 8 && isAllDigits(datePart) {
		year, _ = strconv.Atoi(datePart[:len(datePart)-4])
		month, _ = strconv.Atoi(datePart[len(datePart)-4 : len(datePart)-2])
		day, _ = strconv.Atoi(datePart[len(datePart)-2:])
		if !IsValidDate(year, month, day) {
			return false
		}
	} else {
		return false
	}

	hour, minute, second, nanos, timeConsumed, ok := parseISO8601Time(rest)
	if !ok {
		return false
	}

	// Delegate the heavy lifting (validation, 24:00 handling) to the legacy
	// parser, but also populate pd. Rather than reimplement everything, call
	// legacy then overwrite:
	t, ok := parseISO8601DateTime(str, loc)
	if !ok {
		return false
	}

	pd.SetDate(year, month, day)
	pd.SetTime(hour, minute, second)
	if nanos > 0 {
		pd.SetFraction(float64(nanos) / 1e9)
	}

	// Detect and record timezone metadata if the input had one.
	tzRest := strings.TrimLeft(rest[timeConsumed:], " ")
	if len(tzRest) > 0 {
		recordISO8601TZ(tzRest, pd, t)
	}

	return true
}

// recordISO8601TZ parses a trailing timezone suffix from an ISO 8601 input
// and records its metadata in pd.
func recordISO8601TZ(tzStr string, pd *ParsedDate, t time.Time) {
	// PHP treats "Z" as an abbreviation (zone_type 2), not a UTC offset.
	if strings.EqualFold(tzStr, "Z") {
		pd.SetTZAbbreviation(time.UTC, "Z", 0, false)
		return
	}
	// Numeric offset (+HH:MM, +HHMM, +HH)
	if _, consumed, ok := parseNumericTimezoneOffset(tzStr); ok {
		remaining := strings.TrimSpace(tzStr[consumed:])
		if remaining != "" {
			return
		}
		offset := computeOffsetSeconds(tzStr)
		pd.SetTZOffset(t.Location(), offset)
		return
	}
	// Named (abbreviation or IANA)
	if loc, found := tryParseTimezone(tzStr); found {
		setTZFromLocation(pd, loc, t)
	}
}

// computeOffsetSeconds parses "+HH[:MM]" / "-HHMM" / "+H:MM" / "Z" into an
// offset in seconds.
func computeOffsetSeconds(s string) int {
	s = strings.TrimSpace(s)
	if s == "z" || s == "Z" {
		return 0
	}
	if len(s) < 2 {
		return 0
	}
	sign := 1
	switch s[0] {
	case '+':
		sign = 1
	case '-':
		sign = -1
	default:
		return 0
	}
	body := s[1:]
	// Extended ±HH:MM:SS
	if len(body) == 8 && body[2] == ':' && body[5] == ':' {
		h, _ := strconv.Atoi(body[:2])
		m, _ := strconv.Atoi(body[3:5])
		sec, _ := strconv.Atoi(body[6:])
		return sign * (h*3600 + m*60 + sec)
	}
	// ±H:MM or ±HH:MM
	if idx := strings.Index(body, ":"); idx == 1 || idx == 2 {
		h, _ := strconv.Atoi(body[:idx])
		m, _ := strconv.Atoi(body[idx+1:])
		return sign * (h*3600 + m*60)
	}
	// Compact ±HHMM or ±HH
	if len(body) >= 4 && isAllDigits(body[:4]) {
		h, _ := strconv.Atoi(body[:2])
		m, _ := strconv.Atoi(body[2:4])
		return sign * (h*3600 + m*60)
	}
	if len(body) >= 2 && isAllDigits(body[:2]) {
		h, _ := strconv.Atoi(body[:2])
		return sign * h * 3600
	}
	if len(body) >= 1 && body[0] >= '0' && body[0] <= '9' {
		h := int(body[0] - '0')
		return sign * h * 3600
	}
	return 0
}

func parseDateTimeFormatInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseDateTimeFormat(str, loc)
	if !ok {
		return false
	}
	// Recover the literal hour/minute/second/date from the source string so
	// hour==24 is preserved verbatim (PHP's date_parse emits hour=24 plus a
	// warning; time.Date has already wrapped it into the next day).
	litYear, litMonth, litDay := t.Year(), int(t.Month()), t.Day()
	litHour, litMinute, litSecond := t.Hour(), t.Minute(), t.Second()
	hour24 := false
	if spaceIdx := strings.IndexByte(str, ' '); spaceIdx > 0 {
		rest := strings.TrimSpace(str[spaceIdx+1:])
		restLower := strings.ToLower(rest)
		if strings.HasSuffix(restLower, "a.m.") || strings.HasSuffix(restLower, "p.m.") {
			rest = strings.TrimSpace(rest[:len(rest)-4])
		} else if strings.HasSuffix(restLower, "am") || strings.HasSuffix(restLower, "pm") {
			rest = strings.TrimSpace(rest[:len(rest)-2])
		} else if upperRest := strings.ToUpper(rest); strings.HasSuffix(upperRest, " AM") || strings.HasSuffix(upperRest, " PM") {
			rest = strings.TrimSpace(rest[:len(rest)-3])
		}
		if h, m, s, _, _, ok := parseISO8601Time(rest); ok && h == 24 {
			litHour, litMinute, litSecond = h, m, s
			hour24 = true
			// Date part is the segment before the space — it wasn't wrapped.
			datePart := str[:spaceIdx]
			if dt, dok := parseISOFormat(datePart, loc); dok {
				litYear, litMonth, litDay = dt.Year(), int(dt.Month()), dt.Day()
			} else if dt, dok := parseMonthNameFormat(datePart, loc); dok {
				litYear, litMonth, litDay = dt.Year(), int(dt.Month()), dt.Day()
			}
		}
	}
	pd.SetDate(litYear, litMonth, litDay)
	pd.SetTime(litHour, litMinute, litSecond)
	if hour24 {
		pd.AddWarning(len(str)+1, "The parsed time was invalid")
	}
	if t.Nanosecond() != 0 {
		pd.SetFraction(float64(t.Nanosecond()) / 1e9)
	}
	// Record an explicit trailing timezone even when t.Location() == loc
	// (e.g. both UTC). This recovers cases like "2023-01-01 00:00 Z".
	if tzStr, ok := lastFieldLooksLikeTZ(str); ok {
		if tzStr[0] == '+' || tzStr[0] == '-' {
			pd.SetTZOffset(t.Location(), computeOffsetSeconds(tzStr))
		} else if resolved, found := tryParseTimezone(tzStr); found {
			setTZFromName(pd, tzStr, resolved)
		}
	} else if t.Location() != loc {
		setTZFromLocation(pd, t.Location(), t)
	}
	pd.setMaterialized(t)
	return true
}

func parseWithTimezoneInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseWithTimezone(str, loc)
	if !ok {
		return false
	}
	// Time-only + TZ ("14:30 PST", "10:00 UTC"): record only the time and
	// the TZ, not the date (PHP reports year/month/day as false).
	if isTimeOnlyWithTimezoneInput(str) {
		pd.SetTime(t.Hour(), t.Minute(), t.Second())
		fields := strings.Fields(str)
		if len(fields) > 0 {
			tzStr := fields[len(fields)-1]
			if resolved, found := tryParseTimezone(tzStr); found {
				setTZFromName(pd, tzStr, resolved)
			} else {
				setTZFromLocation(pd, t.Location(), t)
			}
		}
		pd.setMaterialized(t)
		return true
	}

	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	if strings.Contains(str, ":") {
		pd.SetTime(t.Hour(), t.Minute(), t.Second())
	} else if t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0 {
		pd.SetTime(t.Hour(), t.Minute(), t.Second())
	}
	if tzStr, ok := lastFieldLooksLikeTZ(str); ok {
		if tzStr[0] == '+' || tzStr[0] == '-' {
			pd.SetTZOffset(t.Location(), computeOffsetSeconds(tzStr))
		} else if resolved, found := tryParseTimezone(tzStr); found {
			setTZFromName(pd, tzStr, resolved)
		}
	} else if t.Location() != loc {
		setTZFromLocation(pd, t.Location(), t)
	}
	pd.setMaterialized(t)
	return true
}

// isTimeOnlyWithTimezoneInput reports whether str looks like "HH:MM[:SS][.frac] <tz>".
func isTimeOnlyWithTimezoneInput(str string) bool {
	fields := strings.Fields(str)
	if len(fields) != 2 {
		return false
	}
	t := fields[0]
	return strings.Contains(t, ":") && !strings.Contains(t, "-") && !strings.Contains(t, "/")
}

func parseLargeYearAsTimeInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseLargeYearAsTime(str, now, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	if t.Location() != loc {
		setTZFromLocation(pd, t.Location(), t)
	}
	pd.setMaterialized(t)
	return true
}

func parseYearMonthFormatInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	if len(str) == 0 || str[0] < '0' || str[0] > '9' {
		return false
	}
	// Detect ISO ordinal date form "YYYY-DDD" so we can report PHP's shape:
	// year/month=1/day=DoY (not the resolved month/day) plus a warning when
	// DoY exceeds 365 / 366.
	parts := strings.SplitN(str, "-", 2)
	if len(parts) == 2 && len(parts[1]) == 3 && isAllDigits(parts[0]) && isAllDigits(parts[1]) {
		year, _ := strconv.Atoi(parts[0])
		doy, _ := strconv.Atoi(parts[1])
		if doy >= 1 {
			pd.SetDate(year, 1, doy)
			// PHP treats the 3-digit form as (year, month=1, day=DDD) and
			// emits a warning when the day value exceeds 31 (invalid day
			// for January).
			if doy > 31 {
				pd.AddWarning(len(str)+1, "The parsed date was invalid")
			}
			pd.setMaterialized(time.Date(year, 1, 1, 0, 0, 0, 0, loc).AddDate(0, 0, doy-1))
			return true
		}
	}
	t, ok := parseYearMonthFormat(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.setMaterialized(t)
	return true
}

func parseUSDateWithTimeInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseUSDateWithTime(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	pd.setMaterialized(t)
	return true
}

func parseShortYearUSDateWithMilitaryTimeInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseShortYearUSDateWithMilitaryTime(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	pd.setMaterialized(t)
	return true
}

func parseCompactTimestampInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseCompactTimestamp(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	digits := strings.SplitN(str, " ", 2)[0]
	if len(digits) == 14 {
		pd.SetTime(t.Hour(), t.Minute(), t.Second())
	}
	if t.Location() != loc {
		setTZFromLocation(pd, t.Location(), t)
	}
	pd.setMaterialized(t)
	return true
}

func parseCompactTimeFormatsInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseCompactTimeFormats(str, now, loc)
	if !ok {
		return false
	}
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	if t.Nanosecond() != 0 {
		pd.SetFraction(float64(t.Nanosecond()) / 1e9)
	}
	if len(str) == 7 && isAllDigits(str) {
		pd.SetDate(t.Year(), int(t.Month()), t.Day())
	}
	if t.Location() != loc {
		setTZFromLocation(pd, t.Location(), t)
	}
	pd.setMaterialized(t)
	return true
}

func parseMonthNameFormatInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	// Handle 2-part Jan-15 / 15-Jan forms that parseMonthNameFormat rejects.
	if m, d, ok := parseTwoPartMonthDay(str); ok {
		pd.SetMonth(m)
		pd.SetDay(d)
		pd.setMaterialized(time.Date(now.Year(), time.Month(m), d, 0, 0, 0, 0, loc))
		return true
	}
	t, ok := parseMonthNameFormat(str, loc)
	if !ok {
		return false
	}
	// parseMonthNameFormat's recognised 3-part shapes always carry a year.
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.setMaterialized(t)
	return true
}

// parseTwoPartMonthDay matches "Mon-DD" or "DD-Mon" (e.g. Jan-15 / 15-Jan).
func parseTwoPartMonthDay(str string) (month, day int, ok bool) {
	parts := strings.Split(str, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	if m, isMonth := getMonthByNameFlex(parts[0]); isMonth {
		if d, err := strconv.Atoi(parts[1]); err == nil && d >= 1 && d <= 31 {
			return int(m), d, true
		}
	}
	if m, isMonth := getMonthByNameFlex(parts[1]); isMonth {
		if d, err := strconv.Atoi(parts[0]); err == nil && d >= 1 && d <= 31 {
			return int(m), d, true
		}
	}
	return 0, 0, false
}

// hasFourDigitYear reports whether str contains a 4-digit number that could
// serve as an explicit year.
func hasFourDigitYear(str string) bool {
	run := 0
	for i := 0; i <= len(str); i++ {
		if i < len(str) && str[i] >= '0' && str[i] <= '9' {
			run++
			continue
		}
		if run == 4 {
			return true
		}
		run = 0
	}
	return false
}

func parseHTTPLogFormatInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseHTTPLogFormat(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	if t.Location() != loc {
		setTZFromLocation(pd, t.Location(), t)
	}
	pd.setMaterialized(t)
	return true
}

func parseDateTimeTZRelativeInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	// Detect the relative-expression suffix(es) and extract them into the pd
	// Relative block, then parse the remaining date+tz portion.
	type relExpr struct {
		amount int
		unit   string
	}
	var rels []relExpr
	remaining := str

	for {
		remaining = strings.TrimRight(remaining, " ")
		if len(remaining) == 0 {
			break
		}
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
		return false
	}

	datePart := remaining
	sub := newParsedDate()
	ok := parseISO8601Into(datePart, now, loc, opts, sub) ||
		parseDateTimeFormatInto(datePart, now, loc, opts, sub) ||
		parseWithTimezoneInto(datePart, now, loc, opts, sub) ||
		parseDateWithTZInto(datePart, now, loc, opts, sub) ||
		wrapDateOnly(parseISOFormat)(datePart, now, loc, opts, sub) ||
		parseDayMonthYearInto(datePart, now, loc, opts, sub)
	if !ok {
		return false
	}

	copyComponents(pd, sub)
	if sub.hasMaterialized {
		pd.setMaterialized(sub.materialized)
	}
	for i := len(rels) - 1; i >= 0; i-- {
		pd.AddRelative(rels[i].unit, rels[i].amount)
	}
	return true
}

func parseDateWithTZInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	fields := strings.Fields(str)
	if len(fields) != 2 {
		return false
	}
	t, ok := parseISOFormat(fields[0], loc)
	if !ok {
		return false
	}
	tzLoc, found := tryParseTimezone(fields[1])
	if !found {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	setTZFromName(pd, fields[1], tzLoc)
	// Rebuild with target tz since parseISOFormat used the original loc.
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tzLoc)
	pd.setMaterialized(t)
	return true
}

func parseDayMonthYearInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	// Peek weekday prefix so we can record it as relative.weekday.
	_, dayNum, stripped := stripWeekdayPrefix(str)

	t, ok := parseDayMonthYear(str, now, loc)
	if !ok {
		return false
	}
	// Recover the ORIGINAL parsed date (without the weekday-mismatch
	// advance that parseDayMonthYear applies for StrToTime). PHP reports
	// the unadvanced date plus relative.weekday.
	origYear, origMonth, origDay := t.Year(), int(t.Month()), t.Day()
	if stripped {
		if y, m, d, found := extractDMYFromFields(str); found {
			origYear, origMonth, origDay = y, m, d
		}
	}
	pd.SetDate(origYear, origMonth, origDay)
	// Time is recorded whenever a time portion was present. Detect via ":"
	// in the input (post-weekday-prefix, after stripping the date fields).
	if strings.Contains(str, ":") {
		pd.SetTime(t.Hour(), t.Minute(), t.Second())
		if t.Nanosecond() != 0 {
			pd.SetFraction(float64(t.Nanosecond()) / 1e9)
		}
	}
	// Detect an explicit trailing timezone in the input regardless of whether
	// t.Location() == loc — both may happen to be UTC.
	if tzStr, ok := lastFieldLooksLikeTZ(str); ok {
		if len(tzStr) > 0 && (tzStr[0] == '+' || tzStr[0] == '-') {
			offset := computeOffsetSeconds(tzStr)
			pd.SetTZOffset(t.Location(), offset)
		} else if resolved, found := tryParseTimezone(tzStr); found {
			setTZFromName(pd, tzStr, resolved)
		}
	}
	if stripped && dayNum >= 0 {
		pd.SetRelativeWeekday(dayNum)
		// PHP records hour/minute/second=0 when a weekday prefix is present.
		if !pd.Hour.Set {
			pd.SetTime(0, 0, 0)
			pd.SetFraction(0)
		}
	}
	pd.setMaterialized(t)
	return true
}

// extractDMYFromFields pulls a day/month/year triple from a "weekday[,] <day> <Mon> <year> …"
// or hyphenated "<day>-<Mon>-<year>" input. Returns false if the layout
// doesn't match what parseDayMonthYear would have recognised.
func extractDMYFromFields(str string) (year, month, day int, ok bool) {
	rest, _, stripped := stripWeekdayPrefix(str)
	if !stripped {
		rest = str
	}
	fields := strings.Fields(rest)
	if len(fields) < 3 {
		return 0, 0, 0, false
	}
	// Hyphenated date in first field: "24-Jan-2019".
	if strings.Contains(fields[0], "-") {
		parts := strings.SplitN(fields[0], "-", 3)
		if len(parts) == 3 {
			fields = append(parts, fields[1:]...)
		}
	}
	dayStr := stripOrdinalSuffix(fields[0])
	d, err := strconv.Atoi(dayStr)
	if err != nil || d < 1 || d > 31 {
		return 0, 0, 0, false
	}
	m, isMonth := getMonthByNameFlex(fields[1])
	if !isMonth {
		return 0, 0, 0, false
	}
	y, err := strconv.Atoi(fields[2])
	if err != nil || y < 0 {
		return 0, 0, 0, false
	}
	if y < 100 {
		y = parseTwoDigitYear(y)
	}
	return y, int(m), d, true
}

// lastFieldLooksLikeTZ returns the last whitespace-separated field if it
// looks like a timezone token (numeric offset, IANA name with "/", or a
// resolvable abbreviation).
func lastFieldLooksLikeTZ(str string) (string, bool) {
	fields := strings.Fields(str)
	if len(fields) == 0 {
		return "", false
	}
	last := fields[len(fields)-1]
	if len(last) == 0 {
		return "", false
	}
	if last[0] == '+' || last[0] == '-' {
		if _, _, ok := parseNumericTimezoneOffset(last); ok {
			return last, true
		}
	}
	if _, ok := tryParseTimezone(last); ok {
		return last, true
	}
	return "", false
}

func parseMonthYearOnlyInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseMonthYearOnly(str, loc)
	if !ok {
		return false
	}
	// PHP date_parse fills in day=1 for "Month Year" inputs.
	pd.SetDate(t.Year(), int(t.Month()), 1)
	pd.setMaterialized(t)
	return true
}

func parseTimeBeforeDateInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseTimeBeforeDate(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	pd.setMaterialized(t)
	return true
}

func parseMonthDayTimeYearInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseMonthDayTimeYear(str, loc)
	if !ok {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	pd.setMaterialized(t)
	return true
}

func parseFirstLastDayOfDateInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseFirstLastDayOfDate(str, now, loc)
	if !ok {
		return false
	}
	// PHP reports day=1 for "first/last day of <Month> <Year>" and records
	// the first_day_of_month / last_day_of_month flag on the relative block.
	if ordinal, ok := detectFirstLastDayOfMonthYear(str); ok {
		pd.SetDate(t.Year(), int(t.Month()), 1)
		if ordinal == 1 {
			pd.SetFirstLastDayOf(1)
		} else {
			pd.SetFirstLastDayOf(2)
		}
		pd.setMaterialized(t)
		pd.relativeApplied = true
		return true
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.setMaterialized(t) // preserves base-time hour/minute/second for +/- forms
	return true
}

func parseNumberedWeekdayInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	t, ok := parseNumberedWeekday(str, now, loc)
	if !ok {
		return false
	}
	// "first day of next/last/this month/year" is purely relative; report it
	// as such rather than as an absolute date derived from a zero base time.
	if ordinal, unit, direction, isRelFirstLast := detectRelativeFirstLastDay(str); isRelFirstLast {
		if ordinal == 1 {
			pd.SetFirstLastDayOf(1)
		} else {
			pd.SetFirstLastDayOf(2)
		}
		var amount int
		switch direction {
		case DirectionNext:
			amount = 1
		case DirectionLast:
			amount = -1
		}
		if amount != 0 {
			pd.AddRelative(unit, amount)
		}
		pd.setMaterialized(t)
		pd.relativeApplied = true
		return true
	}
	// "first/last day of <Month> [<Year>]" → PHP reports year/month with
	// day=1 and flags the relative first/last_day_of_month, regardless of
	// whether it's actually the last day.
	if ordinal, ok := detectFirstLastDayOfMonthYear(str); ok {
		if ordinal == 1 {
			pd.SetFirstLastDayOf(1)
		} else {
			pd.SetFirstLastDayOf(2)
		}
		pd.SetDate(t.Year(), int(t.Month()), 1)
		pd.setMaterialized(t)
		pd.relativeApplied = true
		return true
	}
	// "<ordinal> <weekday> of <Month> <Year>" → PHP reports year/month/day=1
	// with hour/min/sec/fraction=0 and a relative block carrying the
	// weekday and day-offset (N-1)*7 or -7.
	if info, ok := detectOrdinalWeekdayOfMonthYear(str); ok {
		pd.SetDate(info.year, info.month, 1)
		pd.SetTime(0, 0, 0)
		pd.SetFraction(0)
		pd.SetRelativeWeekday(info.weekday)
		// Digit-ordinals like "1st", "2nd", "31st" trigger PHP's quirky
		// error pattern — word-ordinals ("first", "second") do not and
		// also contribute a (N-1)*7 day offset.
		if info.isDigit {
			warnPos := len(info.ordinalDigits) + len(info.ordinalSuffix) + len(info.weekdayWord) + 2
			pd.AddWarning(warnPos, "Double timezone specification")
			pos := 0
			for range info.ordinalDigits {
				pd.AddError(pos, "Unexpected character")
				pos++
			}
			pd.AddError(pos, "The timezone could not be found in the database")
			pd.IsLocaltime = true
			pd.ZoneType = 0
		} else if info.ordinal == -1 {
			pd.AddRelative(UnitDay, -7)
		} else if info.ordinal > 1 {
			pd.AddRelative(UnitDay, (info.ordinal-1)*7)
		}
		pd.setMaterialized(t)
		pd.relativeApplied = true
		return true
	}
	// "<ordinal> <weekday>" without month context → year/month/day=false,
	// hour/min/sec/fraction=0, relative.day = (N-1)*7, relative.weekday = W.
	if info, ok := detectOrdinalWeekdayOnly(str); ok {
		pd.SetTime(0, 0, 0)
		pd.SetFraction(0)
		pd.SetRelativeWeekday(info.weekday)
		if info.ordinal > 1 {
			pd.AddRelative(UnitDay, (info.ordinal-1)*7)
		}
		pd.setMaterialized(t)
		pd.relativeApplied = true
		return true
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.setMaterialized(t)
	return true
}

type ordinalWeekdayInfo struct {
	ordinal        int    // 1..5 or -1 for "last"
	weekday        int    // 0=Sun..6=Sat (PHP convention)
	year           int
	month          int
	isDigit        bool   // digit-ordinal like "2nd"
	ordinalDigits  string // digit portion, e.g. "2"
	ordinalSuffix  string // suffix portion, e.g. "nd"
	weekdayWord    string // weekday token as parsed
}

// detectOrdinalWeekdayOfMonthYear matches "<ordinal> <weekday> of <Month> <Year>".
func detectOrdinalWeekdayOfMonthYear(str string) (info ordinalWeekdayInfo, ok bool) {
	fields := strings.Fields(str)
	if len(fields) != 5 {
		return info, false
	}
	ord, _, _, parsed := parseOrdinalPrefix(fields, 0)
	if !parsed {
		return info, false
	}
	info.ordinal = ord
	// Was the ordinal spelled with digits + suffix?
	first := fields[0]
	for _, sfx := range []string{"st", "nd", "rd", "th"} {
		if strings.HasSuffix(strings.ToLower(first), sfx) {
			digits := first[:len(first)-len(sfx)]
			if len(digits) > 0 && isAllDigits(digits) {
				info.isDigit = true
				info.ordinalDigits = digits
				info.ordinalSuffix = sfx
			}
			break
		}
	}
	dow := getDayOfWeek(fields[1])
	if dow < 0 {
		return info, false
	}
	info.weekday = dow
	info.weekdayWord = fields[1]
	if strings.ToLower(fields[2]) != "of" {
		return info, false
	}
	month, isMonth := getMonthByNameFlex(fields[3])
	if !isMonth {
		return info, false
	}
	info.month = int(month)
	y, err := strconv.Atoi(fields[4])
	if err != nil {
		return info, false
	}
	info.year = y
	return info, true
}

// detectOrdinalWeekdayOnly matches a bare "<ordinal> <weekday>" with no
// month context (e.g. "third thursday").
func detectOrdinalWeekdayOnly(str string) (info ordinalWeekdayInfo, ok bool) {
	fields := strings.Fields(str)
	if len(fields) != 2 {
		return info, false
	}
	ord, _, _, parsed := parseOrdinalPrefix(fields, 0)
	if !parsed {
		return info, false
	}
	info.ordinal = ord
	dow := getDayOfWeek(fields[1])
	if dow < 0 {
		return info, false
	}
	info.weekday = dow
	return info, true
}

// isoWeekDayOffset returns the number of days from Jan 1 of `year` to the
// specified ISO week-day. The offset may be negative (e.g. 2020-W01-1 falls
// on 2019-12-30) or > 365 (W53 days in a short year).
func isoWeekDayOffset(year, week, dayOfWeek int) int {
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	dow4 := int(jan4.Weekday())
	if dow4 == 0 {
		dow4 = 7
	}
	w1Mon := 4 - (dow4 - 1) // day-of-year of week 1's Monday (may be negative)
	doy := w1Mon + (week-1)*7 + (dayOfWeek - 1)
	return doy - 1
}

// extractWeekDateParts parses "YYYY-Www[-D]" or "YYYYWww[D]" and returns
// (year, week, dayOfWeek). Returns year=0 when the input isn't a week date.
func extractWeekDateParts(str string) (year, week, dayOfWeek int) {
	dayOfWeek = 1
	wIdx := -1
	for i := 1; i < len(str); i++ {
		if str[i] == 'w' && str[i-1] >= '0' && str[i-1] <= '9' {
			wIdx = i
			break
		}
		if str[i] == 'w' && str[i-1] == '-' && i >= 2 && str[i-2] >= '0' && str[i-2] <= '9' {
			wIdx = i
			break
		}
	}
	if wIdx < 0 {
		return 0, 0, 0
	}
	yearPart := strings.TrimSuffix(str[:wIdx], "-")
	if !isAllDigits(yearPart) || len(yearPart) == 0 {
		return 0, 0, 0
	}
	y, err := strconv.Atoi(yearPart)
	if err != nil {
		return 0, 0, 0
	}
	year = y
	rest := str[wIdx+1:]
	if len(rest) < 2 || !isAllDigits(rest[:2]) {
		return 0, 0, 0
	}
	w, _ := strconv.Atoi(rest[:2])
	week = w
	rest = rest[2:]
	if len(rest) > 0 && rest[0] == '-' {
		rest = rest[1:]
	}
	if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		dow, _ := strconv.Atoi(rest[:1])
		if dow >= 1 && dow <= 7 {
			dayOfWeek = dow
		}
	}
	return year, week, dayOfWeek
}

// detectFirstLastDayOfMonthYear matches "first/last day of <MonthName> [<Year>]".
func detectFirstLastDayOfMonthYear(str string) (ordinal int, ok bool) {
	fields := strings.Fields(str)
	if len(fields) < 4 || len(fields) > 5 {
		return 0, false
	}
	switch strings.ToLower(fields[0]) {
	case "first":
		ordinal = 1
	case "last":
		ordinal = -1
	default:
		return 0, false
	}
	if strings.ToLower(fields[1]) != "day" || strings.ToLower(fields[2]) != "of" {
		return 0, false
	}
	if _, isMonth := getMonthByNameFlex(fields[3]); !isMonth {
		return 0, false
	}
	return ordinal, true
}

// detectRelativeFirstLastDay recognises "first/last day of next/last/this
// month/year" inputs and returns the ordinal (1 or -1), unit (month/year),
// direction, and true. Otherwise returns false.
func detectRelativeFirstLastDay(str string) (ordinal int, unit string, direction string, ok bool) {
	fields := strings.Fields(str)
	if len(fields) != 5 {
		return 0, "", "", false
	}
	switch strings.ToLower(fields[0]) {
	case "first":
		ordinal = 1
	case "last":
		ordinal = -1
	default:
		return 0, "", "", false
	}
	if strings.ToLower(fields[1]) != "day" || strings.ToLower(fields[2]) != "of" {
		return 0, "", "", false
	}
	direction = strings.ToLower(fields[3])
	if direction != DirectionNext && direction != DirectionLast && direction != "this" {
		return 0, "", "", false
	}
	unit = normalizeTimeUnit(fields[4])
	if unit != UnitMonth && unit != UnitYear {
		return 0, "", "", false
	}
	return ordinal, unit, direction, true
}

// --- post-pipeline Into adapters ---

func parseUnixTimestampInto(str string, loc *time.Location, pd *ParsedDate) bool {
	t, ok := tryParseUnixTimestamp(str, loc)
	if !ok {
		return false
	}
	// PHP reports @N as an absolute 1970-01-01 00:00:00 UTC plus a relative
	// offset of N seconds. Fractional seconds are parsed but discarded.
	seconds := int64(0)
	hasExtraTZ := false
	tzPos := 0
	if str[0] == '@' {
		body := str[1:]
		// Detect a trailing " <tz>" — PHP emits "Double timezone
		// specification" when present.
		if space := strings.Index(body, " "); space >= 0 {
			tzPos = len(str) - len(body[space:]) + 1 // 1 past the start of the TZ
			trailer := strings.TrimSpace(body[space:])
			if trailer != "" {
				hasExtraTZ = true
			}
			body = body[:space]
		}
		if idx := strings.Index(body, "."); idx >= 0 {
			body = body[:idx]
		}
		seconds, _ = strconv.ParseInt(body, 10, 64)
	}
	pd.SetDate(1970, 1, 1)
	pd.SetTime(0, 0, 0)
	pd.SetFraction(0)
	pd.IsLocaltime = true
	pd.ZoneType = 1
	pd.Zone = 0
	pd.IsDST = false
	pd.sourceLoc = time.UTC
	pd.relative().Second = int(seconds)
	if hasExtraTZ {
		pd.AddWarning(tzPos, "Double timezone specification")
	}
	pd.setMaterialized(t)
	pd.relativeApplied = true
	return true
}

func parseKeywordInto(str string, now time.Time, loc *time.Location, pd *ParsedDate) bool {
	switch str {
	case "now":
		// PHP leaves every component false for "now".
		pd.setMaterialized(now)
		return true
	case "today":
		pd.SetTime(0, 0, 0)
		pd.SetFraction(0)
		y, m, d := now.Date()
		pd.setMaterialized(time.Date(y, m, d, 0, 0, 0, 0, loc))
		return true
	case "midnight":
		pd.SetTime(0, 0, 0)
		pd.SetFraction(0)
		y, m, d := now.Date()
		pd.setMaterialized(time.Date(y, m, d, 0, 0, 0, 0, loc))
		return true
	case "noon":
		pd.SetTime(12, 0, 0)
		pd.SetFraction(0)
		y, m, d := now.Date()
		pd.setMaterialized(time.Date(y, m, d, 12, 0, 0, 0, loc))
		return true
	case "tomorrow":
		pd.SetTime(0, 0, 0)
		pd.SetFraction(0)
		pd.AddRelative(UnitDay, 1)
		t := now.AddDate(0, 0, 1)
		y, m, d := t.Date()
		pd.setMaterialized(time.Date(y, m, d, 0, 0, 0, 0, loc))
		pd.relativeApplied = true
		return true
	case "yesterday":
		pd.SetTime(0, 0, 0)
		pd.SetFraction(0)
		pd.AddRelative(UnitDay, -1)
		t := now.AddDate(0, 0, -1)
		y, m, d := t.Date()
		pd.setMaterialized(time.Date(y, m, d, 0, 0, 0, 0, loc))
		pd.relativeApplied = true
		return true
	}
	return false
}

func parseDateWithRelativeTimeInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	datePart, rest, ok := splitDateAndRest(str)
	if !ok {
		return false
	}
	// Parse date portion into a fresh pd so we capture the un-advanced
	// year/month/day that PHP reports.
	dateSub := newParsedDate()
	dateOpts := append([]Option(nil), opts...)
	dateOpts = append(dateOpts, InTZ(loc))
	if !dispatchStrToTime(datePart, now, loc, dateOpts, dateSub) || dateSub.ErrorCount > 0 {
		return false
	}
	dateResult, err := dateSub.Materialize(now, loc)
	if err != nil {
		return false
	}
	// Parse the remainder anchored at the parsed date so any resulting
	// absolute time we observe is relative to the original date.
	restSub := newParsedDate()
	restOpts := append([]Option(nil), opts...)
	restOpts = append(restOpts, Rel(dateResult), InTZ(loc))
	if !dispatchStrToTime(rest, dateResult, loc, restOpts, restSub) || restSub.ErrorCount > 0 {
		return false
	}

	copyComponents(pd, dateSub)
	if restSub.Hour.Set {
		pd.SetTime(restSub.Hour.V, restSub.Minute.V, restSub.Second.V)
	}
	if restSub.Fraction.Set {
		pd.SetFraction(restSub.Fraction.V)
	}
	if restSub.fractionDefaultsZero {
		pd.fractionDefaultsZero = true
	}
	if restSub.IsLocaltime && !pd.IsLocaltime {
		pd.IsLocaltime = restSub.IsLocaltime
		pd.ZoneType = restSub.ZoneType
		pd.Zone = restSub.Zone
		pd.IsDST = restSub.IsDST
		pd.TzAbbr = restSub.TzAbbr
		pd.TzID = restSub.TzID
		pd.sourceLoc = restSub.sourceLoc
	}
	if restSub.Relative != nil {
		if pd.Relative == nil {
			pd.Relative = restSub.Relative
		} else {
			pd.Relative.Year += restSub.Relative.Year
			pd.Relative.Month += restSub.Relative.Month
			pd.Relative.Day += restSub.Relative.Day
			pd.Relative.Hour += restSub.Relative.Hour
			pd.Relative.Minute += restSub.Relative.Minute
			pd.Relative.Second += restSub.Relative.Second
			if restSub.Relative.Weekday.Set {
				pd.Relative.Weekday = restSub.Relative.Weekday
			}
		}
	}
	// StrToTime still wants the final (relative-applied) materialized time.
	finalT, _ := restSub.Materialize(dateResult, loc)
	pd.setMaterialized(finalT)
	pd.relativeApplied = true
	return true
}

func tryWeekdayPrefixReparseInto(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	rest, dayNum, stripped := stripWeekdayPrefix(str)
	if !stripped {
		return false
	}
	restTrimmed := strings.TrimSpace(rest)
	switch {
	case strings.HasPrefix(restTrimmed, "next "),
		strings.HasPrefix(restTrimmed, "last "),
		strings.HasPrefix(restTrimmed, "this "):
		return false
	}
	switch restTrimmed {
	case "noon", "midnight", "tomorrow", "yesterday", "today", "now":
		return false
	}
	if restLooksLikeBareTime(restTrimmed) {
		return false
	}
	// Parse the rest into a fresh ParsedDate so we can observe exactly what
	// it contributed (year/month/day/hour/.../tz) and avoid PHP-incompatible
	// weekday advancement.
	sub := newParsedDate()
	reparseOpts := append([]Option(nil), opts...)
	reparseOpts = append(reparseOpts, InTZ(loc))
	if !dispatchStrToTime(restTrimmed, now, loc, reparseOpts, sub) {
		return false
	}
	if sub.ErrorCount > 0 {
		return false
	}
	copyComponents(pd, sub)
	if !sub.Hour.Set {
		pd.SetTime(0, 0, 0)
	}
	if dayNum >= 0 {
		pd.SetRelativeWeekday(dayNum)
	}
	// Materialize for StrToTime: advance to the next matching weekday when
	// the parsed absolute date doesn't already fall on that weekday.
	t, err := sub.Materialize(now, loc)
	if err == nil && dayNum >= 0 && int(t.Weekday()) != dayNum {
		daysUntil := (dayNum - int(t.Weekday()) + 7) % 7
		if daysUntil == 0 {
			daysUntil = 7
		}
		t = t.AddDate(0, 0, daysUntil)
	}
	pd.setMaterialized(t)
	return true
}

func parseCompoundExpressionInto(str string, now time.Time, opts []Option, pd *ParsedDate) bool {
	t, err := parseCompoundExpression(str, now, opts)
	if err != nil {
		return false
	}
	pd.SetDate(t.Year(), int(t.Month()), t.Day())
	pd.SetTime(t.Hour(), t.Minute(), t.Second())
	pd.setMaterialized(t)
	return true
}

func parseOrdinalDateInto(str string, now time.Time, loc *time.Location, pd *ParsedDate) bool {
	t, ok := parseOrdinalDate(str, now, loc)
	if !ok {
		return false
	}
	if hasFourDigitYear(str) {
		pd.SetDate(t.Year(), int(t.Month()), t.Day())
	} else {
		pd.SetMonth(int(t.Month()))
		pd.SetDay(t.Day())
	}
	pd.setMaterialized(t)
	return true
}

// --- helpers ---

// copyComponents copies populated fields from src into dst, preserving any
// fields already set on dst. Used by parsers that dispatch to a subparser.
func copyComponents(dst, src *ParsedDate) {
	if src.Year.Set && !dst.Year.Set {
		dst.Year = src.Year
	}
	if src.Month.Set && !dst.Month.Set {
		dst.Month = src.Month
	}
	if src.Day.Set && !dst.Day.Set {
		dst.Day = src.Day
	}
	if src.Hour.Set && !dst.Hour.Set {
		dst.Hour = src.Hour
	}
	if src.Minute.Set && !dst.Minute.Set {
		dst.Minute = src.Minute
	}
	if src.Second.Set && !dst.Second.Set {
		dst.Second = src.Second
	}
	if src.Fraction.Set && !dst.Fraction.Set {
		dst.Fraction = src.Fraction
	}
	if src.IsLocaltime && !dst.IsLocaltime {
		dst.IsLocaltime = src.IsLocaltime
		dst.ZoneType = src.ZoneType
		dst.Zone = src.Zone
		dst.IsDST = src.IsDST
		dst.TzAbbr = src.TzAbbr
		dst.TzID = src.TzID
		dst.sourceLoc = src.sourceLoc
	}
	for pos, msg := range src.Warnings {
		dst.AddWarning(pos, msg)
	}
	for pos, msg := range src.Errors {
		dst.AddError(pos, msg)
	}
	if src.fractionDefaultsZero {
		dst.fractionDefaultsZero = true
	}
}

// setTZFromLocation inspects a resolved *time.Location and populates pd's
// timezone metadata. t is the reference time used for DST detection.
func setTZFromLocation(pd *ParsedDate, loc *time.Location, t time.Time) {
	name, offset := t.In(loc).Zone()
	id := loc.String()

	// Numeric-offset locations built by fixedZone have names like "+09:00".
	if len(name) > 0 && (name[0] == '+' || name[0] == '-') {
		pd.SetTZOffset(loc, offset)
		return
	}

	// IANA identifiers contain a "/".
	if strings.Contains(id, "/") {
		pd.SetTZIdentifier(loc, id)
		return
	}

	applyAbbreviationTZ(pd, loc, name, offset)
}

// setTZFromName records timezone metadata given the original name string
// that matched (e.g. "Asia/Tokyo", "EST", "+09:00") and the resolved location.
func setTZFromName(pd *ParsedDate, name string, loc *time.Location) {
	if strings.Contains(name, "/") {
		pd.SetTZIdentifier(loc, canonicalTZName(name, loc))
		return
	}
	// Numeric offset?
	if len(name) > 0 && (name[0] == '+' || name[0] == '-') {
		offset := computeOffsetSeconds(name)
		pd.SetTZOffset(loc, offset)
		return
	}
	// PHP treats "Z" as an abbreviation.
	if strings.EqualFold(name, "Z") {
		pd.SetTZAbbreviation(time.UTC, "Z", 0, false)
		return
	}
	// Abbreviation (EST, GMT, PDT, etc.) — use the caller-supplied location
	// to read the current offset.
	now := time.Now()
	_, offset := now.In(loc).Zone()
	applyAbbreviationTZ(pd, loc, name, offset)
}

// applyAbbreviationTZ records the timezone metadata for an abbreviation like
// EST/GMT/PDT with PHP-specific quirks:
//   - UTC is zone_type 3 and includes both tz_abbr and tz_id.
//   - DST abbreviations (e.g. PDT, EDT) report the *standard* offset in zone
//     and set is_dst:true, matching PHP's timelib.
func applyAbbreviationTZ(pd *ParsedDate, loc *time.Location, abbr string, offset int) {
	upper := strings.ToUpper(abbr)

	// UTC: zone_type 3 with both tz_id and tz_abbr.
	if upper == "UTC" {
		pd.IsLocaltime = true
		pd.ZoneType = 3
		pd.Zone = 0
		pd.IsDST = false
		pd.TzAbbr = "UTC"
		pd.TzID = "UTC"
		pd.sourceLoc = loc
		return
	}

	// Check whether this abbreviation is a DST form. PHP emits the standard
	// offset for DST abbreviations and sets is_dst:true.
	if stdOffset, isDST, ok := dstAbbreviationOffset(upper); ok {
		pd.SetTZAbbreviation(loc, upper, stdOffset, isDST)
		return
	}

	pd.SetTZAbbreviation(loc, upper, offset, false)
}

// dstAbbreviationOffset returns the standard-time offset (in seconds) and
// is_dst flag for a PHP-recognised timezone abbreviation. Returns ok=false
// for unknown abbreviations so the caller falls back to the parsed offset.
func dstAbbreviationOffset(upper string) (offset int, isDST, ok bool) {
	switch upper {
	case "EDT":
		return -5 * 3600, true, true
	case "CDT":
		return -6 * 3600, true, true
	case "MDT":
		return -7 * 3600, true, true
	case "PDT":
		return -8 * 3600, true, true
	case "AKDT":
		return -9 * 3600, true, true
	case "BST":
		return 0, true, true
	case "CEST":
		return 1 * 3600, true, true
	case "EEST":
		return 2 * 3600, true, true
	case "AEDT":
		return 10 * 3600, true, true
	}
	return 0, false, false
}

// canonicalTZName returns the canonical IANA identifier for a matched name.
// The input may be lowercase ("asia/tokyo"); prefer loc.String() when it
// carries a proper ID.
func canonicalTZName(name string, loc *time.Location) string {
	s := loc.String()
	if strings.Contains(s, "/") {
		return s
	}
	// Fall back to title-casing the input.
	parts := strings.Split(name, "/")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "/")
}
