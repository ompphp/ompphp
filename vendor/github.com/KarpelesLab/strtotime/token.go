package strtotime

type tokenType int

const (
	TypeString tokenType = iota + 1
	TypeNumber
	TypeOperator
	TypeWhitespace
	TypePunctuation
)

// Token represents a token extracted from the input string
type Token struct {
	Val string
	Typ tokenType
	Pos int
}

// Tokenize takes a string and cuts it into tokens.
// Uses substring slicing to avoid per-token allocations.
func Tokenize(s string) []Token {
	if len(s) == 0 {
		return nil
	}

	// Pre-allocate with estimated capacity (most inputs have 3-10 tokens)
	tokens := make([]Token, 0, 8)
	currentType := classifyByte(s[0])
	start := 0

	for i := 1; i < len(s); i++ {
		newType := classifyByte(s[i])
		if newType != currentType {
			tokens = append(tokens, Token{
				Val: s[start:i],
				Typ: currentType,
				Pos: start,
			})
			currentType = newType
			start = i
		}
	}

	// Add the last token
	tokens = append(tokens, Token{
		Val: s[start:],
		Typ: currentType,
		Pos: start,
	})

	return tokens
}

// classifyByte returns the token type for an ASCII byte.
// Since StrToTime lowercases input, we only see ASCII.
func classifyByte(c byte) tokenType {
	switch {
	case c == ' ' || c == '\t' || c == '\n' || c == '\r':
		return TypeWhitespace
	case c >= '0' && c <= '9':
		return TypeNumber
	case c == '+' || c == '-' || c == ':' || c == '/' || c == '.':
		return TypeOperator
	case c == ',' || c == ';' || c == '(' || c == ')' || c == '[' || c == ']':
		return TypePunctuation
	default:
		return TypeString
	}
}

// isOperator checks if a rune is an operator
func isOperator(r rune) bool {
	return r == '+' || r == '-' || r == ':' || r == '/' || r == '.'
}

// isPunctuation checks if a rune is punctuation
func isPunctuation(r rune) bool {
	return r == ',' || r == ';' || r == '(' || r == ')' || r == '[' || r == ']'
}
