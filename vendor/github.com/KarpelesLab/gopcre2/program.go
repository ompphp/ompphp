package gopcre2

import "fmt"

// Program is a compiled PCRE2 pattern ready for execution by the VM.
type Program struct {
	Inst        []Inst           // instruction array
	Start       int              // index of first instruction
	NumCapture  int              // total number of capture groups (0 = whole match)
	CapNames    []string         // capture group names by index (empty string if unnamed)
	NameToGroup map[string][]int // name -> group indices (for DUPNAMES)
	Flags       Flag             // compile-time flags

	// Optimization hints
	Prefix      string // literal prefix for fast scan (empty if none)
	AnchorStart bool   // pattern is anchored at start
	AnchorEnd   bool   // pattern is anchored at end
	MinLen      int    // minimum subject length that can match

	// Safety limits. Set via Regexp.Set*Limit() methods. Inline pattern
	// directives like (*LIMIT_MATCH=N) are only honored when the
	// AllowInlineLimits flag is passed at compile time.
	MatchLimit int // 0 = use default
	DepthLimit int // 0 = use default
	HeapLimit  int // 0 = use default

	// Subroutine entry points: group index -> instruction index
	GroupEntry map[int]int
	GroupEnd   map[int]int
}

// Dump returns a human-readable representation of the program for debugging.
func (p *Program) Dump() string {
	var s string
	for i, inst := range p.Inst {
		s += fmt.Sprintf("%4d: %s\n", i, instString(inst))
	}
	return s
}

// opNames maps opcodes to their display names for debugging.
var opNames = map[Opcode]string{
	OpMatch: "Match", OpFail: "Fail", OpNop: "Nop",
	OpRune: "Rune", OpRuneFold: "RuneFold", OpCharClass: "CharClass",
	OpAnyCharNotNL: "AnyCharNotNL", OpAnyChar: "AnyChar", OpCharType: "CharType",
	OpSplit: "Split", OpSplitLazy: "SplitLazy", OpJump: "Jump",
	OpCaptureStart: "CaptureStart", OpCaptureEnd: "CaptureEnd",
	OpAssertBeginLine: "AssertBeginLine", OpAssertEndLine: "AssertEndLine",
	OpAssertBeginText: "AssertBeginText", OpAssertEndText: "AssertEndText",
	OpAssertEndTextOrNewline: "AssertEndTextOpt",
	OpAssertWordBoundary:     "AssertWordBoundary", OpAssertNonWordBoundary: "AssertNonWordBoundary",
	OpAssertStartOfMatch: "AssertStartOfMatch",
	OpBackref:            "Backref", OpBackrefFold: "BackrefFold",
	OpResetMatchStart: "ResetMatchStart",
	OpLookaheadStart:  "LookaheadStart", OpLookaheadEnd: "LookaheadEnd",
	OpNegLookaheadStart: "NegLookaheadStart", OpNegLookaheadEnd: "NegLookaheadEnd",
	OpLookbehindStart: "LookbehindStart", OpLookbehindEnd: "LookbehindEnd",
	OpNegLookbehindStart: "NegLookbehindStart", OpNegLookbehindEnd: "NegLookbehindEnd",
	OpAtomicStart: "AtomicStart", OpAtomicEnd: "AtomicEnd",
	OpRecurse: "Recurse", OpSubroutineCall: "SubroutineCall", OpSubroutineReturn: "SubroutineReturn",
	OpAccept: "Accept", OpVerbFail: "VerbFail",
	OpCommit: "Commit", OpPrune: "Prune", OpSkip: "Skip", OpSkipName: "SkipName",
	OpThen: "Then", OpMark: "Mark", OpCallout: "Callout",
	OpProperty: "Property", OpPropertyNeg: "PropertyNeg", OpSetFlags: "SetFlags",
}

func instString(inst Inst) string {
	name := opNames[inst.Op]
	if name == "" {
		name = fmt.Sprintf("Op(%d)", inst.Op)
	}
	switch inst.Op {
	case OpMatch, OpFail, OpAccept, OpVerbFail:
		return name
	case OpRune, OpRuneFold:
		return fmt.Sprintf("%s %q -> %d", name, string(inst.Runes), inst.Out)
	case OpCharClass:
		neg := ""
		if inst.Negate {
			neg = "^"
		}
		return fmt.Sprintf("CharClass %s%v -> %d", neg, inst.Runes, inst.Out)
	case OpSplit, OpSplitLazy:
		return fmt.Sprintf("%s -> %d, %d", name, inst.Out, inst.Arg)
	case OpCaptureStart, OpCaptureEnd, OpCharType, OpBackref, OpBackrefFold:
		return fmt.Sprintf("%s(%d) -> %d", name, inst.N, inst.Out)
	default:
		return fmt.Sprintf("%s -> %d", name, inst.Out)
	}
}
