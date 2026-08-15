package transport

import "testing"

func BenchmarkToPHP(b *testing.B) {
	value := Map{
		{Key: Key{String: "position"}, Value: Map{{Key: Key{String: "x"}, Value: 12.5}, {Key: Key{String: "y"}, Value: 8.25}, {Key: Key{String: "z"}, Value: 3.0}}},
		{Key: Key{String: "world"}, Value: int64(0)},
	}
	for i := 0; i < b.N; i++ {
		if _, err := ToPHP(value, DefaultLimits()); err != nil {
			b.Fatal(err)
		}
	}
}
