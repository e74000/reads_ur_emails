# Scheduler Library Documentation

## Overview

The `scheduler` package provides a flexible and powerful task scheduling system. It allows users to schedule tasks with different timing configurations and manage the execution of tasks with various blocking modes.

## Table of Contents

- [Scheduler](#scheduler)
    - [New](#new)
    - [SetLogger](#setlogger)
    - [Add](#add)
    - [Del](#del)
    - [Run](#run)
    - [Stop](#stop)
- [Task](#task)
    - [NewTask](#newtask)
    - [Once](#once)
    - [Every](#every)
    - [RandomInterval](#randominterval)
    - [Daily](#daily)
    - [Weekly](#weekly)
    - [Monthly](#monthly)
    - [Times](#times)
    - [Forever](#forever)
    - [NonBlocking](#nonblocking)
    - [Blocking](#blocking)
    - [GlobalBlocking](#globalblocking)

## Scheduler

The `Scheduler` struct is the core of the package, managing the scheduling and execution of tasks.

### `New`

```go
func New() *Scheduler
```

Creates a new `Scheduler` instance.

### `SetLogger`

```go
func (s *Scheduler) SetLogger(logger *slog.Logger) *Scheduler
```

Sets a custom logger for the scheduler.

### `Add`

```go
func (s *Scheduler) Add(task *Task) uint64
```

Adds a task to the scheduler. Returns the task's ID.

### `Del`

```go
func (s *Scheduler) Del(id uint64)
```

Deletes a task from the scheduler using its ID.

### `Run`

```go
func (s *Scheduler) Run(ctx context.Context)
```

Starts the scheduler, allowing it to run tasks at their specified intervals. Should be called in a goroutine.

### `Stop`

```go
func (s *Scheduler) Stop()
```

Stops the scheduler and cancels all running tasks.

## Task

The `Task` struct represents a job to be scheduled.

### `NewTask`

```go
func NewTask(job func() error) *Task
```

Creates a new `Task` instance with the specified job function.

### `Once`

```go
func (t *Task) Once() *Task
```

Schedules the task to run once and then self-cancel.

### `Every`

```go
func (t *Task) Every(duration time.Duration) *Task
```

Schedules the task to run every specified duration.

### `RandomInterval`

```go
func (t *Task) RandomInterval(min, max time.Duration) *Task
```

Schedules the task to run at random intervals between the specified min and max durations.

### `Daily`

```go
func (t *Task) Daily(at time.Time) *Task
```

Schedules the task to run daily at a specific time.

### `Weekly`

```go
func (t *Task) Weekly(days map[time.Weekday]bool, at time.Time) *Task
```

Schedules the task to run weekly on specified days at a specific time.

### `Monthly`

```go
func (t *Task) Monthly(months map[time.Month]bool, on int, at time.Time) *Task
```

Schedules the task to run monthly on specified months, on a specific day, at a specific time.

### `Times`

```go
func (t *Task) Times(times int) *Task
```

Limits the task to running a specific number of times before self-cancelling.

### `Forever`

```go
func (t *Task) Forever() *Task
```

Schedules the task to run indefinitely.

### `NonBlocking`

```go
func (t *Task) NonBlocking() *Task
```

Allows multiple instances of this task to run simultaneously.

### `Blocking`

```go
func (t *Task) Blocking() *Task
```

Ensures only one instance of this task can run at once.

### `GlobalBlocking`

```go
func (t *Task) GlobalBlocking() *Task
```

Ensures that the task can be the only task running at a given time.

## Additional Notes

- **Task Variants**:
    - `once`: Runs the task once.
    - `every`: Runs the task at regular intervals.
    - `random`: Runs the task at random intervals.
    - `daily`: Runs the task daily at a specific time.
    - `weekly`: Runs the task weekly on specified days.
    - `monthly`: Runs the task monthly on specified months and days.

- **Blocking Modes**:
    - `nonBlocking`: Allows multiple instances of the task to run simultaneously.
    - `blocking`: Ensures only one instance of the task runs at a time.
    - `globalBlocking`: Prevents any other tasks from running while this task is active.