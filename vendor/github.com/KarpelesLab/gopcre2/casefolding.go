package gopcre2

import "unicode"

// caseFoldRune returns all runes in the case-fold orbit of r.
// For example, caseFoldRune('k') returns ['K', 'k', 'K' (Kelvin sign)].
func caseFoldRune(r rune) []rune {
	result := []rune{r}
	for fr := unicode.SimpleFold(r); fr != r; fr = unicode.SimpleFold(fr) {
		result = append(result, fr)
	}
	return result
}

// caseFoldContains reports whether the case-fold orbit of r contains target.
func caseFoldContains(r, target rune) bool {
	if r == target {
		return true
	}
	for fr := unicode.SimpleFold(r); fr != r; fr = unicode.SimpleFold(fr) {
		if fr == target {
			return true
		}
	}
	return false
}

// caseFoldInRange reports whether any rune in the case-fold orbit of r
// falls within the range [lo, hi].
func caseFoldInRange(r, lo, hi rune) bool {
	if r >= lo && r <= hi {
		return true
	}
	for fr := unicode.SimpleFold(r); fr != r; fr = unicode.SimpleFold(fr) {
		if fr >= lo && fr <= hi {
			return true
		}
	}
	return false
}
