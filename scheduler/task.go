package scheduler

import (
	"math/rand"
	"time"
)

type taskVariant uint8

const (
	once taskVariant = iota
	every
	random
	daily
	weekly
	monthly
)

type blockingMode uint8

const (
	nonBlocking blockingMode = iota
	blocking
	globalBlocking
)

func NewTask(job func() error) *Task {
	return &Task{
		job: job,

		variant: once,
		times:   -1,

		blocking: nonBlocking,
	}
}

// Task represents a job to be scheduled
type Task struct {
	// main values
	id    uint64       // id is a unique identifier for the task. will be set automatically - do not set manually
	job   func() error // job is the task to be run
	timer *time.Timer  // timer can be used to cancel the next scheduled task

	// scheduling information
	variant  taskVariant           // variant represents the type of task scheduling to use
	duration time.Duration         // duration represents the frequency to run at
	at       time.Time             // at represents the time of day to run at
	days     map[time.Weekday]bool // days represents the days of the week to run on
	months   map[time.Month]bool   // months represents the months of the year to run on
	on       int                   // on represents the day of the month to run on
	times    int                   // times represents the number of times to run. -1 represents running indefinitely
	randMin  time.Duration         // randMin represents the minimum duration a random task variant could take
	randMax  time.Duration         // randMax represents the maximum duration a random task variant could take

	// other options
	blocking blockingMode
}

// Once runs the task once, and then self-cancels
// if overridden with Times(n), it will behave the same as Every(0) n times.
// if overridden with Forever(), it will behave the same as Every(0)
func (t *Task) Once() *Task {
	t.variant = once
	t.times = 1
	return t
}

// Every runs the task every [duration]
func (t *Task) Every(duration time.Duration) *Task {
	if duration < 0 {
		panic("duration must be a positive value")
	}
	t.variant = every
	t.duration = duration
	return t
}

// RandomInterval runs the task at random intervals between min and max duration
func (t *Task) RandomInterval(min, max time.Duration) *Task {
	if min < 0 || max < 0 {
		panic("both min and max duration must be a positive value")
	}

	if min >= max {
		panic("min duration must be less than max duration")
	}

	t.variant = random
	t.randMin = min
	t.randMax = max
	return t
}

// Daily runs the task every day [at] a specific time
func (t *Task) Daily(at time.Time) *Task {
	if at.IsZero() {
		panic("at time must be a valid non-zero time")
	}
	t.variant = daily
	t.at = at
	return t
}

// Weekly runs the task weekly on specified [days] [at] a specific time
func (t *Task) Weekly(days map[time.Weekday]bool, at time.Time) *Task {
	if len(days) == 0 {
		panic("days map cannot be empty")
	}
	if at.IsZero() {
		panic("at time must be a valid non-zero time")
	}
	t.variant = weekly
	t.days = days
	t.at = at
	return t
}

// Monthly runs the task monthly on specified [months], [on] a specific day, [at] a specific time
func (t *Task) Monthly(months map[time.Month]bool, on int, at time.Time) *Task {
	if len(months) == 0 {
		panic("months map cannot be empty")
	}
	if on <= 0 || on > 31 {
		panic("on must be a valid day of the month (1-31)")
	}
	if at.IsZero() {
		panic("at time must be a valid non-zero time")
	}
	t.variant = monthly
	t.months = months
	t.on = on
	t.at = at
	return t
}

// Times is used to limit the task to running a specific number of times, before self-cancelling
func (t *Task) Times(times int) *Task {
	if times <= 0 {
		panic("the task must be run a positive integer number of times")
	}
	t.times = times
	return t
}

// Forever sets a specific task to run forever. This is the default behaviour of tasks.
// this is used to override the default behaviour of certain task variants, such as once.
func (t *Task) Forever() *Task {
	t.times = -1
	return t
}

// NonBlocking allows multiple instances of this task to run simultaneously
func (t *Task) NonBlocking() *Task {
	t.blocking = nonBlocking
	return t
}

// Blocking ensures only one instance of this task can run at once
func (t *Task) Blocking() *Task {
	t.blocking = blocking
	return t
}

// GlobalBlocking ensures that the task can be the only task running at a given time
func (t *Task) GlobalBlocking() *Task {
	t.blocking = globalBlocking
	return t
}

// next evaluates when and whether the task should be scheduled to run next
func (t *Task) next() (time.Duration, bool) {
	now := time.Now()

	if t.times == 0 {
		return 0, false
	}
	if t.times > 0 {
		t.times--
	}

	var nextRun time.Time
	var found bool

	switch t.variant {
	// run once immediately
	case once:
		nextRun = now

	// run every specified duration
	case every:
		nextRun = now.Add(t.duration)

	// run at random intervals between min and max duration
	case random:
		nextRun = now.Add(t.randMin + time.Duration(rand.Int63n(int64(t.randMax-t.randMin))))

	// run daily at a specific time
	case daily:
		nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.at.Hour(), t.at.Minute(), t.at.Second(), 0, now.Location())
		if nextRun.Before(now) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// run weekly on specified days at a specific time
	case weekly:
		if t.days == nil {
			return 0, false
		}

		// Initialize nextRun to the scheduled time today
		nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.at.Hour(), t.at.Minute(), t.at.Second(), 0, now.Location())

		// If the scheduled time for today has already passed, move to the next day
		if !nextRun.After(now) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Loop through the next 7 days to find the next valid day
		for i := 0; i < 7; i++ {
			if t.days[nextRun.Weekday()] {
				// If the nextRun is in the future and falls on a valid day, stop here
				found = true
				break
			}
			// Otherwise, move to the next day
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Self-cancel if no valid day is found
		if !found {
			return 0, false
		}

	// run monthly on specified months, on a specific day, at a specific time
	case monthly:
		if t.months == nil || t.on <= 0 || t.on > 31 {
			return 0, false
		}
		year, month := now.Year(), now.Month()
		if now.Day() > t.on || (now.Day() == t.on && now.After(time.Date(year, month, t.on, t.at.Hour(), t.at.Minute(), t.at.Second(), 0, now.Location()))) {
			month++
			if month > 12 {
				month = 1
				year++
			}
		}
		found = false
		for i := 0; i < 12; i++ {
			if t.months[month] {
				found = true
				break
			}
			month++
			if month > 12 {
				month = 1
				year++
			}
		}
		// self-cancel if there are no months
		if !found {
			return 0, false
		}
		nextRun = time.Date(year, month, t.on, t.at.Hour(), t.at.Minute(), t.at.Second(), 0, now.Location())

	default:
		// handle unknown task variant
		panic("unknown task variant!")
	}

	return nextRun.Sub(now), true
}
