package gopcre2

// Opcode represents a VM instruction opcode.
type Opcode uint8

const (
	// Basic control
	OpMatch Opcode = iota // successful match
	OpFail                // fail current path
	OpNop                 // no-op

	// Character matching
	OpRune         // match single rune (Runes[0])
	OpRuneFold     // match rune with case folding (Runes[0])
	OpCharClass    // match against character class (ranges in Runes, Negate flag)
	OpAnyCharNotNL // match any char except newline
	OpAnyChar      // match any char including newline (DOTALL)
	OpCharType     // match character type \d, \w, etc. (N = CharTypeKind)

	// Branching
	OpSplit     // greedy split: try Out first, then Arg
	OpSplitLazy // lazy split: try Arg first, then Out
	OpJump      // unconditional jump to Out

	// Capture
	OpCaptureStart // begin capture group (N = group index)
	OpCaptureEnd   // end capture group (N = group index)

	// Zero-width assertions
	OpAssertBeginLine        // ^
	OpAssertEndLine          // $
	OpAssertBeginText        // \A
	OpAssertEndText          // \z
	OpAssertEndTextOrNewline // \Z
	OpAssertWordBoundary     // \b
	OpAssertNonWordBoundary  // \B
	OpAssertStartOfMatch     // \G

	// Lookaround
	OpLookaheadStart     // begin positive lookahead
	OpLookaheadEnd       // end positive lookahead
	OpNegLookaheadStart  // begin negative lookahead
	OpNegLookaheadEnd    // end negative lookahead
	OpLookbehindStart    // begin positive lookbehind (N = min len, N2 = max len)
	OpLookbehindEnd      // end positive lookbehind
	OpNegLookbehindStart // begin negative lookbehind
	OpNegLookbehindEnd   // end negative lookbehind

	// Atomic group
	OpAtomicStart // begin atomic group (push cut point)
	OpAtomicEnd   // end atomic group (discard backtrack entries)

	// Backreference
	OpBackref     // match text of capture group N (case-sensitive)
	OpBackrefFold // match text of capture group N (case-insensitive)

	// Recursion / Subroutines
	OpRecurse          // recurse entire pattern
	OpSubroutineCall   // call subroutine group N
	OpSubroutineReturn // return from subroutine

	// Match point reset
	OpResetMatchStart // \K

	// Backtracking control verbs
	OpAccept   // (*ACCEPT)
	OpVerbFail // (*FAIL)
	OpCommit   // (*COMMIT)
	OpPrune    // (*PRUNE)
	OpSkip     // (*SKIP)
	OpSkipName // (*SKIP:NAME)
	OpThen     // (*THEN)
	OpMark     // (*MARK:NAME)

	// Callout
	OpCallout // invoke callout (N = number, Str = string)

	// Unicode property
	OpProperty    // match \p{Name}
	OpPropertyNeg // match \P{Name}

	// Inline option change
	OpSetFlags // change flags (N = set bits, N2 = unset bits)
)

// Inst is a single VM instruction.
type Inst struct {
	Op     Opcode
	Out    uint32 // primary branch / next instruction
	Arg    uint32 // secondary branch target
	N      int    // numeric operand (group index, char type, etc.)
	N2     int    // second numeric operand
	Runes  []rune // for OpRune, OpRuneFold: single rune; for OpCharClass: range pairs [lo,hi,lo,hi,...]
	Negate bool   // for OpCharClass: negated class
	Str    string // for names, property names, mark names, callout strings
}
