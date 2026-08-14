package gotz

import (
	"fmt"
	"strings"
)

// RuleKind identifies the type of transition rule in a POSIX TZ string.
type RuleKind int

const (
	// RuleJulian is the Jn format: Julian day (1-365), Feb 29 is never counted.
	RuleJulian RuleKind = iota
	// RuleDOY is the n format: zero-based day of year (0-365), Feb 29 is counted.
	RuleDOY
	// RuleMonthWeekDay is the Mm.w.d format: month, week, day-of-week.
	RuleMonthWeekDay
)

// PosixTZ represents a parsed POSIX-style TZ string.
// Example: "EST5EDT,M3.2.0,M11.1.0"
type PosixTZ struct {
	StdAbbrev string // standard time abbreviation
	StdOffset int    // standard time UTC offset in seconds (positive = east)
	DSTAbbrev string // DST abbreviation (empty if no DST)
	DSTOffset int    // DST UTC offset in seconds
	Start     TransitionRule
	End       TransitionRule
}

// TransitionRule specifies when a DST transition occurs within a year.
type TransitionRule struct {
	Kind RuleKind
	Day  int // Julian day (1-365), DOY (0-365), or day-of-week (0=Sunday)
	Week int // week of month (1-5), only for RuleMonthWeekDay
	Mon  int // month (1-12), only for RuleMonthWeekDay
	Time int // seconds after midnight (default 7200 = 02:00:00)
}

// HasDST reports whether the POSIX TZ rule defines daylight saving time.
func (p *PosixTZ) HasDST() bool {
	return p.DSTAbbrev != ""
}

// ParsePosixTZ parses a POSIX-style TZ string.
func ParsePosixTZ(s string) (*PosixTZ, error) {
	p := &PosixTZ{}
	var err error
	var rest string

	// Parse standard time name.
	p.StdAbbrev, rest, err = parseTZName(s)
	if err != nil {
		return nil, err
	}
	if p.StdAbbrev == "" {
		return nil, fmt.Errorf("gotz: empty standard timezone name in %q", s)
	}

	// Parse standard time offset.
	// Note: POSIX offsets are positive west of GMT (opposite of ISO).
	// We negate to store as seconds east of UTC.
	var off int
	off, rest, err = parseTZOffset(rest)
	if err != nil {
		return nil, err
	}
	p.StdOffset = -off

	if rest == "" {
		// No DST.
		return p, nil
	}

	// Parse DST name.
	p.DSTAbbrev, rest, err = parseTZName(rest)
	if err != nil {
		return nil, err
	}
	if p.DSTAbbrev == "" {
		return nil, fmt.Errorf("gotz: empty DST timezone name in %q", s)
	}

	// Parse optional DST offset (default: std offset + 1 hour).
	if rest != "" && rest[0] != ',' {
		off, rest, err = parseTZOffset(rest)
		if err != nil {
			return nil, err
		}
		p.DSTOffset = -off
	} else {
		p.DSTOffset = p.StdOffset + 3600
	}

	// Parse transition rules.
	if rest == "" {
		// Default US rules: M3.2.0,M11.1.0
		p.Start = TransitionRule{Kind: RuleMonthWeekDay, Mon: 3, Week: 2, Day: 0, Time: 7200}
		p.End = TransitionRule{Kind: RuleMonthWeekDay, Mon: 11, Week: 1, Day: 0, Time: 7200}
		return p, nil
	}

	if rest[0] != ',' {
		return nil, fmt.Errorf("gotz: expected ',' before transition rules in %q", s)
	}
	rest = rest[1:]

	p.Start, rest, err = parseTZRule(rest)
	if err != nil {
		return nil, err
	}

	if rest == "" || rest[0] != ',' {
		return nil, fmt.Errorf("gotz: expected ',' between transition rules in %q", s)
	}
	rest = rest[1:]

	p.End, _, err = parseTZRule(rest)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Lookup returns the zone abbreviation, UTC offset, and DST flag in effect
// at the given Unix timestamp according to this POSIX TZ rule.
func (p *PosixTZ) Lookup(unix int64) (abbrev string, offset int, isDST bool) {
	if !p.HasDST() {
		return p.StdAbbrev, p.StdOffset, false
	}

	// Determine the year and compute transition times.
	year, yday, sec := unixToYdaySec(unix)
	yearSec := yday*86400 + sec

	startSec := ruleToYearSec(p.Start, year, p.StdOffset)
	endSec := ruleToYearSec(p.End, year, p.DSTOffset)

	var inDST bool
	if startSec < endSec {
		// Northern hemisphere: DST between start and end.
		inDST = yearSec >= startSec && yearSec < endSec
	} else {
		// Southern hemisphere: DST outside [end, start).
		inDST = yearSec >= startSec || yearSec < endSec
	}

	if inDST {
		return p.DSTAbbrev, p.DSTOffset, true
	}
	return p.StdAbbrev, p.StdOffset, false
}

// TransitionsForYear returns the DST start and end times as Unix timestamps
// for the given year. Returns ok=false if there is no DST.
func (p *PosixTZ) TransitionsForYear(year int) (dstStart, dstEnd int64, ok bool) {
	if !p.HasDST() {
		return 0, 0, false
	}

	yearStart := yearToUnix(year)
	startSec := ruleToYearSec(p.Start, year, p.StdOffset)
	endSec := ruleToYearSec(p.End, year, p.DSTOffset)

	return yearStart + int64(startSec), yearStart + int64(endSec), true
}

// String returns the POSIX TZ string representation.
func (p *PosixTZ) String() string {
	var b strings.Builder

	writeName(&b, p.StdAbbrev)
	writeOffset(&b, -p.StdOffset)

	if !p.HasDST() {
		return b.String()
	}

	writeName(&b, p.DSTAbbrev)
	if p.DSTOffset != p.StdOffset+3600 {
		writeOffset(&b, -p.DSTOffset)
	}

	b.WriteByte(',')
	writeRule(&b, p.Start)
	b.WriteByte(',')
	writeRule(&b, p.End)

	return b.String()
}

// --- Parsing helpers ---

func parseTZName(s string) (name, rest string, err error) {
	if len(s) == 0 {
		return "", "", nil
	}
	if s[0] == '<' {
		// Quoted name: <...>
		end := strings.IndexByte(s, '>')
		if end < 0 {
			return "", "", fmt.Errorf("gotz: unterminated '<' in TZ name %q", s)
		}
		return s[1:end], s[end+1:], nil
	}
	// Unquoted: letters only.
	i := 0
	for i < len(s) && isAlpha(s[i]) {
		i++
	}
	return s[:i], s[i:], nil
}

func parseTZOffset(s string) (offset int, rest string, err error) {
	if len(s) == 0 {
		return 0, "", fmt.Errorf("gotz: expected offset")
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	} else if s[0] == '+' {
		s = s[1:]
	}

	var hours, mins, secs int
	hours, s, err = parseTZNum(s, 0, 167)
	if err != nil {
		return 0, "", err
	}

	if len(s) > 0 && s[0] == ':' {
		s = s[1:]
		mins, s, err = parseTZNum(s, 0, 59)
		if err != nil {
			return 0, "", err
		}
		if len(s) > 0 && s[0] == ':' {
			s = s[1:]
			secs, s, err = parseTZNum(s, 0, 59)
			if err != nil {
				return 0, "", err
			}
		}
	}

	offset = hours*3600 + mins*60 + secs
	if neg {
		offset = -offset
	}
	return offset, s, nil
}

func parseTZRule(s string) (r TransitionRule, rest string, err error) {
	if len(s) == 0 {
		return r, "", fmt.Errorf("gotz: empty transition rule")
	}

	switch {
	case s[0] == 'M':
		// Mm.w.d
		r.Kind = RuleMonthWeekDay
		s = s[1:]
		r.Mon, s, err = parseTZNum(s, 1, 12)
		if err != nil {
			return r, "", err
		}
		if len(s) == 0 || s[0] != '.' {
			return r, "", fmt.Errorf("gotz: expected '.' after month in rule")
		}
		s = s[1:]
		r.Week, s, err = parseTZNum(s, 1, 5)
		if err != nil {
			return r, "", err
		}
		if len(s) == 0 || s[0] != '.' {
			return r, "", fmt.Errorf("gotz: expected '.' after week in rule")
		}
		s = s[1:]
		r.Day, s, err = parseTZNum(s, 0, 6)
		if err != nil {
			return r, "", err
		}

	case s[0] == 'J':
		// Jn (1-365, no leap day)
		r.Kind = RuleJulian
		s = s[1:]
		r.Day, s, err = parseTZNum(s, 1, 365)
		if err != nil {
			return r, "", err
		}

	default:
		// n (0-365, with leap day)
		r.Kind = RuleDOY
		r.Day, s, err = parseTZNum(s, 0, 365)
		if err != nil {
			return r, "", err
		}
	}

	// Optional time component: /time
	r.Time = 7200 // default 02:00:00
	if len(s) > 0 && s[0] == '/' {
		s = s[1:]
		var off int
		off, s, err = parseTZOffset(s)
		if err != nil {
			return r, "", err
		}
		r.Time = off
	}

	return r, s, nil
}

func parseTZNum(s string, min, max int) (int, string, error) {
	if len(s) == 0 || !isDigit(s[0]) {
		return 0, s, fmt.Errorf("gotz: expected digit in %q", s)
	}
	n := 0
	i := 0
	for i < len(s) && isDigit(s[i]) {
		n = n*10 + int(s[i]-'0')
		i++
	}
	if n < min || n > max {
		return 0, s, fmt.Errorf("gotz: number %d out of range [%d, %d]", n, min, max)
	}
	return n, s[i:], nil
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// --- Time computation helpers ---

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

var daysInMonth = [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// yearToUnix returns the Unix timestamp for January 1 00:00:00 UTC of the given year.
func yearToUnix(year int) int64 {
	// Days from Unix epoch (1970-01-01) to Jan 1 of year.
	y := int64(year) - 1970
	days := 365 * y

	// Add leap days. Count leap years in [1970, year).
	if year > 1970 {
		days += (y + 1) / 4
		days -= (y + 69) / 100
		days += (y + 369) / 400
	} else if year < 1970 {
		days += (y - 2) / 4
		days -= (y - 30) / 100
		days += (y - 30) / 400
	}

	return days * 86400
}

// unixToYdaySec converts a Unix timestamp to (year, day-of-year [0-based], second-of-day).
func unixToYdaySec(unix int64) (year int, yday int, sec int) {
	sec = int(unix % 86400)
	if sec < 0 {
		sec += 86400
		unix -= 86400
	}
	days := int(unix / 86400)

	// Compute year from days since epoch.
	// Algorithm: start with an estimate and adjust.
	year = 1970 + int(int64(days)/365)
	for {
		yearStart := int(yearToUnix(year) / 86400)
		if yearStart <= days {
			yearEnd := yearStart + 365
			if isLeapYear(year) {
				yearEnd++
			}
			if days < yearEnd {
				yday = days - yearStart
				return year, yday, sec
			}
			year++
		} else {
			year--
		}
	}
}

// ruleToYearSec converts a TransitionRule to seconds since the start of the year
// (in wall clock time, then adjusted by the given offset to produce UTC year-seconds).
func ruleToYearSec(r TransitionRule, year int, offset int) int {
	var yday int
	leap := isLeapYear(year)

	switch r.Kind {
	case RuleJulian:
		// Jn: 1-365, Feb 29 is never counted.
		yday = r.Day - 1
		if leap && yday >= 59 { // after Feb 28
			yday++
		}

	case RuleDOY:
		// n: 0-365.
		yday = r.Day

	case RuleMonthWeekDay:
		// Mm.w.d: month, week (1-5), day-of-week (0=Sunday).
		// Find the first day-of-week d in month m, then advance to week w.
		// week=5 means "last occurrence".
		m := r.Mon - 1 // 0-indexed

		// Day of year for the 1st of the month.
		firstYday := 0
		for i := 0; i < m; i++ {
			firstYday += daysInMonth[i]
			if i == 1 && leap {
				firstYday++
			}
		}

		// Day of week for Jan 1 of this year.
		// Using Zeller-like: 1970-01-01 was Thursday (4).
		jan1Wday := int((yearToUnix(year)/86400)%7+4+7*53) % 7 // 0=Sunday

		// Day of week for 1st of month.
		firstWday := (jan1Wday + firstYday) % 7

		// Days until the target day-of-week from the 1st.
		daysUntil := (r.Day - firstWday + 7) % 7

		// Advance to the target week.
		yday = firstYday + daysUntil + (r.Week-1)*7

		// week=5 means "last in month". Clamp to the month's length.
		monthDays := daysInMonth[m]
		if m == 1 && leap {
			monthDays++
		}
		for yday-firstYday >= monthDays {
			yday -= 7
		}
	}

	// Convert to seconds from start of year, apply the transition time,
	// then adjust from wall time to UTC.
	return yday*86400 + r.Time - offset
}

// --- String formatting helpers ---

func writeName(b *strings.Builder, name string) {
	needsQuote := false
	for i := 0; i < len(name); i++ {
		if !isAlpha(name[i]) {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		b.WriteByte('<')
		b.WriteString(name)
		b.WriteByte('>')
	} else {
		b.WriteString(name)
	}
}

func writeOffset(b *strings.Builder, posixOff int) {
	if posixOff < 0 {
		b.WriteByte('-')
		posixOff = -posixOff
	}
	hours := posixOff / 3600
	mins := (posixOff % 3600) / 60
	secs := posixOff % 60

	fmt.Fprintf(b, "%d", hours)
	if mins != 0 || secs != 0 {
		fmt.Fprintf(b, ":%02d", mins)
		if secs != 0 {
			fmt.Fprintf(b, ":%02d", secs)
		}
	}
}

func writeRule(b *strings.Builder, r TransitionRule) {
	switch r.Kind {
	case RuleJulian:
		fmt.Fprintf(b, "J%d", r.Day)
	case RuleDOY:
		fmt.Fprintf(b, "%d", r.Day)
	case RuleMonthWeekDay:
		fmt.Fprintf(b, "M%d.%d.%d", r.Mon, r.Week, r.Day)
	}
	if r.Time != 7200 {
		b.WriteByte('/')
		writeOffset(b, r.Time)
	}
}
