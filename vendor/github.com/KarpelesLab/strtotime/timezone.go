package strtotime

import (
	"strings"
	"time"
	"unicode"

	"github.com/KarpelesLab/gotz"
)

// Common timezone abbreviations
var timezoneAbbreviations = map[string]*time.Location{
	// North American time zones — use fixed offsets so parsing preserves the stated timezone
	"est":  time.FixedZone("EST", -5*3600),  // Eastern Standard Time (UTC-5)
	"edt":  time.FixedZone("EDT", -4*3600),  // Eastern Daylight Time (UTC-4)
	"cst":  time.FixedZone("CST", -6*3600),  // Central Standard Time (UTC-6)
	"cdt":  time.FixedZone("CDT", -5*3600),  // Central Daylight Time (UTC-5)
	"mst":  time.FixedZone("MST", -7*3600),  // Mountain Standard Time (UTC-7)
	"mdt":  time.FixedZone("MDT", -6*3600),  // Mountain Daylight Time (UTC-6)
	"pst":  time.FixedZone("PST", -8*3600),  // Pacific Standard Time (UTC-8)
	"pdt":  time.FixedZone("PDT", -7*3600),  // Pacific Daylight Time (UTC-7)
	"akst": time.FixedZone("AKST", -9*3600), // Alaska Standard Time (UTC-9)
	"akdt": time.FixedZone("AKDT", -8*3600), // Alaska Daylight Time (UTC-8)
	"hst":  time.FixedZone("HST", -10*3600), // Hawaii Standard Time (UTC-10)

	// European time zones
	"gmt":  time.UTC,                       // Greenwich Mean Time (UTC+0)
	"bst":  time.FixedZone("BST", 1*3600),  // British Summer Time (UTC+1)
	"iet":  time.FixedZone("IET", 1*3600),  // Irish Standard Time (UTC+1)
	"cet":  time.FixedZone("CET", 1*3600),  // Central European Time (UTC+1)
	"cest": time.FixedZone("CEST", 2*3600), // Central European Summer Time (UTC+2)
	"eet":  time.FixedZone("EET", 2*3600),  // Eastern European Time (UTC+2)
	"eest": time.FixedZone("EEST", 3*3600), // Eastern European Summer Time (UTC+3)

	// Australian time zones — use fixed offsets so abbreviations are preserved
	"awst": time.FixedZone("AWST", 8*3600),       // Australian Western Standard Time (UTC+8)
	"acst": time.FixedZone("ACST", 9*3600+30*60), // Australian Central Standard Time (UTC+9:30)
	"aest": time.FixedZone("AEST", 10*3600),      // Australian Eastern Standard Time (UTC+10)
	"aedt": time.FixedZone("AEDT", 11*3600),      // Australian Eastern Daylight Time (UTC+11)

	// Asian time zones — use fixed offsets so abbreviations are preserved
	"jst": time.FixedZone("JST", 9*3600),       // Japan Standard Time (UTC+9)
	"ct":  time.FixedZone("CT", 8*3600),        // China Standard Time (UTC+8)
	"ist": time.FixedZone("IST", 5*3600+30*60), // Indian Standard Time (UTC+5:30)

	// Other common time zones
	"utc": time.UTC, // Universal Coordinated Time
	"z":   time.UTC, // Z (Zulu time) in ISO format

	// Military single-letter timezone codes
	"a": time.FixedZone("A", 1*3600),   // UTC+1
	"b": time.FixedZone("B", 2*3600),   // UTC+2
	"c": time.FixedZone("C", 3*3600),   // UTC+3
	"d": time.FixedZone("D", 4*3600),   // UTC+4
	"e": time.FixedZone("E", 5*3600),   // UTC+5
	"f": time.FixedZone("F", 6*3600),   // UTC+6
	"g": time.FixedZone("G", 7*3600),   // UTC+7
	"h": time.FixedZone("H", 8*3600),   // UTC+8
	"i": time.FixedZone("I", 9*3600),   // UTC+9
	"k": time.FixedZone("K", 10*3600),  // UTC+10
	"l": time.FixedZone("L", 11*3600),  // UTC+11
	"m": time.FixedZone("M", 12*3600),  // UTC+12
	"n": time.FixedZone("N", -1*3600),  // UTC-1
	"o": time.FixedZone("O", -2*3600),  // UTC-2
	"p": time.FixedZone("P", -3*3600),  // UTC-3
	"q": time.FixedZone("Q", -4*3600),  // UTC-4
	"r": time.FixedZone("R", -5*3600),  // UTC-5
	"s": time.FixedZone("S", -6*3600),  // UTC-6
	"t": time.FixedZone("T", -7*3600),  // UTC-7
	"u": time.FixedZone("U", -8*3600),  // UTC-8
	"v": time.FixedZone("V", -9*3600),  // UTC-9
	"w": time.FixedZone("W", -10*3600), // UTC-10
	"x": time.FixedZone("X", -11*3600), // UTC-11
	"y": time.FixedZone("Y", -12*3600), // UTC-12
}

// Common full timezone names
var timezoneNames = map[string]string{
	// North America
	"eastern time":  "America/New_York",
	"et":            "America/New_York",
	"eastern":       "America/New_York",
	"central time":  "America/Chicago",
	"ct":            "America/Chicago",
	"central":       "America/Chicago",
	"mountain time": "America/Denver",
	"mt":            "America/Denver",
	"mountain":      "America/Denver",
	"pacific time":  "America/Los_Angeles",
	"pt":            "America/Los_Angeles",
	"pacific":       "America/Los_Angeles",
	"alaska time":   "America/Anchorage",
	"alaska":        "America/Anchorage",
	"hawaii time":   "Pacific/Honolulu",
	"hawaii":        "Pacific/Honolulu",

	// Europe
	"greenwich mean time":   "Europe/London",
	"british time":          "Europe/London",
	"british":               "Europe/London",
	"western european time": "Europe/London",
	"central european time": "Europe/Paris",
	"eastern european time": "Europe/Helsinki",

	// Australia
	"australian western time": "Australia/Perth",
	"australian central time": "Australia/Adelaide",
	"australian eastern time": "Australia/Sydney",

	// Asia
	"japan time": "Asia/Tokyo",
	"china time": "Asia/Shanghai",
	"india time": "Asia/Kolkata",
	"india":      "Asia/Kolkata",

	// Universal
	"universal time":             "UTC",
	"universal coordinated time": "UTC",
	"zulu time":                  "UTC",
	"zulu":                       "UTC",
}

// loadLocation loads a timezone location using gotz embedded data with case-insensitive matching.
func loadLocation(name string) (*time.Location, error) {
	z, err := gotz.LoadInsensitive(name)
	if err != nil {
		return nil, err
	}
	return z.Location()
}

// tryParseTimezone attempts to parse a timezone from a string
// It handles both abbreviations (PST, EST) and full names (America/New_York, Europe/Paris)
func tryParseTimezone(tzString string) (*time.Location, bool) {
	// Empty timezone strings are invalid
	if len(tzString) == 0 {
		return nil, false
	}

	// If the timezone contains invalid characters, reject it immediately
	for _, c := range tzString {
		// Valid timezone characters: alphanumeric, /, _, -, + and spaces
		if !isValidTimezoneChar(c) {
			return nil, false
		}
	}

	// Normalize to lowercase for case-insensitive matching
	tzLower := strings.ToLower(tzString)

	// Strategy 1: Check common abbreviations first (most efficient)
	if loc, found := timezoneAbbreviations[tzLower]; found {
		return loc, true
	}

	// Strategy 2: Check common full names
	if tzName, found := timezoneNames[tzLower]; found {
		loc, err := loadLocation(tzName)
		if err == nil {
			return loc, true
		}
	}

	// Strategy 3: Try direct load with original case
	if loc, err := loadLocation(tzString); err == nil {
		return loc, true
	}

	// Strategy 4: Handle region/city format with proper casing
	parts := strings.Split(tzString, "/")
	if len(parts) == 2 {
		// Check that both parts are non-empty
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return nil, false
		}

		// Convert to proper case: first letter uppercase, rest lowercase
		region := titleCase(strings.ToLower(parts[0]))
		city := titleCase(strings.ToLower(parts[1]))
		tzPropCase := region + "/" + city

		if loc, err := loadLocation(tzPropCase); err == nil {
			return loc, true
		}
	}

	// Strategy 5: Handle underscores in timezone names (like "America/New_York")
	if strings.Contains(tzString, "_") {
		// Replace underscores with spaces for proper processing
		tzNoUnderscores := strings.ReplaceAll(tzString, "_", " ")
		parts := strings.Split(tzNoUnderscores, "/")

		if len(parts) == 2 {
			// Check that both parts are non-empty
			if len(parts[0]) == 0 || len(parts[1]) == 0 {
				return nil, false
			}

			// Title case each part and replace spaces with underscore
			region := titleCase(strings.ToLower(parts[0]))
			city := titleCase(strings.ToLower(parts[1]))
			// Replace spaces back with underscores for IANA format
			city = strings.ReplaceAll(city, " ", "_")
			tzPropCase := region + "/" + city

			if loc, err := loadLocation(tzPropCase); err == nil {
				return loc, true
			}
		}
	}

	return nil, false
}

// isValidTimezoneChar checks if a character is valid in a timezone string
func isValidTimezoneChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '/' || c == '_' || c == '-' || c == '+' || c == ' '
}

// titleCase converts the first letter of each word to uppercase
func titleCase(s string) string {
	prev := ' '
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(prev) || prev == '_' || prev == '/' {
			prev = r
			return unicode.ToUpper(r)
		}
		prev = r
		return r
	}, s)
}
