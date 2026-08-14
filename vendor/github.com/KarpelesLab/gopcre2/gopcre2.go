// Package gopcre2 provides a pure Go implementation of the PCRE2 (Perl Compatible
// Regular Expressions) library. It supports advanced features not available in
// Go's standard regexp package, including backreferences, lookahead/lookbehind
// assertions, atomic groups, possessive quantifiers, recursive patterns, and
// backtracking control verbs.
//
// # Security Warning
//
// Unlike Go's standard regexp package (which guarantees linear-time matching),
// this package uses a backtracking engine that can exhibit exponential runtime
// on certain patterns. This creates the possibility of Regular Expression Denial
// of Service (ReDoS) attacks.
//
// Always use configurable match limits (SetMatchLimit, SetDepthLimit, SetHeapLimit)
// when processing untrusted patterns or input. See the README for details.
package gopcre2

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Regexp is a compiled PCRE2 regular expression. It is safe for concurrent use
// by multiple goroutines (matching does not modify the Regexp).
type Regexp struct {
	expr       string // original pattern
	prog       *Program
	flags      Flag
	matchLimit int
	depthLimit int
	heapLimit  int
	callout    CalloutFunc
}

// Compile parses a PCRE2 regular expression and returns, if successful,
// a Regexp object that can be used to match against text.
func Compile(pattern string, flags ...Flag) (*Regexp, error) {
	var f Flag
	for _, fl := range flags {
		f |= fl
	}
	return compile(pattern, f)
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
func MustCompile(pattern string, flags ...Flag) *Regexp {
	re, err := Compile(pattern, flags...)
	if err != nil {
		panic(fmt.Sprintf("gopcre2: Compile(%q): %v", pattern, err))
	}
	return re
}

func compile(pattern string, flags Flag) (*Regexp, error) {
	lex := newLexer(pattern, flags)
	tokens, err := lex.tokenize()
	if err != nil {
		return nil, err
	}

	p := newParser(tokens, flags)
	ast, err := p.Parse()
	if err != nil {
		return nil, err
	}

	c := newCompiler(flags)
	prog, err := c.Compile(ast)
	if err != nil {
		return nil, err
	}

	optimize(prog)

	return &Regexp{
		expr:  pattern,
		prog:  prog,
		flags: flags,
	}, nil
}

// SetMatchLimit sets the maximum number of backtracking steps before a match
// attempt is aborted with ErrMatchLimit. Default: 10,000,000.
// Returns the Regexp for chaining.
func (re *Regexp) SetMatchLimit(n int) *Regexp {
	re.matchLimit = n
	return re
}

// SetDepthLimit sets the maximum recursion/subroutine nesting depth.
// Default: 250. Returns the Regexp for chaining.
func (re *Regexp) SetDepthLimit(n int) *Regexp {
	re.depthLimit = n
	return re
}

// SetHeapLimit sets the maximum heap memory (in bytes) for the backtrack stack.
// Default: 20 MB. Returns the Regexp for chaining.
func (re *Regexp) SetHeapLimit(bytes int) *Regexp {
	re.heapLimit = bytes
	return re
}

// SetCallout registers a callout function that is invoked at (?C...) points
// in the pattern during matching.
func (re *Regexp) SetCallout(fn CalloutFunc) {
	re.callout = fn
}

// String returns the source text used to compile the regular expression.
func (re *Regexp) String() string {
	return re.expr
}

// NumSubexp returns the number of parenthesized subexpressions in the pattern.
func (re *Regexp) NumSubexp() int {
	return re.prog.NumCapture
}

// SubexpNames returns the names of the parenthesized subexpressions.
// The name for the first sub-expression is names[1], so that names[i]
// is the name for the i'th subexpression.
func (re *Regexp) SubexpNames() []string {
	return re.prog.CapNames
}

// SubexpIndex returns the index of the first subexpression with the given name,
// or -1 if there is no subexpression with that name.
func (re *Regexp) SubexpIndex(name string) int {
	groups, ok := re.prog.NameToGroup[name]
	if !ok || len(groups) == 0 {
		return -1
	}
	return groups[0]
}

// newMatchVM creates a VM for matching.
func (re *Regexp) newMatchVM(subject string, startPos int) *vm {
	v := newVM(re.prog, subject, startPos)
	// Apply API-set limits. If both API and inline (from pattern) limits are
	// set, use the lower (more restrictive) value so that inline limits can
	// tighten but never loosen the caller's limits.
	if re.matchLimit > 0 {
		if v.matchLimit == 0 || re.matchLimit < v.matchLimit {
			v.matchLimit = re.matchLimit
		}
	}
	if re.depthLimit > 0 {
		if v.depthLimit == 0 || re.depthLimit < v.depthLimit {
			v.depthLimit = re.depthLimit
		}
	}
	if re.heapLimit > 0 {
		if v.heapLimit == 0 || re.heapLimit < v.heapLimit {
			v.heapLimit = re.heapLimit
		}
	}
	v.callout = re.callout
	return v
}

// execAt tries to match starting at position startPos.
func (re *Regexp) execAt(subject string, startPos int) *vm {
	v := re.newMatchVM(subject, startPos)
	if v.exec() {
		return v
	}
	return nil
}

// findMatch searches for the first match in subject starting from startPos.
func (re *Regexp) findMatch(subject string, startPos int) *vm {
	// If pattern is anchored, only try at startPos
	if re.prog.AnchorStart {
		return re.execAt(subject, startPos)
	}

	// Fast path: if we have a literal prefix, use strings.Index to skip ahead
	if re.prog.Prefix != "" && len(re.prog.Prefix) > 1 {
		i := startPos
		for {
			idx := strings.Index(subject[i:], re.prog.Prefix)
			if idx < 0 {
				return nil
			}
			pos := i + idx
			v := re.newMatchVM(subject, pos)
			v.startPos = startPos
			if v.exec() {
				return v
			}
			if pos >= len(subject) {
				break
			}
			_, size := utf8.DecodeRuneInString(subject[pos:])
			i = pos + size
		}
		return nil
	}

	for i := startPos; i <= len(subject); {
		v := re.newMatchVM(subject, i)
		v.startPos = startPos // \G anchors to the search start, not the attempt position
		if v.exec() {
			return v
		}
		if i >= len(subject) {
			break
		}
		_, size := utf8.DecodeRuneInString(subject[i:])
		i += size
	}
	return nil
}

// Dump returns a human-readable representation of the compiled program.
func (re *Regexp) Dump() string {
	return re.prog.Dump()
}
