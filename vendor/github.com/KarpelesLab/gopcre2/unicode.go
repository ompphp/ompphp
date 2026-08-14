package gopcre2

import "unicode"

// anyRuneTable matches any Unicode rune.
var anyRuneTable = &unicode.RangeTable{
	R16: []unicode.Range16{{Lo: 0, Hi: 0xFFFF, Stride: 1}},
	R32: []unicode.Range32{{Lo: 0x10000, Hi: 0x10FFFF, Stride: 1}},
}

// PCRE2-specific composite Unicode properties.
// These are not directly in Go's unicode tables but are defined by PCRE2.

// xanTable matches alphanumeric characters (union of L and N categories).
var xanTable = mergeRangeTables(unicode.Letter, unicode.Number)

// xwdTable matches word characters (alphanumeric + underscore).
// Since underscore is a single char, we check it separately in isWordChar.

// xspTable matches POSIX space characters (same as \s in UCP mode).
var xspTable = unicode.White_Space

// mergeRangeTables creates a new RangeTable that is the union of the given tables.
func mergeRangeTables(tables ...*unicode.RangeTable) *unicode.RangeTable {
	// Simple approach: combine all R16 and R32 entries
	var r16 []unicode.Range16
	var r32 []unicode.Range32
	for _, t := range tables {
		r16 = append(r16, t.R16...)
		r32 = append(r32, t.R32...)
	}
	return &unicode.RangeTable{R16: r16, R32: r32}
}

// pcre2PropertyMap maps PCRE2 property names to Go unicode.RangeTable values.
// This includes all standard Unicode general categories, scripts, and
// PCRE2-specific composite properties.
var pcre2PropertyMap = buildPropertyMap()

func buildPropertyMap() map[string]*unicode.RangeTable {
	m := make(map[string]*unicode.RangeTable)

	// General categories
	for name, tab := range unicode.Categories {
		m[name] = tab
	}

	// Scripts
	for name, tab := range unicode.Scripts {
		m[name] = tab
	}

	// Properties
	for name, tab := range unicode.Properties {
		m[name] = tab
	}

	// PCRE2-specific composite properties
	m["Xan"] = xanTable // Alphanumeric
	m["Xps"] = xspTable // POSIX space
	m["Xsp"] = xspTable // Perl space (same as Xps in PCRE2)

	// Common aliases
	m["Letter"] = unicode.Letter
	m["Uppercase_Letter"] = unicode.Lu
	m["Lowercase_Letter"] = unicode.Ll
	m["Titlecase_Letter"] = unicode.Lt
	m["Modifier_Letter"] = unicode.Lm
	m["Other_Letter"] = unicode.Lo
	m["Mark"] = unicode.Mark
	m["Number"] = unicode.Number
	m["Decimal_Number"] = unicode.Nd
	m["Digit"] = unicode.Nd
	m["Letter_Number"] = unicode.Nl
	m["Other_Number"] = unicode.No
	m["Punctuation"] = unicode.Punct
	m["Symbol"] = unicode.Symbol
	m["Separator"] = unicode.Zs
	m["Space_Separator"] = unicode.Zs
	m["Line_Separator"] = unicode.Zl
	m["Paragraph_Separator"] = unicode.Zp
	m["Other"] = unicode.Other
	m["Control"] = unicode.Cc
	m["Format"] = unicode.Cf
	m["Private_Use"] = unicode.Co
	m["Surrogate"] = unicode.Cs
	m["Any"] = anyRuneTable

	// Single-letter PCRE2 aliases for general categories
	m["L"] = unicode.Letter
	m["M"] = unicode.Mark
	m["N"] = unicode.Number
	m["P"] = unicode.Punct
	m["S"] = unicode.Symbol
	m["Z"] = mergeRangeTables(unicode.Zs, unicode.Zl, unicode.Zp)
	m["C"] = unicode.Other

	// Subcategory aliases with underscore variants
	m["Lu"] = unicode.Lu
	m["Ll"] = unicode.Ll
	m["Lt"] = unicode.Lt
	m["Lm"] = unicode.Lm
	m["Lo"] = unicode.Lo
	m["Mn"] = unicode.Mn
	m["Mc"] = unicode.Mc
	m["Me"] = unicode.Me
	m["Nd"] = unicode.Nd
	m["Nl"] = unicode.Nl
	m["No"] = unicode.No
	m["Pc"] = unicode.Pc
	m["Pd"] = unicode.Pd
	m["Ps"] = unicode.Ps
	m["Pe"] = unicode.Pe
	m["Pi"] = unicode.Pi
	m["Pf"] = unicode.Pf
	m["Po"] = unicode.Po
	m["Sm"] = unicode.Sm
	m["Sc"] = unicode.Sc
	m["Sk"] = unicode.Sk
	m["So"] = unicode.So
	m["Zs"] = unicode.Zs
	m["Zl"] = unicode.Zl
	m["Zp"] = unicode.Zp
	m["Cc"] = unicode.Cc
	m["Cf"] = unicode.Cf
	m["Co"] = unicode.Co
	m["Cs"] = unicode.Cs

	// L& = Lc = cased letters (Lu + Ll + Lt)
	cased := mergeRangeTables(unicode.Lu, unicode.Ll, unicode.Lt)
	m["L&"] = cased
	m["Lc"] = cased
	m["LC"] = cased

	return m
}

// resolveProperty looks up a PCRE2 property name and returns the matching RangeTable.
func resolveProperty(name string) *unicode.RangeTable {
	if tab, ok := pcre2PropertyMap[name]; ok {
		return tab
	}
	// Try case-insensitive lookup with normalized name
	normalized := normalizePropertyName(name)
	for k, v := range pcre2PropertyMap {
		if normalizePropertyName(k) == normalized {
			return v
		}
	}
	return nil
}

// normalizePropertyName normalizes a Unicode property name by removing
// hyphens, underscores, and spaces, and lowercasing.
func normalizePropertyName(name string) string {
	var buf []byte
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '-' || ch == '_' || ch == ' ' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		buf = append(buf, ch)
	}
	return string(buf)
}
