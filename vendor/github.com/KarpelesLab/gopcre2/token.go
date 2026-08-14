package gopcre2

// TokenKind identifies the type of a lexer token.
type TokenKind uint16

const (
	// Literals and special chars
	TokLiteral    TokenKind = iota // a literal rune
	TokDot                         // .
	TokEscapeSeq                   // resolved escape: \n, \t, \xhh, etc.
	TokCharType                    // \d, \D, \w, \W, \s, \S, \h, \H, \v, \V
	TokProperty                    // \p{...}, \P{...}
	TokBackslashK                  // \K match point reset

	// Quantifiers
	TokStar       // *
	TokPlus       // +
	TokQuestion   // ?
	TokRepeat     // {n}, {n,}, {n,m}
	TokLazy       // ? after quantifier
	TokPossessive // + after quantifier

	// Anchors
	TokCaret  // ^
	TokDollar // $
	TokAnchor // \A, \z, \Z, \G, \b, \B

	// Groups
	TokGroupOpen     // (  capturing
	TokGroupClose    // )
	TokNonCapture    // (?:
	TokNamedCapture  // (?<name>, (?P<name>, (?'name'
	TokAtomicGroup   // (?>
	TokLookahead     // (?=
	TokNegLookahead  // (?!
	TokLookbehind    // (?<=
	TokNegLookbehind // (?<!
	TokBranchReset   // (?|
	TokComment       // (?#...)
	TokInlineOption  // (?imsx-imsx) or (?imsx-imsx:
	TokConditional   // (?(

	// Alternation
	TokAlternate // |

	// Backreferences
	TokBackref // \1, \g{1}, \k<name>, etc.

	// Subroutines / Recursion
	TokRecurse    // (?R), (?0)
	TokSubroutine // (?1), (?&name), etc.

	// Control verbs
	TokVerb // (*ACCEPT), (*FAIL), (*COMMIT), etc.

	// Callout
	TokCallout // (?C), (?Cn), (?C"text")

	// Character class
	TokCharClassOpen  // [
	TokCharClassClose // ]
	TokCharClassNeg   // [^
	TokCharClassRange // - inside class
	TokPOSIXClass     // [:alpha:] etc.

	TokEOF
)

// AnchorKind specifies which anchor token this is.
type AnchorKind uint8

const (
	AnchorBeginText    AnchorKind = iota // \A
	AnchorEndText                        // \z
	AnchorEndTextOpt                     // \Z (before optional final newline)
	AnchorWordBoundary                   // \b
	AnchorNonWord                        // \B
	AnchorStartOfMatch                   // \G
)

// CharTypeKind identifies which character type shorthand this is.
type CharTypeKind uint8

const (
	CharTypeDigit     CharTypeKind = iota // \d
	CharTypeNonDigit                      // \D
	CharTypeWord                          // \w
	CharTypeNonWord                       // \W
	CharTypeSpace                         // \s
	CharTypeNonSpace                      // \S
	CharTypeHSpace                        // \h
	CharTypeNonHSpace                     // \H
	CharTypeVSpace                        // \v
	CharTypeNonVSpace                     // \V
)

// VerbKind identifies a backtracking control verb.
type VerbKind uint8

const (
	VerbUnknown VerbKind = iota // unrecognized or limit directive
	VerbAccept
	VerbFail
	VerbCommit
	VerbPrune
	VerbSkip
	VerbSkipName
	VerbThen
	VerbMark
)

// Token represents a single token produced by the lexer.
type Token struct {
	Kind       TokenKind
	Pos        int          // byte offset in pattern
	Literal    rune         // resolved rune for TokLiteral, TokEscapeSeq
	Str        string       // name for named captures, backrefs, verbs, properties
	Num        int          // repeat count, backref number, callout number
	Num2       int          // second repeat count (for {n,m})
	Negate     bool         // negation flag (\P{}, [^...])
	AnchorType AnchorKind   // for TokAnchor
	CharType   CharTypeKind // for TokCharType
	VerbType   VerbKind     // for TokVerb
	Flags      Flag         // for TokInlineOption: flags to set
	UnsetFlags Flag         // for TokInlineOption: flags to clear
	HasColon   bool         // for TokInlineOption: (?i:...) scoped form
}
