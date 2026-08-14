package gopcre2

// NodeKind identifies the type of an AST node.
type NodeKind uint8

const (
	NdLiteral            NodeKind = iota // single literal rune
	NdDot                                // . (any char)
	NdCharClass                          // character class [...]
	NdConcat                             // concatenation of children
	NdAlternate                          // alternation (|)
	NdCapture                            // capturing group
	NdNonCapture                         // non-capturing group (?:...)
	NdNamedCapture                       // named capturing group
	NdBranchReset                        // branch reset group (?|...)
	NdRepeat                             // quantifier: *, +, ?, {n,m}
	NdAnchorBeginLine                    // ^
	NdAnchorEndLine                      // $
	NdAnchorBeginText                    // \A
	NdAnchorEndText                      // \z
	NdAnchorEndTextOpt                   // \Z
	NdAnchorStartOfMatch                 // \G
	NdWordBoundary                       // \b
	NdNonWordBoundary                    // \B
	NdBackref                            // backreference \1, \k<name>
	NdLookahead                          // (?=...)
	NdNegLookahead                       // (?!...)
	NdLookbehind                         // (?<=...)
	NdNegLookbehind                      // (?<!...)
	NdAtomicGroup                        // (?>...)
	NdConditional                        // (?(cond)yes|no)
	NdRecursion                          // (?R)
	NdSubroutineCall                     // (?1), (?&name)
	NdCharType                           // \d, \w, \s, etc.
	NdProperty                           // \p{...}, \P{...}
	NdMatchPointReset                    // \K
	NdVerb                               // backtracking control verb
	NdCallout                            // (?C...)
	NdInlineOption                       // (?imsx-imsx)
	NdEmpty                              // empty expression
)

// GreedyKind specifies the greediness of a quantifier.
type GreedyKind uint8

const (
	Greedy     GreedyKind = iota // default
	Lazy                         // ? suffix
	Possessive                   // + suffix
)

// CondKind specifies the type of condition in a conditional pattern.
type CondKind uint8

const (
	CondBackref     CondKind = iota // (?(1)...) reference to capture group
	CondNamedRef                    // (?(name)...)
	CondRecursion                   // (?(R)...)
	CondRecurseNum                  // (?(R1)...)
	CondRecurseName                 // (?(R&name)...)
	CondDefine                      // (?(DEFINE)...)
	CondAssert                      // (?(assert)...) lookahead/lookbehind condition
)

// Node is a node in the AST for a PCRE2 pattern.
type Node struct {
	Kind     NodeKind
	Children []*Node

	// Data fields (used depending on Kind)
	Rune     rune         // NdLiteral
	Ranges   []RuneRange  // NdCharClass: character ranges
	Negate   bool         // NdCharClass negation, NdProperty negation
	Min, Max int          // NdRepeat: min and max (-1 = unbounded)
	Greedy   GreedyKind   // NdRepeat
	Index    int          // NdCapture: group number; NdBackref: group number; NdSubroutineCall: group number
	Name     string       // NdNamedCapture, NdBackref by name, verb name, property name
	CharType CharTypeKind // NdCharType
	VerbKind VerbKind     // NdVerb
	CondType CondKind     // NdConditional
	Flags    Flag         // NdInlineOption: flags to set
	UnFlags  Flag         // NdInlineOption: flags to clear
	CallNum  int          // NdCallout: callout number
	CallStr  string       // NdCallout: callout string
	Pos      int          // position in source pattern
}

// RuneRange represents an inclusive range of runes [Lo, Hi].
type RuneRange struct {
	Lo, Hi rune
}
