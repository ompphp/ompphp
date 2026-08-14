package gopcre2

import "unicode"

// compiler compiles an AST into a Program (bytecode).
type compiler struct {
	prog     *Program
	flags    Flag
	caseFold bool
}

func newCompiler(flags Flag) *compiler {
	return &compiler{
		prog: &Program{
			Inst:        make([]Inst, 0, 64),
			CapNames:    []string{""}, // index 0 is the whole match (unnamed)
			NameToGroup: make(map[string][]int),
			GroupEntry:  make(map[int]int),
			GroupEnd:    make(map[int]int),
			Flags:       flags,
		},
		flags:    flags,
		caseFold: flags&Caseless != 0,
	}
}

func (c *compiler) emit(inst Inst) int {
	idx := len(c.prog.Inst)
	c.prog.Inst = append(c.prog.Inst, inst)
	return idx
}

func (c *compiler) patch(addr int, target uint32) {
	c.prog.Inst[addr].Out = target
}

func (c *compiler) patchArg(addr int, target uint32) {
	c.prog.Inst[addr].Arg = target
}

func (c *compiler) pc() int {
	return len(c.prog.Inst)
}

// Compile compiles an AST into a Program.
func (c *compiler) Compile(ast *Node) (*Program, error) {
	// Emit capture 0 start (whole match)
	c.emit(Inst{Op: OpCaptureStart, N: 0})
	if err := c.compileNode(ast); err != nil {
		return nil, err
	}
	c.emit(Inst{Op: OpCaptureEnd, N: 0})
	c.emit(Inst{Op: OpMatch})

	// Wire up Out pointers for sequential instructions
	c.wireSequential()

	c.prog.NumCapture = len(c.prog.CapNames) - 1
	c.prog.Start = 0
	return c.prog, nil
}

// wireSequential sets Out for instructions that don't already have it set
// to point to the next instruction.
func (c *compiler) wireSequential() {
	for i := range c.prog.Inst {
		inst := &c.prog.Inst[i]
		switch inst.Op {
		case OpMatch, OpFail, OpVerbFail, OpAccept:
			// terminal — no Out needed
		case OpJump, OpSplit, OpSplitLazy:
			// already wired
		default:
			if inst.Out == 0 && i+1 < len(c.prog.Inst) {
				inst.Out = uint32(i + 1)
			}
		}
	}
}

func (c *compiler) compileNode(node *Node) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case NdEmpty:
		// nothing
	case NdLiteral:
		return c.compileLiteral(node)
	case NdDot:
		return c.compileDot(node)
	case NdConcat:
		return c.compileConcat(node)
	case NdAlternate:
		return c.compileAlternate(node)
	case NdRepeat:
		return c.compileRepeat(node)
	case NdCapture:
		return c.compileCapture(node)
	case NdNonCapture:
		return c.compileNode(node.Children[0])
	case NdNamedCapture:
		return c.compileNamedCapture(node)
	case NdCharClass:
		return c.compileCharClass(node)
	case NdCharType:
		return c.compileCharType(node)
	case NdProperty:
		return c.compileProperty(node)
	case NdAnchorBeginLine:
		c.emit(Inst{Op: OpAssertBeginLine})
	case NdAnchorEndLine:
		c.emit(Inst{Op: OpAssertEndLine})
	case NdAnchorBeginText:
		c.emit(Inst{Op: OpAssertBeginText})
	case NdAnchorEndText:
		c.emit(Inst{Op: OpAssertEndText})
	case NdAnchorEndTextOpt:
		c.emit(Inst{Op: OpAssertEndTextOrNewline})
	case NdAnchorStartOfMatch:
		c.emit(Inst{Op: OpAssertStartOfMatch})
	case NdWordBoundary:
		c.emit(Inst{Op: OpAssertWordBoundary})
	case NdNonWordBoundary:
		c.emit(Inst{Op: OpAssertNonWordBoundary})
	case NdMatchPointReset:
		c.emit(Inst{Op: OpResetMatchStart})
	case NdBackref:
		return c.compileBackref(node)
	case NdLookahead:
		return c.compileLookahead(node, false)
	case NdNegLookahead:
		return c.compileLookahead(node, true)
	case NdLookbehind:
		return c.compileLookbehind(node, false)
	case NdNegLookbehind:
		return c.compileLookbehind(node, true)
	case NdAtomicGroup:
		return c.compileAtomicGroup(node)
	case NdRecursion:
		c.emit(Inst{Op: OpRecurse})
	case NdSubroutineCall:
		c.emit(Inst{Op: OpSubroutineCall, N: node.Index, Str: node.Name})
	case NdConditional:
		return c.compileConditional(node)
	case NdBranchReset:
		return c.compileNode(node.Children[0])
	case NdVerb:
		return c.compileVerb(node)
	case NdCallout:
		c.emit(Inst{Op: OpCallout, N: node.CallNum, Str: node.CallStr})
	case NdInlineOption:
		return c.compileInlineOption(node)
	default:
		return &CompileError{Offset: node.Pos, Message: "unsupported AST node"}
	}
	return nil
}

func (c *compiler) compileLiteral(node *Node) error {
	if c.caseFold {
		c.emit(Inst{Op: OpRuneFold, Runes: []rune{node.Rune}})
	} else {
		c.emit(Inst{Op: OpRune, Runes: []rune{node.Rune}})
	}
	return nil
}

func (c *compiler) compileDot(node *Node) error {
	if c.flags&DotAll != 0 {
		c.emit(Inst{Op: OpAnyChar})
	} else {
		c.emit(Inst{Op: OpAnyCharNotNL})
	}
	return nil
}

func (c *compiler) compileConcat(node *Node) error {
	for _, child := range node.Children {
		if err := c.compileNode(child); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileAlternate(node *Node) error {
	// alt1 | alt2 | alt3
	// Compile as:
	//   split L1, L2
	//   L1: <alt1>; jump END
	//   L2: split L3, L4
	//   L3: <alt2>; jump END
	//   L4: <alt3>
	//   END:

	n := len(node.Children)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return c.compileNode(node.Children[0])
	}

	// For each alternative except the last, emit a split
	var jumpPatches []int
	for i := 0; i < n-1; i++ {
		splitPC := c.emit(Inst{Op: OpSplit})
		altStart := uint32(c.pc())
		c.patch(splitPC, altStart)

		if err := c.compileNode(node.Children[i]); err != nil {
			return err
		}
		jumpPC := c.emit(Inst{Op: OpJump})
		jumpPatches = append(jumpPatches, jumpPC)

		// Patch the split's Arg to point to the next alternative
		c.patchArg(splitPC, uint32(c.pc()))
	}

	// Last alternative
	if err := c.compileNode(node.Children[n-1]); err != nil {
		return err
	}

	// Patch all jumps to point after the last alternative
	end := uint32(c.pc())
	for _, jp := range jumpPatches {
		c.patch(jp, end)
	}

	return nil
}

func (c *compiler) compileRepeat(node *Node) error {
	child := node.Children[0]
	min, max := node.Min, node.Max

	// For possessive quantifiers, wrap in atomic group
	if node.Greedy == Possessive {
		atomStart := c.emit(Inst{Op: OpAtomicStart})
		_ = atomStart
		// Compile as greedy repeat
		err := c.compileRepeatInner(child, min, max, true)
		if err != nil {
			return err
		}
		c.emit(Inst{Op: OpAtomicEnd})
		return nil
	}

	greedy := node.Greedy == Greedy
	return c.compileRepeatInner(child, min, max, greedy)
}

func (c *compiler) compileRepeatInner(child *Node, min, max int, greedy bool) error {
	// Emit min required copies
	for i := 0; i < min; i++ {
		if err := c.compileNode(child); err != nil {
			return err
		}
	}

	if max == -1 {
		// Unlimited: min already emitted, now emit star loop
		// split body, done (greedy) or split done, body (lazy)
		splitPC := c.pc()
		op := OpSplit
		if !greedy {
			op = OpSplitLazy
		}
		c.emit(Inst{Op: Opcode(op)})

		bodyStart := uint32(c.pc())
		if err := c.compileNode(child); err != nil {
			return err
		}
		c.emit(Inst{Op: OpJump, Out: uint32(splitPC)})
		afterLoop := uint32(c.pc())

		if greedy {
			c.patch(splitPC, bodyStart)
			c.patchArg(splitPC, afterLoop)
		} else {
			c.patch(splitPC, afterLoop)
			c.patchArg(splitPC, bodyStart)
		}
		return nil
	}

	// Bounded: emit (max - min) optional copies
	var splitPatches []int
	for i := min; i < max; i++ {
		op := OpSplit
		if !greedy {
			op = OpSplitLazy
		}
		splitPC := c.emit(Inst{Op: Opcode(op)})
		splitPatches = append(splitPatches, splitPC)

		bodyStart := uint32(c.pc())
		if err := c.compileNode(child); err != nil {
			return err
		}

		if greedy {
			c.patch(splitPC, bodyStart)
		} else {
			c.patchArg(splitPC, bodyStart)
		}
	}

	// Patch all splits to point to after the optional copies
	after := uint32(c.pc())
	for _, sp := range splitPatches {
		if greedy {
			c.patchArg(sp, after)
		} else {
			c.patch(sp, after)
		}
	}

	return nil
}

func (c *compiler) compileCapture(node *Node) error {
	idx := node.Index
	// Ensure CapNames has room
	for len(c.prog.CapNames) <= idx {
		c.prog.CapNames = append(c.prog.CapNames, "")
	}

	entry := c.pc()
	c.emit(Inst{Op: OpCaptureStart, N: idx})
	if err := c.compileNode(node.Children[0]); err != nil {
		return err
	}
	c.emit(Inst{Op: OpCaptureEnd, N: idx})
	end := c.pc()

	c.prog.GroupEntry[idx] = entry
	c.prog.GroupEnd[idx] = end
	return nil
}

func (c *compiler) compileNamedCapture(node *Node) error {
	idx := node.Index
	for len(c.prog.CapNames) <= idx {
		c.prog.CapNames = append(c.prog.CapNames, "")
	}
	c.prog.CapNames[idx] = node.Name
	c.prog.NameToGroup[node.Name] = append(c.prog.NameToGroup[node.Name], idx)

	entry := c.pc()
	c.emit(Inst{Op: OpCaptureStart, N: idx})
	if err := c.compileNode(node.Children[0]); err != nil {
		return err
	}
	c.emit(Inst{Op: OpCaptureEnd, N: idx})
	end := c.pc()

	c.prog.GroupEntry[idx] = entry
	c.prog.GroupEnd[idx] = end
	return nil
}

func (c *compiler) compileCharClass(node *Node) error {
	hasChildren := len(node.Children) > 0

	if !hasChildren {
		// Simple char class with only rune ranges
		var runes []rune
		for _, rr := range node.Ranges {
			runes = append(runes, rr.Lo, rr.Hi)
		}
		c.emit(Inst{Op: OpCharClass, Runes: runes, Negate: node.Negate})
		return nil
	}

	// Complex char class with char types and/or properties.
	// Compile as: a single check instruction that checks ranges + children.
	// We expand char types to ranges, and store property names for VM lookup.
	var runes []rune
	for _, rr := range node.Ranges {
		runes = append(runes, rr.Lo, rr.Hi)
	}

	var propertyNames []string
	for _, child := range node.Children {
		switch child.Kind {
		case NdCharType:
			rr := charTypeToRanges(child.CharType)
			for _, r := range rr {
				runes = append(runes, r.Lo, r.Hi)
			}
			// Also handle negated char types by adding a property-like check
			if isNegatedCharType(child.CharType) {
				// Negated char types in a class are handled differently
				// For now, we'll just include the positive ranges and
				// the negation is implicit in the class being [^\d] etc.
			}
		case NdProperty:
			if child.Negate {
				propertyNames = append(propertyNames, "^"+child.Name)
			} else {
				propertyNames = append(propertyNames, child.Name)
			}
		}
	}

	if len(propertyNames) == 0 {
		// Only rune ranges (char types expanded)
		c.emit(Inst{Op: OpCharClass, Runes: runes, Negate: node.Negate})
		return nil
	}

	// Emit a composite char class that checks both ranges and properties
	// Store property names as semicolon-separated string
	propStr := ""
	for i, name := range propertyNames {
		if i > 0 {
			propStr += ";"
		}
		propStr += name
	}
	c.emit(Inst{Op: OpCharClass, Runes: runes, Negate: node.Negate, Str: propStr})
	return nil
}

func isNegatedCharType(ct CharTypeKind) bool {
	switch ct {
	case CharTypeNonDigit, CharTypeNonWord, CharTypeNonSpace,
		CharTypeNonHSpace, CharTypeNonVSpace:
		return true
	}
	return false
}

func (c *compiler) compileCharType(node *Node) error {
	c.emit(Inst{Op: OpCharType, N: int(node.CharType)})
	return nil
}

func (c *compiler) compileProperty(node *Node) error {
	op := OpProperty
	if node.Negate {
		op = OpPropertyNeg
	}
	c.emit(Inst{Op: op, Str: node.Name})
	return nil
}

func (c *compiler) compileBackref(node *Node) error {
	op := OpBackref
	if c.caseFold {
		op = OpBackrefFold
	}
	c.emit(Inst{Op: op, N: node.Index, Str: node.Name})
	return nil
}

func (c *compiler) compileLookahead(node *Node, negative bool) error {
	startOp := OpLookaheadStart
	endOp := OpLookaheadEnd
	if negative {
		startOp = OpNegLookaheadStart
		endOp = OpNegLookaheadEnd
	}
	c.emit(Inst{Op: startOp})
	if err := c.compileNode(node.Children[0]); err != nil {
		return err
	}
	c.emit(Inst{Op: endOp})
	return nil
}

func (c *compiler) compileLookbehind(node *Node, negative bool) error {
	startOp := OpLookbehindStart
	endOp := OpLookbehindEnd
	if negative {
		startOp = OpNegLookbehindStart
		endOp = OpNegLookbehindEnd
	}
	// Compute min/max lookbehind length from the inner pattern
	minLen, maxLen := c.computeLength(node.Children[0])
	c.emit(Inst{Op: startOp, N: minLen, N2: maxLen})
	if err := c.compileNode(node.Children[0]); err != nil {
		return err
	}
	c.emit(Inst{Op: endOp})
	return nil
}

// computeLength computes the min and max character length of a pattern.
// Returns (min, max) where max=-1 means unbounded.
func (c *compiler) computeLength(node *Node) (int, int) {
	if node == nil {
		return 0, 0
	}
	switch node.Kind {
	case NdEmpty:
		return 0, 0
	case NdLiteral, NdDot, NdCharClass, NdCharType, NdProperty:
		return 1, 1
	case NdConcat:
		minT, maxT := 0, 0
		for _, ch := range node.Children {
			mn, mx := c.computeLength(ch)
			minT += mn
			if maxT >= 0 && mx >= 0 {
				maxT += mx
			} else {
				maxT = -1
			}
		}
		return minT, maxT
	case NdAlternate:
		if len(node.Children) == 0 {
			return 0, 0
		}
		minT, maxT := c.computeLength(node.Children[0])
		for _, ch := range node.Children[1:] {
			mn, mx := c.computeLength(ch)
			if mn < minT {
				minT = mn
			}
			if mx < 0 || maxT < 0 {
				maxT = -1
			} else if mx > maxT {
				maxT = mx
			}
		}
		return minT, maxT
	case NdRepeat:
		mn, mx := c.computeLength(node.Children[0])
		minT := mn * node.Min
		if node.Max < 0 || mx < 0 {
			return minT, -1
		}
		return minT, mx * node.Max
	case NdCapture, NdNonCapture, NdNamedCapture, NdAtomicGroup, NdBranchReset:
		if len(node.Children) > 0 {
			return c.computeLength(node.Children[0])
		}
		return 0, 0
	default:
		// Anchors, assertions, verbs, etc. are zero-width
		return 0, 0
	}
}

func (c *compiler) compileAtomicGroup(node *Node) error {
	c.emit(Inst{Op: OpAtomicStart})
	if err := c.compileNode(node.Children[0]); err != nil {
		return err
	}
	c.emit(Inst{Op: OpAtomicEnd})
	return nil
}

func (c *compiler) compileConditional(node *Node) error {
	// Children: [condition, yes-branch, optional no-branch]
	cond := node.Children[0]
	yes := node.Children[1]
	var no *Node
	if len(node.Children) > 2 {
		no = node.Children[2]
	}

	switch cond.CondType {
	case CondBackref, CondNamedRef:
		// Check if capture group is set
		// Emit: check if group N is set, if yes goto yesPC, else goto noPC
		// We use a special split that checks the capture state
		// For simplicity, emit as a lookahead on the backref
		// Actually, we need to emit a conditional check instruction
		// Let's just compile it as a split with the condition check
		splitPC := c.emit(Inst{Op: OpSplit, N: cond.Index, Str: cond.Name})
		yesStart := uint32(c.pc())
		if err := c.compileNode(yes); err != nil {
			return err
		}
		jumpPC := c.emit(Inst{Op: OpJump})
		noStart := uint32(c.pc())
		if no != nil {
			if err := c.compileNode(no); err != nil {
				return err
			}
		}
		end := uint32(c.pc())
		c.patch(splitPC, yesStart)
		c.patchArg(splitPC, noStart)
		c.patch(jumpPC, end)
		return nil

	case CondAssert:
		// Compile assertion, then branch
		if err := c.compileNode(cond); err != nil {
			return err
		}
		// The assertion consumes no input; the VM treats it as pass/fail
		if err := c.compileNode(yes); err != nil {
			return err
		}
		return nil

	case CondDefine:
		// DEFINE: compile yes-branch but make it unreachable (subroutine only)
		jumpPC := c.emit(Inst{Op: OpJump})
		if err := c.compileNode(yes); err != nil {
			return err
		}
		c.patch(jumpPC, uint32(c.pc()))
		return nil

	default:
		return c.compileNode(yes)
	}
}

func (c *compiler) compileVerb(node *Node) error {
	switch node.VerbKind {
	case VerbAccept:
		c.emit(Inst{Op: OpAccept})
	case VerbFail:
		c.emit(Inst{Op: OpVerbFail})
	case VerbCommit:
		c.emit(Inst{Op: OpCommit})
	case VerbPrune:
		c.emit(Inst{Op: OpPrune, Str: node.Name})
	case VerbSkip:
		c.emit(Inst{Op: OpSkip})
	case VerbSkipName:
		c.emit(Inst{Op: OpSkipName, Str: node.Name})
	case VerbThen:
		c.emit(Inst{Op: OpThen, Str: node.Name})
	case VerbMark:
		c.emit(Inst{Op: OpMark, Str: node.Name})
	default:
		// Handle (*LIMIT_MATCH=N), (*LIMIT_DEPTH=N), (*LIMIT_HEAP=N)
		// Only honored when AllowInlineLimits flag is set.
		if c.flags&AllowInlineLimits != 0 {
			c.applyInlineLimit(node.Name)
		}
	}
	return nil
}

// applyInlineLimit parses a "LIMIT_MATCH:10000" style verb name and applies it.
func (c *compiler) applyInlineLimit(name string) {
	// name is in the form "DIRECTIVE:VALUE" (colon-joined by the lexer)
	for i := 0; i < len(name); i++ {
		if name[i] == ':' {
			directive := name[:i]
			value := name[i+1:]
			n := 0
			for _, ch := range value {
				if ch < '0' || ch > '9' {
					return
				}
				n = n*10 + int(ch-'0')
				if n > 1<<30 {
					return // overflow guard
				}
			}
			if n == 0 {
				return
			}
			switch directive {
			case "LIMIT_MATCH":
				c.prog.MatchLimit = n
			case "LIMIT_DEPTH":
				c.prog.DepthLimit = n
			case "LIMIT_HEAP":
				c.prog.HeapLimit = n
			}
			return
		}
	}
}

func (c *compiler) compileInlineOption(node *Node) error {
	if len(node.Children) > 0 {
		// Scoped: save flags, apply, compile inner, restore
		savedFlags := c.flags
		savedFold := c.caseFold
		c.flags = (c.flags | node.Flags) &^ node.UnFlags
		c.caseFold = c.flags&Caseless != 0

		c.emit(Inst{Op: OpSetFlags, N: int(node.Flags), N2: int(node.UnFlags)})
		if err := c.compileNode(node.Children[0]); err != nil {
			return err
		}
		c.emit(Inst{Op: OpSetFlags, N: int(savedFlags), N2: int(^savedFlags)})

		c.flags = savedFlags
		c.caseFold = savedFold
	} else {
		// Global: apply flags going forward
		c.flags = (c.flags | node.Flags) &^ node.UnFlags
		c.caseFold = c.flags&Caseless != 0
		c.emit(Inst{Op: OpSetFlags, N: int(node.Flags), N2: int(node.UnFlags)})
	}
	return nil
}

// charTypeToRanges converts a character type to ASCII rune ranges.
func charTypeToRanges(ct CharTypeKind) []RuneRange {
	switch ct {
	case CharTypeDigit:
		return []RuneRange{{'0', '9'}}
	case CharTypeWord:
		return []RuneRange{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}}
	case CharTypeSpace:
		return []RuneRange{{'\t', '\r'}, {' ', ' '}}
	case CharTypeHSpace:
		return []RuneRange{{'\t', '\t'}, {' ', ' '}, {0xA0, 0xA0}, {0x1680, 0x1680},
			{0x2000, 0x200A}, {0x202F, 0x202F}, {0x205F, 0x205F}, {0x3000, 0x3000}}
	case CharTypeVSpace:
		return []RuneRange{{'\n', '\r'}, {0x85, 0x85}, {0x2028, 0x2029}}
	default:
		// Negated types are handled in the VM
		return nil
	}
}

// isWordChar reports whether r is a word character (\w).
func isWordChar(r rune, ucp bool) bool {
	if ucp {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
