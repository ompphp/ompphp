package gopcre2

import "unicode/utf8"

// MatchData holds the result of a match operation.
type MatchData struct {
	subject string
	ovector []int  // [start0, end0, start1, end1, ...]
	mark    string // last (*MARK:NAME) value
}

// CalloutFunc is the type for callout callback functions.
// Return 0 to continue matching, non-zero to force backtracking.
type CalloutFunc func(cb *CalloutBlock) int

// CalloutBlock contains information passed to a callout function.
type CalloutBlock struct {
	Version         int
	CalloutNumber   int
	CalloutString   string
	Subject         string
	CurrentPosition int
	CaptureTop      int
	CaptureLast     int
	StartMatch      int
	PatternPosition int
}

// --- Boolean matching ---

// MatchString reports whether the string s contains any match of the pattern.
func (re *Regexp) MatchString(s string) bool {
	return re.findMatch(s, 0) != nil
}

// Match reports whether the byte slice b contains any match of the pattern.
func (re *Regexp) Match(b []byte) bool {
	return re.MatchString(string(b))
}

// --- Find (first match, return text) ---

// FindString returns the text of the leftmost match in s.
// If there is no match, the return value is an empty string.
func (re *Regexp) FindString(s string) string {
	v := re.findMatch(s, 0)
	if v == nil {
		return ""
	}
	return s[v.captures[0]:v.captures[1]]
}

// Find returns the byte slice of the leftmost match in b.
func (re *Regexp) Find(b []byte) []byte {
	s := string(b)
	v := re.findMatch(s, 0)
	if v == nil {
		return nil
	}
	return b[v.captures[0]:v.captures[1]]
}

// --- FindIndex (first match, return position) ---

// FindStringIndex returns a two-element slice of integers defining the
// location of the leftmost match in s. Returns nil if no match.
func (re *Regexp) FindStringIndex(s string) []int {
	v := re.findMatch(s, 0)
	if v == nil {
		return nil
	}
	return []int{v.captures[0], v.captures[1]}
}

// FindIndex returns a two-element slice of integers defining the location
// of the leftmost match of the pattern in b.
func (re *Regexp) FindIndex(b []byte) []int {
	return re.FindStringIndex(string(b))
}

// --- FindSubmatch (first match with subgroups) ---

// FindStringSubmatch returns a slice of strings holding the text of the
// leftmost match and its submatches. A nil return indicates no match.
func (re *Regexp) FindStringSubmatch(s string) []string {
	v := re.findMatch(s, 0)
	if v == nil {
		return nil
	}
	n := re.prog.NumCapture + 1
	result := make([]string, n)
	for i := 0; i < n; i++ {
		start := v.captures[i*2]
		end := v.captures[i*2+1]
		if start >= 0 && end >= 0 {
			result[i] = s[start:end]
		}
	}
	return result
}

// FindSubmatch returns a slice of byte slices holding the text of the
// leftmost match and its submatches.
func (re *Regexp) FindSubmatch(b []byte) [][]byte {
	s := string(b)
	v := re.findMatch(s, 0)
	if v == nil {
		return nil
	}
	n := re.prog.NumCapture + 1
	result := make([][]byte, n)
	for i := 0; i < n; i++ {
		start := v.captures[i*2]
		end := v.captures[i*2+1]
		if start >= 0 && end >= 0 {
			result[i] = b[start:end]
		}
	}
	return result
}

// --- FindSubmatchIndex (first match, positions with subgroups) ---

// FindStringSubmatchIndex returns a slice of integers identifying the
// location of the leftmost match and its submatches.
func (re *Regexp) FindStringSubmatchIndex(s string) []int {
	v := re.findMatch(s, 0)
	if v == nil {
		return nil
	}
	n := (re.prog.NumCapture + 1) * 2
	result := make([]int, n)
	copy(result, v.captures[:n])
	return result
}

// FindSubmatchIndex returns a slice of integers identifying the location
// of the leftmost match and its submatches in b.
func (re *Regexp) FindSubmatchIndex(b []byte) []int {
	return re.FindStringSubmatchIndex(string(b))
}

// --- FindAll (all matches) ---

// FindAllString returns a slice of all successive matches of the pattern.
// n limits the number of matches; -1 means all.
func (re *Regexp) FindAllString(s string, n int) []string {
	if n == 0 {
		return nil
	}
	var result []string
	pos := 0
	prevMatch := -1
	for {
		if n >= 0 && len(result) >= n {
			break
		}
		v := re.findMatch(s, pos)
		if v == nil {
			break
		}
		matchStart := v.captures[0]
		matchEnd := v.captures[1]

		result = append(result, s[matchStart:matchEnd])

		// Advance position
		if matchEnd == matchStart {
			// Empty match — advance by one rune to avoid infinite loop
			if matchEnd == prevMatch {
				if pos >= len(s) {
					break
				}
				_, size := utf8.DecodeRuneInString(s[pos:])
				pos += size
				continue
			}
			pos = matchEnd
		} else {
			pos = matchEnd
		}
		prevMatch = matchEnd
	}
	return result
}

// FindAll returns a slice of all successive matches of the pattern in b.
func (re *Regexp) FindAll(b []byte, n int) [][]byte {
	s := string(b)
	matches := re.FindAllString(s, n)
	if matches == nil {
		return nil
	}
	result := make([][]byte, len(matches))
	for i, m := range matches {
		result[i] = []byte(m)
	}
	return result
}

// --- FindAllIndex ---

// FindAllStringIndex returns a slice of all successive match indices.
func (re *Regexp) FindAllStringIndex(s string, n int) [][]int {
	if n == 0 {
		return nil
	}
	var result [][]int
	pos := 0
	prevMatch := -1
	for {
		if n >= 0 && len(result) >= n {
			break
		}
		v := re.findMatch(s, pos)
		if v == nil {
			break
		}
		matchStart := v.captures[0]
		matchEnd := v.captures[1]

		result = append(result, []int{matchStart, matchEnd})

		if matchEnd == matchStart {
			if matchEnd == prevMatch {
				if pos >= len(s) {
					break
				}
				_, size := utf8.DecodeRuneInString(s[pos:])
				pos += size
				continue
			}
			pos = matchEnd
		} else {
			pos = matchEnd
		}
		prevMatch = matchEnd
	}
	return result
}

// FindAllIndex returns a slice of all successive match indices in b.
func (re *Regexp) FindAllIndex(b []byte, n int) [][]int {
	return re.FindAllStringIndex(string(b), n)
}

// --- FindAllSubmatch ---

// FindAllStringSubmatch returns a slice of all successive matches with subgroups.
func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	if n == 0 {
		return nil
	}
	var result [][]string
	pos := 0
	prevMatch := -1
	for {
		if n >= 0 && len(result) >= n {
			break
		}
		v := re.findMatch(s, pos)
		if v == nil {
			break
		}
		matchStart := v.captures[0]
		matchEnd := v.captures[1]

		numGroups := re.prog.NumCapture + 1
		submatch := make([]string, numGroups)
		for i := 0; i < numGroups; i++ {
			start := v.captures[i*2]
			end := v.captures[i*2+1]
			if start >= 0 && end >= 0 {
				submatch[i] = s[start:end]
			}
		}
		result = append(result, submatch)

		if matchEnd == matchStart {
			if matchEnd == prevMatch {
				if pos >= len(s) {
					break
				}
				_, size := utf8.DecodeRuneInString(s[pos:])
				pos += size
				continue
			}
			pos = matchEnd
		} else {
			pos = matchEnd
		}
		prevMatch = matchEnd
	}
	return result
}

// FindAllSubmatch returns a slice of all successive matches with subgroups in b.
func (re *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	s := string(b)
	matches := re.FindAllStringSubmatch(s, n)
	if matches == nil {
		return nil
	}
	result := make([][][]byte, len(matches))
	for i, match := range matches {
		sub := make([][]byte, len(match))
		for j, s := range match {
			if s != "" {
				sub[j] = []byte(s)
			}
		}
		result[i] = sub
	}
	return result
}

// --- FindAllSubmatchIndex ---

// FindAllStringSubmatchIndex returns a slice of all successive match index pairs.
func (re *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	if n == 0 {
		return nil
	}
	var result [][]int
	pos := 0
	prevMatch := -1
	for {
		if n >= 0 && len(result) >= n {
			break
		}
		v := re.findMatch(s, pos)
		if v == nil {
			break
		}
		matchStart := v.captures[0]
		matchEnd := v.captures[1]

		numSlots := (re.prog.NumCapture + 1) * 2
		indices := make([]int, numSlots)
		copy(indices, v.captures[:numSlots])
		result = append(result, indices)

		if matchEnd == matchStart {
			if matchEnd == prevMatch {
				if pos >= len(s) {
					break
				}
				_, size := utf8.DecodeRuneInString(s[pos:])
				pos += size
				continue
			}
			pos = matchEnd
		} else {
			pos = matchEnd
		}
		prevMatch = matchEnd
	}
	return result
}

// FindAllSubmatchIndex returns a slice of all match index pairs in b.
func (re *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
	return re.FindAllStringSubmatchIndex(string(b), n)
}
