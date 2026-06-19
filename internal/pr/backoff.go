package pr

import "time"

// Backoff schedules refreshes: the normal interval after a success, and an
// escalating retry delay after consecutive failures (capped at the last step).
type Backoff struct {
	normal time.Duration
	steps  []time.Duration
	fails  int
}

// NewBackoff returns a Backoff with the given normal interval and the fixed
// retry ladder 5, 10, 20, 40, 60 seconds.
func NewBackoff(normal time.Duration) *Backoff {
	return &Backoff{
		normal: normal,
		steps: []time.Duration{
			5 * time.Second, 10 * time.Second, 20 * time.Second,
			40 * time.Second, 60 * time.Second,
		},
	}
}

// RecordSuccess resets the failure count.
func (b *Backoff) RecordSuccess() { b.fails = 0 }

// RecordFailure increments the failure count.
func (b *Backoff) RecordFailure() { b.fails++ }

// Failures is the current consecutive-failure count.
func (b *Backoff) Failures() int { return b.fails }

// Delay is the time until the next refresh should run.
func (b *Backoff) Delay() time.Duration {
	if b.fails == 0 {
		return b.normal
	}
	i := b.fails - 1
	if i >= len(b.steps) {
		i = len(b.steps) - 1
	}
	return b.steps[i]
}
