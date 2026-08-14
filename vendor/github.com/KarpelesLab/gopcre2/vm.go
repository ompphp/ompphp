package gopcre2

import (
	"unicode"
	"unicode/utf8"
)

// vm executes a compiled Program against a subject string.
type vm struct {
	prog       *Program
	subject    string
	pos        int   // current position in subject
	matchStart int   // where the reported match starts (\K can change this)
	startPos   int   // where the current match attempt began (\G)
	captures   []int // ovector: pairs of [start, end) per group; -1 = unset
	stack      *btStack
	flags      Flag   // runtime flags
	steps      int    // step counter for match limit
	depth      int    // current recursion depth
	mark       string // most recent (*MARK:NAME) value
	nextSubPC  uint32 // used by execOne for sub-VM stepping

	// Limits
	matchLimit int
	depthLimit int
	heapLimit  int

	// Callout
	callout CalloutFunc

	// Error from limit exceeded
	limitErr error
}

func newVM(prog *Program, subject string, startPos int) *vm {
	numSlots := (prog.NumCapture + 1) * 2
	caps := make([]int, numSlots)
	for i := range caps {
		caps[i] = -1
	}

	matchLimit := prog.MatchLimit
	if matchLimit == 0 {
		matchLimit = DefaultMatchLimit
	}
	depthLimit := prog.DepthLimit
	if depthLimit == 0 {
		depthLimit = DefaultDepthLimit
	}
	heapLimit := prog.HeapLimit
	if heapLimit == 0 {
		heapLimit = DefaultHeapLimit
	}

	return &vm{
		prog:       prog,
		subject:    subject,
		pos:        startPos,
		matchStart: startPos,
		startPos:   startPos,
		captures:   caps,
		stack:      newBTStack(),
		flags:      prog.Flags,
		matchLimit: matchLimit,
		depthLimit: depthLimit,
		heapLimit:  heapLimit,
	}
}

// exec runs the VM from the current position. Returns true if a match was found.
func (v *vm) exec() bool {
	pc := uint32(v.prog.Start)

	for {
		// Check limits
		v.steps++
		if v.steps > v.matchLimit {
			v.limitErr = ErrMatchLimit
			return false
		}
		if v.stack.heapUsed > v.heapLimit {
			v.limitErr = ErrHeapLimit
			return false
		}

		if int(pc) >= len(v.prog.Inst) {
			return false
		}
		inst := &v.prog.Inst[pc]

		switch inst.Op {
		case OpMatch:
			// Set capture 0
			v.captures[0] = v.matchStart
			v.captures[1] = v.pos
			return true

		case OpFail:
			if !v.backtrack() {
				return false
			}
			pc = v.nextPC()
			continue

		case OpNop:
			pc = inst.Out
			continue

		case OpRune:
			if v.pos >= len(v.subject) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
			if r != inst.Runes[0] {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			v.pos += size
			pc = inst.Out

		case OpRuneFold:
			if v.pos >= len(v.subject) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
			if !equalFold(r, inst.Runes[0]) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			v.pos += size
			pc = inst.Out

		case OpAnyCharNotNL:
			if v.pos >= len(v.subject) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
			if r == '\n' {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			_ = r
			v.pos += size
			pc = inst.Out

		case OpAnyChar:
			if v.pos >= len(v.subject) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			_, size := utf8.DecodeRuneInString(v.subject[v.pos:])
			v.pos += size
			pc = inst.Out

		case OpCharClass:
			if v.pos >= len(v.subject) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
			if !v.matchCharClass(r, inst) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			v.pos += size
			pc = inst.Out

		case OpCharType:
			if v.pos >= len(v.subject) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
			if !v.matchCharType(r, CharTypeKind(inst.N)) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			v.pos += size
			pc = inst.Out

		case OpProperty, OpPropertyNeg:
			if v.pos >= len(v.subject) {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
			match := matchProperty(r, inst.Str)
			if inst.Op == OpPropertyNeg {
				match = !match
			}
			if !match {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			v.pos += size
			pc = inst.Out

		case OpSplit:
			// Greedy: try Out first, save Arg as alternative
			v.stack.push(stackFrame{
				kind: frameSplit,
				pc:   inst.Arg,
				pos:  v.pos,
			})
			v.saveCapturesForSplit()
			pc = inst.Out

		case OpSplitLazy:
			// Lazy: try Out first (skip/done), save Arg as alternative (body)
			v.stack.push(stackFrame{
				kind: frameSplit,
				pc:   inst.Arg,
				pos:  v.pos,
			})
			v.saveCapturesForSplit()
			pc = inst.Out

		case OpJump:
			pc = inst.Out

		case OpCaptureStart:
			slot := inst.N * 2
			if slot < len(v.captures) {
				v.stack.push(stackFrame{
					kind:    frameCapture,
					capSlot: slot,
					capVal:  v.captures[slot],
				})
				v.captures[slot] = v.pos
			}
			pc = inst.Out

		case OpCaptureEnd:
			slot := inst.N*2 + 1
			if slot < len(v.captures) {
				v.stack.push(stackFrame{
					kind:    frameCapture,
					capSlot: slot,
					capVal:  v.captures[slot],
				})
				v.captures[slot] = v.pos
			}
			pc = inst.Out

		case OpAssertBeginLine:
			if v.pos == 0 || (v.flags&Multiline != 0 && v.pos > 0 && v.subject[v.pos-1] == '\n') {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpAssertEndLine:
			atEnd := v.pos == len(v.subject)
			beforeNL := v.pos < len(v.subject) && v.subject[v.pos] == '\n'
			matched := false
			if v.flags&Multiline != 0 {
				matched = atEnd || beforeNL
			} else if v.flags&DollarEndOnly != 0 {
				// DollarEndOnly: $ only matches at the very end
				matched = atEnd
			} else {
				// Default: $ matches at end, or before final \n at end
				matched = atEnd || (beforeNL && v.pos+1 == len(v.subject))
			}
			if matched {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpAssertBeginText:
			if v.pos == 0 {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpAssertEndText:
			if v.pos == len(v.subject) {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpAssertEndTextOrNewline:
			atEnd := v.pos == len(v.subject)
			beforeFinalNL := v.pos+1 == len(v.subject) && v.subject[v.pos] == '\n'
			if atEnd || beforeFinalNL {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpAssertWordBoundary:
			if v.atWordBoundary() {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpAssertNonWordBoundary:
			if !v.atWordBoundary() {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpAssertStartOfMatch:
			if v.pos == v.startPos {
				pc = inst.Out
			} else {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
			}

		case OpResetMatchStart:
			v.stack.push(stackFrame{
				kind:       frameMark,
				matchStart: v.matchStart,
			})
			v.matchStart = v.pos
			pc = inst.Out

		case OpBackref, OpBackrefFold:
			if inst.N*2+1 >= len(v.captures) {
				// Backreference to non-existent group
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			groupStart := v.captures[inst.N*2]
			groupEnd := v.captures[inst.N*2+1]
			if groupStart < 0 || groupEnd < 0 {
				// Unset backreference
				if v.flags&MatchUnsetBackref != 0 {
					// Match empty string
					pc = inst.Out
				} else {
					if !v.backtrack() {
						return false
					}
					pc = v.nextPC()
				}
				continue
			}
			ref := v.subject[groupStart:groupEnd]
			if inst.Op == OpBackrefFold {
				if !v.matchStringFold(ref) {
					if !v.backtrack() {
						return false
					}
					pc = v.nextPC()
					continue
				}
			} else {
				remaining := v.subject[v.pos:]
				if len(remaining) < len(ref) || remaining[:len(ref)] != ref {
					if !v.backtrack() {
						return false
					}
					pc = v.nextPC()
					continue
				}
				v.pos += len(ref)
			}
			pc = inst.Out

		case OpLookaheadStart:
			// Run sub-VM for lookahead
			endPC := v.findLookaheadEnd(pc+1, OpLookaheadEnd)
			savedPos := v.pos
			matched := v.runSubMatch(pc+1, endPC)
			v.pos = savedPos // restore position (zero-width)
			if !matched {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			pc = uint32(endPC) + 1
			// Wire to Out of the end instruction
			if int(endPC) < len(v.prog.Inst) {
				pc = v.prog.Inst[endPC].Out
			}

		case OpLookaheadEnd:
			// Should not be reached directly; handled by sub-VM
			pc = inst.Out

		case OpNegLookaheadStart:
			endPC := v.findLookaheadEnd(pc+1, OpNegLookaheadEnd)
			savedPos := v.pos
			savedCaps := make([]int, len(v.captures))
			copy(savedCaps, v.captures)
			matched := v.runSubMatch(pc+1, endPC)
			v.pos = savedPos
			copy(v.captures, savedCaps) // restore captures for neg lookahead
			if matched {
				// Inner matched → negative lookahead fails
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			pc = uint32(endPC) + 1
			if int(endPC) < len(v.prog.Inst) {
				pc = v.prog.Inst[endPC].Out
			}

		case OpNegLookaheadEnd:
			pc = inst.Out

		case OpLookbehindStart:
			minLen := inst.N
			maxLen := inst.N2
			if maxLen < 0 {
				maxLen = v.pos // can't look back further than start
			}
			if maxLen > v.pos {
				maxLen = v.pos
			}
			if minLen > v.pos {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			// Try each possible lookbehind start position from maxLen to minLen
			savedPos := v.pos
			matched := false
			for lookback := maxLen; lookback >= minLen; lookback-- {
				tryPos := v.pos - lookback
				if tryPos < len(v.subject) && tryPos > 0 && !utf8.RuneStart(v.subject[tryPos]) {
					for tryPos > 0 && !utf8.RuneStart(v.subject[tryPos]) {
						tryPos--
					}
					lookback = v.pos - tryPos
				}
				if lookback < minLen {
					break
				}
				subVM := newVM(v.prog, v.subject, tryPos)
				subVM.flags = v.flags
				subVM.captures = make([]int, len(v.captures))
				copy(subVM.captures, v.captures)
				subVM.matchLimit = v.matchLimit - v.steps
				subVM.pos = tryPos
				if v.execLookbehindBody(subVM, inst.Out, savedPos) {
					matched = true
					break
				}
			}
			if !matched {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			v.pos = savedPos
			// Skip past the lookbehind body to the LookbehindEnd
			pc = v.findLookbehindEnd(inst.Out)

		case OpLookbehindEnd:
			pc = inst.Out

		case OpNegLookbehindStart:
			minLen := inst.N
			maxLen := inst.N2
			if maxLen < 0 {
				maxLen = v.pos
			}
			if maxLen > v.pos {
				maxLen = v.pos
			}
			if minLen > v.pos {
				// Can't look back far enough, so nothing matches → neg lookbehind succeeds
				pc = v.findLookbehindEnd(inst.Out)
				continue
			}
			savedPos := v.pos
			matched := false
			for lookback := maxLen; lookback >= minLen; lookback-- {
				tryPos := v.pos - lookback
				if tryPos < len(v.subject) && tryPos > 0 && !utf8.RuneStart(v.subject[tryPos]) {
					for tryPos > 0 && !utf8.RuneStart(v.subject[tryPos]) {
						tryPos--
					}
					lookback = v.pos - tryPos
				}
				if lookback < minLen {
					break
				}
				subVM := newVM(v.prog, v.subject, tryPos)
				subVM.flags = v.flags
				subVM.captures = make([]int, len(v.captures))
				copy(subVM.captures, v.captures)
				subVM.matchLimit = v.matchLimit - v.steps
				subVM.pos = tryPos
				if v.execLookbehindBody(subVM, inst.Out, savedPos) {
					matched = true
					break
				}
			}
			if matched {
				// Neg lookbehind fails
				v.pos = savedPos
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			v.pos = savedPos
			pc = v.findLookbehindEnd(inst.Out)

		case OpNegLookbehindEnd:
			pc = inst.Out

		case OpAtomicStart:
			// Run the inner pattern as a sub-match.
			// If it succeeds, accept the result without allowing backtracking into it.
			endPC := v.findAtomicEnd(pc + 1)
			savedCaps := make([]int, len(v.captures))
			copy(savedCaps, v.captures)
			savedPos := v.pos
			matched := v.runSubMatchAtomic(pc+1, endPC)
			if !matched {
				// Atomic group failed to match — restore and backtrack
				copy(v.captures, savedCaps)
				v.pos = savedPos
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			// Atomic group matched — v.pos and v.captures are updated
			// Continue past the atomic end without any backtrack alternatives
			pc = uint32(endPC) + 1
			if int(endPC) < len(v.prog.Inst) {
				pc = v.prog.Inst[endPC].Out
			}

		case OpAtomicEnd:
			// Should not be reached directly; handled by sub-VM
			pc = inst.Out

		case OpRecurse:
			if v.depth >= v.depthLimit {
				v.limitErr = ErrDepthLimit
				return false
			}
			savedCaps := make([]int, len(v.captures))
			copy(savedCaps, v.captures)
			v.stack.push(stackFrame{
				kind:     frameRecursion,
				pc:       inst.Out,
				captures: savedCaps,
				depth:    v.depth,
			})
			v.depth++
			pc = uint32(v.prog.Start)

		case OpSubroutineCall:
			if v.depth >= v.depthLimit {
				v.limitErr = ErrDepthLimit
				return false
			}
			groupIdx := inst.N
			entry, ok := v.prog.GroupEntry[groupIdx]
			if !ok {
				if !v.backtrack() {
					return false
				}
				pc = v.nextPC()
				continue
			}
			savedCaps := make([]int, len(v.captures))
			copy(savedCaps, v.captures)
			v.stack.push(stackFrame{
				kind:     frameRecursion,
				pc:       inst.Out,
				captures: savedCaps,
				depth:    v.depth,
			})
			v.depth++
			pc = uint32(entry)

		case OpAccept:
			v.captures[0] = v.matchStart
			v.captures[1] = v.pos
			return true

		case OpVerbFail:
			if !v.backtrack() {
				return false
			}
			pc = v.nextPC()

		case OpCommit:
			v.stack.push(stackFrame{
				kind:     frameVerb,
				verbKind: VerbCommit,
			})
			pc = inst.Out

		case OpPrune:
			v.stack.push(stackFrame{
				kind:     frameVerb,
				verbKind: VerbPrune,
				pos:      v.pos,
			})
			pc = inst.Out

		case OpSkip:
			v.stack.push(stackFrame{
				kind:     frameVerb,
				verbKind: VerbSkip,
				pos:      v.pos,
			})
			pc = inst.Out

		case OpSkipName:
			v.stack.push(stackFrame{
				kind:     frameVerb,
				verbKind: VerbSkipName,
				markName: inst.Str,
			})
			pc = inst.Out

		case OpThen:
			v.stack.push(stackFrame{
				kind:     frameVerb,
				verbKind: VerbThen,
			})
			pc = inst.Out

		case OpMark:
			v.mark = inst.Str
			v.stack.push(stackFrame{
				kind:     frameMark,
				markName: inst.Str,
				markPos:  v.pos,
			})
			pc = inst.Out

		case OpCallout:
			if v.callout != nil {
				cb := &CalloutBlock{
					CalloutNumber:   inst.N,
					CalloutString:   inst.Str,
					Subject:         v.subject,
					CurrentPosition: v.pos,
					StartMatch:      v.matchStart,
					PatternPosition: int(pc),
				}
				result := v.callout(cb)
				if result != 0 {
					if !v.backtrack() {
						return false
					}
					pc = v.nextPC()
					continue
				}
			}
			pc = inst.Out

		case OpSetFlags:
			v.stack.push(stackFrame{
				kind:  frameFlagSave,
				flags: v.flags,
			})
			v.flags = Flag(inst.N) | (v.flags &^ Flag(inst.N2))
			pc = inst.Out

		default:
			// Unknown opcode — fail
			if !v.backtrack() {
				return false
			}
			pc = v.nextPC()
		}
	}
}

// nextPC returns the PC from the last popped frame (used after backtrack).
// This is a helper — the actual PC is stored in the vm after backtrack.
var btPC uint32

func (v *vm) nextPC() uint32 {
	return btPC
}

// backtrack pops the stack looking for a split frame to resume from.
func (v *vm) backtrack() bool {
	for {
		f, ok := v.stack.pop()
		if !ok {
			return false
		}
		switch f.kind {
		case frameSplit:
			v.pos = f.pos
			btPC = f.pc
			return true
		case frameCapture:
			v.captures[f.capSlot] = f.capVal
		case frameLookaround:
			v.pos = f.pos
			v.matchStart = f.matchStart
			if f.captures != nil {
				copy(v.captures, f.captures)
			}
		case frameAtomic:
			// Backtracking past an atomic group — continue popping
			continue
		case frameRecursion:
			v.depth = f.depth
			copy(v.captures, f.captures)
		case frameVerb:
			switch f.verbKind {
			case VerbCommit:
				// Fail the entire match
				return false
			case VerbPrune:
				// Advance start and fail this attempt
				return false
			case VerbSkip:
				return false
			case VerbThen:
				// Continue backtracking to next alternation split
				continue
			}
		case frameMark:
			if f.matchStart != 0 {
				v.matchStart = f.matchStart
			}
		case frameFlagSave:
			v.flags = f.flags
		}
	}
}

// saveCapturesForSplit saves capture state onto the stack so it can be
// restored if we backtrack to this split point.
func (v *vm) saveCapturesForSplit() {
	// We rely on individual capture save/restore frames rather than
	// snapshotting the entire capture array for performance.
}

func (v *vm) matchCharClass(r rune, inst *Inst) bool {
	matched := false
	runes := inst.Runes
	caseFold := v.flags&Caseless != 0

	// Check rune ranges (pairs: lo, hi, lo, hi, ...)
	for i := 0; i+1 < len(runes); i += 2 {
		lo, hi := runes[i], runes[i+1]
		if r >= lo && r <= hi {
			matched = true
			break
		}
		if caseFold {
			for fr := unicode.SimpleFold(r); fr != r; fr = unicode.SimpleFold(fr) {
				if fr >= lo && fr <= hi {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
	}

	// Check properties if present (stored as semicolon-separated names in Str)
	if !matched && inst.Str != "" {
		for _, propName := range splitProperties(inst.Str) {
			negate := false
			name := propName
			if len(name) > 0 && name[0] == '^' {
				negate = true
				name = name[1:]
			}
			m := matchProperty(r, name)
			if negate {
				m = !m
			}
			if m {
				matched = true
				break
			}
		}
	}

	if inst.Negate {
		return !matched
	}
	return matched
}

func splitProperties(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ';' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func (v *vm) matchCharType(r rune, ct CharTypeKind) bool {
	ucp := v.flags&UCP != 0
	switch ct {
	case CharTypeDigit:
		if ucp {
			return unicode.IsDigit(r)
		}
		return r >= '0' && r <= '9'
	case CharTypeNonDigit:
		if ucp {
			return !unicode.IsDigit(r)
		}
		return !(r >= '0' && r <= '9')
	case CharTypeWord:
		return isWordChar(r, ucp)
	case CharTypeNonWord:
		return !isWordChar(r, ucp)
	case CharTypeSpace:
		if ucp {
			return unicode.IsSpace(r)
		}
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
	case CharTypeNonSpace:
		if ucp {
			return !unicode.IsSpace(r)
		}
		return !(r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v')
	case CharTypeHSpace:
		return isHSpace(r)
	case CharTypeNonHSpace:
		return !isHSpace(r)
	case CharTypeVSpace:
		return isVSpace(r)
	case CharTypeNonVSpace:
		return !isVSpace(r)
	}
	return false
}

func (v *vm) atWordBoundary() bool {
	ucp := v.flags&UCP != 0
	prevWord := false
	nextWord := false
	if v.pos > 0 {
		r, _ := utf8.DecodeLastRuneInString(v.subject[:v.pos])
		prevWord = isWordChar(r, ucp)
	}
	if v.pos < len(v.subject) {
		r, _ := utf8.DecodeRuneInString(v.subject[v.pos:])
		nextWord = isWordChar(r, ucp)
	}
	return prevWord != nextWord
}

func (v *vm) matchStringFold(ref string) bool {
	pos := v.pos
	for i := 0; i < len(ref); {
		if pos >= len(v.subject) {
			return false
		}
		r1, size1 := utf8.DecodeRuneInString(ref[i:])
		r2, size2 := utf8.DecodeRuneInString(v.subject[pos:])
		if !equalFold(r1, r2) {
			return false
		}
		i += size1
		pos += size2
	}
	v.pos = pos
	return true
}

// execLookbehindBody executes the lookbehind body from startPC, checking if it ends at targetPos.
func (v *vm) execLookbehindBody(subVM *vm, startPC uint32, targetPos int) bool {
	// Run the sub-VM on the body instructions
	subVM.stack = newBTStack()
	subPC := startPC
	for {
		subVM.steps++
		if subVM.steps > subVM.matchLimit {
			return false
		}
		if int(subPC) >= len(v.prog.Inst) {
			return false
		}
		inst := &v.prog.Inst[subPC]
		if inst.Op == OpLookbehindEnd || inst.Op == OpNegLookbehindEnd {
			return subVM.pos == targetPos
		}
		// Execute one step
		ok := subVM.execOne(subPC, inst)
		if !ok {
			return false
		}
		subPC = subVM.nextSubPC
	}
}

var _ = (*vm).execOne // ensure method exists

// nextSubPC is set by execOne for the lookbehind sub-VM.
// This is a field on vm that we'll add.

// execOne executes a single instruction, returning true if execution should continue.
func (v *vm) execOne(pc uint32, inst *Inst) bool {
	switch inst.Op {
	case OpRune:
		if v.pos >= len(v.subject) {
			return v.backtrackSub()
		}
		r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
		if r != inst.Runes[0] {
			return v.backtrackSub()
		}
		v.pos += size
		v.nextSubPC = inst.Out
		return true
	case OpRuneFold:
		if v.pos >= len(v.subject) {
			return v.backtrackSub()
		}
		r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
		if !equalFold(r, inst.Runes[0]) {
			return v.backtrackSub()
		}
		v.pos += size
		v.nextSubPC = inst.Out
		return true
	case OpAnyCharNotNL:
		if v.pos >= len(v.subject) {
			return v.backtrackSub()
		}
		r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
		if r == '\n' {
			return v.backtrackSub()
		}
		v.pos += size
		v.nextSubPC = inst.Out
		return true
	case OpAnyChar:
		if v.pos >= len(v.subject) {
			return v.backtrackSub()
		}
		_, size := utf8.DecodeRuneInString(v.subject[v.pos:])
		v.pos += size
		v.nextSubPC = inst.Out
		return true
	case OpCharClass:
		if v.pos >= len(v.subject) {
			return v.backtrackSub()
		}
		r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
		if !v.matchCharClass(r, inst) {
			return v.backtrackSub()
		}
		v.pos += size
		v.nextSubPC = inst.Out
		return true
	case OpCharType:
		if v.pos >= len(v.subject) {
			return v.backtrackSub()
		}
		r, size := utf8.DecodeRuneInString(v.subject[v.pos:])
		if !v.matchCharType(r, CharTypeKind(inst.N)) {
			return v.backtrackSub()
		}
		v.pos += size
		v.nextSubPC = inst.Out
		return true
	case OpSplit:
		v.stack.push(stackFrame{kind: frameSplit, pc: inst.Arg, pos: v.pos})
		v.nextSubPC = inst.Out
		return true
	case OpSplitLazy:
		v.stack.push(stackFrame{kind: frameSplit, pc: inst.Arg, pos: v.pos})
		v.nextSubPC = inst.Out
		return true
	case OpJump:
		v.nextSubPC = inst.Out
		return true
	case OpCaptureStart:
		slot := inst.N * 2
		if slot < len(v.captures) {
			v.stack.push(stackFrame{kind: frameCapture, capSlot: slot, capVal: v.captures[slot]})
			v.captures[slot] = v.pos
		}
		v.nextSubPC = inst.Out
		return true
	case OpCaptureEnd:
		slot := inst.N*2 + 1
		if slot < len(v.captures) {
			v.stack.push(stackFrame{kind: frameCapture, capSlot: slot, capVal: v.captures[slot]})
			v.captures[slot] = v.pos
		}
		v.nextSubPC = inst.Out
		return true
	case OpAssertBeginLine, OpAssertEndLine, OpAssertBeginText, OpAssertEndText,
		OpAssertEndTextOrNewline, OpAssertWordBoundary, OpAssertNonWordBoundary:
		// Zero-width assertions — delegate to main assertion logic
		ok := v.checkAssertion(inst.Op)
		if !ok {
			return v.backtrackSub()
		}
		v.nextSubPC = inst.Out
		return true
	case OpNop:
		v.nextSubPC = inst.Out
		return true
	default:
		v.nextSubPC = inst.Out
		return true
	}
}

func (v *vm) backtrackSub() bool {
	for {
		f, ok := v.stack.pop()
		if !ok {
			return false
		}
		switch f.kind {
		case frameSplit:
			v.pos = f.pos
			v.nextSubPC = f.pc
			return true
		case frameCapture:
			v.captures[f.capSlot] = f.capVal
		default:
			continue
		}
	}
}

func (v *vm) checkAssertion(op Opcode) bool {
	switch op {
	case OpAssertBeginLine:
		return v.pos == 0 || (v.flags&Multiline != 0 && v.pos > 0 && v.subject[v.pos-1] == '\n')
	case OpAssertEndLine:
		atEnd := v.pos == len(v.subject)
		beforeNL := v.pos < len(v.subject) && v.subject[v.pos] == '\n'
		if v.flags&Multiline != 0 {
			return atEnd || beforeNL
		}
		return atEnd || (beforeNL && v.pos+1 == len(v.subject))
	case OpAssertBeginText:
		return v.pos == 0
	case OpAssertEndText:
		return v.pos == len(v.subject)
	case OpAssertEndTextOrNewline:
		return v.pos == len(v.subject) || (v.pos+1 == len(v.subject) && v.subject[v.pos] == '\n')
	case OpAssertWordBoundary:
		return v.atWordBoundary()
	case OpAssertNonWordBoundary:
		return !v.atWordBoundary()
	}
	return false
}

// findLookaheadEnd scans instructions from startPC to find the matching lookahead end opcode.
func (v *vm) findLookaheadEnd(startPC uint32, endOp Opcode) int {
	depth := 0
	startOp := OpLookaheadStart
	if endOp == OpNegLookaheadEnd {
		startOp = OpNegLookaheadStart
	}
	for pc := startPC; int(pc) < len(v.prog.Inst); pc++ {
		op := v.prog.Inst[pc].Op
		if op == startOp {
			depth++
		} else if op == endOp {
			if depth == 0 {
				return int(pc)
			}
			depth--
		}
	}
	return int(startPC)
}

// findAtomicEnd scans instructions from startPC to find the matching OpAtomicEnd.
func (v *vm) findAtomicEnd(startPC uint32) int {
	depth := 0
	for pc := startPC; int(pc) < len(v.prog.Inst); pc++ {
		op := v.prog.Inst[pc].Op
		if op == OpAtomicStart {
			depth++
		} else if op == OpAtomicEnd {
			if depth == 0 {
				return int(pc)
			}
			depth--
		}
	}
	return int(startPC)
}

// runSubMatch runs a sub-VM to test if the instructions from startPC to endPC match
// at the current position. Returns true if matched. Updates v.pos and v.captures on success.
func (v *vm) runSubMatch(startPC uint32, endPC int) bool {
	// Create a sub-VM sharing the same subject and captures
	subVM := &vm{
		prog:       v.prog,
		subject:    v.subject,
		pos:        v.pos,
		matchStart: v.matchStart,
		startPos:   v.startPos,
		captures:   make([]int, len(v.captures)),
		stack:      newBTStack(),
		flags:      v.flags,
		matchLimit: v.matchLimit - v.steps,
		depthLimit: v.depthLimit,
		heapLimit:  v.heapLimit,
		callout:    v.callout,
		depth:      v.depth,
	}
	copy(subVM.captures, v.captures)

	// Run from startPC, treating endPC as a "match" instruction
	subPC := startPC
	for {
		subVM.steps++
		v.steps++ // count toward parent limit
		if v.steps > v.matchLimit {
			v.limitErr = ErrMatchLimit
			return false
		}
		if int(subPC) >= len(v.prog.Inst) || int(subPC) == endPC {
			// Reached the end — match succeeded
			v.pos = subVM.pos
			copy(v.captures, subVM.captures)
			return true
		}

		inst := &v.prog.Inst[subPC]

		// Handle Match opcode within sub-match as success
		if inst.Op == OpMatch {
			v.pos = subVM.pos
			copy(v.captures, subVM.captures)
			return true
		}

		ok := subVM.execOne(subPC, inst)
		if !ok {
			return false
		}
		subPC = subVM.nextSubPC
	}
}

// runSubMatchAtomic runs a sub-VM for an atomic group. Returns true if matched.
// On success, v.pos and v.captures are updated to the FIRST successful match
// (no backtracking alternatives are preserved).
func (v *vm) runSubMatchAtomic(startPC uint32, endPC int) bool {
	subVM := &vm{
		prog:       v.prog,
		subject:    v.subject,
		pos:        v.pos,
		matchStart: v.matchStart,
		startPos:   v.startPos,
		captures:   make([]int, len(v.captures)),
		stack:      newBTStack(),
		flags:      v.flags,
		matchLimit: v.matchLimit - v.steps,
		depthLimit: v.depthLimit,
		heapLimit:  v.heapLimit,
		callout:    v.callout,
		depth:      v.depth,
	}
	copy(subVM.captures, v.captures)

	subPC := startPC
	for {
		subVM.steps++
		v.steps++
		if v.steps > v.matchLimit {
			v.limitErr = ErrMatchLimit
			return false
		}
		if int(subPC) >= len(v.prog.Inst) || int(subPC) == endPC {
			v.pos = subVM.pos
			copy(v.captures, subVM.captures)
			return true
		}

		inst := &v.prog.Inst[subPC]
		if inst.Op == OpMatch {
			v.pos = subVM.pos
			copy(v.captures, subVM.captures)
			return true
		}

		ok := subVM.execOne(subPC, inst)
		if !ok {
			return false
		}
		subPC = subVM.nextSubPC
	}
}

// findLookbehindEnd scans instructions from startPC to find the matching lookbehind end.
func (v *vm) findLookbehindEnd(startPC uint32) uint32 {
	depth := 0
	for pc := startPC; int(pc) < len(v.prog.Inst); pc++ {
		switch v.prog.Inst[pc].Op {
		case OpLookbehindStart, OpNegLookbehindStart:
			depth++
		case OpLookbehindEnd, OpNegLookbehindEnd:
			if depth == 0 {
				return v.prog.Inst[pc].Out
			}
			depth--
		}
	}
	return startPC // shouldn't happen
}

// Helper functions

func equalFold(r1, r2 rune) bool {
	if r1 == r2 {
		return true
	}
	// Use unicode.SimpleFold to iterate through the case-fold orbit
	for fr := unicode.SimpleFold(r1); fr != r1; fr = unicode.SimpleFold(fr) {
		if fr == r2 {
			return true
		}
	}
	return false
}

func isHSpace(r rune) bool {
	switch r {
	case '\t', ' ', 0xA0, 0x1680, 0x180E,
		0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200A,
		0x202F, 0x205F, 0x3000:
		return true
	}
	return false
}

func isVSpace(r rune) bool {
	switch r {
	case '\n', '\v', '\f', '\r', 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

func matchProperty(r rune, name string) bool {
	tab := resolveProperty(name)
	if tab != nil {
		return unicode.Is(tab, r)
	}
	return false
}
