# strtotime

[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/strtotime.svg)](https://pkg.go.dev/github.com/KarpelesLab/strtotime)
[![CI](https://github.com/KarpelesLab/strtotime/actions/workflows/ci.yml/badge.svg)](https://github.com/KarpelesLab/strtotime/actions/workflows/ci.yml)
[![Coverage Status](https://coveralls.io/repos/github/KarpelesLab/strtotime/badge.svg?branch=master)](https://coveralls.io/github/KarpelesLab/strtotime?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/KarpelesLab/strtotime)](https://goreportcard.com/report/github.com/KarpelesLab/strtotime)

A Go library that parses human-readable date/time strings into `time.Time` objects, inspired by PHP's [strtotime()](https://www.php.net/manual/en/function.strtotime.php) function.

## Features

* Parses natural language date/time expressions
* Supports relative dates ("tomorrow", "next Friday", "+2 days")
* Handles various date formats (ISO, US, European)
* Custom base time reference support
* Timezone specification (3-letter codes and IANA timezone names)
* Extensible architecture

## Installation

```bash
go get github.com/KarpelesLab/strtotime
```

## Usage

### Basic Usage

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/KarpelesLab/strtotime"
)

func main() {
    // Parse a simple date string
    t, err := strtotime.StrToTime("2023-05-15")
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    fmt.Printf("Date: %s\n", t.Format("2006-01-02 15:04:05"))
    
    // Parse a relative date using the current time as reference
    t, err = strtotime.StrToTime("tomorrow")
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    fmt.Printf("Tomorrow: %s\n", t.Format("2006-01-02"))
    
    // Parse a more complex expression
    t, err = strtotime.StrToTime("next Friday +2 weeks")
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    fmt.Printf("Next Friday +2 weeks: %s\n", t.Format("2006-01-02"))
}
```

### Using Options

You can customize parsing behavior with options:

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/KarpelesLab/strtotime"
)

func main() {
    // Set a specific base time for relative calculations
    baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
    t, err := strtotime.StrToTime("next month", strtotime.Rel(baseTime))
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    fmt.Printf("Next month from 2023-01-01: %s\n", t.Format("2006-01-02"))
    
    // Specify a timezone as an option
    loc, _ := time.LoadLocation("America/New_York")
    t, err = strtotime.StrToTime("today", strtotime.InTZ(loc))
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    fmt.Printf("Today in New York: %s\n", t.Format("2006-01-02 15:04:05 MST"))
    
    // Or specify timezone in the string
    t, err = strtotime.StrToTime("January 1 2023 EST")
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    fmt.Printf("Date with timezone: %s\n", t.Format("2006-01-02 15:04:05 MST"))
    
    // Combine multiple options
    t, err = strtotime.StrToTime("tomorrow", strtotime.Rel(baseTime), strtotime.InTZ(loc))
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    fmt.Printf("Tomorrow from 2023-01-01 in New York: %s\n", t.Format("2006-01-02 15:04:05 MST"))
}
```

### PHP-Compatible Date Parsing (`DateParse`)

`DateParse(str)` returns a `*ParsedDate` describing exactly which components
were present in the input — equivalent to PHP's `date_parse()`. Unlike
`StrToTime`, it does not fold relative offsets into a `time.Time`; it
reports them in a nested `relative` block.

```go
pd := strtotime.DateParse("2006-12-12T10:00:00.5+01:00")
b, _ := json.Marshal(pd)
fmt.Println(string(b))
// {"year":2006,"month":12,"day":12,"hour":10,"minute":0,"second":0,
//  "fraction":0.5,"warning_count":0,"warnings":[],"error_count":0,
//  "errors":[],"is_localtime":true,"zone_type":1,"zone":3600,
//  "is_dst":false}
```

Unset scalar fields marshal as `false`, matching PHP. Timezone metadata
follows PHP's `zone_type`: `1` for UTC offsets, `2` for abbreviations, `3`
for IANA identifiers. The `relative` block is included only when the input
contained relative offsets (`+3 days`, `next monday`, …). You can
materialize a `ParsedDate` back into a `time.Time` with `pd.Time(loc)` or
`pd.Materialize(now, loc)`.

## Supported Date/Time Formats

The library can understand many different formats and expressions, including:

### Simple Words
- `now` - current time
- `today` - current date at midnight
- `tomorrow` - tomorrow at midnight
- `yesterday` - yesterday at midnight

### Relative Dates
- `next Monday`, `last Friday` - next/last occurrence of a weekday
- `next week`, `last week` - next/last Monday
- `next month`, `last month` - same day next/last month
- `next year`, `last year` - same day next/last year

### Relative Time Adjustments
- `+1 day`, `-2 days` - add/subtract specific time units
- `+1 week`, `-3 weeks` - with various time units (day, week, month, year, hour, minute, second)
- `4 days` - implicit positive adjustment (same as +4 days)

### Date Formats
- ISO format: `2023-05-15`
- Slash format: `2023/05/15`
- US format: `05/15/2023`
- European format: `15.05.2023`
- With timezone: `January 1 2023 EST`, `June 1 1985 16:30:00 Europe/Paris`

### Month Names
- Full names: `January 15 2023`
- Abbreviated: `Jan 15, 2023`
- With/without commas: `Jan 15 2023`
- With ordinal suffixes: `April 4th`
- Month only: `January` (first day of the month in current year)

### Compound Expressions
- `next year + 4 days`
- `next month - 1 week`
- `next week + 3 days`
- `tomorrow + 12 hours`
- Complex combinations: `next year + 1 month + 1 week`

## Time Unit Handling

The library recognizes various formats for time units:
- Standard forms: day, week, month, year, hour, minute, second
- Plural forms: days, weeks, months, years, hours, minutes, seconds
- Abbreviations: d, w, wk, m, y, yr, h, hr, min, sec
- Common variations: hrs, mon, mins, secs

## Timezone Support

The library supports multiple timezone formats:
- 3-letter abbreviations: `EST`, `PST`, `GMT`, `UTC`, etc.
- IANA timezone names: `America/New_York`, `Europe/Paris`, `Asia/Tokyo`, etc.
- Timezone can be specified in the string: `January 1 2023 EST`, `June 1 1985 16:30:00 Europe/Paris`
- Timezone can also be provided as an option: `strtotime.InTZ(loc)`

## Error Handling

The library returns detailed error messages when it fails to parse a string:

```go
t, err := strtotime.StrToTime("invalid date format")
if err != nil {
    fmt.Printf("Error: %s\n", err)
    // Handle the error
}
```

## License

This library is available under the [LICENSE](LICENSE) included in the repository.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.