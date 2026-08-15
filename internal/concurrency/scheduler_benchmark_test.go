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

func BenchmarkActorPoolRoundTrip(b *testing.B) {
	s, err := New(Config{Workers: 4, TaskQueue: 1024, NativeWorkers: 2, NativeQueue: 128, CompletionQueue: 1024, ActorMailbox: 256}, func(int) (Runner, error) {
		return &fakeRunner{actors: make(map[uint64]int64)}, nil
	})
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	_, actors, err := s.SpawnActorPool("streamer-shard", 4, int64(0))
	if err != nil {
		b.Fatal(err)
	}
	for range actors {
		completion := waitBenchmarkCompletion(b, s)
		s.Acknowledge(completion.ID)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, actor := range actors {
			if _, err := s.CallActor(actor.ActorID, "move", int64(1)); err != nil {
				b.Fatal(err)
			}
		}
		for range actors {
			completion := waitBenchmarkCompletion(b, s)
			s.Acknowledge(completion.ID)
		}
	}
}

type benchmarkTestingB interface {
	Helper()
	Fatal(...any)
}

func waitBenchmarkCompletion(b benchmarkTestingB, scheduler *Scheduler) Completion {
	b.Helper()
	for {
		if completions := scheduler.Drain(1); len(completions) == 1 {
			return completions[0]
		}
	}
}
