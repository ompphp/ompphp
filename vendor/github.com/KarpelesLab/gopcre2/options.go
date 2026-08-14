package gopcre2

// Flag represents PCRE2 compile and match options.
type Flag uint64

const (
	Caseless          Flag = 1 << iota // (?i) case-insensitive matching
	Multiline                          // (?m) ^ and $ match at newlines
	DotAll                             // (?s) . matches newline
	Extended                           // (?x) ignore whitespace and # comments
	ExtendedMore                       // (?xx) extended + ignore space/tab in char classes
	Anchored                           // force match at start of subject
	EndAnchored                        // force match at end of subject
	DollarEndOnly                      // $ matches only at end, not before final newline
	Ungreedy                           // (?U) invert greediness of quantifiers
	UTF                                // treat pattern and subject as UTF-8
	UCP                                // use Unicode properties for \d, \w, \s
	DupNames                           // (?J) allow duplicate subpattern names
	NoAutoCapture                      // (?n) plain () are non-capturing
	FirstLine                          // match must complete before first newline
	Literal                            // treat pattern as literal string
	NewlineCR                          // \r is newline
	NewlineLF                          // \n is newline (default)
	NewlineCRLF                        // \r\n is newline
	NewlineAny                         // any Unicode newline
	NewlineAnyCRLF                     // \r, \n, or \r\n
	NewlineNUL                         // \0 is newline
	BSRAnyCRLF                         // \R matches CR, LF, CRLF only
	BSRUnicode                         // \R matches any Unicode newline
	NoAutoPos                          // disable auto-possessification
	NoDotStarAnchor                    // disable .* anchoring optimization
	NoStartOptimize                    // disable start-of-match optimizations
	MatchUnsetBackref                  // unset backreferences match empty string
	AllowInlineLimits                  // honor (*LIMIT_MATCH=N) etc. from patterns (see security note)
)

const (
	// DefaultMatchLimit is the default maximum number of backtracking steps.
	DefaultMatchLimit = 10_000_000

	// DefaultDepthLimit is the default maximum recursion/subroutine depth.
	DefaultDepthLimit = 250

	// DefaultHeapLimit is the default maximum heap memory for the backtrack stack (bytes).
	DefaultHeapLimit = 20 * 1024 * 1024 // 20 MB
)
