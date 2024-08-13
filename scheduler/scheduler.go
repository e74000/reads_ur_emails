package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// New creates a new *Scheduler
func New() *Scheduler {
	return &Scheduler{
		tasks:   make(map[uint64]*Task),
		taskMus: make(map[uint64]*sync.Mutex),

		run: make(chan uint64, 256),
		add: make(chan *Task, 256),
		del: make(chan uint64, 256),

		logger: slog.Default(),
	}
}

type Scheduler struct {
	tasks   map[uint64]*Task
	tasksMu sync.Mutex
	nextID  atomic.Uint64
	stopped atomic.Bool

	taskMus      map[uint64]*sync.Mutex
	taskMusMu    sync.Mutex
	globalTaskMu sync.RWMutex

	run chan uint64
	add chan *Task
	del chan uint64

	logger *slog.Logger
}

// SetLogger allows users to set a custom logger.
func (s *Scheduler) SetLogger(logger *slog.Logger) *Scheduler {
	s.logger = logger
	return s
}

func (s *Scheduler) Add(task *Task) uint64 {
	task.id = s.nextID.Add(1)
	s.logger.Debug("Adding task", "task_id", task.id)
	s.add <- task
	return task.id
}

func (s *Scheduler) Del(id uint64) {
	s.logger.Debug("Deleting task", "task_id", id)
	s.del <- id
}

// Run starts the scheduler to run tasks at their specified intervals.
func (s *Scheduler) Run(ctx context.Context) {
	s.logger.Debug("Scheduler started")
	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("Scheduler shutting down")
			s.Stop()
			return

		case id, ok := <-s.run:
			if !ok {
				return
			}

			s.tasksMu.Lock()
			task, exists := s.tasks[id]
			s.tasksMu.Unlock()

			if !exists {
				s.logger.Warn("Task does not exist", "task_id", id)
				continue
			}

			// fetch task and time until next run
			next, ok := task.next()

			if ok { // if task is due to run again, schedule it
				s.logger.Debug("Scheduling task", "task_id", task.id, "next_run", next)
				task.timer = time.AfterFunc(next, s.taskCallbackGenerator(id))
				s.tasksMu.Lock()
				s.tasks[id] = task
				s.tasksMu.Unlock()
			} else { // otherwise dispose of the task
				s.logger.Debug("Disposing task", "task_id", task.id)
				s.delTask(task.id)
			}

			// run task
			go s.taskRunner(task)

		case task, ok := <-s.add:
			if !ok {
				return
			}

			s.addTask(task)

		case id, ok := <-s.del:
			if !ok {
				return
			}

			s.delTask(id)
		}
	}
}

func (s *Scheduler) Stop() {
	s.logger.Debug("Stopping scheduler")

	s.stopped.Store(true)

	// Stop all active tasks
	s.tasksMu.Lock()
	for id, task := range s.tasks {
		if task.timer != nil {
			task.timer.Stop()
		}
		delete(s.tasks, id)
	}
	s.tasksMu.Unlock()

	// Clear task mutexes
	s.taskMusMu.Lock()
	for id := range s.taskMus {
		delete(s.taskMus, id)
	}
	s.taskMusMu.Unlock()

	// Close channels to prevent any further task addition
	close(s.run)
	close(s.add)
	close(s.del)
}

func (s *Scheduler) addTask(task *Task) {
	s.tasksMu.Lock()
	s.tasks[task.id] = task
	s.tasksMu.Unlock()

	s.taskMusMu.Lock()
	s.taskMus[task.id] = new(sync.Mutex)
	s.taskMusMu.Unlock()

	s.logger.Debug("Task added", "task_id", task.id)

	// Schedule the task immediately
	next, ok := task.next()
	if ok {
		s.logger.Debug("Scheduling task", "task_id", task.id, "next_run", next)
		task.timer = time.AfterFunc(next, s.taskCallbackGenerator(task.id))
		s.tasksMu.Lock()
		s.tasks[task.id] = task
		s.tasksMu.Unlock()
	} else {
		s.logger.Debug("Disposing task", "task_id", task.id)
		s.delTask(task.id)
	}
}

func (s *Scheduler) delTask(id uint64) {
	s.tasksMu.Lock()
	if task, exists := s.tasks[id]; exists {
		if task.timer != nil {
			task.timer.Stop()
		}

		delete(s.tasks, id)
	}
	s.tasksMu.Unlock()

	s.taskMusMu.Lock()
	delete(s.taskMus, id)
	s.taskMusMu.Unlock()

	s.logger.Debug("Task deleted", "task_id", id)
}

func (s *Scheduler) taskRunner(task *Task) {
	switch task.blocking {
	case nonBlocking:
		s.globalTaskMu.RLock()
		defer s.globalTaskMu.RUnlock()
	case blocking:
		s.taskMusMu.Lock()
		taskMu := s.taskMus[task.id]
		s.taskMusMu.Unlock()

		taskMu.Lock()
		defer taskMu.Unlock()

		s.globalTaskMu.RLock()
		defer s.globalTaskMu.RUnlock()
	case globalBlocking:
		s.globalTaskMu.Lock()
		defer s.globalTaskMu.Unlock()
	default:
		s.logger.Error("unknown blocking mode!", "task_id", task.id)
		panic("unknown blocking mode!")
	}

	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Task panicked", "task_id", task.id, "panic", r)
		}
	}()
	if err := task.job(); err != nil {
		s.logger.Error("Task returned error", "task_id", task.id, "error", err)
	} else {
		s.logger.Debug("Task completed successfully", "task_id", task.id)
	}
}

func (s *Scheduler) taskCallbackGenerator(id uint64) func() {
	return func() {
		if !s.stopped.Load() { // check before sending to the channel
			s.run <- id
		}
	}
}
