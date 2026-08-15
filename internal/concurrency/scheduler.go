package concurrency

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrClosed       = errors.New("concurrency scheduler is shut down")
	ErrOverloaded   = errors.New("concurrency scheduler queue is full")
	ErrNotFound     = errors.New("concurrency handle was not found")
	ErrActorFull    = errors.New("actor mailbox is full")
	ErrActorStopped = errors.New("actor is stopped")
)

type Config struct {
	Workers         int
	TaskQueue       int
	CompletionQueue int
	ActorMailbox    int
	ShutdownTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{Workers: 4, TaskQueue: 256, CompletionQueue: 512, ActorMailbox: 64, ShutdownTimeout: 5 * time.Second}
}

type RemoteError struct {
	Class    string
	Message  string
	File     string
	Line     int64
	Trace    string
	WorkerID int
	TaskID   uint64
}

func (e *RemoteError) Error() string {
	if e.Class == "" {
		return e.Message
	}
	return e.Class + ": " + e.Message
}

type CompletionKind uint8

const (
	CompletionFuture CompletionKind = iota
	CompletionTimer
)

type Completion struct {
	Kind      CompletionKind
	ID        uint64
	Value     any
	Error     *RemoteError
	Cancelled bool
}

type Runner interface {
	Run(class string, payload any) (any, *RemoteError)
	SpawnActor(id uint64, class string, payload any) *RemoteError
	CallActor(id uint64, method string, payload any) (any, *RemoteError)
	StopActor(id uint64) *RemoteError
	Close()
}

type RunnerFactory func(workerID int) (Runner, error)
type NativeFunc func(context.Context, any) (any, error)

type Scheduler struct {
	config      Config
	ctx         context.Context
	cancel      context.CancelFunc
	workers     []*worker
	completions chan Completion
	closed      atomic.Bool
	submitMu    sync.RWMutex
	nextID      atomic.Uint64
	nextWorker  atomic.Uint64
	wg          sync.WaitGroup
	mu          sync.Mutex
	futures     map[uint64]*futureState
	actors      map[uint64]*actorState
	timers      map[uint64]*timerState
	providers   map[string]NativeFunc
	stats       Stats
}

type futureState struct {
	cancel    context.CancelFunc
	cancelled bool
	settled   bool
}

type actorState struct {
	worker  int
	pending int
	stopped bool
}

type timerState struct {
	stop chan struct{}
}

type Stats struct {
	Workers             int
	BusyWorkers         int64
	QueuedTasks         int
	RunningTasks        uint64
	CompletedTasks      uint64
	FailedTasks         uint64
	CancelledTasks      uint64
	TimedOutTasks       uint64
	Actors              int
	QueuedActorMessages int
	Timers              int
	CompletionQueue     int
}

type workKind uint8

const (
	workTask workKind = iota
	workActorSpawn
	workActorCall
	workActorStop
	workNative
)

type work struct {
	kind       workKind
	id         uint64
	actorID    uint64
	class      string
	method     string
	provider   NativeFunc
	payload    any
	ctx        context.Context
	actorCount bool
}

type worker struct {
	id     int
	runner Runner
	queue  chan work
	busy   atomic.Bool
}

func New(config Config, factory RunnerFactory) (*Scheduler, error) {
	config = normalizeConfig(config)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		config: config, ctx: ctx, cancel: cancel,
		completions: make(chan Completion, config.CompletionQueue),
		futures:     make(map[uint64]*futureState), actors: make(map[uint64]*actorState),
		timers: make(map[uint64]*timerState), providers: make(map[string]NativeFunc),
	}
	for id := range config.Workers {
		runner, err := factory(id + 1)
		if err != nil {
			s.Close()
			return nil, fmt.Errorf("start PHP worker %d: %w", id+1, err)
		}
		w := &worker{id: id + 1, runner: runner, queue: make(chan work, config.TaskQueue)}
		s.workers = append(s.workers, w)
		s.wg.Add(1)
		go s.workerLoop(w)
	}
	return s, nil
}

func normalizeConfig(config Config) Config {
	defaults := DefaultConfig()
	if config.Workers <= 0 {
		config.Workers = defaults.Workers
	}
	if config.TaskQueue <= 0 {
		config.TaskQueue = defaults.TaskQueue
	}
	if config.CompletionQueue <= 0 {
		config.CompletionQueue = defaults.CompletionQueue
	}
	if config.ActorMailbox <= 0 {
		config.ActorMailbox = defaults.ActorMailbox
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = defaults.ShutdownTimeout
	}
	return config
}

func (s *Scheduler) Submit(class string, payload any) (uint64, error) {
	return s.submit(work{kind: workTask, class: class, payload: payload}, -1)
}

func (s *Scheduler) RegisterNative(name string, fn NativeFunc) error {
	if name == "" || fn == nil {
		return errors.New("native async provider needs a name and function")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrClosed
	}
	if _, exists := s.providers[name]; exists {
		return fmt.Errorf("native async provider %q is already registered", name)
	}
	s.providers[name] = fn
	return nil
}

func (s *Scheduler) SubmitNative(name string, payload any) (uint64, error) {
	s.mu.Lock()
	provider := s.providers[name]
	s.mu.Unlock()
	if provider == nil {
		return 0, fmt.Errorf("native async provider %q: %w", name, ErrNotFound)
	}
	return s.submit(work{kind: workNative, provider: provider, payload: payload}, -1)
}

func (s *Scheduler) submit(item work, workerID int) (uint64, error) {
	s.submitMu.RLock()
	defer s.submitMu.RUnlock()
	if s.closed.Load() {
		return 0, ErrClosed
	}
	id := s.nextID.Add(1)
	ctx, cancel := context.WithCancel(s.ctx)
	item.id, item.ctx = id, ctx
	s.mu.Lock()
	s.futures[id] = &futureState{cancel: cancel}
	s.mu.Unlock()
	worker := s.pickWorker(workerID)
	select {
	case worker.queue <- item:
		return id, nil
	default:
		s.mu.Lock()
		delete(s.futures, id)
		s.mu.Unlock()
		cancel()
		return 0, ErrOverloaded
	}
}

func (s *Scheduler) SpawnActor(class string, payload any) (uint64, uint64, error) {
	if s.closed.Load() {
		return 0, 0, ErrClosed
	}
	actorID := s.nextID.Add(1)
	workerID := int(s.nextWorker.Add(1)-1) % len(s.workers)
	s.mu.Lock()
	s.actors[actorID] = &actorState{worker: workerID}
	s.mu.Unlock()
	futureID, err := s.submit(work{kind: workActorSpawn, actorID: actorID, class: class, payload: payload}, workerID)
	if err != nil {
		s.mu.Lock()
		delete(s.actors, actorID)
		s.mu.Unlock()
		return 0, 0, err
	}
	return actorID, futureID, nil
}

func (s *Scheduler) CallActor(actorID uint64, method string, payload any) (uint64, error) {
	s.mu.Lock()
	actor := s.actors[actorID]
	if actor == nil {
		s.mu.Unlock()
		return 0, ErrNotFound
	}
	if actor.stopped {
		s.mu.Unlock()
		return 0, ErrActorStopped
	}
	if actor.pending >= s.config.ActorMailbox {
		s.mu.Unlock()
		return 0, ErrActorFull
	}
	actor.pending++
	workerID := actor.worker
	s.mu.Unlock()
	id, err := s.submit(work{kind: workActorCall, actorID: actorID, method: method, payload: payload, actorCount: true}, workerID)
	if err != nil {
		s.decrementActor(actorID)
	}
	return id, err
}

func (s *Scheduler) StopActor(actorID uint64) (uint64, error) {
	s.mu.Lock()
	actor := s.actors[actorID]
	if actor == nil {
		s.mu.Unlock()
		return 0, ErrNotFound
	}
	if actor.stopped {
		s.mu.Unlock()
		return 0, ErrActorStopped
	}
	actor.stopped = true
	workerID := actor.worker
	s.mu.Unlock()
	id, err := s.submit(work{kind: workActorStop, actorID: actorID}, workerID)
	if err != nil {
		s.mu.Lock()
		if current := s.actors[actorID]; current != nil {
			current.stopped = false
		}
		s.mu.Unlock()
	}
	return id, err
}

func (s *Scheduler) Cancel(id uint64) bool {
	return s.cancelFuture(id, false)
}

func (s *Scheduler) cancelFuture(id uint64, timeout bool) bool {
	s.mu.Lock()
	future := s.futures[id]
	if future == nil || future.cancelled || future.settled {
		s.mu.Unlock()
		return false
	}
	completion := Completion{Kind: CompletionFuture, ID: id, Cancelled: !timeout}
	if timeout {
		completion.Error = &RemoteError{Class: "TimeoutException", Message: "The asynchronous operation timed out.", TaskID: id}
	}
	if timeout {
		select {
		case s.completions <- completion:
		case <-s.ctx.Done():
			s.mu.Unlock()
			return false
		}
	} else {
		select {
		case s.completions <- completion:
		default:
			s.mu.Unlock()
			return false
		}
	}
	future.cancelled = true
	future.settled = true
	future.cancel()
	if timeout {
		s.stats.TimedOutTasks++
	} else {
		s.stats.CancelledTasks++
	}
	s.mu.Unlock()
	return true
}

func (s *Scheduler) Timeout(id uint64, duration time.Duration) error {
	if duration <= 0 {
		return errors.New("timeout must be greater than zero")
	}
	s.mu.Lock()
	if s.futures[id] == nil {
		s.mu.Unlock()
		return ErrNotFound
	}
	s.mu.Unlock()
	time.AfterFunc(duration, func() {
		s.cancelFuture(id, true)
	})
	return nil
}

func (s *Scheduler) TimerAfter(duration time.Duration, repeat bool) (uint64, error) {
	s.submitMu.RLock()
	defer s.submitMu.RUnlock()
	if duration <= 0 {
		return 0, errors.New("timer interval must be greater than zero")
	}
	if s.closed.Load() {
		return 0, ErrClosed
	}
	id := s.nextID.Add(1)
	state := &timerState{stop: make(chan struct{})}
	s.mu.Lock()
	s.timers[id] = state
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		timer := time.NewTimer(duration)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				s.deliver(Completion{Kind: CompletionTimer, ID: id})
				if !repeat {
					s.mu.Lock()
					delete(s.timers, id)
					s.mu.Unlock()
					return
				}
				timer.Reset(duration)
			case <-state.stop:
				return
			case <-s.ctx.Done():
				return
			}
		}
	}()
	return id, nil
}

func (s *Scheduler) CancelTimer(id uint64) bool {
	s.mu.Lock()
	timer := s.timers[id]
	if timer != nil {
		delete(s.timers, id)
	}
	s.mu.Unlock()
	if timer == nil {
		return false
	}
	close(timer.stop)
	return true
}

func (s *Scheduler) Drain(limit int) []Completion {
	if limit <= 0 {
		return nil
	}
	result := make([]Completion, 0, limit)
	for len(result) < limit {
		select {
		case completion := <-s.completions:
			result = append(result, completion)
		default:
			return result
		}
	}
	return result
}

func (s *Scheduler) Stats() Stats {
	s.mu.Lock()
	stats := s.stats
	stats.Actors, stats.Timers = len(s.actors), len(s.timers)
	for _, actor := range s.actors {
		stats.QueuedActorMessages += actor.pending
	}
	s.mu.Unlock()
	stats.Workers = len(s.workers)
	stats.CompletionQueue = len(s.completions)
	for _, worker := range s.workers {
		stats.QueuedTasks += len(worker.queue)
		if worker.busy.Load() {
			stats.BusyWorkers++
		}
	}
	return stats
}

func (s *Scheduler) Close() {
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.cancel()
	s.mu.Lock()
	for _, future := range s.futures {
		future.cancel()
	}
	for _, timer := range s.timers {
		close(timer.stop)
	}
	s.timers = make(map[uint64]*timerState)
	s.mu.Unlock()
	for _, worker := range s.workers {
		close(worker.queue)
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		for _, worker := range s.workers {
			worker.runner.Close()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(s.config.ShutdownTimeout):
	}
}

func (s *Scheduler) pickWorker(workerID int) *worker {
	if workerID >= 0 {
		return s.workers[workerID]
	}
	return s.workers[int(s.nextWorker.Add(1)-1)%len(s.workers)]
}

func (s *Scheduler) workerLoop(worker *worker) {
	defer s.wg.Done()
	for item := range worker.queue {
		if item.ctx.Err() != nil {
			if item.actorCount {
				s.decrementActor(item.actorID)
			}
			continue
		}
		worker.busy.Store(true)
		s.mu.Lock()
		s.stats.RunningTasks++
		s.mu.Unlock()
		value, remote := s.execute(worker, item)
		if item.kind == workActorSpawn && remote != nil {
			s.mu.Lock()
			delete(s.actors, item.actorID)
			s.mu.Unlock()
		}
		if item.kind == workActorStop {
			s.mu.Lock()
			delete(s.actors, item.actorID)
			s.mu.Unlock()
		}
		worker.busy.Store(false)
		s.mu.Lock()
		if s.stats.RunningTasks > 0 {
			s.stats.RunningTasks--
		}
		s.mu.Unlock()
		if item.actorCount {
			s.decrementActor(item.actorID)
		}
		if remote != nil {
			remote.WorkerID, remote.TaskID = worker.id, item.id
		}
		s.mu.Lock()
		future := s.futures[item.id]
		cancelled := future == nil || future.cancelled || future.settled
		if !cancelled {
			future.settled = true
			if remote == nil {
				s.stats.CompletedTasks++
			} else {
				s.stats.FailedTasks++
			}
		}
		s.mu.Unlock()
		if !cancelled {
			s.deliver(Completion{Kind: CompletionFuture, ID: item.id, Value: value, Error: remote})
		}
	}
}

func (s *Scheduler) execute(worker *worker, item work) (value any, remote *RemoteError) {
	defer func() {
		if recovered := recover(); recovered != nil {
			value = nil
			remote = &RemoteError{Class: "WorkerPanic", Message: fmt.Sprintf("worker panicked: %v", recovered)}
		}
	}()
	switch item.kind {
	case workTask:
		value, remote = worker.runner.Run(item.class, item.payload)
	case workNative:
		var err error
		value, err = item.provider(item.ctx, item.payload)
		if err != nil {
			remote = &RemoteError{Class: "NativeTaskError", Message: err.Error()}
		}
	case workActorSpawn:
		remote = worker.runner.SpawnActor(item.actorID, item.class, item.payload)
	case workActorCall:
		value, remote = worker.runner.CallActor(item.actorID, item.method, item.payload)
	case workActorStop:
		remote = worker.runner.StopActor(item.actorID)
	}
	return value, remote
}

func (s *Scheduler) deliver(completion Completion) {
	select {
	case s.completions <- completion:
	case <-s.ctx.Done():
	}
}

func (s *Scheduler) Acknowledge(id uint64) {
	s.mu.Lock()
	if future := s.futures[id]; future != nil {
		future.cancel()
		delete(s.futures, id)
	}
	s.mu.Unlock()
}

func (s *Scheduler) decrementActor(id uint64) {
	s.mu.Lock()
	if actor := s.actors[id]; actor != nil && actor.pending > 0 {
		actor.pending--
	}
	s.mu.Unlock()
}
