package gopcre2

// charClassContains reports whether the rune r is matched by the character class
// defined by the given ranges. If negate is true, the class is inverted.
func charClassContains(r rune, ranges []RuneRange, negate bool) bool {
	for _, rr := range ranges {
		if r >= rr.Lo && r <= rr.Hi {
			return !negate
		}
	}
	return negate
}

// mergeCharClassRanges sorts and merges overlapping ranges.
func mergeCharClassRanges(ranges []RuneRange) []RuneRange {
	if len(ranges) <= 1 {
		return ranges
	}
	// Simple bubble sort (ranges are typically small)
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[j].Lo < ranges[i].Lo {
				ranges[i], ranges[j] = ranges[j], ranges[i]
			}
		}
	}
	merged := []RuneRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.Lo <= last.Hi+1 {
			if r.Hi > last.Hi {
				last.Hi = r.Hi
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// hSpaceRanges returns the character ranges for horizontal whitespace (\h).
func hSpaceRanges() []RuneRange {
	return []RuneRange{
		{'\t', '\t'},
		{' ', ' '},
		{0xA0, 0xA0},
		{0x1680, 0x1680},
		{0x180E, 0x180E},
		{0x2000, 0x200A},
		{0x202F, 0x202F},
		{0x205F, 0x205F},
		{0x3000, 0x3000},
	}
}

// vSpaceRanges returns the character ranges for vertical whitespace (\v).
func vSpaceRanges() []RuneRange {
	return []RuneRange{
		{'\n', '\r'},     // LF, VT, FF, CR
		{0x85, 0x85},     // NEL
		{0x2028, 0x2029}, // LS, PS
	}
}
