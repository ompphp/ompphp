package gopcre2

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// ReplaceAllString returns a copy of src, replacing matches of the pattern
// with the replacement string repl. Inside repl, $ signs are interpreted
// as in regexp.Regexp.ReplaceAllString: $1, ${name}, etc.
func (re *Regexp) ReplaceAllString(src, repl string) string {
	var buf strings.Builder
	pos := 0
	prevMatch := -1

	for {
		v := re.findMatch(src, pos)
		if v == nil {
			break
		}
		matchStart := v.captures[0]
		matchEnd := v.captures[1]

		// Append text before the match
		buf.WriteString(src[pos:matchStart])

		// Expand the replacement template
		buf.WriteString(re.expandRepl(repl, src, v.captures))

		if matchEnd == matchStart {
			if matchEnd == prevMatch {
				if pos >= len(src) {
					break
				}
				_, size := utf8.DecodeRuneInString(src[pos:])
				buf.WriteString(src[pos : pos+size])
				pos += size
				continue
			}
			pos = matchEnd
		} else {
			pos = matchEnd
		}
		prevMatch = matchEnd
	}
	buf.WriteString(src[pos:])
	return buf.String()
}

// ReplaceAll returns a copy of src, replacing matches with repl.
func (re *Regexp) ReplaceAll(src, repl []byte) []byte {
	return []byte(re.ReplaceAllString(string(src), string(repl)))
}

// ReplaceAllStringFunc returns a copy of src in which all matches have been
// replaced by the return value of the function repl.
func (re *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	var buf strings.Builder
	pos := 0
	prevMatch := -1

	for {
		v := re.findMatch(src, pos)
		if v == nil {
			break
		}
		matchStart := v.captures[0]
		matchEnd := v.captures[1]

		buf.WriteString(src[pos:matchStart])
		buf.WriteString(repl(src[matchStart:matchEnd]))

		if matchEnd == matchStart {
			if matchEnd == prevMatch {
				if pos >= len(src) {
					break
				}
				_, size := utf8.DecodeRuneInString(src[pos:])
				buf.WriteString(src[pos : pos+size])
				pos += size
				continue
			}
			pos = matchEnd
		} else {
			pos = matchEnd
		}
		prevMatch = matchEnd
	}
	buf.WriteString(src[pos:])
	return buf.String()
}

// ReplaceAllFunc returns a copy of src in which all matches have been
// replaced by the return value of the function repl.
func (re *Regexp) ReplaceAllFunc(src []byte, repl func([]byte) []byte) []byte {
	return []byte(re.ReplaceAllStringFunc(string(src), func(s string) string {
		return string(repl([]byte(s)))
	}))
}

// expandRepl expands a replacement string template with capture group values.
func (re *Regexp) expandRepl(repl, src string, caps []int) string {
	var buf strings.Builder
	for i := 0; i < len(repl); i++ {
		if repl[i] != '$' {
			buf.WriteByte(repl[i])
			continue
		}
		i++
		if i >= len(repl) {
			buf.WriteByte('$')
			break
		}
		ch := repl[i]
		switch {
		case ch == '$':
			buf.WriteByte('$')
		case ch >= '1' && ch <= '9':
			// Numeric capture reference
			num := int(ch - '0')
			for i+1 < len(repl) && repl[i+1] >= '0' && repl[i+1] <= '9' {
				i++
				num = num*10 + int(repl[i]-'0')
			}
			start, end := captureSlot(caps, num)
			if start >= 0 && end >= 0 {
				buf.WriteString(src[start:end])
			}
		case ch == '{':
			// Named or numbered reference ${name} or ${1}
			end := strings.IndexByte(repl[i:], '}')
			if end < 0 {
				buf.WriteByte('$')
				buf.WriteByte('{')
				continue
			}
			ref := repl[i+1 : i+end]
			i += end
			// Try as number first
			if num, err := strconv.Atoi(ref); err == nil {
				start, end := captureSlot(caps, num)
				if start >= 0 && end >= 0 {
					buf.WriteString(src[start:end])
				}
			} else {
				// Named reference
				idx := re.SubexpIndex(ref)
				if idx > 0 {
					start, end := captureSlot(caps, idx)
					if start >= 0 && end >= 0 {
						buf.WriteString(src[start:end])
					}
				}
			}
		case ch == '0':
			// Whole match
			start, end := captureSlot(caps, 0)
			if start >= 0 && end >= 0 {
				buf.WriteString(src[start:end])
			}
		default:
			buf.WriteByte('$')
			buf.WriteByte(ch)
		}
	}
	return buf.String()
}

func captureSlot(caps []int, n int) (int, int) {
	idx := n * 2
	if idx+1 >= len(caps) {
		return -1, -1
	}
	return caps[idx], caps[idx+1]
}

// Split slices s into substrings separated by the pattern and returns a slice.
func (re *Regexp) Split(s string, n int) []string {
	if n == 0 {
		return nil
	}

	matches := re.FindAllStringIndex(s, n-1)
	if matches == nil {
		return []string{s}
	}

	result := make([]string, 0, len(matches)+1)
	pos := 0
	for _, match := range matches {
		result = append(result, s[pos:match[0]])
		pos = match[1]
	}
	result = append(result, s[pos:])
	return result
}
