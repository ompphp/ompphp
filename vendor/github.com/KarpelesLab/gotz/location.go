package gotz

import "time"

// Location returns a *time.Location equivalent to this Zone.
// This uses time.LoadLocationFromTZData with the original TZif binary data.
func (z *Zone) Location() (*time.Location, error) {
	if z.rawData != nil {
		return time.LoadLocationFromTZData(z.name, z.rawData)
	}
	// For zones without raw data (e.g., synthetic UTC zone),
	// fall back to time.LoadLocation.
	if z.name == "UTC" || z.name == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(z.name)
}
