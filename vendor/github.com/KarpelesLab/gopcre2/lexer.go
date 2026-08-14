package gopcre2

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// lexer tokenizes a PCRE2 pattern string.
type lexer struct {
	src    string
	pos    int
	tokens []Token
	flags  Flag // current compile-time flags (for (?x) mode awareness)
}

func newLexer(pattern string, flags Flag) *lexer {
	return &lexer{src: pattern, flags: flags}
}

func (l *lexer) tokenize() ([]Token, error) {
	for l.pos < len(l.src) {
		if err := l.nextToken(); err != nil {
			return nil, err
		}
	}
	l.tokens = append(l.tokens, Token{Kind: TokEOF, Pos: l.pos})
	return l.tokens, nil
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) peekAt(offset int) byte {
	i := l.pos + offset
	if i >= len(l.src) {
		return 0
	}
	return l.src[i]
}

func (l *lexer) advance() byte {
	b := l.src[l.pos]
	l.pos++
	return b
}

func (l *lexer) readRune() (rune, int) {
	return utf8.DecodeRuneInString(l.src[l.pos:])
}

func (l *lexer) emit(tok Token) {
	l.tokens = append(l.tokens, tok)
}

func (l *lexer) errorf(offset int, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return &CompileError{Pattern: l.src, Offset: offset, Message: msg}
}

func (l *lexer) nextToken() error {
	startPos := l.pos

	// Handle extended mode: skip whitespace and comments
	if l.flags&Extended != 0 {
		for l.pos < len(l.src) {
			ch := l.src[l.pos]
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f' {
				l.pos++
				continue
			}
			if ch == '#' {
				// skip to end of line
				for l.pos < len(l.src) && l.src[l.pos] != '\n' {
					l.pos++
				}
				continue
			}
			break
		}
		if l.pos >= len(l.src) {
			return nil
		}
		startPos = l.pos
	}

	ch := l.src[l.pos]
	switch ch {
	case '\\':
		return l.lexEscape(startPos)
	case '.':
		l.pos++
		l.emit(Token{Kind: TokDot, Pos: startPos})
	case '^':
		l.pos++
		l.emit(Token{Kind: TokCaret, Pos: startPos})
	case '$':
		l.pos++
		l.emit(Token{Kind: TokDollar, Pos: startPos})
	case '|':
		l.pos++
		l.emit(Token{Kind: TokAlternate, Pos: startPos})
	case ')':
		l.pos++
		l.emit(Token{Kind: TokGroupClose, Pos: startPos})
	case '*':
		l.pos++
		if l.pos < len(l.src) && l.src[l.pos] == '(' {
			// backtrack — this might be (*VERB)
			l.pos--
			// Actually * at beginning or after ( could be (*VERB), but
			// (*VERB) is handled only at specific positions.
			// Let's check: if we're at start or after ( and pattern is (*...
			// For now handle as quantifier; verbs are lexed by the group opener
			l.pos++
		}
		l.emitQuantifier(startPos, TokStar)
	case '+':
		l.pos++
		l.emitQuantifier(startPos, TokPlus)
	case '?':
		l.pos++
		l.emitQuantifier(startPos, TokQuestion)
	case '{':
		return l.lexRepeat(startPos)
	case '[':
		return l.lexCharClass(startPos)
	case '(':
		return l.lexGroupOpen(startPos)
	default:
		// literal rune
		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += size
		l.emit(Token{Kind: TokLiteral, Pos: startPos, Literal: r})
	}
	return nil
}

func (l *lexer) emitQuantifier(pos int, kind TokenKind) {
	tok := Token{Kind: kind, Pos: pos}
	// Check for lazy or possessive modifier
	if l.pos < len(l.src) {
		switch l.src[l.pos] {
		case '?':
			l.pos++
			tok.Kind = kind // keep the quantifier kind
			l.emit(tok)
			l.emit(Token{Kind: TokLazy, Pos: l.pos - 1})
			return
		case '+':
			l.pos++
			l.emit(tok)
			l.emit(Token{Kind: TokPossessive, Pos: l.pos - 1})
			return
		}
	}
	l.emit(tok)
}

func (l *lexer) lexRepeat(startPos int) error {
	l.pos++ // skip {
	// Try to parse {n}, {n,}, {n,m}
	// If it doesn't look like a valid repeat, treat { as literal
	savedPos := l.pos
	n, ok := l.parseInt()
	if !ok {
		// Not a valid repeat — treat { as literal
		l.pos = savedPos - 1
		l.pos++ // re-consume {
		l.emit(Token{Kind: TokLiteral, Pos: startPos, Literal: '{'})
		return nil
	}

	n2 := n
	hasComma := false
	if l.pos < len(l.src) && l.src[l.pos] == ',' {
		l.pos++
		hasComma = true
		n2 = -1 // unbounded
		if l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
			n2, _ = l.parseInt()
		}
	}
	if l.pos >= len(l.src) || l.src[l.pos] != '}' {
		// Not a valid repeat — treat { as literal
		l.pos = savedPos - 1
		l.pos++
		l.emit(Token{Kind: TokLiteral, Pos: startPos, Literal: '{'})
		return nil
	}
	l.pos++ // skip }

	if !hasComma {
		n2 = n // {n} means exactly n
	}

	tok := Token{Kind: TokRepeat, Pos: startPos, Num: n, Num2: n2}
	// Check for lazy or possessive
	if l.pos < len(l.src) {
		switch l.src[l.pos] {
		case '?':
			l.pos++
			l.emit(tok)
			l.emit(Token{Kind: TokLazy, Pos: l.pos - 1})
			return nil
		case '+':
			l.pos++
			l.emit(tok)
			l.emit(Token{Kind: TokPossessive, Pos: l.pos - 1})
			return nil
		}
	}
	l.emit(tok)
	return nil
}

func (l *lexer) parseInt() (int, bool) {
	start := l.pos
	n := 0
	for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
		d := int(l.src[l.pos] - '0')
		if n > 100000 { // cap to prevent overflow
			l.pos++
			continue
		}
		n = n*10 + d
		l.pos++
	}
	return n, l.pos > start
}

// charTypeEscapes maps single-character escapes to their CharTypeKind.
var charTypeEscapes = map[byte]CharTypeKind{
	'd': CharTypeDigit, 'D': CharTypeNonDigit,
	'w': CharTypeWord, 'W': CharTypeNonWord,
	's': CharTypeSpace, 'S': CharTypeNonSpace,
	'h': CharTypeHSpace, 'H': CharTypeNonHSpace,
	'v': CharTypeVSpace, 'V': CharTypeNonVSpace,
}

// anchorEscapes maps single-character escapes to their AnchorKind.
var anchorEscapes = map[byte]AnchorKind{
	'A': AnchorBeginText, 'z': AnchorEndText, 'Z': AnchorEndTextOpt,
	'b': AnchorWordBoundary, 'B': AnchorNonWord, 'G': AnchorStartOfMatch,
}

// simpleEscapes maps single-character escapes to their literal rune value.
var simpleEscapes = map[byte]rune{
	'n': '\n', 'r': '\r', 't': '\t', 'f': '\f', 'a': '\a', 'e': 0x1B,
}

func (l *lexer) lexEscape(startPos int) error {
	l.pos++ // skip backslash
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "trailing backslash")
	}
	ch := l.src[l.pos]
	l.pos++

	if ct, ok := charTypeEscapes[ch]; ok {
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: ct})
		return nil
	}
	if ak, ok := anchorEscapes[ch]; ok {
		l.emit(Token{Kind: TokAnchor, Pos: startPos, AnchorType: ak})
		return nil
	}
	if r, ok := simpleEscapes[ch]; ok {
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: r})
		return nil
	}

	switch ch {
	case 'K':
		l.emit(Token{Kind: TokBackslashK, Pos: startPos})
	case 'x':
		return l.lexHexEscape(startPos)
	case 'o':
		return l.lexOctalBrace(startPos)
	case 'c':
		return l.lexControlChar(startPos)
	case 'p', 'P':
		return l.lexProperty(startPos, ch == 'P')
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return l.lexBackrefDigit(startPos, ch)
	case 'g':
		return l.lexBackrefG(startPos)
	case 'k':
		return l.lexBackrefK(startPos)
	case '\\', '.', '^', '$', '|', '(', ')', '[', ']', '{', '}', '*', '+', '?', '/':
		l.emit(Token{Kind: TokLiteral, Pos: startPos, Literal: rune(ch)})
	case '0':
		l.lexOctalZero(startPos)
	case 'R':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeVSpace, Str: "R"})
	case 'N':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeNonVSpace, Str: "N"})
	case 'X':
		l.emit(Token{Kind: TokCharType, Pos: startPos, Str: "X"})
	default:
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			return l.errorf(startPos, "unrecognized escape sequence \\%c", ch)
		}
		l.emit(Token{Kind: TokLiteral, Pos: startPos, Literal: rune(ch)})
	}
	return nil
}

func (l *lexer) lexBackrefDigit(startPos int, ch byte) error {
	num := int(ch - '0')
	for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
		d := int(l.src[l.pos] - '0')
		if num > 100000 {
			l.pos++
			continue
		}
		num = num*10 + d
		l.pos++
	}
	l.emit(Token{Kind: TokBackref, Pos: startPos, Num: num})
	return nil
}

func (l *lexer) lexOctalZero(startPos int) {
	val := 0
	count := 0
	for count < 2 && l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '7' {
		val = val*8 + int(l.src[l.pos]-'0')
		l.pos++
		count++
	}
	l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: rune(val)})
}

func (l *lexer) lexHexEscape(startPos int) error {
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "incomplete hex escape")
	}
	if l.src[l.pos] == '{' {
		l.pos++ // skip {
		val := 0
		count := 0
		for l.pos < len(l.src) && l.src[l.pos] != '}' {
			d := hexDigit(l.src[l.pos])
			if d < 0 {
				return l.errorf(l.pos, "invalid hex digit in \\x{}")
			}
			val = val*16 + d
			l.pos++
			count++
		}
		if l.pos >= len(l.src) {
			return l.errorf(startPos, "unterminated \\x{}")
		}
		l.pos++ // skip }
		if count == 0 {
			return l.errorf(startPos, "empty \\x{}")
		}
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: rune(val)})
		return nil
	}
	// \xhh — exactly 2 hex digits
	if l.pos+1 >= len(l.src) {
		return l.errorf(startPos, "incomplete \\xhh escape")
	}
	d1 := hexDigit(l.src[l.pos])
	d2 := hexDigit(l.src[l.pos+1])
	if d1 < 0 || d2 < 0 {
		return l.errorf(startPos, "invalid hex digit in \\xhh")
	}
	l.pos += 2
	l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: rune(d1*16 + d2)})
	return nil
}

func (l *lexer) lexOctalBrace(startPos int) error {
	if l.pos >= len(l.src) || l.src[l.pos] != '{' {
		return l.errorf(startPos, "\\o must be followed by {")
	}
	l.pos++ // skip {
	val := 0
	count := 0
	for l.pos < len(l.src) && l.src[l.pos] != '}' {
		if l.src[l.pos] < '0' || l.src[l.pos] > '7' {
			return l.errorf(l.pos, "invalid octal digit in \\o{}")
		}
		val = val*8 + int(l.src[l.pos]-'0')
		l.pos++
		count++
	}
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "unterminated \\o{}")
	}
	l.pos++ // skip }
	if count == 0 {
		return l.errorf(startPos, "empty \\o{}")
	}
	l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: rune(val)})
	return nil
}

func (l *lexer) lexControlChar(startPos int) error {
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "incomplete \\c escape")
	}
	ch := l.src[l.pos]
	l.pos++
	// Control character: \cA = 0x01, \cZ = 0x1A, \c[ = 0x1B, etc.
	var val rune
	if ch >= 'a' && ch <= 'z' {
		val = rune(ch) - 'a' + 1
	} else if ch >= 'A' && ch <= 'Z' {
		val = rune(ch) - 'A' + 1
	} else {
		val = rune(ch) ^ 0x40
	}
	l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: val})
	return nil
}

func (l *lexer) lexProperty(startPos int, negate bool) error {
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "incomplete property escape")
	}
	if l.src[l.pos] == '{' {
		l.pos++
		end := strings.IndexByte(l.src[l.pos:], '}')
		if end < 0 {
			return l.errorf(startPos, "unterminated \\p{}")
		}
		name := l.src[l.pos : l.pos+end]
		l.pos += end + 1
		// handle \p{^Name} as negation
		neg := negate
		if len(name) > 0 && name[0] == '^' {
			neg = !neg
			name = name[1:]
		}
		l.emit(Token{Kind: TokProperty, Pos: startPos, Str: name, Negate: neg})
		return nil
	}
	// Single-letter shorthand: \pL, \PL
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "incomplete property escape")
	}
	ch := l.src[l.pos]
	l.pos++
	l.emit(Token{Kind: TokProperty, Pos: startPos, Str: string(ch), Negate: negate})
	return nil
}

func (l *lexer) lexBackrefG(startPos int) error {
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "incomplete \\g escape")
	}
	ch := l.src[l.pos]
	if ch == '{' {
		l.pos++
		end := strings.IndexByte(l.src[l.pos:], '}')
		if end < 0 {
			return l.errorf(startPos, "unterminated \\g{}")
		}
		ref := l.src[l.pos : l.pos+end]
		l.pos += end + 1
		// Could be number (positive, negative, or +relative) or name
		if len(ref) > 0 && (ref[0] >= '0' && ref[0] <= '9' || ref[0] == '-' || ref[0] == '+') {
			num := parseSignedInt(ref)
			l.emit(Token{Kind: TokBackref, Pos: startPos, Num: num, Str: ref})
		} else {
			l.emit(Token{Kind: TokBackref, Pos: startPos, Str: ref})
		}
		return nil
	}
	if ch >= '0' && ch <= '9' || ch == '-' || ch == '+' {
		// \g1, \g-1, \g+1
		sign := 1
		if ch == '-' {
			sign = -1
			l.pos++
		} else if ch == '+' {
			l.pos++
		}
		num, ok := l.parseInt()
		if !ok {
			return l.errorf(startPos, "invalid \\g backreference")
		}
		l.emit(Token{Kind: TokBackref, Pos: startPos, Num: num * sign})
		return nil
	}
	// \g<name> or \g'name' — these are subroutine calls
	if ch == '<' || ch == '\'' {
		closer := byte('>')
		if ch == '\'' {
			closer = '\''
		}
		l.pos++
		end := strings.IndexByte(l.src[l.pos:], closer)
		if end < 0 {
			return l.errorf(startPos, "unterminated \\g subroutine call")
		}
		name := l.src[l.pos : l.pos+end]
		l.pos += end + 1
		// Could be a number or name
		if len(name) > 0 && (name[0] >= '0' && name[0] <= '9' || name[0] == '-' || name[0] == '+') {
			num := parseSignedInt(name)
			if num == 0 {
				l.emit(Token{Kind: TokRecurse, Pos: startPos})
			} else {
				l.emit(Token{Kind: TokSubroutine, Pos: startPos, Num: num, Str: name})
			}
		} else {
			l.emit(Token{Kind: TokSubroutine, Pos: startPos, Str: name})
		}
		return nil
	}
	return l.errorf(startPos, "invalid \\g escape")
}

func (l *lexer) lexBackrefK(startPos int) error {
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "incomplete \\k escape")
	}
	ch := l.src[l.pos]
	var closer byte
	switch ch {
	case '<':
		closer = '>'
	case '\'':
		closer = '\''
	case '{':
		closer = '}'
	default:
		return l.errorf(startPos, "\\k must be followed by <, ', or {")
	}
	l.pos++
	end := strings.IndexByte(l.src[l.pos:], closer)
	if end < 0 {
		return l.errorf(startPos, "unterminated \\k backreference")
	}
	name := l.src[l.pos : l.pos+end]
	l.pos += end + 1
	l.emit(Token{Kind: TokBackref, Pos: startPos, Str: name})
	return nil
}

func (l *lexer) lexGroupOpen(startPos int) error {
	l.pos++ // skip (

	if l.pos >= len(l.src) {
		l.emit(Token{Kind: TokGroupOpen, Pos: startPos})
		return nil
	}

	// Check for (* verbs
	if l.src[l.pos] == '*' {
		return l.lexVerb(startPos)
	}

	if l.src[l.pos] != '?' {
		// Plain capturing group
		l.emit(Token{Kind: TokGroupOpen, Pos: startPos})
		return nil
	}

	l.pos++ // skip ?
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "unexpected end after (?")
	}

	ch := l.src[l.pos]
	switch ch {
	case ':':
		l.pos++
		l.emit(Token{Kind: TokNonCapture, Pos: startPos})
	case '>':
		l.pos++
		l.emit(Token{Kind: TokAtomicGroup, Pos: startPos})
	case '=':
		l.pos++
		l.emit(Token{Kind: TokLookahead, Pos: startPos})
	case '!':
		l.pos++
		l.emit(Token{Kind: TokNegLookahead, Pos: startPos})
	case '<':
		l.pos++
		if l.pos >= len(l.src) {
			return l.errorf(startPos, "unexpected end after (?<")
		}
		if l.src[l.pos] == '=' {
			l.pos++
			l.emit(Token{Kind: TokLookbehind, Pos: startPos})
		} else if l.src[l.pos] == '!' {
			l.pos++
			l.emit(Token{Kind: TokNegLookbehind, Pos: startPos})
		} else {
			// Named capture (?<name>...)
			return l.lexNamedCapture(startPos, '>')
		}
	case '\'':
		l.pos++
		return l.lexNamedCapture(startPos, '\'')
	case 'P':
		l.pos++
		if l.pos >= len(l.src) {
			return l.errorf(startPos, "unexpected end after (?P")
		}
		if l.src[l.pos] == '<' {
			l.pos++
			return l.lexNamedCapture(startPos, '>')
		} else if l.src[l.pos] == '=' {
			// (?P=name) backreference
			l.pos++
			end := strings.IndexByte(l.src[l.pos:], ')')
			if end < 0 {
				return l.errorf(startPos, "unterminated (?P=name)")
			}
			name := l.src[l.pos : l.pos+end]
			l.pos += end + 1
			l.emit(Token{Kind: TokBackref, Pos: startPos, Str: name})
			return nil
		} else if l.src[l.pos] == '>' {
			// (?P>name) subroutine call
			l.pos++
			end := strings.IndexByte(l.src[l.pos:], ')')
			if end < 0 {
				return l.errorf(startPos, "unterminated (?P>name)")
			}
			name := l.src[l.pos : l.pos+end]
			l.pos += end + 1
			l.emit(Token{Kind: TokSubroutine, Pos: startPos, Str: name})
			l.emit(Token{Kind: TokGroupClose, Pos: l.pos - 1})
			return nil
		}
		return l.errorf(startPos, "unrecognized (?P sequence")
	case '|':
		l.pos++
		l.emit(Token{Kind: TokBranchReset, Pos: startPos})
	case '#':
		// Comment group (?#...)
		l.pos++
		end := strings.IndexByte(l.src[l.pos:], ')')
		if end < 0 {
			return l.errorf(startPos, "unterminated comment (?#...)")
		}
		l.pos += end + 1
		// Comments are silently consumed, no token emitted
	case 'R':
		l.pos++
		if l.pos < len(l.src) && l.src[l.pos] == ')' {
			l.pos++
			l.emit(Token{Kind: TokRecurse, Pos: startPos})
			l.emit(Token{Kind: TokGroupClose, Pos: l.pos - 1})
			return nil
		}
		// Not (?R) — might be (?R&name) etc., fall through to conditional handling
		l.pos-- // put R back
		return l.lexInlineOptions(startPos)
	case '(':
		l.pos++
		l.emit(Token{Kind: TokConditional, Pos: startPos})
	case '&':
		// (?&name) subroutine call
		l.pos++
		end := strings.IndexByte(l.src[l.pos:], ')')
		if end < 0 {
			return l.errorf(startPos, "unterminated (?&name)")
		}
		name := l.src[l.pos : l.pos+end]
		l.pos += end + 1
		l.emit(Token{Kind: TokSubroutine, Pos: startPos, Str: name})
		l.emit(Token{Kind: TokGroupClose, Pos: l.pos - 1})
		return nil
	case 'C':
		return l.lexCallout(startPos)
	default:
		// Could be a number for subroutine (?1), (?+1), (?-1)
		if ch >= '0' && ch <= '9' || ch == '+' || ch == '-' {
			return l.lexSubroutineNum(startPos)
		}
		// Otherwise inline options: (?imsx-imsx) or (?imsx-imsx:...)
		return l.lexInlineOptions(startPos)
	}
	return nil
}

func (l *lexer) lexNamedCapture(startPos int, closer byte) error {
	nameStart := l.pos
	for l.pos < len(l.src) && l.src[l.pos] != closer {
		l.pos++
	}
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "unterminated named capture group")
	}
	name := l.src[nameStart:l.pos]
	l.pos++ // skip closer
	l.emit(Token{Kind: TokNamedCapture, Pos: startPos, Str: name})
	return nil
}

func (l *lexer) lexSubroutineNum(startPos int) error {
	sign := 1
	if l.src[l.pos] == '+' {
		l.pos++
	} else if l.src[l.pos] == '-' {
		sign = -1
		l.pos++
	}
	num, ok := l.parseInt()
	if !ok {
		return l.errorf(startPos, "invalid subroutine call number")
	}
	if l.pos >= len(l.src) || l.src[l.pos] != ')' {
		return l.errorf(startPos, "unterminated subroutine call")
	}
	l.pos++ // skip )
	num *= sign
	if num == 0 {
		l.emit(Token{Kind: TokRecurse, Pos: startPos})
	} else {
		l.emit(Token{Kind: TokSubroutine, Pos: startPos, Num: num})
	}
	l.emit(Token{Kind: TokGroupClose, Pos: l.pos - 1})
	return nil
}

func (l *lexer) lexCallout(startPos int) error {
	l.pos++ // skip C
	if l.pos >= len(l.src) || l.src[l.pos] == ')' {
		if l.pos < len(l.src) {
			l.pos++ // skip )
		}
		l.emit(Token{Kind: TokCallout, Pos: startPos, Num: 0})
		l.emit(Token{Kind: TokGroupClose, Pos: l.pos - 1})
		return nil
	}
	if l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
		num, _ := l.parseInt()
		if l.pos >= len(l.src) || l.src[l.pos] != ')' {
			return l.errorf(startPos, "unterminated callout")
		}
		l.pos++
		l.emit(Token{Kind: TokCallout, Pos: startPos, Num: num})
		l.emit(Token{Kind: TokGroupClose, Pos: l.pos - 1})
		return nil
	}
	if l.src[l.pos] == '"' || l.src[l.pos] == '\'' || l.src[l.pos] == '`' {
		quote := l.src[l.pos]
		l.pos++
		strStart := l.pos
		for l.pos < len(l.src) && l.src[l.pos] != quote {
			l.pos++
		}
		if l.pos >= len(l.src) {
			return l.errorf(startPos, "unterminated callout string")
		}
		str := l.src[strStart:l.pos]
		l.pos++ // skip closing quote
		if l.pos >= len(l.src) || l.src[l.pos] != ')' {
			return l.errorf(startPos, "unterminated callout")
		}
		l.pos++
		l.emit(Token{Kind: TokCallout, Pos: startPos, Str: str})
		l.emit(Token{Kind: TokGroupClose, Pos: l.pos - 1})
		return nil
	}
	return l.errorf(startPos, "invalid callout syntax")
}

func (l *lexer) lexInlineOptions(startPos int) error {
	var setFlags, unsetFlags Flag
	unsetting := false

	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		var flag Flag
		switch ch {
		case 'i':
			flag = Caseless
		case 'm':
			flag = Multiline
		case 's':
			flag = DotAll
		case 'x':
			flag = Extended
		case 'U':
			flag = Ungreedy
		case 'J':
			flag = DupNames
		case 'n':
			flag = NoAutoCapture
		case '-':
			if unsetting {
				return l.errorf(l.pos, "double - in options")
			}
			unsetting = true
			l.pos++
			continue
		case ':':
			l.pos++
			l.emit(Token{Kind: TokInlineOption, Pos: startPos, Flags: setFlags, UnsetFlags: unsetFlags, HasColon: true})
			return nil
		case ')':
			l.pos++
			l.emit(Token{Kind: TokInlineOption, Pos: startPos, Flags: setFlags, UnsetFlags: unsetFlags})
			return nil
		default:
			return l.errorf(l.pos, "unrecognized option flag '%c'", ch)
		}
		if unsetting {
			unsetFlags |= flag
		} else {
			setFlags |= flag
		}
		l.pos++
	}
	return l.errorf(startPos, "unterminated inline options")
}

func (l *lexer) lexVerb(startPos int) error {
	l.pos++ // skip *
	// Read verb name
	nameStart := l.pos
	for l.pos < len(l.src) && l.src[l.pos] != ')' && l.src[l.pos] != ':' && l.src[l.pos] != '=' {
		l.pos++
	}
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "unterminated verb (*...)")
	}
	name := l.src[nameStart:l.pos]

	var arg string
	if l.src[l.pos] == ':' || l.src[l.pos] == '=' {
		l.pos++
		argStart := l.pos
		for l.pos < len(l.src) && l.src[l.pos] != ')' {
			l.pos++
		}
		if l.pos >= len(l.src) {
			return l.errorf(startPos, "unterminated verb (*...)")
		}
		arg = l.src[argStart:l.pos]
	}
	l.pos++ // skip )

	switch strings.ToUpper(name) {
	case "ACCEPT":
		l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbAccept})
	case "FAIL", "F":
		l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbFail})
	case "COMMIT":
		l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbCommit})
	case "PRUNE":
		l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbPrune, Str: arg})
	case "SKIP":
		if arg != "" {
			l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbSkipName, Str: arg})
		} else {
			l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbSkip})
		}
	case "THEN":
		l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbThen, Str: arg})
	case "MARK", "":
		l.emit(Token{Kind: TokVerb, Pos: startPos, VerbType: VerbMark, Str: arg})
	default:
		// Could be (*LIMIT_MATCH=N) etc. — only honored when AllowInlineLimits is set
		l.emit(Token{Kind: TokVerb, Pos: startPos, Str: name + ":" + arg})
	}
	return nil
}

func (l *lexer) lexCharClass(startPos int) error {
	l.pos++ // skip [
	negate := false
	if l.pos < len(l.src) && l.src[l.pos] == '^' {
		negate = true
		l.pos++
	}
	if negate {
		l.emit(Token{Kind: TokCharClassNeg, Pos: startPos})
	} else {
		l.emit(Token{Kind: TokCharClassOpen, Pos: startPos})
	}

	first := true
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == ']' && !first {
			l.pos++
			l.emit(Token{Kind: TokCharClassClose, Pos: l.pos - 1})
			return nil
		}
		first = false

		if ch == '[' && l.pos+1 < len(l.src) && l.src[l.pos+1] == ':' {
			// POSIX class
			if err := l.lexPOSIXClass(l.pos); err != nil {
				// Not a valid POSIX class — treat [ as literal
				l.emit(Token{Kind: TokLiteral, Pos: l.pos, Literal: '['})
				l.pos++
				continue
			}
			continue
		}

		if ch == '\\' {
			escPos := l.pos
			if err := l.lexClassEscape(escPos); err != nil {
				return err
			}
			continue
		}

		// Check for range: a-z
		if l.pos+2 < len(l.src) && l.src[l.pos+1] == '-' && l.src[l.pos+2] != ']' {
			r1, sz1 := utf8.DecodeRuneInString(l.src[l.pos:])
			l.pos += sz1
			l.emit(Token{Kind: TokLiteral, Pos: l.pos - sz1, Literal: r1})
			l.pos++ // skip -
			l.emit(Token{Kind: TokCharClassRange, Pos: l.pos - 1})
			r2, sz2 := utf8.DecodeRuneInString(l.src[l.pos:])
			l.pos += sz2
			l.emit(Token{Kind: TokLiteral, Pos: l.pos - sz2, Literal: r2})
			continue
		}

		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += size
		l.emit(Token{Kind: TokLiteral, Pos: l.pos - size, Literal: r})
	}
	return l.errorf(startPos, "unterminated character class")
}

func (l *lexer) lexPOSIXClass(startPos int) error {
	// Pattern: [:name:]
	l.pos += 2 // skip [:
	negate := false
	if l.pos < len(l.src) && l.src[l.pos] == '^' {
		negate = true
		l.pos++
	}
	nameStart := l.pos
	for l.pos < len(l.src) && l.src[l.pos] != ':' {
		if l.src[l.pos] == ']' {
			return l.errorf(startPos, "invalid POSIX class")
		}
		l.pos++
	}
	if l.pos+1 >= len(l.src) || l.src[l.pos+1] != ']' {
		return l.errorf(startPos, "invalid POSIX class")
	}
	name := l.src[nameStart:l.pos]
	l.pos += 2 // skip :]
	l.emit(Token{Kind: TokPOSIXClass, Pos: startPos, Str: name, Negate: negate})
	return nil
}

func (l *lexer) lexClassEscape(startPos int) error {
	// Inside character class, handle escapes
	l.pos++ // skip backslash
	if l.pos >= len(l.src) {
		return l.errorf(startPos, "trailing backslash in character class")
	}
	ch := l.src[l.pos]
	l.pos++
	switch ch {
	case 'd':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeDigit})
	case 'D':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeNonDigit})
	case 'w':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeWord})
	case 'W':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeNonWord})
	case 's':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeSpace})
	case 'S':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeNonSpace})
	case 'h':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeHSpace})
	case 'H':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeNonHSpace})
	case 'v':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeVSpace})
	case 'V':
		l.emit(Token{Kind: TokCharType, Pos: startPos, CharType: CharTypeNonVSpace})
	case 'n':
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: '\n'})
	case 'r':
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: '\r'})
	case 't':
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: '\t'})
	case 'f':
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: '\f'})
	case 'a':
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: '\a'})
	case 'e':
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: 0x1B})
	case 'x':
		l.pos-- // let lexHexEscape re-read 'x'
		l.pos-- // back to backslash
		// Actually we already consumed \ and x. lexHexEscape expects to be positioned after \x
		l.pos += 2 // restore: positioned after \x
		return l.lexHexEscape(startPos)
	case '0':
		val := 0
		count := 0
		for count < 2 && l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '7' {
			val = val*8 + int(l.src[l.pos]-'0')
			l.pos++
			count++
		}
		l.emit(Token{Kind: TokEscapeSeq, Pos: startPos, Literal: rune(val)})
	case 'p', 'P':
		l.pos--    // back up to p/P
		l.pos--    // back to backslash
		l.pos += 2 // positioned after \p
		return l.lexProperty(startPos, ch == 'P')
	default:
		// Metacharacters in class: ] \ ^ -
		l.emit(Token{Kind: TokLiteral, Pos: startPos, Literal: rune(ch)})
	}
	return nil
}

func hexDigit(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	default:
		return -1
	}
}

func parseSignedInt(s string) int {
	if len(s) == 0 {
		return 0
	}
	sign := 1
	i := 0
	if s[0] == '-' {
		sign = -1
		i = 1
	} else if s[0] == '+' {
		i = 1
	}
	n := 0
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			break
		}
		if n > 100000 {
			continue
		}
		n = n*10 + int(s[i]-'0')
	}
	return n * sign
}
