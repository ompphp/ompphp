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
	NativeWorkers   int
	NativeQueue     int
	FutureLimit     int
	CompletionQueue int
	ActorMailbox    int
	ShutdownTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{Workers: 4, TaskQueue: 256, NativeWorkers: 8, NativeQueue: 256, CompletionQueue: 512, ActorMailbox: 64, ShutdownTimeout: 5 * time.Second}
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
	nativeQueue chan work
	nativeBusy  atomic.Int64
	completions chan Completion
	closed      atomic.Bool
	submitMu    sync.RWMutex
	nextID      atomic.Uint64
	nextWorker  atomic.Uint64
	wg          sync.WaitGroup
	mu          sync.Mutex
	futures     map[uint64]*futureState
	actors      map[uint64]*actorState
	pools       map[uint64]*poolState
	timers      map[uint64]*timerState
	providers   map[string]NativeFunc
	stats       Stats
}

type futureState struct {
	cancel     context.CancelFunc
	timeout    *time.Timer
	cancelled  bool
	settled    bool
	notified   bool
	completion *Completion
}

type actorState struct {
	worker  int
	poolID  uint64
	pending int
	stopped bool
}

type poolState struct {
	actors   map[uint64]struct{}
	stopping bool
}

type timerState struct {
	stop chan struct{}
}

type Stats struct {
	Workers             int
	BusyWorkers         int64
	QueuedTasks         int
	PHPWorkers          int
	BusyPHPWorkers      int64
	QueuedPHPTasks      int
	NativeWorkers       int
	BusyNativeWorkers   int64
	QueuedNativeTasks   int
	RunningTasks        uint64
	CompletedTasks      uint64
	FailedTasks         uint64
	CancelledTasks      uint64
	TimedOutTasks       uint64
	Actors              int
	ActorPools          int
	QueuedActorMessages int
	PendingFutures      int
	Timers              int
	CompletionQueue     int
}

type workKind uint8

const (
	workTask workKind = iota
	workActorSpawn
	workActorCall
	workActorStop
)

type ActorHandle struct {
	ActorID  uint64
	FutureID uint64
}

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
		nativeQueue: make(chan work, config.NativeQueue),
		futures:     make(map[uint64]*futureState), actors: make(map[uint64]*actorState),
		pools: make(map[uint64]*poolState), timers: make(map[uint64]*timerState), providers: make(map[string]NativeFunc),
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
	for range config.NativeWorkers {
		s.wg.Add(1)
		go s.nativeLoop()
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
	if config.NativeWorkers <= 0 {
		config.NativeWorkers = defaults.NativeWorkers
	}
	if config.NativeQueue <= 0 {
		config.NativeQueue = defaults.NativeQueue
	}
	if config.FutureLimit <= 0 {
		config.FutureLimit = config.Workers*(config.TaskQueue+1) + config.NativeWorkers + config.NativeQueue + config.CompletionQueue
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
	return s.submitNative(work{provider: provider, payload: payload})
}

func (s *Scheduler) submitNative(item work) (uint64, error) {
	s.submitMu.RLock()
	defer s.submitMu.RUnlock()
	if s.closed.Load() {
		return 0, ErrClosed
	}
	id := s.nextID.Add(1)
	ctx, cancel := context.WithCancel(s.ctx)
	item.id, item.ctx = id, ctx
	s.mu.Lock()
	if len(s.futures) >= s.config.FutureLimit {
		s.mu.Unlock()
		cancel()
		return 0, ErrOverloaded
	}
	s.futures[id] = &futureState{cancel: cancel}
	s.mu.Unlock()
	select {
	case s.nativeQueue <- item:
		return id, nil
	default:
		s.mu.Lock()
		delete(s.futures, id)
		s.mu.Unlock()
		cancel()
		return 0, ErrOverloaded
	}
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
	if len(s.futures) >= s.config.FutureLimit {
		s.mu.Unlock()
		cancel()
		return 0, ErrOverloaded
	}
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
	workerID := s.pickActorWorker()
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

func (s *Scheduler) SpawnActorPool(class string, count int, payload any) (uint64, []ActorHandle, error) {
	if count <= 0 {
		return 0, nil, errors.New("actor pool size must be greater than zero")
	}
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	if s.closed.Load() {
		return 0, nil, ErrClosed
	}
	placements := make([]int, count)
	loads := make([]int, len(s.workers))
	s.mu.Lock()
	if len(s.futures)+count > s.config.FutureLimit {
		s.mu.Unlock()
		return 0, nil, ErrOverloaded
	}
	for _, actor := range s.actors {
		loads[actor.worker]++
	}
	s.mu.Unlock()
	needed := make([]int, len(s.workers))
	for index := range count {
		best := 0
		for worker := 1; worker < len(loads); worker++ {
			if loads[worker]+needed[worker] < loads[best]+needed[best] {
				best = worker
			}
		}
		placements[index] = best
		needed[best]++
	}
	for worker, amount := range needed {
		if len(s.workers[worker].queue)+amount > cap(s.workers[worker].queue) {
			return 0, nil, ErrOverloaded
		}
	}
	poolID := s.nextID.Add(1)
	pool := &poolState{actors: make(map[uint64]struct{}, count)}
	s.mu.Lock()
	s.pools[poolID] = pool
	s.mu.Unlock()
	handles := make([]ActorHandle, 0, count)
	for _, workerID := range placements {
		actorID, futureID := s.nextID.Add(1), s.nextID.Add(1)
		ctx, cancel := context.WithCancel(s.ctx)
		s.mu.Lock()
		s.actors[actorID] = &actorState{worker: workerID, poolID: poolID}
		s.futures[futureID] = &futureState{cancel: cancel}
		pool.actors[actorID] = struct{}{}
		s.mu.Unlock()
		s.workers[workerID].queue <- work{kind: workActorSpawn, id: futureID, actorID: actorID, class: class, payload: payload, ctx: ctx}
		handles = append(handles, ActorHandle{ActorID: actorID, FutureID: futureID})
	}
	return poolID, handles, nil
}

func (s *Scheduler) StopActorPool(poolID uint64) ([]uint64, error) {
	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	if s.closed.Load() {
		return nil, ErrClosed
	}
	s.mu.Lock()
	pool := s.pools[poolID]
	if pool == nil {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	if pool.stopping {
		s.mu.Unlock()
		return nil, ErrActorStopped
	}
	actors := make([]uint64, 0, len(pool.actors))
	if len(s.futures)+len(pool.actors) > s.config.FutureLimit {
		s.mu.Unlock()
		return nil, ErrOverloaded
	}
	needed := make([]int, len(s.workers))
	for id := range pool.actors {
		actors = append(actors, id)
		actor := s.actors[id]
		if actor == nil || actor.stopped {
			s.mu.Unlock()
			return nil, ErrActorStopped
		}
		needed[actor.worker]++
	}
	for worker, amount := range needed {
		if len(s.workers[worker].queue)+amount > cap(s.workers[worker].queue) {
			s.mu.Unlock()
			return nil, ErrOverloaded
		}
	}
	pool.stopping = true
	futures := make([]uint64, 0, len(actors))
	for _, actor := range actors {
		state := s.actors[actor]
		state.stopped = true
		id := s.nextID.Add(1)
		ctx, cancel := context.WithCancel(s.ctx)
		s.futures[id] = &futureState{cancel: cancel}
		s.workers[state.worker].queue <- work{kind: workActorStop, id: id, actorID: actor, ctx: ctx}
		futures = append(futures, id)
	}
	s.mu.Unlock()
	return futures, nil
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
	future.cancelled = true
	future.settled = true
	future.completion = &completion
	if future.timeout != nil {
		future.timeout.Stop()
		future.timeout = nil
	}
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
	future := s.futures[id]
	if future == nil || future.settled {
		s.mu.Unlock()
		return ErrNotFound
	}
	if future.timeout != nil {
		future.timeout.Stop()
	}
	future.timeout = time.AfterFunc(duration, func() {
		s.cancelFuture(id, true)
	})
	s.mu.Unlock()
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
	s.mu.Lock()
	for _, future := range s.futures {
		if len(result) == limit {
			break
		}
		if future.completion != nil && !future.notified {
			result = append(result, *future.completion)
			future.notified = true
		}
	}
	s.mu.Unlock()
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
	stats.Actors, stats.ActorPools, stats.Timers = len(s.actors), len(s.pools), len(s.timers)
	stats.PendingFutures = len(s.futures)
	for _, actor := range s.actors {
		stats.QueuedActorMessages += actor.pending
	}
	s.mu.Unlock()
	stats.Workers, stats.PHPWorkers = len(s.workers), len(s.workers)
	stats.NativeWorkers = s.config.NativeWorkers
	stats.BusyNativeWorkers = s.nativeBusy.Load()
	stats.QueuedNativeTasks = len(s.nativeQueue)
	stats.CompletionQueue = len(s.completions)
	for _, worker := range s.workers {
		stats.QueuedTasks += len(worker.queue)
		if worker.busy.Load() {
			stats.BusyWorkers++
		}
	}
	stats.BusyPHPWorkers, stats.QueuedPHPTasks = stats.BusyWorkers, stats.QueuedTasks
	return stats
}

func (s *Scheduler) Close() {
	s.submitMu.Lock()
	if !s.closed.CompareAndSwap(false, true) {
		s.submitMu.Unlock()
		return
	}
	s.cancel()
	s.mu.Lock()
	for _, future := range s.futures {
		if future.timeout != nil {
			future.timeout.Stop()
			future.timeout = nil
		}
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
	close(s.nativeQueue)
	s.submitMu.Unlock()
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
	start := int(s.nextWorker.Add(1)-1) % len(s.workers)
	best := start
	bestLoad := len(s.workers[best].queue)
	if s.workers[best].busy.Load() {
		bestLoad++
	}
	for offset := 1; offset < len(s.workers); offset++ {
		candidate := (start + offset) % len(s.workers)
		load := len(s.workers[candidate].queue)
		if s.workers[candidate].busy.Load() {
			load++
		}
		if load < bestLoad {
			best, bestLoad = candidate, load
		}
	}
	return s.workers[best]
}

func (s *Scheduler) pickActorWorker() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	loads := make([]int, len(s.workers))
	for _, actor := range s.actors {
		loads[actor.worker]++
	}
	start := int(s.nextWorker.Add(1)-1) % len(s.workers)
	best := start
	for offset := 1; offset < len(s.workers); offset++ {
		candidate := (start + offset) % len(s.workers)
		if loads[candidate] < loads[best] {
			best = candidate
		}
	}
	return best
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
			s.removeActorLocked(item.actorID)
			s.mu.Unlock()
		}
		if item.kind == workActorStop {
			s.mu.Lock()
			s.removeActorLocked(item.actorID)
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
		s.finish(item.id, value, remote)
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
	case workActorSpawn:
		remote = worker.runner.SpawnActor(item.actorID, item.class, item.payload)
	case workActorCall:
		value, remote = worker.runner.CallActor(item.actorID, item.method, item.payload)
	case workActorStop:
		remote = worker.runner.StopActor(item.actorID)
	}
	return value, remote
}

func (s *Scheduler) nativeLoop() {
	defer s.wg.Done()
	for item := range s.nativeQueue {
		if item.ctx.Err() != nil {
			continue
		}
		s.nativeBusy.Add(1)
		s.mu.Lock()
		s.stats.RunningTasks++
		s.mu.Unlock()
		value, remote := func() (value any, remote *RemoteError) {
			defer func() {
				if recovered := recover(); recovered != nil {
					value = nil
					remote = &RemoteError{Class: "NativePanic", Message: fmt.Sprintf("native provider panicked: %v", recovered)}
				}
			}()
			result, err := item.provider(item.ctx, item.payload)
			if err != nil {
				return nil, &RemoteError{Class: "NativeTaskError", Message: err.Error()}
			}
			return result, nil
		}()
		s.nativeBusy.Add(-1)
		s.mu.Lock()
		if s.stats.RunningTasks > 0 {
			s.stats.RunningTasks--
		}
		s.mu.Unlock()
		s.finish(item.id, value, remote)
	}
}

func (s *Scheduler) finish(id uint64, value any, remote *RemoteError) {
	s.mu.Lock()
	future := s.futures[id]
	cancelled := future == nil || future.cancelled || future.settled
	if !cancelled {
		future.settled = true
		if future.timeout != nil {
			future.timeout.Stop()
			future.timeout = nil
		}
		if remote == nil {
			s.stats.CompletedTasks++
		} else {
			s.stats.FailedTasks++
		}
	}
	s.mu.Unlock()
	if !cancelled {
		s.deliver(Completion{Kind: CompletionFuture, ID: id, Value: value, Error: remote})
	}
}

func (s *Scheduler) removeActorLocked(id uint64) {
	actor := s.actors[id]
	delete(s.actors, id)
	if actor == nil || actor.poolID == 0 {
		return
	}
	pool := s.pools[actor.poolID]
	if pool == nil {
		return
	}
	delete(pool.actors, id)
	if len(pool.actors) == 0 {
		delete(s.pools, actor.poolID)
	}
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
		if future.timeout != nil {
			future.timeout.Stop()
			future.timeout = nil
		}
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
