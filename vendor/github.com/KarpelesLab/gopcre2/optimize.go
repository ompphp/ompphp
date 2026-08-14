package gopcre2

import "strings"

// optimize applies post-compilation optimizations to the program.
func optimize(prog *Program) {
	detectAnchor(prog)
	extractPrefix(prog)
	computeMinLen(prog)
}

// detectAnchor checks if the pattern is anchored at the start.
func detectAnchor(prog *Program) {
	if len(prog.Inst) == 0 {
		return
	}
	// Walk from start looking for the first consuming instruction
	for pc := prog.Start; pc < len(prog.Inst); pc++ {
		inst := &prog.Inst[pc]
		switch inst.Op {
		case OpCaptureStart, OpCaptureEnd, OpNop, OpSetFlags:
			continue
		case OpAssertBeginText:
			prog.AnchorStart = true
			return
		case OpAssertBeginLine:
			// ^ without multiline acts like \A
			if prog.Flags&Multiline == 0 {
				prog.AnchorStart = true
			}
			return
		default:
			return
		}
	}
}

// extractPrefix extracts a literal prefix from the start of the pattern.
func extractPrefix(prog *Program) {
	var prefix strings.Builder
	for pc := prog.Start; pc < len(prog.Inst); pc++ {
		inst := &prog.Inst[pc]
		switch inst.Op {
		case OpCaptureStart, OpCaptureEnd, OpNop, OpSetFlags,
			OpAssertBeginLine, OpAssertBeginText:
			continue
		case OpRune:
			if len(inst.Runes) == 1 {
				prefix.WriteRune(inst.Runes[0])
				continue
			}
			goto done
		default:
			goto done
		}
	}
done:
	prog.Prefix = prefix.String()
}

// computeMinLen computes the minimum subject length for any match.
func computeMinLen(prog *Program) {
	// Simple: count minimum consuming instructions from start to match
	minLen := 0
	for pc := prog.Start; pc < len(prog.Inst); pc++ {
		inst := &prog.Inst[pc]
		switch inst.Op {
		case OpRune, OpRuneFold, OpAnyChar, OpAnyCharNotNL,
			OpCharClass, OpCharType, OpProperty, OpPropertyNeg:
			minLen++
		case OpMatch:
			prog.MinLen = minLen
			return
		case OpSplit, OpSplitLazy:
			// Can't easily compute across branches; stop
			prog.MinLen = minLen
			return
		}
	}
	prog.MinLen = minLen
}
