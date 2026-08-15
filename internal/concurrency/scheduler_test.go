package concurrency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu         sync.Mutex
	actors     map[uint64]int64
	block      <-chan struct{}
	actorBlock <-chan struct{}
}

type concurrentActorRunner struct {
	*fakeRunner
	started chan<- struct{}
	release <-chan struct{}
}

func (r *concurrentActorRunner) CallActor(id uint64, method string, payload any) (any, *RemoteError) {
	r.started <- struct{}{}
	<-r.release
	return r.fakeRunner.CallActor(id, method, payload)
}

func (r *fakeRunner) Run(class string, payload any) (any, *RemoteError) {
	if r.block != nil {
		<-r.block
	}
	if class == "fail" {
		return nil, &RemoteError{Class: "RuntimeException", Message: "broken"}
	}
	return payload, nil
}
func (r *fakeRunner) SpawnActor(id uint64, _ string, payload any) *RemoteError {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actors[id] = payload.(int64)
	return nil
}
func (r *fakeRunner) CallActor(id uint64, _ string, payload any) (any, *RemoteError) {
	if r.actorBlock != nil {
		<-r.actorBlock
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actors[id] += payload.(int64)
	return r.actors[id], nil
}
func (r *fakeRunner) StopActor(id uint64) *RemoteError {
	r.mu.Lock()
	delete(r.actors, id)
	r.mu.Unlock()
	return nil
}
func (r *fakeRunner) Close() {}

func newTestScheduler(t *testing.T, config Config, block <-chan struct{}) *Scheduler {
	t.Helper()
	s, err := New(config, func(int) (Runner, error) { return &fakeRunner{actors: make(map[uint64]int64), block: block}, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func waitCompletion(t *testing.T, scheduler *Scheduler) Completion {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if completions := scheduler.Drain(1); len(completions) == 1 {
			return completions[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("completion timed out")
	return Completion{}
}

func TestSubmitCompleteAndReject(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 2, TaskQueue: 4, CompletionQueue: 4}, nil)
	id, err := s.Submit("echo", int64(7))
	if err != nil {
		t.Fatal(err)
	}
	completion := waitCompletion(t, s)
	if completion.ID != id || completion.Value != int64(7) || completion.Error != nil {
		t.Fatalf("completion = %#v", completion)
	}
	s.Acknowledge(id)
	id, err = s.Submit("fail", nil)
	if err != nil {
		t.Fatal(err)
	}
	completion = waitCompletion(t, s)
	if completion.ID != id || completion.Error == nil || completion.Error.Class != "RuntimeException" {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestCompletionWinsCancellationRace(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 2, CompletionQueue: 2}, nil)
	id, err := s.Submit("echo", int64(1))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for s.Stats().CompletedTasks == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if s.Cancel(id) {
		t.Fatal("settled task was cancelled")
	}
	if completion := waitCompletion(t, s); completion.ID != id || completion.Cancelled {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestQueueBackpressure(t *testing.T) {
	block := make(chan struct{})
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 1, CompletionQueue: 4}, block)
	if _, err := s.Submit("block", nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for !s.workers[0].busy.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if _, err := s.Submit("queued", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Submit("full", nil); !errors.Is(err, ErrOverloaded) {
		t.Fatalf("queue error = %v", err)
	}
	close(block)
}

func TestCancellationAndNativeContext(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 4, CompletionQueue: 4}, nil)
	started := make(chan struct{})
	if err := s.RegisterNative("wait", func(ctx context.Context, _ any) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	id, err := s.SubmitNative("wait", nil)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if !s.Cancel(id) {
		t.Fatal("cancel returned false")
	}
	completion := waitCompletion(t, s)
	if completion.ID != id || !completion.Cancelled {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestNativePanicRejectsWithoutStoppingWorker(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 4, CompletionQueue: 4}, nil)
	if err := s.RegisterNative("panic", func(context.Context, any) (any, error) { panic("injected") }); err != nil {
		t.Fatal(err)
	}
	id, err := s.SubmitNative("panic", nil)
	if err != nil {
		t.Fatal(err)
	}
	completion := waitCompletion(t, s)
	if completion.ID != id || completion.Error == nil || completion.Error.Class != "WorkerPanic" {
		t.Fatalf("panic completion = %#v", completion)
	}
	if _, err := s.Submit("echo", int64(1)); err != nil {
		t.Fatal(err)
	}
	if completion := waitCompletion(t, s); completion.Value != int64(1) {
		t.Fatalf("worker did not recover: %#v", completion)
	}
}

func TestTimeoutCancelsNativeContext(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 4, CompletionQueue: 4}, nil)
	if err := s.RegisterNative("wait", func(ctx context.Context, _ any) (any, error) { <-ctx.Done(); return nil, ctx.Err() }); err != nil {
		t.Fatal(err)
	}
	id, err := s.SubmitNative("wait", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Timeout(id, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	completion := waitCompletion(t, s)
	if completion.Error == nil || completion.Error.Class != "TimeoutException" || completion.Cancelled {
		t.Fatalf("timeout completion = %#v", completion)
	}
}

func TestTimeoutIsStoppedWhenFutureSettles(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 4, CompletionQueue: 4}, nil)
	id, err := s.Submit("echo", int64(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Timeout(id, time.Hour); err != nil {
		t.Fatal(err)
	}
	completion := waitCompletion(t, s)
	s.mu.Lock()
	state := s.futures[id]
	timerStopped := state != nil && state.timeout == nil
	s.mu.Unlock()
	if !timerStopped {
		t.Fatal("settled future retained its timeout timer")
	}
	s.Acknowledge(completion.ID)
	if err := s.Timeout(id, time.Second); !errors.Is(err, ErrNotFound) {
		t.Fatalf("timeout after acknowledgement = %v", err)
	}
}

func TestReplacingTimeoutStopsPreviousDeadline(t *testing.T) {
	block := make(chan struct{})
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 2, CompletionQueue: 2}, block)
	id, err := s.Submit("block", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Timeout(id, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := s.Timeout(id, time.Hour); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if completions := s.Drain(1); len(completions) != 0 {
		t.Fatalf("replaced timeout fired: %#v", completions)
	}
	close(block)
}

func TestActorOrderingAndState(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 2, TaskQueue: 8, CompletionQueue: 8, ActorMailbox: 4}, nil)
	actor, _, err := s.SpawnActor("counter", int64(10))
	if err != nil {
		t.Fatal(err)
	}
	_ = waitCompletion(t, s)
	first, err := s.CallActor(actor, "add", int64(1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CallActor(actor, "add", int64(2))
	if err != nil {
		t.Fatal(err)
	}
	a := waitCompletion(t, s)
	b := waitCompletion(t, s)
	if a.ID != first || a.Value != int64(11) || b.ID != second || b.Value != int64(13) {
		t.Fatalf("actor completions = %#v %#v", a, b)
	}
}

func TestActorMailboxBackpressure(t *testing.T) {
	block := make(chan struct{})
	s, err := New(Config{Workers: 1, TaskQueue: 4, CompletionQueue: 4, ActorMailbox: 1}, func(int) (Runner, error) {
		return &fakeRunner{actors: make(map[uint64]int64), actorBlock: block}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	actor, _, err := s.SpawnActor("counter", int64(0))
	if err != nil {
		t.Fatal(err)
	}
	_ = waitCompletion(t, s)
	if _, err := s.CallActor(actor, "add", int64(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CallActor(actor, "add", int64(1)); !errors.Is(err, ErrActorFull) {
		t.Fatalf("mailbox error = %v", err)
	}
	close(block)
}

func TestDifferentActorsRunConcurrently(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	s, err := New(Config{Workers: 2, TaskQueue: 4, CompletionQueue: 8, ActorMailbox: 2}, func(int) (Runner, error) {
		return &concurrentActorRunner{fakeRunner: &fakeRunner{actors: make(map[uint64]int64)}, started: started, release: release}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	first, _, err := s.SpawnActor("counter", int64(0))
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := s.SpawnActor("counter", int64(0))
	if err != nil {
		t.Fatal(err)
	}
	_ = waitCompletion(t, s)
	_ = waitCompletion(t, s)
	if _, err := s.CallActor(first, "add", int64(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CallActor(second, "add", int64(1)); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(time.Second)
	for range 2 {
		select {
		case <-started:
		case <-deadline:
			t.Fatal("actors did not run concurrently")
		}
	}
	close(release)
}

func TestConcurrentSubmitAndShutdown(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 2, TaskQueue: 32, CompletionQueue: 64}, nil)
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 100 {
				_, _ = s.Submit("echo", nil)
			}
		}()
	}
	time.Sleep(time.Millisecond)
	s.Close()
	group.Wait()
}

func TestTimerAndShutdown(t *testing.T) {
	s := newTestScheduler(t, Config{Workers: 1, TaskQueue: 2, CompletionQueue: 2}, nil)
	id, err := s.TimerAfter(time.Millisecond, false)
	if err != nil {
		t.Fatal(err)
	}
	completion := waitCompletion(t, s)
	if completion.Kind != CompletionTimer || completion.ID != id {
		t.Fatalf("timer completion = %#v", completion)
	}
	s.Close()
	if _, err := s.Submit("echo", nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("submit after close = %v", err)
	}
}
