# gopcre2

[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/gopcre2.svg)](https://pkg.go.dev/github.com/KarpelesLab/gopcre2)
[![Test](https://github.com/KarpelesLab/gopcre2/actions/workflows/test.yml/badge.svg)](https://github.com/KarpelesLab/gopcre2/actions/workflows/test.yml)
[![Coverage Status](https://coveralls.io/repos/github/KarpelesLab/gopcre2/badge.svg?branch=master)](https://coveralls.io/github/KarpelesLab/gopcre2?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/KarpelesLab/gopcre2)](https://goreportcard.com/report/github.com/KarpelesLab/gopcre2)

A pure Go implementation of [PCRE2](https://www.pcre.org/) (Perl Compatible Regular Expressions).
No cgo, no external dependencies.

## Why?

Go's standard `regexp` package uses the RE2 algorithm, which guarantees linear-time
matching but deliberately omits advanced features like backreferences, lookahead/lookbehind
assertions, atomic groups, and recursive patterns. This package provides those features
with an API similar to the standard library.

## Features

- **Backreferences**: `\1`, `\g{-1}`, `\k<name>`, `(?P=name)`
- **Lookahead**: `(?=...)`, `(?!...)`
- **Lookbehind**: `(?<=...)`, `(?<!...)` (fixed and variable length)
- **Atomic groups**: `(?>...)` — prevents backtracking
- **Possessive quantifiers**: `*+`, `++`, `?+`, `{n,m}+`
- **Named captures**: `(?<name>...)`, `(?P<name>...)`, `(?'name'...)`
- **Recursive patterns**: `(?R)`, `(?1)`, `(?&name)`
- **Conditional patterns**: `(?(1)yes|no)`, `(?(R)...)`, `(?(DEFINE)...)`
- **Branch reset**: `(?|...)`
- **Backtracking control verbs**: `(*ACCEPT)`, `(*FAIL)`, `(*COMMIT)`, `(*PRUNE)`, `(*SKIP)`, `(*THEN)`, `(*MARK:NAME)`
- **Unicode properties**: `\p{Lu}`, `\p{Greek}`, `\P{N}`, etc.
- **POSIX classes**: `[:alpha:]`, `[:digit:]`, etc.
- **Character types**: `\d`, `\w`, `\s`, `\h`, `\v` (and negations)
- **Inline options**: `(?i)`, `(?m)`, `(?s)`, `(?x)`, `(?U)`, `(?J)`, `(?i:...)`
- **Match point reset**: `\K`
- **Callouts**: `(?C)`, `(?Cn)`, `(?C"text")`
- **PCRE2-style substitution**: `\U`, `\L`, `\u`, `\l`, `\E` case conversion
- **All standard quantifiers**: `*`, `+`, `?`, `{n}`, `{n,}`, `{n,m}` (greedy and lazy)
- **All anchors**: `^`, `$`, `\A`, `\z`, `\Z`, `\G`, `\b`, `\B`
- **Escape sequences**: `\xhh`, `\x{hhhh}`, `\o{ooo}`, `\cX`, `\n`, `\r`, `\t`, `\f`, `\a`, `\e`

## Installation

```bash
go get github.com/KarpelesLab/gopcre2
```

## Usage

```go
package main

import (
    "fmt"
    "github.com/KarpelesLab/gopcre2"
)

func main() {
    // Basic matching
    re := gopcre2.MustCompile(`(\w+)\s(\w+)`)
    fmt.Println(re.MatchString("hello world")) // true

    // Submatch extraction
    match := re.FindStringSubmatch("hello world")
    fmt.Println(match) // ["hello world", "hello", "world"]

    // Named captures
    re2 := gopcre2.MustCompile(`(?P<first>\w+)\s(?P<last>\w+)`)
    m := re2.FindStringSubmatch("John Doe")
    fmt.Println(m[re2.SubexpIndex("first")]) // "John"
    fmt.Println(m[re2.SubexpIndex("last")])  // "Doe"

    // Lookahead
    re3 := gopcre2.MustCompile(`\w+(?=\.)`)
    fmt.Println(re3.FindString("foo.bar")) // "foo"

    // Backreference
    re4 := gopcre2.MustCompile(`(["'])(.*?)\1`)
    fmt.Println(re4.FindString(`"hello"`)) // "hello"

    // Case-insensitive
    re5 := gopcre2.MustCompile(`abc`, gopcre2.Caseless)
    fmt.Println(re5.MatchString("ABC")) // true

    // Replace
    re6 := gopcre2.MustCompile(`(\w+)@(\w+)`)
    fmt.Println(re6.ReplaceAllString("user@host", "$2@$1")) // "host@user"

    // PCRE2-style substitution with case conversion
    re7 := gopcre2.MustCompile(`(\w+)`)
    result, _ := re7.Substitute("hello", `\U$1`)
    fmt.Println(result) // "HELLO"
}
```

## Security Warning: Denial of Service (ReDoS)

**This package uses a backtracking regex engine that can exhibit exponential runtime
on certain patterns.** Unlike Go's standard `regexp` package (which guarantees linear
time), PCRE2-compatible features like backreferences and lookaround require backtracking,
which means adversarial patterns can cause extremely long match times.

### Vulnerable patterns

Patterns with nested quantifiers are particularly dangerous:

```
(a+)+b          — exponential on "aaaaaaaaac"
(a|a)*b         — exponential on "aaaaaaaaac"
(.*a){10}       — exponential on long strings without 'a'
```

### Configurable limits

**Always set limits when processing untrusted patterns or input:**

```go
re := gopcre2.MustCompile(pattern)

// Limit backtracking steps (default: 10,000,000)
re.SetMatchLimit(100000)

// Limit recursion depth (default: 250)
re.SetDepthLimit(50)

// Limit heap memory for backtrack stack (default: 20 MB)
re.SetHeapLimit(1024 * 1024) // 1 MB

// When limits are exceeded, matching returns false
// and the limit error is available internally
```

By default, inline limit directives (`(*LIMIT_MATCH=N)` etc.) in patterns are
**ignored**. If an attacker can control the pattern, they could use inline directives
to override the safety limits you set. If you trust your patterns and want to allow
them to set their own limits, pass the `AllowInlineLimits` flag at compile time:

```go
re := gopcre2.MustCompile(`(*LIMIT_MATCH=5000)pattern`, gopcre2.AllowInlineLimits)
```

Without this flag, limits can only be configured via the `Set*Limit()` API methods.

### Recommendations

| Use case | Recommendation |
|----------|---------------|
| **Trusted patterns, trusted input** | Use freely |
| **Trusted patterns, untrusted input** | Set `MatchLimit` and `HeapLimit` |
| **Untrusted patterns** | Set all three limits to conservative values |
| **Need linear-time guarantees** | Use Go's standard `regexp` package instead |

## API Reference

The API mirrors Go's standard `regexp` package where possible:

### Compilation

```go
func Compile(pattern string, flags ...Flag) (*Regexp, error)
func MustCompile(pattern string, flags ...Flag) *Regexp
```

### Matching

```go
func (re *Regexp) MatchString(s string) bool
func (re *Regexp) Match(b []byte) bool
func (re *Regexp) FindString(s string) string
func (re *Regexp) FindStringIndex(s string) []int
func (re *Regexp) FindStringSubmatch(s string) []string
func (re *Regexp) FindStringSubmatchIndex(s string) []int
func (re *Regexp) FindAllString(s string, n int) []string
func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string
// ... and []byte variants of all the above
```

### Replacement

```go
func (re *Regexp) ReplaceAllString(src, repl string) string
func (re *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string
func (re *Regexp) Substitute(subject, replacement string) (string, error)
func (re *Regexp) SubstituteAll(subject, replacement string) (string, error)
func (re *Regexp) Split(s string, n int) []string
```

### Metadata

```go
func (re *Regexp) String() string
func (re *Regexp) NumSubexp() int
func (re *Regexp) SubexpNames() []string
func (re *Regexp) SubexpIndex(name string) int
```

### Limits

```go
func (re *Regexp) SetMatchLimit(n int) *Regexp
func (re *Regexp) SetDepthLimit(n int) *Regexp
func (re *Regexp) SetHeapLimit(bytes int) *Regexp
```

### Callouts

```go
func (re *Regexp) SetCallout(fn CalloutFunc)
```

## Compile Flags

| Flag | Description |
|------|-------------|
| `Caseless` | Case-insensitive matching (`(?i)`) |
| `Multiline` | `^` and `$` match at newlines (`(?m)`) |
| `DotAll` | `.` matches newline (`(?s)`) |
| `Extended` | Ignore whitespace and `#` comments (`(?x)`) |
| `Ungreedy` | Invert quantifier greediness (`(?U)`) |
| `UTF` | Treat pattern/subject as UTF-8 |
| `UCP` | Use Unicode properties for `\d`, `\w`, `\s` |
| `DupNames` | Allow duplicate subpattern names (`(?J)`) |
| `NoAutoCapture` | Plain `()` are non-capturing (`(?n)`) |
| `Anchored` | Force match at start of subject |
| `DollarEndOnly` | `$` matches only at end, not before final newline |
| `AllowInlineLimits` | Honor `(*LIMIT_MATCH=N)` etc. from patterns (off by default for safety) |

## License

MIT License - see [LICENSE](LICENSE) file.
