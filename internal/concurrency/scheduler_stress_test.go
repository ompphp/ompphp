package concurrency

import (
	"errors"
	"runtime"
	"testing"
)

func TestSchedulerHighVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("high-volume stress test")
	}
	const tasks = 100_000
	s := newTestScheduler(t, Config{Workers: 8, TaskQueue: 1024, CompletionQueue: 1024}, nil)
	done := make(chan struct{})
	go func() {
		completed := 0
		for completed < tasks {
			for _, completion := range s.Drain(1024) {
				s.Acknowledge(completion.ID)
				completed++
			}
			runtime.Gosched()
		}
		close(done)
	}()
	for submitted := 0; submitted < tasks; {
		if _, err := s.Submit("echo", int64(submitted)); err == nil {
			submitted++
		} else if !errors.Is(err, ErrOverloaded) {
			t.Fatal(err)
		} else {
			runtime.Gosched()
		}
	}
	<-done
	s.mu.Lock()
	remaining := len(s.futures)
	s.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d Future handles remain", remaining)
	}
	stats := s.Stats()
	if stats.CompletedTasks != tasks || stats.FailedTasks != 0 {
		t.Fatalf("stats = %#v", stats)
	}
}
