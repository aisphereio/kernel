package taskx

import (
	"errors"
	"time"
)

type everySchedule struct {
	interval time.Duration
}

// Every returns a fixed-rate schedule. Missed ticks are skipped rather than
// replayed when a service was stopped or a previous run took too long.
func Every(interval time.Duration) Schedule {
	return everySchedule{interval: interval}
}

func (s everySchedule) Next(after time.Time) time.Time {
	if s.interval <= 0 {
		return time.Time{}
	}
	return after.Add(s.interval)
}

func (s everySchedule) validate() error {
	if s.interval <= 0 {
		return errors.New("interval must be greater than zero")
	}
	return nil
}

type atSchedule struct {
	at time.Time
}

// At returns a one-shot schedule.
func At(at time.Time) Schedule {
	return atSchedule{at: at}
}

func (s atSchedule) Next(after time.Time) time.Time {
	if s.at.After(after) {
		return s.at
	}
	return time.Time{}
}

func (s atSchedule) validate() error {
	if s.at.IsZero() {
		return errors.New("run time is required")
	}
	return nil
}

type scheduleFunc func(time.Time) time.Time

// ScheduleFunc adapts a function to Schedule. The function must return either
// zero or a time strictly after its argument.
func ScheduleFunc(fn func(time.Time) time.Time) Schedule {
	return scheduleFunc(fn)
}

func (f scheduleFunc) Next(after time.Time) time.Time {
	if f == nil {
		return time.Time{}
	}
	return f(after)
}

func (f scheduleFunc) validate() error {
	if f == nil {
		return errors.New("schedule function is required")
	}
	return nil
}
