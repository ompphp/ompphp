package gopcre2

// parser converts a token stream into an AST.
type parser struct {
	tokens   []Token
	pos      int
	captures int // next capture group number
	flags    Flag
}

func newParser(tokens []Token, flags Flag) *parser {
	return &parser{tokens: tokens, flags: flags}
}

func (p *parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Kind: TokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() Token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *parser) expect(kind TokenKind) (Token, error) {
	tok := p.peek()
	if tok.Kind != kind {
		return tok, &CompileError{Offset: tok.Pos, Message: "unexpected token"}
	}
	return p.advance(), nil
}

// Parse parses the entire pattern and returns the AST root.
func (p *parser) Parse() (*Node, error) {
	node, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != TokEOF {
		return nil, &CompileError{Offset: p.peek().Pos, Message: "unexpected token after pattern"}
	}
	return node, nil
}

// parseAlternation handles: expr | expr | ...
func (p *parser) parseAlternation() (*Node, error) {
	left, err := p.parseConcatenation()
	if err != nil {
		return nil, err
	}

	if p.peek().Kind != TokAlternate {
		return left, nil
	}

	alts := []*Node{left}
	for p.peek().Kind == TokAlternate {
		p.advance() // skip |
		right, err := p.parseConcatenation()
		if err != nil {
			return nil, err
		}
		alts = append(alts, right)
	}
	return &Node{Kind: NdAlternate, Children: alts}, nil
}

// parseConcatenation handles: atom atom atom ...
func (p *parser) parseConcatenation() (*Node, error) {
	var nodes []*Node
	for {
		tok := p.peek()
		if tok.Kind == TokEOF || tok.Kind == TokAlternate || tok.Kind == TokGroupClose {
			break
		}
		node, err := p.parseQuantified()
		if err != nil {
			return nil, err
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	switch len(nodes) {
	case 0:
		return &Node{Kind: NdEmpty}, nil
	case 1:
		return nodes[0], nil
	default:
		return &Node{Kind: NdConcat, Children: nodes}, nil
	}
}

// parseQuantified handles: atom (quantifier)?
func (p *parser) parseQuantified() (*Node, error) {
	node, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, nil
	}

	tok := p.peek()
	var min, max int
	hasQuantifier := true

	switch tok.Kind {
	case TokStar:
		p.advance()
		min, max = 0, -1
	case TokPlus:
		p.advance()
		min, max = 1, -1
	case TokQuestion:
		p.advance()
		min, max = 0, 1
	case TokRepeat:
		t := p.advance()
		min, max = t.Num, t.Num2
	default:
		hasQuantifier = false
	}

	if !hasQuantifier {
		return node, nil
	}

	greedy := Greedy
	if p.flags&Ungreedy != 0 {
		greedy = Lazy // invert default
	}
	if p.peek().Kind == TokLazy {
		p.advance()
		if greedy == Lazy {
			greedy = Greedy // double-invert
		} else {
			greedy = Lazy
		}
	} else if p.peek().Kind == TokPossessive {
		p.advance()
		greedy = Possessive
	}

	return &Node{
		Kind:     NdRepeat,
		Children: []*Node{node},
		Min:      min,
		Max:      max,
		Greedy:   greedy,
		Pos:      tok.Pos,
	}, nil
}

// parseAtom handles a single atomic expression.
func (p *parser) parseAtom() (*Node, error) {
	tok := p.peek()
	switch tok.Kind {
	case TokLiteral:
		p.advance()
		return &Node{Kind: NdLiteral, Rune: tok.Literal, Pos: tok.Pos}, nil

	case TokEscapeSeq:
		p.advance()
		return &Node{Kind: NdLiteral, Rune: tok.Literal, Pos: tok.Pos}, nil

	case TokDot:
		p.advance()
		return &Node{Kind: NdDot, Pos: tok.Pos}, nil

	case TokCaret:
		p.advance()
		return &Node{Kind: NdAnchorBeginLine, Pos: tok.Pos}, nil

	case TokDollar:
		p.advance()
		return &Node{Kind: NdAnchorEndLine, Pos: tok.Pos}, nil

	case TokAnchor:
		p.advance()
		switch tok.AnchorType {
		case AnchorBeginText:
			return &Node{Kind: NdAnchorBeginText, Pos: tok.Pos}, nil
		case AnchorEndText:
			return &Node{Kind: NdAnchorEndText, Pos: tok.Pos}, nil
		case AnchorEndTextOpt:
			return &Node{Kind: NdAnchorEndTextOpt, Pos: tok.Pos}, nil
		case AnchorWordBoundary:
			return &Node{Kind: NdWordBoundary, Pos: tok.Pos}, nil
		case AnchorNonWord:
			return &Node{Kind: NdNonWordBoundary, Pos: tok.Pos}, nil
		case AnchorStartOfMatch:
			return &Node{Kind: NdAnchorStartOfMatch, Pos: tok.Pos}, nil
		}

	case TokCharType:
		p.advance()
		return &Node{Kind: NdCharType, CharType: tok.CharType, Pos: tok.Pos, Name: tok.Str}, nil

	case TokProperty:
		p.advance()
		return &Node{Kind: NdProperty, Name: tok.Str, Negate: tok.Negate, Pos: tok.Pos}, nil

	case TokBackslashK:
		p.advance()
		return &Node{Kind: NdMatchPointReset, Pos: tok.Pos}, nil

	case TokBackref:
		p.advance()
		return &Node{Kind: NdBackref, Index: tok.Num, Name: tok.Str, Pos: tok.Pos}, nil

	case TokRecurse:
		p.advance()
		// consume implicit group close if present
		if p.peek().Kind == TokGroupClose {
			p.advance()
		}
		return &Node{Kind: NdRecursion, Pos: tok.Pos}, nil

	case TokSubroutine:
		p.advance()
		// consume implicit group close if present
		if p.peek().Kind == TokGroupClose {
			p.advance()
		}
		return &Node{Kind: NdSubroutineCall, Index: tok.Num, Name: tok.Str, Pos: tok.Pos}, nil

	case TokVerb:
		p.advance()
		return &Node{Kind: NdVerb, VerbKind: tok.VerbType, Name: tok.Str, Pos: tok.Pos}, nil

	case TokCallout:
		p.advance()
		// consume implicit group close if present
		if p.peek().Kind == TokGroupClose {
			p.advance()
		}
		return &Node{Kind: NdCallout, CallNum: tok.Num, CallStr: tok.Str, Pos: tok.Pos}, nil

	case TokInlineOption:
		p.advance()
		if tok.HasColon {
			// Scoped options (?i:...)
			inner, err := p.parseAlternation()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(TokGroupClose); err != nil {
				return nil, &CompileError{Offset: tok.Pos, Message: "unterminated option group"}
			}
			return &Node{
				Kind:     NdInlineOption,
				Flags:    tok.Flags,
				UnFlags:  tok.UnsetFlags,
				Children: []*Node{inner},
				Pos:      tok.Pos,
			}, nil
		}
		// Standalone options (?i) — affects rest of current group
		return &Node{Kind: NdInlineOption, Flags: tok.Flags, UnFlags: tok.UnsetFlags, Pos: tok.Pos}, nil

	case TokGroupOpen:
		return p.parseCapture(tok)

	case TokNonCapture:
		return p.parseNonCapture(tok)

	case TokNamedCapture:
		return p.parseNamedCapture(tok)

	case TokAtomicGroup:
		return p.parseAtomicGroup(tok)

	case TokLookahead:
		return p.parseLookaround(tok, NdLookahead)

	case TokNegLookahead:
		return p.parseLookaround(tok, NdNegLookahead)

	case TokLookbehind:
		return p.parseLookaround(tok, NdLookbehind)

	case TokNegLookbehind:
		return p.parseLookaround(tok, NdNegLookbehind)

	case TokBranchReset:
		return p.parseBranchReset(tok)

	case TokConditional:
		return p.parseConditional(tok)

	case TokCharClassOpen, TokCharClassNeg:
		return p.parseCharClass(tok)

	case TokEOF, TokAlternate, TokGroupClose:
		return nil, nil

	default:
		p.advance()
		return nil, &CompileError{Offset: tok.Pos, Message: "unexpected token in pattern"}
	}
	return nil, nil
}

func (p *parser) parseCapture(tok Token) (*Node, error) {
	p.advance() // skip TokGroupOpen
	p.captures++
	idx := p.captures
	inner, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokGroupClose); err != nil {
		return nil, &CompileError{Offset: tok.Pos, Message: "unterminated group"}
	}
	return &Node{Kind: NdCapture, Index: idx, Children: []*Node{inner}, Pos: tok.Pos}, nil
}

func (p *parser) parseNonCapture(tok Token) (*Node, error) {
	p.advance()
	inner, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokGroupClose); err != nil {
		return nil, &CompileError{Offset: tok.Pos, Message: "unterminated non-capturing group"}
	}
	return &Node{Kind: NdNonCapture, Children: []*Node{inner}, Pos: tok.Pos}, nil
}

func (p *parser) parseNamedCapture(tok Token) (*Node, error) {
	p.advance()
	p.captures++
	idx := p.captures
	inner, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokGroupClose); err != nil {
		return nil, &CompileError{Offset: tok.Pos, Message: "unterminated named group"}
	}
	return &Node{Kind: NdNamedCapture, Index: idx, Name: tok.Str, Children: []*Node{inner}, Pos: tok.Pos}, nil
}

func (p *parser) parseAtomicGroup(tok Token) (*Node, error) {
	p.advance()
	inner, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokGroupClose); err != nil {
		return nil, &CompileError{Offset: tok.Pos, Message: "unterminated atomic group"}
	}
	return &Node{Kind: NdAtomicGroup, Children: []*Node{inner}, Pos: tok.Pos}, nil
}

func (p *parser) parseLookaround(tok Token, kind NodeKind) (*Node, error) {
	p.advance()
	inner, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokGroupClose); err != nil {
		return nil, &CompileError{Offset: tok.Pos, Message: "unterminated lookaround"}
	}
	return &Node{Kind: kind, Children: []*Node{inner}, Pos: tok.Pos}, nil
}

func (p *parser) parseBranchReset(tok Token) (*Node, error) {
	p.advance()
	inner, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokGroupClose); err != nil {
		return nil, &CompileError{Offset: tok.Pos, Message: "unterminated branch reset group"}
	}
	return &Node{Kind: NdBranchReset, Children: []*Node{inner}, Pos: tok.Pos}, nil
}

func (p *parser) parseConditional(tok Token) (*Node, error) {
	p.advance() // skip TokConditional

	// Parse condition
	condTok := p.peek()
	var cond *Node

	switch {
	case condTok.Kind == TokLiteral && condTok.Literal >= '0' && condTok.Literal <= '9':
		// (?(1)...) — numeric backref condition
		p.advance()
		num := int(condTok.Literal - '0')
		// read remaining digits
		for p.peek().Kind == TokLiteral && p.peek().Literal >= '0' && p.peek().Literal <= '9' {
			t := p.advance()
			num = num*10 + int(t.Literal-'0')
		}
		if _, err := p.expect(TokGroupClose); err != nil {
			return nil, &CompileError{Offset: tok.Pos, Message: "unterminated conditional reference"}
		}
		cond = &Node{Kind: NdBackref, Index: num, CondType: CondBackref, Pos: condTok.Pos}

	case condTok.Kind == TokLiteral:
		// (?(name)...) or (?(R)...) or (?(DEFINE)...)
		// Read name until )
		name := ""
		for p.peek().Kind == TokLiteral || p.peek().Kind == TokEscapeSeq {
			t := p.advance()
			name += string(t.Literal)
		}
		if _, err := p.expect(TokGroupClose); err != nil {
			return nil, &CompileError{Offset: tok.Pos, Message: "unterminated conditional"}
		}
		switch name {
		case "R":
			cond = &Node{Kind: NdRecursion, CondType: CondRecursion, Pos: condTok.Pos}
		case "DEFINE":
			cond = &Node{Kind: NdEmpty, CondType: CondDefine, Pos: condTok.Pos}
		default:
			cond = &Node{Kind: NdBackref, Name: name, CondType: CondNamedRef, Pos: condTok.Pos}
		}

	case condTok.Kind == TokLookahead || condTok.Kind == TokNegLookahead ||
		condTok.Kind == TokLookbehind || condTok.Kind == TokNegLookbehind:
		// assertion condition
		var kind NodeKind
		switch condTok.Kind {
		case TokLookahead:
			kind = NdLookahead
		case TokNegLookahead:
			kind = NdNegLookahead
		case TokLookbehind:
			kind = NdLookbehind
		case TokNegLookbehind:
			kind = NdNegLookbehind
		}
		var err error
		cond, err = p.parseLookaround(condTok, kind)
		if err != nil {
			return nil, err
		}
		cond.CondType = CondAssert

	default:
		return nil, &CompileError{Offset: tok.Pos, Message: "invalid conditional pattern"}
	}

	// Parse yes pattern
	yes, err := p.parseAlternation()
	if err != nil {
		return nil, err
	}

	// Optional no pattern
	var no *Node
	if p.peek().Kind == TokAlternate {
		p.advance()
		no, err = p.parseAlternation()
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.expect(TokGroupClose); err != nil {
		return nil, &CompileError{Offset: tok.Pos, Message: "unterminated conditional pattern"}
	}

	children := []*Node{cond, yes}
	if no != nil {
		children = append(children, no)
	}
	return &Node{Kind: NdConditional, Children: children, Pos: tok.Pos}, nil
}

func (p *parser) parseCharClass(tok Token) (*Node, error) {
	p.advance() // skip TokCharClassOpen or TokCharClassNeg
	negate := tok.Kind == TokCharClassNeg

	var ranges []RuneRange
	var charTypes []CharTypeKind
	var properties []propertyRef

	for p.peek().Kind != TokCharClassClose {
		if p.peek().Kind == TokEOF {
			return nil, &CompileError{Offset: tok.Pos, Message: "unterminated character class"}
		}

		t := p.peek()
		switch t.Kind {
		case TokLiteral, TokEscapeSeq:
			p.advance()
			r := t.Literal
			// Check for range
			if p.peek().Kind == TokCharClassRange {
				p.advance() // skip -
				if p.peek().Kind == TokCharClassClose {
					// trailing - : treat both as literals
					ranges = append(ranges, RuneRange{r, r})
					ranges = append(ranges, RuneRange{'-', '-'})
					continue
				}
				hi := p.advance()
				ranges = append(ranges, RuneRange{r, hi.Literal})
			} else {
				ranges = append(ranges, RuneRange{r, r})
			}
		case TokCharType:
			p.advance()
			charTypes = append(charTypes, t.CharType)
		case TokPOSIXClass:
			p.advance()
			rr := posixClassRanges(t.Str, t.Negate)
			ranges = append(ranges, rr...)
		case TokProperty:
			p.advance()
			properties = append(properties, propertyRef{name: t.Str, negate: t.Negate})
		default:
			p.advance()
		}
	}
	p.advance() // skip TokCharClassClose

	node := &Node{
		Kind:   NdCharClass,
		Ranges: ranges,
		Negate: negate,
		Pos:    tok.Pos,
	}

	// Store char types and properties as additional information
	// We'll embed them by expanding into the node's children or additional fields
	// For now, store char types as child NdCharType nodes
	for _, ct := range charTypes {
		node.Children = append(node.Children, &Node{Kind: NdCharType, CharType: ct})
	}
	for _, pr := range properties {
		node.Children = append(node.Children, &Node{Kind: NdProperty, Name: pr.name, Negate: pr.negate})
	}

	return node, nil
}

type propertyRef struct {
	name   string
	negate bool
}

// posixClasses maps POSIX class names to their rune ranges.
var posixClasses = map[string][]RuneRange{
	"alpha":  {{'A', 'Z'}, {'a', 'z'}},
	"digit":  {{'0', '9'}},
	"alnum":  {{'0', '9'}, {'A', 'Z'}, {'a', 'z'}},
	"ascii":  {{0, 127}},
	"blank":  {{'\t', '\t'}, {' ', ' '}},
	"cntrl":  {{0, 31}, {127, 127}},
	"graph":  {{'!', '~'}},
	"lower":  {{'a', 'z'}},
	"print":  {{' ', '~'}},
	"punct":  {{'!', '/'}, {':', '@'}, {'[', '`'}, {'{', '~'}},
	"space":  {{'\t', '\r'}, {' ', ' '}},
	"upper":  {{'A', 'Z'}},
	"word":   {{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}},
	"xdigit": {{'0', '9'}, {'A', 'F'}, {'a', 'f'}},
}

// posixClassRanges returns the rune ranges for a POSIX character class name.
func posixClassRanges(name string, negate bool) []RuneRange {
	_ = negate // negation is handled at the NdCharClass level
	return posixClasses[name]
}
