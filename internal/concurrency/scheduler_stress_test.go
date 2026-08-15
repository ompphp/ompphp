package concurrency

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
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

func TestCancellationHighVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("high-volume cancellation test")
	}
	const tasks = 100_000
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 8, NativeWorkers: 4, NativeQueue: 1024, CompletionQueue: 8}, nil)
	if err := s.RegisterNative("cancel", func(ctx context.Context, _ any) (any, error) { <-ctx.Done(); return nil, ctx.Err() }); err != nil {
		t.Fatal(err)
	}
	for submitted := 0; submitted < tasks; {
		id, err := s.SubmitNative("cancel", nil)
		if errors.Is(err, ErrOverloaded) {
			runtime.Gosched()
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !s.Cancel(id) {
			t.Fatalf("cancel %d failed", id)
		}
		completion := s.Drain(1)
		if len(completion) != 1 || !completion[0].Cancelled {
			t.Fatalf("cancel completion = %#v", completion)
		}
		s.Acknowledge(id)
		submitted++
	}
	s.mu.Lock()
	remaining := len(s.futures)
	s.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d cancelled futures remain", remaining)
	}
}

func TestSchedulerSoakAndLifecycleCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("scheduler soak test")
	}
	durationText := os.Getenv("OMPPHP_STRESS_DURATION")
	if durationText == "" {
		t.Skip("set OMPPHP_STRESS_DURATION to run the scheduler soak test")
	}
	duration, err := time.ParseDuration(durationText)
	if err != nil || duration <= 0 {
		t.Fatalf("invalid OMPPHP_STRESS_DURATION %q", durationText)
	}

	s := newTestScheduler(t, Config{Workers: 8, TaskQueue: 512, CompletionQueue: 1024, ActorMailbox: 128}, nil)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	deadline := time.Now().Add(duration)
	var operations uint64
	for time.Now().Before(deadline) {
		ids := make(map[uint64]struct{}, 256)
		for len(ids) < 256 {
			id, submitErr := s.Submit("echo", int64(operations))
			if errors.Is(submitErr, ErrOverloaded) {
				runtime.Gosched()
				continue
			}
			if submitErr != nil {
				t.Fatal(submitErr)
			}
			if id%2 == 0 {
				if timeoutErr := s.Timeout(id, time.Hour); timeoutErr != nil && !errors.Is(timeoutErr, ErrNotFound) {
					t.Fatal(timeoutErr)
				}
			}
			ids[id] = struct{}{}
			operations++
		}
		for len(ids) != 0 {
			for _, completion := range s.Drain(512) {
				if _, expected := ids[completion.ID]; !expected {
					t.Fatalf("unexpected completion %#v", completion)
				}
				delete(ids, completion.ID)
				s.Acknowledge(completion.ID)
			}
			runtime.Gosched()
		}

		actor, ready, spawnErr := s.SpawnActor("counter", int64(0))
		if spawnErr != nil {
			t.Fatal(spawnErr)
		}
		if completion := waitCompletion(t, s); completion.ID != ready || completion.Error != nil {
			t.Fatalf("actor spawn completion = %#v", completion)
		} else {
			s.Acknowledge(completion.ID)
		}
		for value := int64(1); value <= 16; value++ {
			if _, err := s.CallActor(actor, "add", value); err != nil {
				t.Fatal(err)
			}
		}
		for range 16 {
			completion := waitCompletion(t, s)
			if completion.Error != nil {
				t.Fatalf("actor call completion = %#v", completion)
			}
			s.Acknowledge(completion.ID)
		}
		stopped, stopErr := s.StopActor(actor)
		if stopErr != nil {
			t.Fatal(stopErr)
		}
		if completion := waitCompletion(t, s); completion.ID != stopped || completion.Error != nil {
			t.Fatalf("actor stop completion = %#v", completion)
		} else {
			s.Acknowledge(completion.ID)
		}

		timer, timerErr := s.TimerAfter(time.Hour, false)
		if timerErr != nil {
			t.Fatal(timerErr)
		}
		if !s.CancelTimer(timer) {
			t.Fatal("fresh timer could not be cancelled")
		}

		pool, shards, poolErr := s.SpawnActorPool("counter", 4, int64(0))
		if poolErr != nil {
			t.Fatal(poolErr)
		}
		for range shards {
			completion := waitCompletion(t, s)
			s.Acknowledge(completion.ID)
		}
		poolStops, poolStopErr := s.StopActorPool(pool)
		if poolStopErr != nil {
			t.Fatal(poolStopErr)
		}
		for range poolStops {
			completion := waitCompletion(t, s)
			s.Acknowledge(completion.ID)
		}
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	s.mu.Lock()
	futures, actors, timers := len(s.futures), len(s.actors), len(s.timers)
	s.mu.Unlock()
	stats := s.Stats()
	if futures != 0 || actors != 0 || timers != 0 || stats.RunningTasks != 0 || stats.QueuedTasks != 0 || stats.CompletionQueue != 0 {
		t.Fatalf("state remained after %d operations: futures=%d actors=%d timers=%d stats=%#v", operations, futures, actors, timers, stats)
	}
	const maxHeapGrowth = 32 << 20
	if after.HeapAlloc > before.HeapAlloc+maxHeapGrowth {
		t.Fatalf("live heap grew by %d bytes after cleanup", after.HeapAlloc-before.HeapAlloc)
	}
	t.Logf("completed %d task operations plus actor and timer churn in %s; live heap changed from %d to %d bytes", operations, duration, before.HeapAlloc, after.HeapAlloc)
}
