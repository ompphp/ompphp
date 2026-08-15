package concurrency

import (
	"testing"
)

func BenchmarkSchedulerRoundTrip(b *testing.B) {
	for _, workers := range []int{1, 2, 4, 8} {
		b.Run(string(rune('0'+workers))+"-workers", func(b *testing.B) {
			s, err := New(Config{Workers: workers, TaskQueue: b.N + 1, CompletionQueue: b.N + 1}, func(int) (Runner, error) {
				return &fakeRunner{actors: make(map[uint64]int64)}, nil
			})
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.Submit("echo", int64(i)); err != nil {
					b.Fatal(err)
				}
			}
			completed := 0
			for completed < b.N {
				completed += len(s.Drain(b.N - completed))
			}
		})
	}
}
