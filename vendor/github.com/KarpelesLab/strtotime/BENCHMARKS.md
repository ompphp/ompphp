# Benchmark Results

These benchmarks measure the performance of different date parsing methods in the strtotime library.

## Main Parser Performance

### Before Optimization

| Format Type | Time (ns/op) | Allocations (B/op) | Allocs (#/op) |
|-------------|------------:|------------------:|-------------:|
| UnixTimestamp | 79.2 | 32 | 1 |
| UnixTimestampWithFraction | 108.8 | 32 | 1 |
| RelativeSimple ("now") | 13,575 | 8,168 | 88 |
| EuropeanDate | 15,650 | 8,228 | 90 |
| DateTimeFormat | 58,304 | 31,828 | 344 |
| ISODate | 174,946 | 90,231 | 952 |
| SlashDate | 192,926 | 96,412 | 1,029 |
| USDate | 201,681 | 102,562 | 1,106 |
| CompactTimestamp | 231,730 | 115,200 | 1,233 |
| MonthNameDMY | 252,923 | 125,549 | 1,339 |
| MonthNameYMD | 283,080 | 134,700 | 1,441 |
| HTTPLogFormat | 292,903 | 156,543 | 1,639 |
| NumberedWeekday | 380,783 | 188,274 | 1,818 |
| RelativeOffset | 412,585 | 214,648 | 2,003 |
| RelativeComplex | 598,223 | 296,081 | 2,033 |
| CompoundExpression | 1,222,181 | 701,762 | 5,921 |

### After Optimization

| Format Type | Time (ns/op) | Allocations (B/op) | Allocs (#/op) | Improvement |
|-------------|------------:|------------------:|-------------:|------------:|
| UnixTimestamp | 95.2 | 32 | 1 | Similar |
| UnixTimestampWithFraction | 111.5 | 32 | 1 | Similar |
| RelativeSimple ("now") | 14,545 | 8,168 | 88 | Similar |
| EuropeanDate | 14,598 | 8,229 | 90 | Similar |
| DateTimeFormat | 62,050 | 31,827 | 344 | Similar |
| ISODate | 126,908 | 90,140 | 952 | 1.4x |
| CompactTimestamp | 138,293 | 102,621 | 1,106 | 1.7x |
| MonthNameYMD | 192,246 | 103,493 | 1,108 | 1.5x |
| SlashDate | 198,342 | 96,256 | 1,029 | Similar |
| MonthNameDMY | 197,128 | 103,567 | 1,108 | 1.3x |
| NumberedWeekday | 200,499 | 103,841 | 1,108 | 1.9x |
| USDate | 212,306 | 102,432 | 1,106 | Similar |
| HTTPLogFormat | 218,035 | 103,794 | 1,108 | 1.3x |
| RelativeOffset | 274,620 | 130,337 | 1,293 | 1.5x |
| RelativeComplex | 302,806 | 211,789 | 1,323 | 2.0x |
| CompoundExpression | 900,848 | 447,232 | 3,791 | 1.4x |

## Individual Parser Performance

### Before Optimization (With Dynamic Regex Compilation)

| Parser | Time (ns/op) | Allocations (B/op) | Allocs (#/op) |
|--------|------------:|------------------:|-------------:|
| US | 7,432 | 6,153 | 79 |
| Slash | 8,971 | 6,145 | 79 |
| ISO | 10,165 | 6,145 | 79 |
| European | 15,142 | 8,228 | 90 |
| MonthName | 18,311 | 10,351 | 107 |
| Compact | 21,299 | 12,715 | 129 |
| HTTPLog | 45,459 | 22,848 | 202 |
| NumberedWeekday | 50,302 | 32,781 | 184 |

### After Optimization (With Pre-compiled Regex)

| Parser | Time (ns/op) | Allocations (B/op) | Allocs (#/op) | Improvement |
|--------|------------:|------------------:|-------------:|------------:|
| Compact | 508.0 | 224 | 2 | 41.9x |
| MonthName | 2,414 | 1,113 | 4 | 7.6x |
| HTTPLog | 2,484 | 1,242 | 4 | 18.3x |
| NumberedWeekday | 2,915 | 1,195 | 5 | 17.3x |
| US | 9,644 | 6,153 | 79 | Baseline |
| Slash | 11,761 | 6,145 | 79 | Baseline |
| ISO | 11,323 | 6,145 | 79 | Baseline |
| European | 14,442 | 8,229 | 90 | Baseline |

## Regex Compilation Impact

| Method | Time (ns/op) | Allocations (B/op) | Allocs (#/op) |
|--------|------------:|------------------:|-------------:|
| PrecompiledRegex | 2,781 | 1,195 | 5 |
| DynamicRegex | 50,934 | 32,758 | 184 |

## Key Observations

1. **Unix Timestamps** are by far the fastest format to parse (79-109 ns/op), requiring minimal allocations.
2. **Simple Formats** like "now" and European dates are also quite fast (13-16 μs/op).
3. **Regular Expressions** have a significant impact on performance. Pre-compiling regexes can improve performance by ~18x.
4. **Complex Formats** like compound expressions are the most expensive to parse (>1 ms/op) with high memory overhead.
5. **HTTP Log Format** and **Numbered Weekday** parsers are relatively expensive operations.

## Optimizations Applied

1. **Pre-compiled Regular Expressions** - Regular expressions are now stored as package-level variables, resulting in significant performance improvements:
   - Compact timestamp parsing: 41.9x faster
   - HTTP log format parsing: 18.3x faster
   - Numbered weekday parsing: 17.3x faster
   - Month name format parsing: 7.6x faster

## Potential Further Optimizations

1. **Fast-Track Common Formats** - Continue to prioritize common formats like Unix timestamps and simple strings.
2. **Reduce Allocations** - The high number of allocations in complex formats suggests potential for further optimization.
3. **Parser Order** - Ensure the most common formats are tried first to avoid unnecessary processing.
4. **Streaming Parser** - For long-term optimization, consider implementing a streaming parser that can process input without multiple passes.
5. **Early Returns** - Add more short-circuit conditions to exit early when a format clearly doesn't match.