package gotz

import (
	"math"
	"strings"
	"sync"
)

// ZoneMeta contains metadata about a timezone from zone1970.tab and iso3166.tab.
type ZoneMeta struct {
	Countries  []Country // countries that overlap this timezone
	Lat        float64   // latitude of principal location (degrees, north positive)
	Lon        float64   // longitude of principal location (degrees, east positive)
	Commentary string    // optional commentary (e.g., region description)
}

// Country represents an ISO 3166 country associated with a timezone.
type Country struct {
	Code string // ISO 3166-1 alpha-2 code (e.g., "US")
	Name string // country name (e.g., "United States")
}

var (
	metaOnce sync.Once
	metaMap  map[string]*ZoneMeta // keyed by IANA timezone name
	isoMap   map[string]string    // country code -> country name
)

func loadMeta() {
	metaOnce.Do(func() {
		metaMap = make(map[string]*ZoneMeta)
		isoMap = make(map[string]string)

		// Parse iso3166.tab.
		if data, err := readFromZip("iso3166.tab"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" || line[0] == '#' {
					continue
				}
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) == 2 {
					isoMap[parts[0]] = parts[1]
				}
			}
		}

		// Parse zone1970.tab.
		if data, err := readFromZip("zone1970.tab"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" || line[0] == '#' {
					continue
				}
				fields := strings.Split(line, "\t")
				if len(fields) < 3 {
					continue
				}

				codes := strings.Split(fields[0], ",")
				lat, lon := parseISO6709(fields[1])
				name := fields[2]
				var commentary string
				if len(fields) >= 4 {
					commentary = fields[3]
				}

				countries := make([]Country, len(codes))
				for i, code := range codes {
					countries[i] = Country{Code: code, Name: isoMap[code]}
				}

				metaMap[name] = &ZoneMeta{
					Countries:  countries,
					Lat:        lat,
					Lon:        lon,
					Commentary: commentary,
				}
			}
		}
	})
}

// Meta returns metadata (countries, coordinates) for this timezone,
// or nil if no metadata is available.
func (z *Zone) Meta() *ZoneMeta {
	loadMeta()
	return metaMap[z.name]
}

// parseISO6709 parses coordinates in ISO 6709 format:
// ±DDMM±DDDMM or ±DDMMSS±DDDMMSS
func parseISO6709(s string) (lat, lon float64) {
	// Find the sign character that starts the longitude part.
	// The latitude starts at index 0; the longitude starts at the second +/- sign.
	lonStart := -1
	for i := 1; i < len(s); i++ {
		if s[i] == '+' || s[i] == '-' {
			lonStart = i
			break
		}
	}
	if lonStart < 0 {
		return 0, 0
	}

	lat = parseDMS(s[:lonStart], 2)
	lon = parseDMS(s[lonStart:], 3)
	return lat, lon
}

// parseDMS parses a ±DD[D]MM[SS] string into decimal degrees.
// degDigits is the number of digits in the degrees part (2 for lat, 3 for lon).
func parseDMS(s string, degDigits int) float64 {
	if len(s) < 1+degDigits+2 {
		return 0
	}

	neg := s[0] == '-'
	s = s[1:] // skip sign

	deg := atoi(s[:degDigits])
	s = s[degDigits:]

	min := atoi(s[:2])
	s = s[2:]

	var sec int
	if len(s) >= 2 {
		sec = atoi(s[:2])
	}

	v := float64(deg) + float64(min)/60 + float64(sec)/3600
	// Round to 4 decimal places to avoid floating point noise.
	v = math.Round(v*10000) / 10000
	if neg {
		v = -v
	}
	return v
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
