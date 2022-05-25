package cloud_metrics

import (
	"sync/atomic"
)

// Counter implements a thread-safe Counter that can be used to record a tally which is
// racked up through calling Counter.Count.
type Counter struct {

	// count tracks the current running total provided to this Counter through
	// calls to Counter.Count.
	count *int64
}

// newCounter returns an instantiated Counter, storing the provided metric information
// for reporting later.
func newCounter() *Counter {

	var zero int64 = 0

	return &Counter{
		count: &zero,
	}
}

// Count adds 1 to the running total of this Counter.
func (c *Counter) Count() {
	atomic.AddInt64(c.count, 1)
}

// fetchAndReset resets the Counter to zero and returns the count previously stored by it.
func (c *Counter) fetchAndReset() int64 {
	return atomic.SwapInt64(c.count, 0)
}
