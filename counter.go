package cloud_metrics

import (
	"sync/atomic"
)

// counter implements a thread-safe counter that can be used to record a tally which is
// racked up through calling counter.Count.
type counter struct {

	// count tracks the current running total provided to this counter through
	// calls to counter.Count.
	count *int64
}

// newCounter returns an instantiated counter, storing the provided metric information
// for reporting later.
func newCounter() *counter {

	var zero int64 = 0

	return &counter{
		count: &zero,
	}
}

// Count adds 1 to the running total of this counter.
func (c *counter) Count() {
	atomic.AddInt64(c.count, 1)
}

// fetchAndReset resets the counter to zero and returns the count previously stored by it.
func (c *counter) fetchAndReset() int64 {
	return atomic.SwapInt64(c.count, 0)
}
