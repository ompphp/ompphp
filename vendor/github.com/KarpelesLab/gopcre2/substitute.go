package gopcre2

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Substitute performs PCRE2-style substitution. It matches the pattern against
// the subject and replaces the first match (or all matches with SubstituteAll
// flag) using the PCRE2 replacement syntax.
//
// Replacement syntax:
//   - $0 or $& : entire match
//   - $1..$n   : numbered capture group
//   - ${name}  : named capture group
//   - $$       : literal $
//   - \U       : uppercase following text until \E
//   - \L       : lowercase following text until \E
//   - \u       : uppercase next character
//   - \l       : lowercase next character
//   - \E       : end case conversion
func (re *Regexp) Substitute(subject, replacement string) (string, error) {
	v := re.findMatch(subject, 0)
	if v == nil {
		return subject, nil
	}

	var buf strings.Builder
	matchStart := v.captures[0]
	matchEnd := v.captures[1]

	buf.WriteString(subject[:matchStart])
	expanded := re.expandPCRE2Repl(replacement, subject, v.captures)
	buf.WriteString(expanded)
	buf.WriteString(subject[matchEnd:])

	return buf.String(), nil
}

// SubstituteAll replaces all matches using PCRE2 replacement syntax.
func (re *Regexp) SubstituteAll(subject, replacement string) (string, error) {
	var buf strings.Builder
	pos := 0
	prevMatch := -1

	for {
		v := re.findMatch(subject, pos)
		if v == nil {
			break
		}
		matchStart := v.captures[0]
		matchEnd := v.captures[1]

		buf.WriteString(subject[pos:matchStart])
		buf.WriteString(re.expandPCRE2Repl(replacement, subject, v.captures))

		if matchEnd == matchStart {
			if matchEnd == prevMatch {
				if pos >= len(subject) {
					break
				}
				_, size := utf8.DecodeRuneInString(subject[pos:])
				buf.WriteString(subject[pos : pos+size])
				pos += size
				continue
			}
			pos = matchEnd
		} else {
			pos = matchEnd
		}
		prevMatch = matchEnd
	}
	buf.WriteString(subject[pos:])
	return buf.String(), nil
}

type caseMode uint8

const (
	caseNone      caseMode = iota
	caseUpper              // \U — uppercase all
	caseLower              // \L — lowercase all
	caseUpperNext          // \u — uppercase next char only
	caseLowerNext          // \l — lowercase next char only
)

// expandPCRE2Repl expands a PCRE2-style replacement string.
func (re *Regexp) expandPCRE2Repl(repl, src string, caps []int) string {
	var buf strings.Builder
	mode := caseNone

	for i := 0; i < len(repl); i++ {
		ch := repl[i]

		// Handle backslash escapes for case conversion
		if ch == '\\' && i+1 < len(repl) {
			next := repl[i+1]
			switch next {
			case 'U':
				mode = caseUpper
				i++
				continue
			case 'L':
				mode = caseLower
				i++
				continue
			case 'u':
				mode = caseUpperNext
				i++
				continue
			case 'l':
				mode = caseLowerNext
				i++
				continue
			case 'E':
				mode = caseNone
				i++
				continue
			case '\\':
				writeWithCase(&buf, '\\', &mode)
				i++
				continue
			case 'n':
				writeWithCase(&buf, '\n', &mode)
				i++
				continue
			case 't':
				writeWithCase(&buf, '\t', &mode)
				i++
				continue
			}
		}

		if ch == '$' {
			i++
			if i >= len(repl) {
				writeWithCase(&buf, '$', &mode)
				break
			}
			next := repl[i]
			switch {
			case next == '$':
				writeWithCase(&buf, '$', &mode)
			case next == '&' || next == '0':
				start, end := captureSlot(caps, 0)
				if start >= 0 && end >= 0 {
					writeStringWithCase(&buf, src[start:end], &mode)
				}
			case next >= '1' && next <= '9':
				num := int(next - '0')
				for i+1 < len(repl) && repl[i+1] >= '0' && repl[i+1] <= '9' {
					i++
					num = num*10 + int(repl[i]-'0')
				}
				start, end := captureSlot(caps, num)
				if start >= 0 && end >= 0 {
					writeStringWithCase(&buf, src[start:end], &mode)
				}
			case next == '{':
				closeBrace := strings.IndexByte(repl[i:], '}')
				if closeBrace < 0 {
					writeWithCase(&buf, '$', &mode)
					writeWithCase(&buf, '{', &mode)
					continue
				}
				ref := repl[i+1 : i+closeBrace]
				i += closeBrace

				// Check for conditional: ${n:+set:unset}
				if colonIdx := strings.IndexByte(ref, ':'); colonIdx >= 0 {
					groupRef := ref[:colonIdx]
					rest := ref[colonIdx+1:]
					groupNum := re.resolveGroupRef(groupRef)
					if groupNum >= 0 {
						start, end := captureSlot(caps, groupNum)
						isSet := start >= 0 && end >= 0
						if len(rest) > 0 && rest[0] == '+' {
							// ${n:+set:unset}
							parts := strings.SplitN(rest[1:], ":", 2)
							if isSet {
								writeStringWithCase(&buf, parts[0], &mode)
							} else if len(parts) > 1 {
								writeStringWithCase(&buf, parts[1], &mode)
							}
						} else if len(rest) > 0 && rest[0] == '-' {
							// ${n:-default}
							if isSet {
								writeStringWithCase(&buf, src[start:end], &mode)
							} else {
								writeStringWithCase(&buf, rest[1:], &mode)
							}
						}
						continue
					}
				}

				// Simple reference ${n} or ${name}
				groupNum := re.resolveGroupRef(ref)
				if groupNum >= 0 {
					start, end := captureSlot(caps, groupNum)
					if start >= 0 && end >= 0 {
						writeStringWithCase(&buf, src[start:end], &mode)
					}
				}
			default:
				writeWithCase(&buf, '$', &mode)
				writeWithCase(&buf, rune(next), &mode)
			}
			continue
		}

		r, size := utf8.DecodeRuneInString(repl[i:])
		writeWithCase(&buf, r, &mode)
		i += size - 1
	}
	return buf.String()
}

func (re *Regexp) resolveGroupRef(ref string) int {
	if num, err := strconv.Atoi(ref); err == nil {
		return num
	}
	return re.SubexpIndex(ref)
}

func writeWithCase(buf *strings.Builder, r rune, mode *caseMode) {
	switch *mode {
	case caseUpper:
		buf.WriteRune(unicode.ToUpper(r))
	case caseLower:
		buf.WriteRune(unicode.ToLower(r))
	case caseUpperNext:
		buf.WriteRune(unicode.ToUpper(r))
		*mode = caseNone
	case caseLowerNext:
		buf.WriteRune(unicode.ToLower(r))
		*mode = caseNone
	default:
		buf.WriteRune(r)
	}
}

func writeStringWithCase(buf *strings.Builder, s string, mode *caseMode) {
	for _, r := range s {
		writeWithCase(buf, r, mode)
	}
}
