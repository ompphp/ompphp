# gotz

[![Tests](https://github.com/KarpelesLab/gotz/actions/workflows/test.yml/badge.svg)](https://github.com/KarpelesLab/gotz/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/gotz.svg)](https://pkg.go.dev/github.com/KarpelesLab/gotz)

Go package that parses IANA TZif timezone files and exposes raw timezone data that Go's `time.Location` keeps private — transitions, zone types, POSIX TZ rules, and leap second records.

Timezone data is compiled from the [official IANA source](https://data.iana.org/time-zones/releases/) and embedded in the package, so there is no dependency on the host system's timezone files.

## Install

```bash
go get github.com/KarpelesLab/gotz
```

## Usage

```go
z, err := gotz.Load("America/New_York")
if err != nil {
    log.Fatal(err)
}

// Inspect zone types (EST, EDT, etc.)
for _, zt := range z.Types() {
    fmt.Printf("%s  offset=%d  dst=%v\n", zt.Abbrev, zt.Offset, zt.IsDST)
}

// Iterate historical transitions
for _, t := range z.Transitions() {
    zt := z.Types()[t.Type]
    fmt.Printf("%s  -> %s\n", time.Unix(t.When, 0).UTC(), zt.Abbrev)
}

// Look up the active zone at a specific time
zt := z.Lookup(time.Now())
fmt.Printf("Current zone: %s (UTC%+d)\n", zt.Abbrev, zt.Offset/3600)

// Get the POSIX TZ rule for future transitions
if rule := z.Extend(); rule != nil {
    start, end, _ := rule.TransitionsForYear(2025)
    fmt.Printf("DST starts: %s\n", time.Unix(start, 0).UTC())
    fmt.Printf("DST ends:   %s\n", time.Unix(end, 0).UTC())
}

// Country and coordinates metadata
if m := z.Meta(); m != nil {
    fmt.Printf("Country: %s (%s)\n", m.Countries[0].Name, m.Countries[0].Code)
    fmt.Printf("Location: %.4f, %.4f\n", m.Lat, m.Lon)
}

// Convert to *time.Location for use with the standard library
loc, err := z.Location()
fmt.Println(time.Now().In(loc))
```

## API

### Loading

| Function | Description |
|---|---|
| `Load(name string) (*Zone, error)` | Load by IANA name (cached) |
| `Parse(name string, data []byte) (*Zone, error)` | Parse raw TZif binary data |

### Zone

| Method | Description |
|---|---|
| `Name() string` | IANA timezone name |
| `Version() int` | TZif format version (1–4) |
| `Types() []ZoneType` | Zone type definitions (abbreviation, offset, DST flag) |
| `Transitions() []Transition` | Historical transition records (from TZif file only) |
| `TransitionsForRange(start, end time.Time) []Transition` | All transitions in range, including generated from POSIX rule |
| `LeapSeconds() []LeapSecond` | Leap second records |
| `Extend() *PosixTZ` | Parsed POSIX TZ rule for future transitions |
| `ExtendRaw() string` | Raw POSIX TZ footer string |
| `Lookup(time.Time) ZoneType` | Zone type active at a given time |
| `Location() (*time.Location, error)` | Convert to `*time.Location` |
| `Meta() *ZoneMeta` | Country codes/names, lat/lon, commentary (from zone1970.tab) |

### PosixTZ

| Method | Description |
|---|---|
| `HasDST() bool` | Whether DST is defined |
| `Lookup(unix int64) (abbrev, offset, isDST)` | Zone in effect at a Unix timestamp |
| `TransitionsForYear(year int) (start, end int64, ok bool)` | DST transition times for a year |
| `String() string` | POSIX TZ string representation |

## Updating timezone data

Run the update script to download and compile the latest IANA timezone data:

```bash
./update.sh
```

This downloads `tzcode` and `tzdata` from `data.iana.org`, compiles them with `zic`, and packages the result into `zoneinfo.zip`. Edit the `CODE` and `DATA` variables at the top of the script to target a specific release.

## License

MIT — see [LICENSE](LICENSE).
