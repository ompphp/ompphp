package strtotime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// resolveOptions processes StrToTime options and returns the base time and location.
func resolveOptions(opts []Option) (time.Time, *time.Location) {
	var now time.Time
	loc := time.Local
	tzExplicit := false

	for _, opt := range opts {
		switch v := opt.(type) {
		case Rel:
			now = time.Time(v)
		case tzOption:
			if v.loc != nil {
				loc = v.loc
				tzExplicit = true
			}
		}
	}

	if !now.IsZero() && !tzExplicit {
		loc = now.Location()
	}

	if now.IsZero() {
		now = time.Now().In(loc)
	} else if now.Location() != loc {
		now = now.In(loc)
	}

	return now, loc
}

// tryParseUnixTimestamp handles "@timestamp" and "@timestamp.fraction [TZ]" format.
func tryParseUnixTimestamp(str string, loc *time.Location) (time.Time, bool) {
	if len(str) == 0 || str[0] != '@' {
		return time.Time{}, false
	}
	unixTimeStr := str[1:]
	tzParts := strings.SplitN(unixTimeStr, " ", 2)
	timestamp := tzParts[0]

	applyTZ := func(result time.Time) time.Time {
		if len(tzParts) > 1 && tzParts[1] != "" {
			if tzLoc, found := tryParseTimezone(tzParts[1]); found {
				return result.In(tzLoc)
			}
		}
		return result
	}

	if idx := strings.Index(timestamp, "."); idx != -1 {
		unixTime, err := strconv.ParseInt(timestamp[:idx], 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		fracStr := timestamp[idx+1:]
		// PHP rejects fractional seconds with more than 6 digits
		if len(fracStr) > 6 {
			return time.Time{}, false
		}
		fracPart, err := strconv.ParseFloat("0."+fracStr, 64)
		if err != nil {
			fracPart = 0.0
		}
		nanoSec := int64(fracPart * 1e9)
		return applyTZ(time.Unix(unixTime, nanoSec).In(loc)), true
	}

	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return applyTZ(time.Unix(unixTime, 0).In(loc)), true
}

// tryKeyword handles keyword time expressions: now, today, tomorrow, yesterday, midnight, noon.
func tryKeyword(str string, now time.Time, loc *time.Location) (time.Time, bool) {
	switch str {
	case "now":
		return now, true
	case "today", "midnight":
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, loc), true
	case "tomorrow":
		t := now.AddDate(0, 0, 1)
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, loc), true
	case "yesterday":
		t := now.AddDate(0, 0, -1)
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, loc), true
	case "noon":
		y, m, d := now.Date()
		return time.Date(y, m, d, 12, 0, 0, 0, loc), true
	}
	return time.Time{}, false
}

// tryWeekdayPrefixReparse strips a leading weekday name and reparses the rest.
// Handles: "Sun 2017-01-01", "Fri Aug 20 1993 23:59:59", etc.
func tryWeekdayPrefixReparse(str string, now time.Time, loc *time.Location, opts []Option) (time.Time, bool) {
	rest, prefixDayNum, stripped := stripWeekdayPrefix(str)
	if !stripped {
		return time.Time{}, false
	}

	// Don't strip if rest is "next/last/this week" — the token parser handles these
	restTrimmed := strings.TrimSpace(rest)
	if strings.HasPrefix(restTrimmed, "next ") || strings.HasPrefix(restTrimmed, "last ") || strings.HasPrefix(restTrimmed, "this ") {
		return time.Time{}, false
	}

	// Don't strip the weekday when the remaining text is just a time
	// keyword — PHP treats "Monday noon" as weekday + time offset, not
	// "Monday" anchoring an absolute date.
	switch restTrimmed {
	case "noon", "midnight", "tomorrow", "yesterday", "today", "now":
		return time.Time{}, false
	}
	// Similarly reject when the rest is a bare time expression without any
	// date component, so "Monday 10am" / "Tuesday 13:00" reach the token
	// parser which records a weekday + time rather than anchoring today.
	if restLooksLikeBareTime(restTrimmed) {
		return time.Time{}, false
	}

	// Propagate the effective location so DateParse (zero base time, UTC)
	// doesn't leak the caller's local timezone into the reparse.
	reparseOpts := append([]Option(nil), opts...)
	reparseOpts = append(reparseOpts, InTZ(loc))
	t, err := StrToTime(rest, reparseOpts...)
	if err != nil {
		return time.Time{}, false
	}

	// PHP behavior: if the weekday prefix doesn't match the parsed date,
	// advance to the next matching weekday
	if prefixDayNum >= 0 && int(t.Weekday()) != prefixDayNum {
		daysUntil := (prefixDayNum - int(t.Weekday()) + 7) % 7
		if daysUntil == 0 {
			daysUntil = 7
		}
		t = t.AddDate(0, 0, daysUntil)
	}
	return t, true
}

// restLooksLikeBareTime reports whether s is a time-of-day expression that
// lacks any date component (no dash/slash date markers, no 4-digit year).
func restLooksLikeBareTime(s string) bool {
	if strings.Contains(s, "-") || strings.Contains(s, "/") {
		return false
	}
	// HH:MM / HH:MM:SS variants.
	if strings.Contains(s, ":") && !strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyz") {
		return true
	}
	// Bare hour + am/pm ("10am", "9 pm", "8:30 am").
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "am") || strings.HasSuffix(lower, "pm") {
		prefix := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(lower, "am"), "pm"))
		for _, r := range prefix {
			if (r >= '0' && r <= '9') || r == ':' || r == ' ' || r == '.' {
				continue
			}
			return false
		}
		return len(prefix) > 0
	}
	return false
}

// StrToTime will convert the provided string into a time similarly to how PHP strtotime() works.
func StrToTime(str string, opts ...Option) (time.Time, error) {
	now, loc := resolveOptions(opts)

	str = strings.ToLower(strings.TrimSpace(str))
	if str == "" {
		return time.Time{}, ErrEmptyTimeString
	}

	pd := newParsedDate()
	if !dispatchStrToTime(str, now, loc, opts, pd) {
		return time.Time{}, fmt.Errorf("unable to parse time string: %s", str)
	}
	if pd.ErrorCount > 0 {
		return time.Time{}, fmt.Errorf("unable to parse time string: %s: %s", str, pd.firstError())
	}
	return pd.Materialize(now, loc)
}

// dispatchStrToTime runs the shared parse pipeline and returns true if any
// stage matched. It is also the body of DateParse (with a zero base time).
func dispatchStrToTime(str string, now time.Time, loc *time.Location, opts []Option, pd *ParsedDate) bool {
	if parseUnixTimestampInto(str, loc, pd) {
		return true
	}
	if parseKeywordInto(str, now, loc, pd) {
		return true
	}
	for _, parser := range formatParsers {
		sub := newParsedDate()
		if parser(str, now, loc, opts, sub) {
			copyComponents(pd, sub)
			if sub.hasMaterialized {
				pd.setMaterialized(sub.materialized)
			}
			if sub.Relative != nil {
				pd.Relative = sub.Relative
			}
			if sub.relativeApplied {
				pd.relativeApplied = true
			}
			return true
		}
	}
	if parseDateWithRelativeTimeInto(str, now, loc, opts, pd) {
		return true
	}
	if tryWeekdayPrefixReparseInto(str, now, loc, opts, pd) {
		return true
	}
	if isCompoundExpression(str) {
		// Try to parse purely-relative compounds (e.g. "-1 week +2 days")
		// by accumulating into the Relative block without collapsing to an
		// absolute time.
		if parseCompoundRelativeInto(str, now, loc, opts, pd) {
			return true
		}
		if t, err := parseCompoundExpression(str, now, opts); err == nil {
			pd.SetDate(t.Year(), int(t.Month()), t.Day())
			pd.SetTime(t.Hour(), t.Minute(), t.Second())
			pd.setMaterialized(t)
			return true
		} else {
			pd.AddError(0, err.Error())
			return false
		}
	}
	if parseOrdinalDateInto(str, now, loc, pd) {
		return true
	}

	parser := &Parser{
		tokens:   Tokenize(str),
		position: 0,
		result:   now,
		loc:      loc,
		pd:       pd,
	}
	result, err := parser.Parse()
	if err != nil {
		// The parser may have populated per-character errors already;
		// only emit a fallback if nothing was recorded.
		if pd.ErrorCount == 0 {
			pd.AddError(0, err.Error())
		}
		return false
	}
	// Token parser mutates p.result in place, so the returned time already
	// has any relative offsets baked in. Record relativeApplied so that
	// Materialize doesn't double-apply the Relative block the parser also
	// populated for DateParse reporting.
	pd.setMaterialized(result)
	pd.relativeApplied = true
	return true
}

// Parser represents a token stream parser for time expressions
type Parser struct {
	tokens     []Token
	position   int
	result     time.Time
	loc        *time.Location
	tzFound    bool        // Flag to indicate if a timezone was parsed from the input
	monthFound bool        // Flag to indicate if a month name was parsed (affects 4-digit number interpretation)
	pd         *ParsedDate // optional; when non-nil, tryParse* methods populate components
}

// Parse processes the token stream and returns a time.Time result
func (p *Parser) Parse() (time.Time, error) {
	// Skip any leading whitespace
	p.skipWhitespace()

	// Try standard date formats first
	if t, ok, err := p.tryParseStandardDate(); ok {
		return t, err
	}

	// Try relative expressions
	for p.position < len(p.tokens) {
		// Skip whitespace between expressions
		p.skipWhitespace()

		if p.position >= len(p.tokens) {
			break
		}

		// Try to parse each expression type
		parsed := false

		// Try to parse timezone
		if !p.tzFound {
			if ok := p.tryParseTimezone(); ok {
				parsed = true
			}
		}

		// Try "first/last day of this/next/last month/year"
		if !parsed {
			if t, ok, err := p.tryParseFirstLastDayOfExpression(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try "next/last" expressions
		if !parsed {
			if t, ok, err := p.tryParseNextLastExpression(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try bare weekday or "weekday next/last week [time]"
		if !parsed {
			if t, ok, err := p.tryParseBareWeekday(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try +/- relative time
		if !parsed {
			if t, ok, err := p.tryParseRelativeTime(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try implicit positive relative time (e.g., "4 days" without explicit +)
		if !parsed {
			if t, ok, err := p.tryParseImplicitRelativeTime(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try "N weekday ago"
		if !parsed {
			if t, ok, err := p.tryParseWeekdayAgo(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try ordinal word + unit (e.g., "eighth day")
		if !parsed {
			if t, ok, err := p.tryParseOrdinalRelativeTime(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try standalone time expression "HH:MM[:SS]"
		if !parsed {
			if t, ok, err := p.tryParseTimeExpression(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try bare hour with am/pm "10am", "10 pm"
		if !parsed {
			if t, ok, err := p.tryParseBareHourAMPM(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try day keywords "tomorrow" / "yesterday" / "today" / "now"
		if !parsed {
			if t, ok, err := p.tryParseDayKeyword(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try time keywords "midnight" / "noon"
		if !parsed {
			if t, ok, err := p.tryParseTimeKeyword(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Try month only format (e.g., "January" or "Feb")
		if !parsed {
			if t, ok, err := p.tryParseMonthOnlyFormat(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				p.monthFound = true
				parsed = true
			}
		}

		// Try month name format
		if !parsed {
			if t, ok, err := p.tryParseMonthNameFormat(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				p.monthFound = true
				parsed = true
			}
		}

		// Try bare 4-digit year (must be last number check)
		if !parsed {
			if t, ok, err := p.tryParseYearOnly(); ok {
				if err != nil {
					return time.Time{}, err
				}
				p.result = t
				parsed = true
			}
		}

		// Handle unrecognized token
		if !parsed && p.position < len(p.tokens) {
			currentToken := p.tokens[p.position]
			p.position++
			if currentToken.Typ != TypeWhitespace {
				// PHP quirk: the filler words "at"/"on" inside an otherwise
				// valid expression are reported as unknown-TZ errors, not
				// per-character unexpected characters.
				lower := strings.ToLower(currentToken.Val)
				if (lower == "at" || lower == "on") && p.pd != nil {
					p.pd.IsLocaltime = true
					p.pd.ZoneType = 0
					p.pd.AddError(currentToken.Pos, "The timezone could not be found in the database")
					continue
				}
				if p.pd != nil {
					// PHP-compatible: one "Unexpected character" per byte.
					for i := range currentToken.Val {
						p.pd.AddError(currentToken.Pos+i, "Unexpected character")
					}
				}
				return time.Time{}, fmt.Errorf("unexpected token: %s", currentToken.Val)
			}
		}

		// Skip whitespace after expressions
		p.skipWhitespace()
	}

	return p.result, nil
}

// skipWhitespace advances the position past any whitespace tokens
func (p *Parser) skipWhitespace() {
	for p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeWhitespace {
		p.position++
	}
}

// tryParseTimezone attempts to parse a timezone from the token stream
// This handles abbreviations (PST, EST), slash-separated paths (America/New_York,
// America/Argentina/Buenos_Aires), hyphenated names (America/Port-au-Prince),
// and multi-word names (Eastern Time).
func (p *Parser) tryParseTimezone() bool {
	if p.position >= len(p.tokens) {
		return false
	}

	// Must start with a string token
	if p.tokens[p.position].Typ != TypeString {
		return false
	}

	startPos := p.position

	// Try single token timezone first (EST, GMT, etc.)
	tzString := p.tokens[p.position].Val
	if loc, found := tryParseTimezone(tzString); found {
		p.loc = loc
		p.tzFound = true
		p.position++
		p.result = p.result.In(p.loc)
		if p.pd != nil {
			setTZFromName(p.pd, tzString, loc)
		}
		return true
	}

	// Try extending with / and - to build timezone paths like
	// America/New_York, America/Argentina/Buenos_Aires, America/Port-au-Prince
	var bestLoc *time.Location
	var bestName string
	bestPos := -1

	pos := p.position + 1
	for pos+1 < len(p.tokens) {
		sep := p.tokens[pos]
		if sep.Typ != TypeOperator || (sep.Val != "/" && sep.Val != "-") {
			break
		}
		next := p.tokens[pos+1]
		if next.Typ != TypeString {
			break
		}
		tzString = tzString + sep.Val + next.Val
		pos += 2

		if loc, found := tryParseTimezone(tzString); found {
			bestLoc = loc
			bestName = tzString
			bestPos = pos
		}
	}

	if bestLoc != nil {
		p.loc = bestLoc
		p.tzFound = true
		p.position = bestPos
		p.result = p.result.In(p.loc)
		if p.pd != nil {
			setTZFromName(p.pd, bestName, bestLoc)
		}
		return true
	}

	// Try parsing multi-word timezone names (like "Eastern Time")
	if p.position+2 < len(p.tokens) &&
		p.tokens[p.position+1].Typ == TypeWhitespace &&
		p.tokens[p.position+2].Typ == TypeString {

		tzString = p.tokens[p.position].Val + " " + p.tokens[p.position+2].Val

		if loc, found := tryParseTimezone(tzString); found {
			p.loc = loc
			p.tzFound = true
			p.position += 3
			p.result = p.result.In(p.loc)
			if p.pd != nil {
				setTZFromName(p.pd, tzString, loc)
			}
			return true
		}
	}

	p.position = startPos
	return false
}

// tryParseStandardDate attempts to parse standard date formats like ISO dates
func (p *Parser) tryParseStandardDate() (time.Time, bool, error) {
	// Check if we have enough tokens for a date format (at least 5 tokens: num op num op num)
	if p.position+4 >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	// First make sure we have potential date format tokens
	if p.tokens[p.position].Typ != TypeNumber ||
		p.tokens[p.position+2].Typ != TypeNumber ||
		p.tokens[p.position+4].Typ != TypeNumber {
		return time.Time{}, false, nil
	}

	// Get the first three numbers (potential year, month, day in some order)
	firstNum, err1 := strconv.Atoi(p.tokens[p.position].Val)
	if err1 != nil {
		return time.Time{}, false, fmt.Errorf("invalid number in date: %w", err1)
	}

	secondNum, err2 := strconv.Atoi(p.tokens[p.position+2].Val)
	if err2 != nil {
		return time.Time{}, false, fmt.Errorf("invalid number in date: %w", err2)
	}

	thirdNum, err3 := strconv.Atoi(p.tokens[p.position+4].Val)
	if err3 != nil {
		return time.Time{}, false, fmt.Errorf("invalid number in date: %w", err3)
	}

	// Check the separators
	if p.tokens[p.position+1].Typ != TypeOperator || p.tokens[p.position+3].Typ != TypeOperator {
		return time.Time{}, false, nil
	}

	separator := p.tokens[p.position+1].Val
	if separator != p.tokens[p.position+3].Val {
		return time.Time{}, false, nil
	}

	// Determine the format based on the separators and numbers
	var year, month, day int

	switch separator {
	case "-":
		// ISO format: YYYY-MM-DD or D-M-YYYY
		if len(p.tokens[p.position].Val) >= 4 {
			year, month, day = firstNum, secondNum, thirdNum
			// PHP doesn't support years > 9999 in YYYY-MM-DD format
			if year > 9999 {
				return time.Time{}, false, nil
			}
		} else if len(p.tokens[p.position+4].Val) >= 4 {
			// D-M-YYYY (European style with dashes)
			day, month, year = firstNum, secondNum, thirdNum
		} else {
			// Short year, try as Y-M-D
			year, month, day = firstNum, secondNum, thirdNum
			if year < 100 {
				year = parseTwoDigitYear(year)
			}
		}
	case "/":
		// Could be YYYY/MM/DD or MM/DD/YYYY
		if len(p.tokens[p.position].Val) >= 4 {
			year, month, day = firstNum, secondNum, thirdNum
		} else if len(p.tokens[p.position+4].Val) >= 4 {
			month, day, year = firstNum, secondNum, thirdNum
		} else {
			return time.Time{}, false, nil
		}
	case ".":
		// European format: DD.MM.YY or DD.MM.YYYY
		day, month, year = firstNum, secondNum, thirdNum
		// Handle 2-digit years
		if year < 100 {
			year = parseTwoDigitYear(year)
		}
	default:
		return time.Time{}, false, nil
	}

	// Validate date components using our utility function
	if !IsValidDate(year, month, day) {
		return time.Time{}, false, NewInvalidDateError(year, month, day)
	}

	// Advance position past the parsed date
	p.position += 5

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, p.loc), true, nil
}

// tryParseNextLastExpression attempts to parse expressions like "next Monday" or "last year"
func (p *Parser) tryParseNextLastExpression() (time.Time, bool, error) {
	if p.position >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	// Check for "next", "last", or "this"
	token := p.tokens[p.position]
	if token.Typ != TypeString || (token.Val != DirectionNext && token.Val != DirectionLast && token.Val != "this") {
		return time.Time{}, false, nil
	}

	isNext := token.Val == DirectionNext
	isThis := token.Val == "this"
	p.position++
	p.skipWhitespace()

	// Check for the unit token
	if p.position >= len(p.tokens) {
		return time.Time{}, false, fmt.Errorf("%w after %s", ErrExpectedTimeUnit, token.Val)
	}

	unitToken := p.tokens[p.position]
	if unitToken.Typ != TypeString {
		return time.Time{}, false, fmt.Errorf("%w after %s, got %s", ErrExpectedTimeUnit, token.Val, unitToken.Val)
	}

	p.position++

	// Handle special case: "next week" and "last week"
	// PHP treats Monday as the first day of the week.
	if unitToken.Val == UnitWeek {
		if p.pd != nil {
			// PHP represents "next/last/this week" as weekday=1 (Monday)
			// plus a +/-7 day offset.
			p.pd.SetRelativeWeekday(1)
			if isNext {
				p.pd.AddRelative(UnitDay, 7)
			} else if !isThis {
				p.pd.AddRelative(UnitDay, -7)
			}
		}
		dayOfWeek := int(p.result.Weekday())
		// Days since this week's Monday (Go weekday: 0=Sun,1=Mon,...,6=Sat)
		daysSinceMonday := (dayOfWeek + 6) % 7

		if isNext {
			// Next week = next Monday (always 1-7 days ahead)
			return p.result.AddDate(0, 0, 7-daysSinceMonday), true, nil
		} else if isThis {
			// This week = this Monday
			return p.result.AddDate(0, 0, -daysSinceMonday), true, nil
		} else {
			// Last week = previous week's Monday (always 7-13 days back)
			return p.result.AddDate(0, 0, -(daysSinceMonday + 7)), true, nil
		}
	}

	// Check if it's a day of the week
	dayNum := getDayOfWeek(unitToken.Val)
	if dayNum >= 0 {
		if p.pd != nil {
			p.pd.SetRelativeWeekday(dayNum)
			// PHP sets hour/minute/second to 0 for "next/last/this <weekday>".
			p.pd.SetTime(0, 0, 0)
			// "last <weekday>" adds a -7 day offset to the relative block.
			if !isNext && !isThis {
				p.pd.AddRelative(UnitDay, -7)
			}
		}
		// Handle day of week
		currentDay := int(p.result.Weekday())
		if isThis {
			// "this X" = the X of the current week (can be past or future)
			daysUntil := (dayNum - currentDay + 7) % 7
			targetDay := p.result.AddDate(0, 0, daysUntil)
			year, month, day := targetDay.Date()
			return time.Date(year, month, day, 0, 0, 0, 0, p.loc), true, nil
		} else if isNext {
			// "next X" = the upcoming occurrence of that day
			daysUntil := (dayNum - currentDay + 7) % 7
			if daysUntil == 0 {
				daysUntil = 7 // If today is the target day, go to next week
			}
			nextDay := p.result.AddDate(0, 0, daysUntil)
			year, month, day := nextDay.Date()
			return time.Date(year, month, day, 0, 0, 0, 0, p.loc), true, nil
		} else {
			// Calculate days since the last occurrence
			daysSince := (currentDay - dayNum + 7) % 7
			if daysSince == 0 {
				daysSince = 7 // If today is the target day, go to last week
			}
			lastDay := p.result.AddDate(0, 0, -daysSince)
			year, month, day := lastDay.Date()
			return time.Date(year, month, day, 0, 0, 0, 0, p.loc), true, nil
		}
	}

	// Handle other time units
	switch unitToken.Val {
	case UnitDay:
		if p.pd != nil {
			p.pd.relative()
			if isNext {
				p.pd.AddRelative(UnitDay, 1)
			} else if !isThis {
				p.pd.AddRelative(UnitDay, -1)
			}
		}
		if isNext {
			return p.result.AddDate(0, 0, 1), true, nil
		}
		if isThis {
			return p.result, true, nil
		}
		return p.result.AddDate(0, 0, -1), true, nil
	case UnitMonth:
		if p.pd != nil {
			// PHP always emits the relative block for this/next/last X.
			p.pd.relative()
			if isNext {
				p.pd.AddRelative(UnitMonth, 1)
			} else if !isThis {
				p.pd.AddRelative(UnitMonth, -1)
			}
		}
		if isNext {
			return p.result.AddDate(0, 1, 0), true, nil
		} else {
			return p.result.AddDate(0, -1, 0), true, nil
		}
	case UnitYear:
		if p.pd != nil {
			p.pd.relative()
			if isNext {
				p.pd.AddRelative(UnitYear, 1)
			} else if !isThis {
				p.pd.AddRelative(UnitYear, -1)
			}
		}
		if isNext {
			return p.result.AddDate(1, 0, 0), true, nil
		} else {
			return p.result.AddDate(-1, 0, 0), true, nil
		}
	case UnitHour, UnitMinute, UnitSecond:
		if p.pd != nil {
			p.pd.relative()
			if isNext {
				p.pd.AddRelative(unitToken.Val, 1)
			} else if !isThis {
				p.pd.AddRelative(unitToken.Val, -1)
			}
		}
		if isNext {
			return applyTimeOffset(p.result, 1, unitToken.Val), true, nil
		}
		if isThis {
			return p.result, true, nil
		}
		return applyTimeOffset(p.result, -1, unitToken.Val), true, nil
	default:
		return time.Time{}, false, fmt.Errorf("%w: %s", ErrInvalidTimeUnit, unitToken.Val)
	}
}

// daysInMonth returns the number of days in a given month and year

// isCompoundExpression checks if a string is a compound time expression (contains + or - in the middle)
func isCompoundExpression(str string) bool {
	// Don't classify a numeric timezone suffix (+09:00, -0500) as a compound
	// expression. A trailing " +HHMM" / " +HH:MM" / " -HHMM" is a TZ marker.
	if idx := strings.LastIndexAny(str, "+-"); idx > 0 && str[idx-1] == ' ' {
		tail := str[idx:]
		if _, _, ok := parseNumericTimezoneOffset(tail); ok {
			trimmed := strings.TrimSpace(str[:idx])
			// If the prefix contains no other +/- in the middle, this isn't
			// a compound expression.
			if !containsInfixSign(trimmed) {
				return false
			}
		}
	}

	// Normalize spaces around operators
	spaceOperatorRe := strings.NewReplacer(" + ", "+", " - ", "-", "+ ", "+", "- ", "-")
	normalizedStr := spaceOperatorRe.Replace(str)

	// Check if we have + or - in the middle of the string (not at the start)
	return (strings.Contains(normalizedStr, "+") && !strings.HasPrefix(normalizedStr, "+")) ||
		(strings.Contains(normalizedStr, "-") && !strings.HasPrefix(normalizedStr, "-"))
}

// containsInfixSign reports whether s has a '+' or '-' after position 0.
func containsInfixSign(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			return true
		}
	}
	return false
}

// parseDateWithRelativeTime parses a date followed by a relative time adjustment
// Examples: "2023-05-30 -1 month" or "2022-01-01 +1 year"
func parseDateWithRelativeTime(str string, now time.Time, loc *time.Location, opts []Option) (time.Time, bool) {
	// Split on first whitespace to get date part and rest
	datePart, timePart, ok := splitDateAndRest(str)
	if !ok {
		return time.Time{}, false
	}

	// Parse the date part
	dateResult, err := StrToTime(datePart, append(opts, Rel(now))...)
	if err != nil {
		return time.Time{}, false
	}

	// Handle special case for month end dates when subtracting months
	if timePart == "-1 month" {
		year, month, day := dateResult.Date()

		// Check if it's the last day of the month
		if day == daysInMonth(year, month) {
			// Create a date for the first day of the current month
			firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
			// Subtract one day to get the last day of the previous month
			prevMonth := firstOfMonth.AddDate(0, -1, 0)
			// Get the last day of the previous month
			lastDay := daysInMonth(prevMonth.Year(), prevMonth.Month())

			// Create the final date with the last day of the previous month,
			// preserving hour, minute, second from the original date
			return time.Date(prevMonth.Year(), prevMonth.Month(), lastDay,
				dateResult.Hour(), dateResult.Minute(), dateResult.Second(),
				dateResult.Nanosecond(), loc), true
		}
	}

	// Parse the time part using the date as reference
	finalResult, err := StrToTime(timePart, append(opts, Rel(dateResult))...)
	if err != nil {
		return time.Time{}, false
	}

	return finalResult, true
}

// parseCompoundExpression parses a compound time expression like "next year+4 days"
func parseCompoundExpression(str string, now time.Time, opts []Option) (time.Time, error) {
	// Normalize compound expressions like "next year+4 days" or "next year + 4 days"
	// Replace spaces around + and - with nothing to make parsing easier
	spaceOperatorRe := strings.NewReplacer(" + ", "+", " - ", "-", "+ ", "+", "- ", "-")
	normalizedStr := spaceOperatorRe.Replace(str)

	// Split the string at + and - operators
	var parts []string
	var operators []string

	// Find all + and - operators (not at the beginning)
	currentPart := ""
	for i := 0; i < len(normalizedStr); i++ {
		if (normalizedStr[i] == '+' || normalizedStr[i] == '-') && i > 0 {
			parts = append(parts, currentPart)
			operators = append(operators, string(normalizedStr[i]))
			currentPart = ""
		} else {
			currentPart += string(normalizedStr[i])
		}
	}

	// Add the last part
	if currentPart != "" {
		parts = append(parts, currentPart)
	}

	// Validate that we have at least one part and one operator
	if len(parts) < 2 || len(operators) < 1 {
		return time.Time{}, errors.New("invalid compound expression format")
	}

	// Process the first part
	result, err := StrToTime(parts[0], append(opts, Rel(now))...)
	if err != nil {
		return time.Time{}, err
	}

	// Process each remaining part with its operator
	for i := 0; i < len(operators); i++ {
		// Check if we have a corresponding part for this operator
		if i+1 >= len(parts) {
			return time.Time{}, errors.New("missing operand after operator in compound expression")
		}

		// Apply the operator to the part
		opPart := operators[i] + parts[i+1]
		nextResult, err := StrToTime(opPart, append(opts, Rel(result))...)
		if err != nil {
			return time.Time{}, err
		}
		result = nextResult
	}

	return result, nil
}

// applyTimeUnitOffset applies a time unit offset to the parser's result time.
func (p *Parser) applyTimeUnitOffset(amount int, unitStr string) (time.Time, error) {
	canonical := normalizeTimeUnit(unitStr)
	switch canonical {
	case UnitDay, UnitWeek, UnitWeekDay, UnitMonth, UnitYear, UnitHour, UnitMinute, UnitSecond:
		if p.pd != nil {
			p.pd.AddRelative(canonical, amount)
		}
		return applyTimeOffset(p.result, amount, unitStr), nil
	default:
		return time.Time{}, fmt.Errorf("%w: %s", ErrInvalidTimeUnit, unitStr)
	}
}

// tryParseRelativeTime attempts to parse expressions like "+1 day" or "-3 weeks"
func (p *Parser) tryParseRelativeTime() (time.Time, bool, error) {
	if p.position >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	// Check for +/- operator (may be multi-character: "+-" means negative, "--" means positive)
	// PHP rejects "++" but allows "--" and "+-"
	token := p.tokens[p.position]
	if token.Typ != TypeOperator {
		return time.Time{}, false, nil
	}
	// Multi-character sign operators must contain at least one '-'
	if len(token.Val) > 1 && !strings.Contains(token.Val, "-") {
		return time.Time{}, false, nil
	}
	// Count minus signs to determine final sign
	sign := 1
	for _, c := range token.Val {
		if c == '-' {
			sign = -sign
		} else if c != '+' {
			return time.Time{}, false, nil
		}
	}
	p.position++

	// Check for the amount
	if p.position >= len(p.tokens) {
		return time.Time{}, false, fmt.Errorf("%w after %s", ErrMissingAmount, token.Val)
	}

	amountToken := p.tokens[p.position]
	if amountToken.Typ != TypeNumber {
		return time.Time{}, false, fmt.Errorf("expected number after %s, got %s", token.Val, amountToken.Val)
	}

	amount, err := strconv.Atoi(amountToken.Val)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%w: %s", ErrInvalidNumber, amountToken.Val)
	}
	amount *= sign
	p.position++

	// Skip whitespace
	p.skipWhitespace()

	// Check for the unit
	if p.position >= len(p.tokens) {
		return time.Time{}, false, fmt.Errorf("%w after %d", ErrExpectedTimeUnit, amount)
	}

	unitToken := p.tokens[p.position]
	if unitToken.Typ != TypeString {
		return time.Time{}, false, fmt.Errorf("%w after %d, got %s", ErrExpectedTimeUnit, amount, unitToken.Val)
	}

	p.position++

	// Process the unit by calling the common helper function
	result, err := p.applyTimeUnitOffset(amount, unitToken.Val)
	if err != nil {
		return time.Time{}, false, err
	}

	return result, true, nil
}

// tryParseImplicitRelativeTime attempts to parse expressions like "4 days" or "10 minutes" (without explicit + operator)
func (p *Parser) tryParseImplicitRelativeTime() (time.Time, bool, error) {
	if p.position >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	// Save position for rollback on failure
	startPos := p.position

	// Check for a number (the amount)
	token := p.tokens[p.position]
	if token.Typ != TypeNumber {
		return time.Time{}, false, nil
	}

	amount, err := strconv.Atoi(token.Val)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%w: %s", ErrInvalidNumber, token.Val)
	}

	// Always treat as positive (implicit +)
	p.position++

	// Skip whitespace
	p.skipWhitespace()

	// Check for the unit
	if p.position >= len(p.tokens) {
		p.position = startPos
		return time.Time{}, false, nil
	}

	unitToken := p.tokens[p.position]
	if unitToken.Typ != TypeString {
		p.position = startPos
		return time.Time{}, false, nil
	}

	p.position++

	// Check for "ago" keyword — negates the amount
	savedPosAfterUnit := p.position
	p.skipWhitespace()
	if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeString && p.tokens[p.position].Val == "ago" {
		amount = -amount
		p.position++
	} else {
		p.position = savedPosAfterUnit
	}

	// Process the unit by calling the common helper function
	result, err := p.applyTimeUnitOffset(amount, unitToken.Val)
	if err != nil {
		p.position = startPos
		return time.Time{}, false, nil
	}

	return result, true, nil
}

// tryParseMonthOnlyFormat attempts to parse just a month name like "January" or "Feb"
func (p *Parser) tryParseMonthOnlyFormat() (time.Time, bool, error) {
	if p.position >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	// Check if this is actually a month name followed by a day
	// If it is, we should let tryParseMonthNameFormat handle it.
	// Exception: a number followed by ':' is a time expression, not a day,
	// so we should handle the month here and let tryParseTimeExpression consume the time.
	if p.position+1 < len(p.tokens) {
		nextToken := p.tokens[p.position+1]
		// If next token is a number, this might be "Month Day" format
		if nextToken.Typ == TypeNumber {
			numIdx := p.position + 1
			if !(numIdx+1 < len(p.tokens) &&
				p.tokens[numIdx+1].Typ == TypeOperator && p.tokens[numIdx+1].Val == ":") {
				return time.Time{}, false, nil
			}
		}
		// If next token is "." (period after abbreviation like "Dec."), defer to month name format
		if nextToken.Typ == TypeOperator && nextToken.Val == "." {
			return time.Time{}, false, nil
		}
		// Or if it's whitespace followed by a number, it might be "Month Day" with space
		if nextToken.Typ == TypeWhitespace && p.position+2 < len(p.tokens) &&
			p.tokens[p.position+2].Typ == TypeNumber {
			numIdx := p.position + 2
			if !(numIdx+1 < len(p.tokens) &&
				p.tokens[numIdx+1].Typ == TypeOperator && p.tokens[numIdx+1].Val == ":") {
				return time.Time{}, false, nil
			}
		}
	}

	// Check for a month name
	monthToken := p.tokens[p.position]
	if monthToken.Typ != TypeString {
		return time.Time{}, false, nil
	}

	month, ok := getMonthByName(monthToken.Val)
	if !ok {
		return time.Time{}, false, nil
	}

	// Consume the month token
	p.position++

	// Use the current year and preserve the day from the base time (PHP behavior)
	year := p.result.Year()
	day := p.result.Day()

	// Clamp day to the max days in the target month
	maxDays := daysInMonth(year, month)
	if day > maxDays {
		day = maxDays
	}

	return time.Date(year, month, day, 0, 0, 0, 0, p.loc), true, nil
}

// tryParseMonthNameFormat attempts to parse expressions like "January 15 2023", "Jan 15, 2023", "April 4th", or "June 1 1985 16:30:00 Europe/Paris"
func (p *Parser) tryParseMonthNameFormat() (time.Time, bool, error) {
	if p.position >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	// Check for a month name
	monthToken := p.tokens[p.position]
	if monthToken.Typ != TypeString {
		return time.Time{}, false, nil
	}

	month, ok := getMonthByName(monthToken.Val)
	if !ok {
		return time.Time{}, false, nil
	}
	p.position++

	// Skip optional period after month abbreviation (e.g., "Dec.")
	if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeOperator && p.tokens[p.position].Val == "." {
		p.position++
	}
	p.skipWhitespace()

	// Check for day number
	if p.position >= len(p.tokens) {
		return time.Time{}, false, fmt.Errorf("expected day after month name")
	}

	dayToken := p.tokens[p.position]
	if dayToken.Typ != TypeNumber {
		return time.Time{}, false, fmt.Errorf("expected day number after month name, got %s", dayToken.Val)
	}

	day, err := strconv.Atoi(dayToken.Val)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid day number: %s", dayToken.Val)
	}
	p.position++

	// Check for ordinal suffix (like "th", "st", "nd", "rd")
	if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeString {
		suffix := strings.ToLower(p.tokens[p.position].Val)
		if suffix == "st" || suffix == "nd" || suffix == "rd" || suffix == "th" {
			// Skip the ordinal suffix
			p.position++
		}
	}

	// Skip optional punctuation (comma) and whitespace
	if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypePunctuation {
		p.position++
	}
	p.skipWhitespace()

	// Check for year (optional - if not present, use current year)
	year := p.result.Year() // Default to current year
	yearFromInput := false

	if p.position < len(p.tokens) {
		yearToken := p.tokens[p.position]
		if yearToken.Typ == TypeNumber {
			// Check if this number is actually a time (followed by ':')
			// If so, don't consume it as a year
			isTime := p.position+1 < len(p.tokens) &&
				p.tokens[p.position+1].Typ == TypeOperator &&
				p.tokens[p.position+1].Val == ":"
			if !isTime {
				yearVal, err := strconv.Atoi(yearToken.Val)
				if err != nil {
					return time.Time{}, false, fmt.Errorf("invalid year: %s", yearToken.Val)
				}
				year = yearVal
				yearFromInput = true
				p.position++
			}
		}
	}

	// Validate date components before returning
	if !IsValidDate(year, int(month), day) {
		return time.Time{}, false, fmt.Errorf("invalid date: %s %d, %d", month, day, year)
	}
	if p.pd != nil {
		if yearFromInput {
			p.pd.SetDate(year, int(month), day)
		} else {
			p.pd.SetMonth(int(month))
			p.pd.SetDay(day)
		}
	}

	// Default time components
	hour, minute, second := 0, 0, 0

	// Check for time (optional)
	// Format: HH:MM:SS
	p.skipWhitespace()
	if p.position+2 < len(p.tokens) &&
		p.tokens[p.position].Typ == TypeNumber && // HH
		p.tokens[p.position+1].Typ == TypeOperator && p.tokens[p.position+1].Val == ":" &&
		p.tokens[p.position+2].Typ == TypeNumber { // MM

		// Parse hour
		hourVal, err := strconv.Atoi(p.tokens[p.position].Val)
		if err == nil && hourVal >= 0 && hourVal <= 23 {
			hour = hourVal
			p.position += 2 // Skip HH:

			// Parse minute
			minuteVal, err := strconv.Atoi(p.tokens[p.position].Val)
			if err == nil && minuteVal >= 0 && minuteVal <= 59 {
				minute = minuteVal
				p.position++

				// Check for seconds (optional)
				if p.position+1 < len(p.tokens) &&
					p.tokens[p.position].Typ == TypeOperator && p.tokens[p.position].Val == ":" &&
					p.tokens[p.position+1].Typ == TypeNumber {

					p.position++ // Skip :
					secondVal, err := strconv.Atoi(p.tokens[p.position].Val)
					if err == nil && secondVal >= 0 && secondVal <= 59 {
						second = secondVal
						p.position++
					}
				}

				// Check for AM/PM (attached or space-separated)
				p.skipWhitespace()
				if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeString {
					ampm := strings.ToLower(p.tokens[p.position].Val)
					if ampm == "am" || ampm == "pm" {
						hour = applyAMPM(hour, ampm)
						p.position++
					}
				}
			}
		}
	}

	// Check for timezone (optional)
	p.skipWhitespace()
	tzStartPos := p.position
	if ok := p.tryParseTimezone(); ok {
		// Timezone was successfully parsed and p.loc has been updated
	} else {
		// No timezone found, restore position
		p.position = tzStartPos
	}

	// Record hour/minute/second if we actually parsed any time.
	if p.pd != nil && (hour != 0 || minute != 0 || second != 0) {
		p.pd.SetTime(hour, minute, second)
	}

	return time.Date(year, month, day, hour, minute, second, 0, p.loc), true, nil
}

// getMonthByName converts a month name to its number

// tryParseBareWeekday handles:
// - Bare weekday name: "tuesday" (next occurrence)
// - Weekday + "next/last week" with optional time: "monday next week 13:00"
// - Weekday + month [year]: "thursday nov 2007" (first occurrence in that month)
func (p *Parser) tryParseBareWeekday() (time.Time, bool, error) {
	if p.position >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	token := p.tokens[p.position]
	if token.Typ != TypeString {
		return time.Time{}, false, nil
	}

	dayNum := getDayOfWeek(token.Val)
	if dayNum < 0 {
		return time.Time{}, false, nil
	}

	p.position++
	p.skipWhitespace()

	// Check for "next/last/this week" after weekday
	if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeString {
		direction := p.tokens[p.position].Val
		if direction == DirectionNext || direction == DirectionLast || direction == "this" {
			savedPos := p.position
			p.position++
			p.skipWhitespace()

			if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeString &&
				p.tokens[p.position].Val == UnitWeek {
				p.position++

				// Calculate: find the Monday of next/last/this week, then offset to target weekday
				currentDay := int(p.result.Weekday())
				daysSinceMonday := (currentDay + 6) % 7

				var monday time.Time
				if direction == DirectionNext {
					monday = p.result.AddDate(0, 0, 7-daysSinceMonday)
				} else if direction == "this" {
					monday = p.result.AddDate(0, 0, -daysSinceMonday)
				} else {
					monday = p.result.AddDate(0, 0, -(daysSinceMonday + 7))
				}

				// Offset from Monday to target weekday (Mon=0, Tue=1, ..., Sun=6)
				targetOffset := (dayNum + 6) % 7
				result := monday.AddDate(0, 0, targetOffset)

				year, month, day := result.Date()
				hour, minute, second := 0, 0, 0

				// Check for optional time
				p.skipWhitespace()
				if p.position+2 < len(p.tokens) &&
					p.tokens[p.position].Typ == TypeNumber &&
					p.tokens[p.position+1].Typ == TypeOperator && p.tokens[p.position+1].Val == ":" &&
					p.tokens[p.position+2].Typ == TypeNumber {

					h, err := strconv.Atoi(p.tokens[p.position].Val)
					if err == nil && h >= 0 && h <= 23 {
						hour = h
						p.position += 2 // Skip HH:
						m, err := strconv.Atoi(p.tokens[p.position].Val)
						if err == nil && m >= 0 && m <= 59 {
							minute = m
							p.position++
						}
					}
				}

				return time.Date(year, month, day, hour, minute, second, 0, p.loc), true, nil
			}

			// Not "next/last week", restore position
			p.position = savedPos
		}

		// Check for weekday + month [year]: "thursday nov 2007"
		if m, ok := getMonthByName(p.tokens[p.position].Val); ok {
			p.position++
			p.skipWhitespace()

			year := p.result.Year()
			if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeNumber {
				if y, err := strconv.Atoi(p.tokens[p.position].Val); err == nil && y > 0 {
					year = y
					p.position++
				}
			}

			// Find 1st occurrence of the weekday in that month
			firstOfMonth := time.Date(year, m, 1, 0, 0, 0, 0, p.loc)
			firstDayOfWeek := int(firstOfMonth.Weekday())
			daysUntilFirst := (dayNum - firstDayOfWeek + 7) % 7
			resultDay := 1 + daysUntilFirst

			return time.Date(year, m, resultDay, 0, 0, 0, 0, p.loc), true, nil
		}
	}

	// Bare weekday name — PHP returns same day if base matches, else next occurrence
	if p.pd != nil {
		p.pd.SetRelativeWeekday(dayNum)
		// PHP sets hour/minute/second to 0 (and fraction defaults to 0)
		// for bare weekday expressions.
		p.pd.SetTime(0, 0, 0)
	}
	currentDay := int(p.result.Weekday())
	daysUntil := (dayNum - currentDay + 7) % 7
	nextDay := p.result.AddDate(0, 0, daysUntil)
	year, month, day := nextDay.Date()

	return time.Date(year, month, day, 0, 0, 0, 0, p.loc), true, nil
}

// tryParseFirstLastDayOfExpression handles "first/last day of this/next/last month/year"
func (p *Parser) tryParseFirstLastDayOfExpression() (time.Time, bool, error) {
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		return time.Time{}, false, nil
	}

	startPos := p.position
	token := p.tokens[p.position]

	var isFirst bool
	switch token.Val {
	case "first":
		isFirst = true
	case "last":
		isFirst = false
	default:
		return time.Time{}, false, nil
	}
	p.position++
	p.skipWhitespace()

	// Must be followed by "day"
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString || p.tokens[p.position].Val != "day" {
		p.position = startPos
		return time.Time{}, false, nil
	}
	p.position++
	p.skipWhitespace()

	// Must be followed by "of"
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString || p.tokens[p.position].Val != "of" {
		p.position = startPos
		return time.Time{}, false, nil
	}
	p.position++
	p.skipWhitespace()

	// Must be followed by "this"/"next"/"last"
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		p.position = startPos
		return time.Time{}, false, nil
	}

	direction := p.tokens[p.position].Val
	if direction != "this" && direction != DirectionNext && direction != DirectionLast {
		p.position = startPos
		return time.Time{}, false, nil
	}
	p.position++
	p.skipWhitespace()

	// Must be followed by "month" or "year"
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		p.position = startPos
		return time.Time{}, false, nil
	}

	unit := normalizeTimeUnit(p.tokens[p.position].Val)
	p.position++

	year := p.result.Year()
	month := p.result.Month()

	switch unit {
	case UnitMonth:
		// Use day=1 as reference to avoid overflow (e.g., Jan 31 + 1 month
		// would produce March 2, but "first day of next month" must land in Feb).
		firstOfCurrent := time.Date(year, month, 1, 0, 0, 0, 0, p.loc)
		if direction == DirectionNext {
			ref := firstOfCurrent.AddDate(0, 1, 0)
			year, month, _ = ref.Date()
			if p.pd != nil {
				p.pd.AddRelative(UnitMonth, 1)
			}
		} else if direction == DirectionLast {
			ref := firstOfCurrent.AddDate(0, -1, 0)
			year, month, _ = ref.Date()
			if p.pd != nil {
				p.pd.AddRelative(UnitMonth, -1)
			}
		}
	case UnitYear:
		if direction == DirectionNext {
			year = p.result.Year() + 1
			month = time.January
			if p.pd != nil {
				p.pd.AddRelative(UnitYear, 1)
			}
		} else if direction == DirectionLast {
			year = p.result.Year() - 1
			month = time.December
			if p.pd != nil {
				p.pd.AddRelative(UnitYear, -1)
			}
		}
	default:
		p.position = startPos
		return time.Time{}, false, nil
	}

	var day int
	if isFirst {
		day = 1
	} else {
		day = daysInMonth(year, month)
	}

	if p.pd != nil {
		if isFirst {
			p.pd.SetFirstLastDayOf(1)
		} else {
			p.pd.SetFirstLastDayOf(2)
		}
	}

	return time.Date(year, month, day, p.result.Hour(), p.result.Minute(), p.result.Second(), 0, p.loc), true, nil
}

// tryParseDayKeyword handles "tomorrow" / "yesterday" / "today" / "now"
// keywords when they appear as a token in the stream. PHP combines them with
// other tokens by applying a day offset to the relative block and resetting
// hour/minute/second/fraction to 0.
func (p *Parser) tryParseDayKeyword() (time.Time, bool, error) {
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		return time.Time{}, false, nil
	}
	switch p.tokens[p.position].Val {
	case "tomorrow":
		p.position++
		if p.pd != nil {
			p.pd.SetTime(0, 0, 0)
			p.pd.SetFraction(0)
			// Overwrite existing relative.day with +1 (replace semantics).
			r := p.pd.relative()
			r.Day = 1
		}
		return p.result.AddDate(0, 0, 1), true, nil
	case "yesterday":
		p.position++
		if p.pd != nil {
			p.pd.SetTime(0, 0, 0)
			p.pd.SetFraction(0)
			r := p.pd.relative()
			r.Day = -1
		}
		return p.result.AddDate(0, 0, -1), true, nil
	case "today":
		p.position++
		if p.pd != nil {
			p.pd.SetTime(0, 0, 0)
			p.pd.SetFraction(0)
		}
		y, m, d := p.result.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, p.loc), true, nil
	case "now":
		p.position++
		return p.result, true, nil
	}
	return time.Time{}, false, nil
}

// tryParseTimeKeyword handles "midnight" and "noon" keywords in token stream
func (p *Parser) tryParseTimeKeyword() (time.Time, bool, error) {
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		return time.Time{}, false, nil
	}

	year, month, day := p.result.Date()
	switch p.tokens[p.position].Val {
	case "midnight":
		p.position++
		if p.pd != nil {
			p.pd.SetTime(0, 0, 0)
			p.pd.SetFraction(0)
		}
		return time.Date(year, month, day, 0, 0, 0, 0, p.loc), true, nil
	case "noon":
		p.position++
		if p.pd != nil {
			p.pd.SetTime(12, 0, 0)
			p.pd.SetFraction(0)
		}
		return time.Date(year, month, day, 12, 0, 0, 0, p.loc), true, nil
	default:
		return time.Time{}, false, nil
	}
}

// tryParseWeekdayAgo handles "N weekday ago" or "N weekdays ago"
func (p *Parser) tryParseWeekdayAgo() (time.Time, bool, error) {
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeNumber {
		return time.Time{}, false, nil
	}

	startPos := p.position
	amount, err := strconv.Atoi(p.tokens[p.position].Val)
	if err != nil || amount <= 0 {
		return time.Time{}, false, nil
	}
	p.position++
	p.skipWhitespace()

	// Check for weekday name (singular or plural)
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		p.position = startPos
		return time.Time{}, false, nil
	}

	dayName := p.tokens[p.position].Val
	singularName := strings.TrimSuffix(dayName, "s")
	dayNum := getDayOfWeek(singularName)
	if dayNum < 0 {
		dayNum = getDayOfWeek(dayName)
	}
	if dayNum < 0 {
		p.position = startPos
		return time.Time{}, false, nil
	}
	p.position++
	p.skipWhitespace()

	// Check for "ago"
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString || p.tokens[p.position].Val != "ago" {
		p.position = startPos
		return time.Time{}, false, nil
	}
	p.position++

	// Go back N occurrences of that weekday
	currentDay := int(p.result.Weekday())
	daysSince := (currentDay - dayNum + 7) % 7
	if daysSince == 0 {
		daysSince = 7
	}
	totalDays := daysSince + (amount-1)*7
	result := p.result.AddDate(0, 0, -totalDays)

	if p.pd != nil {
		// PHP records "N weekdays ago" as relative.day=-(N-1)*7 with
		// weekday=-1 (a negative-direction flag, not a count).
		if amount > 1 {
			p.pd.AddRelative(UnitDay, -(amount-1)*7)
		}
		p.pd.SetRelativeWeekday(-1)
	}

	// Preserve time from p.result
	year, month, day := result.Date()
	return time.Date(year, month, day, p.result.Hour(), p.result.Minute(), p.result.Second(), p.result.Nanosecond(), p.loc), true, nil
}

// tryParseTimeExpression handles standalone time like "HH:MM" or "HH:MM:SS"
func (p *Parser) tryParseTimeExpression() (time.Time, bool, error) {
	if p.position+2 >= len(p.tokens) {
		return time.Time{}, false, nil
	}

	if p.tokens[p.position].Typ != TypeNumber ||
		p.tokens[p.position+1].Typ != TypeOperator || p.tokens[p.position+1].Val != ":" ||
		p.tokens[p.position+2].Typ != TypeNumber {
		return time.Time{}, false, nil
	}

	hour, err := strconv.Atoi(p.tokens[p.position].Val)
	if err != nil || hour < 0 || hour > 24 {
		return time.Time{}, false, nil
	}
	p.position += 2 // Skip HH:

	minute, err := strconv.Atoi(p.tokens[p.position].Val)
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, false, nil
	}
	p.position++

	second := 0
	fraction := 0.0
	hasFraction := false
	if p.position+1 < len(p.tokens) &&
		p.tokens[p.position].Typ == TypeOperator && p.tokens[p.position].Val == ":" &&
		p.tokens[p.position+1].Typ == TypeNumber {
		p.position++ // Skip :
		s, err := strconv.Atoi(p.tokens[p.position].Val)
		if err == nil && s >= 0 && s <= 59 {
			second = s
			p.position++
		}
		// Optional fractional seconds: . followed by digits
		if p.position+1 < len(p.tokens) &&
			p.tokens[p.position].Typ == TypeOperator && p.tokens[p.position].Val == "." &&
			p.tokens[p.position+1].Typ == TypeNumber {
			p.position++ // Skip .
			fracStr := p.tokens[p.position].Val
			if f, err := strconv.ParseFloat("0."+fracStr, 64); err == nil {
				fraction = f
				hasFraction = true
				p.position++
			}
		}
	}

	// Optional trailing am/pm (with or without separating whitespace).
	// Also accepts PHP's dotted forms "a.m." / "p.m." as a 3-token sequence.
	savedPos := p.position
	p.skipWhitespace()
	if p.position < len(p.tokens) && p.tokens[p.position].Typ == TypeString {
		tok := strings.ToLower(p.tokens[p.position].Val)
		switch tok {
		case "am", "pm":
			hour = applyAMPM(hour, tok)
			p.position++
		case "z":
			// Trailing Z marks UTC (PHP treats it as abbreviation).
			if p.pd != nil {
				p.pd.SetTZAbbreviation(time.UTC, "Z", 0, false)
			}
			p.loc = time.UTC
			p.tzFound = true
			p.position++
		case "a", "p":
			// "a.m." / "p.m." — require ".m." follow-up.
			if p.position+3 < len(p.tokens) &&
				p.tokens[p.position+1].Typ == TypeOperator && p.tokens[p.position+1].Val == "." &&
				p.tokens[p.position+2].Typ == TypeString && strings.ToLower(p.tokens[p.position+2].Val) == "m" &&
				p.tokens[p.position+3].Typ == TypeOperator && p.tokens[p.position+3].Val == "." {
				hour = applyAMPM(hour, tok+"m")
				p.position += 4
			} else {
				p.position = savedPos
			}
		default:
			p.position = savedPos
		}
	} else {
		p.position = savedPos
	}

	if p.pd != nil {
		p.pd.SetTime(hour, minute, second)
		p.pd.fractionDefaultsZero = true
		if hasFraction {
			p.pd.SetFraction(fraction)
		}
		// PHP emits a warning when the time has hour == 24.
		if hour == 24 {
			// Position: first char after the parsed time tokens.
			pos := 0
			if p.position > 0 && p.position <= len(p.tokens) {
				// Use start of next token if any, else len(input) + 1.
				if p.position < len(p.tokens) {
					pos = p.tokens[p.position].Pos
				} else {
					// Last parsed token
					last := p.tokens[p.position-1]
					pos = last.Pos + len(last.Val)
				}
				pos++ // PHP uses 1-past-end.
			}
			p.pd.AddWarning(pos, "The parsed time was invalid")
		}
	}
	year, month, day := p.result.Date()
	nanos := int(fraction * 1e9)
	return time.Date(year, month, day, hour, minute, second, nanos, p.loc), true, nil
}

// tryParseBareHourAMPM handles a bare hour followed by am/pm like "10am" or "10 pm"
func (p *Parser) tryParseBareHourAMPM() (time.Time, bool, error) {
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeNumber {
		return time.Time{}, false, nil
	}
	hour, err := strconv.Atoi(p.tokens[p.position].Val)
	if err != nil || hour < 1 || hour > 12 {
		return time.Time{}, false, nil
	}

	next := p.position + 1
	if next < len(p.tokens) && p.tokens[next].Typ == TypeWhitespace {
		next++
	}
	if next >= len(p.tokens) || p.tokens[next].Typ != TypeString {
		return time.Time{}, false, nil
	}
	ampm := strings.ToLower(p.tokens[next].Val)
	if ampm != "am" && ampm != "pm" {
		return time.Time{}, false, nil
	}

	hour = applyAMPM(hour, ampm)
	p.position = next + 1

	if p.pd != nil {
		p.pd.SetTime(hour, 0, 0)
	}
	year, month, day := p.result.Date()
	return time.Date(year, month, day, hour, 0, 0, 0, p.loc), true, nil
}

// tryParseOrdinalRelativeTime handles ordinal words as implicit relative time
// e.g., "eighth day" = +8 days
func (p *Parser) tryParseOrdinalRelativeTime() (time.Time, bool, error) {
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		return time.Time{}, false, nil
	}

	startPos := p.position
	amount := ordinalWordToNumber(p.tokens[p.position].Val)
	if amount <= 0 {
		return time.Time{}, false, nil
	}
	p.position++
	p.skipWhitespace()

	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeString {
		p.position = startPos
		return time.Time{}, false, nil
	}

	result, err := p.applyTimeUnitOffset(amount, p.tokens[p.position].Val)
	if err != nil {
		p.position = startPos
		return time.Time{}, false, nil
	}
	p.position++
	return result, true, nil
}

// tryParseYearOnly handles a bare 4-digit year number, setting the year on current result
func (p *Parser) tryParseYearOnly() (time.Time, bool, error) {
	if p.position >= len(p.tokens) || p.tokens[p.position].Typ != TypeNumber {
		return time.Time{}, false, nil
	}
	val := p.tokens[p.position].Val
	if len(val) != 4 {
		return time.Time{}, false, nil
	}
	num, err := strconv.Atoi(val)
	if err != nil || num < 0 {
		return time.Time{}, false, nil
	}

	// Only treat as year if it's the last token (or followed only by whitespace)
	nextPos := p.position + 1
	for nextPos < len(p.tokens) && p.tokens[nextPos].Typ == TypeWhitespace {
		nextPos++
	}
	if nextPos != len(p.tokens) {
		return time.Time{}, false, nil
	}

	p.position++

	// PHP behavior: a bare 4-digit number is interpreted as HHMM (military
	// time) when the minute portion is ≤ 59 AND the hour portion is ≤ 24.
	// Otherwise it's treated as a year. The HHMM interpretation also applies
	// when a month name was already parsed (e.g. "March 1 eighth day 2009").
	hhmmHour := num / 100
	hhmmMinute := num % 100
	if hhmmMinute <= 59 && hhmmHour <= 24 && (p.monthFound || num < 1000 || hhmmMinute <= 59 && hhmmHour <= 24) {
		// Emit as HHMM time, not year. Exception: if the input is just a
		// plain 4-digit year that also parses as HHMM (like 1200), PHP
		// prefers the time. If the minute is > 59 it's obviously a year.
		if p.monthFound {
			year, month, day := p.result.Date()
			if p.pd != nil {
				p.pd.SetTime(hhmmHour, hhmmMinute, 0)
				p.pd.fractionDefaultsZero = false
			}
			return time.Date(year, month, day, hhmmHour, hhmmMinute, 0, 0, p.loc), true, nil
		}
	}
	// For bare 4-digit inputs: if HHMM is valid, use it; else year.
	if !p.monthFound && hhmmMinute <= 59 && hhmmHour <= 24 {
		if p.pd != nil {
			p.pd.SetTime(hhmmHour, hhmmMinute, 0)
			// Bare HHMM — PHP emits fraction:false, not 0.
			p.pd.fractionDefaultsZero = false
			// Warn when hour == 24 (same as colon-style handling).
			if hhmmHour == 24 {
				p.pd.AddWarning(len(p.tokens[p.position-1].Val)+1, "The parsed time was invalid")
			}
		}
		year, month, day := p.result.Date()
		return time.Date(year, month, day, hhmmHour, hhmmMinute, 0, 0, p.loc), true, nil
	}

	if p.pd != nil {
		p.pd.SetYear(num)
	}
	return time.Date(num, p.result.Month(), p.result.Day(), p.result.Hour(), p.result.Minute(), p.result.Second(), p.result.Nanosecond(), p.loc), true, nil
}

// ordinalWordToNumber converts ordinal words to numbers
