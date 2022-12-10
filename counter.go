package quantify

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// count represents a tally over a duration of time.
type count struct {

	// start is used to mark the count's duration start time (inclusive)
	start time.Time

	// end is used to mark the count's duration end time (exclusive)
	end time.Time

	// count is the total recorded within the specified duration.
	count int64
}

// Counter implements a thread-safe Counter that can be used to record a tally which is
// racked up through calling Counter.Count.
type Counter struct {

	// interval is the number of seconds a single count should be tallied up
	// to before moving on to the next point.
	interval int64

	// counts is used to track the the running total of the counter in it's current
	// time frame. Each entry within this map represents the count over a provided
	// interval of time.
	counts *sync.Map

	mu *sync.Mutex

	// clock used to retrieve time.
	clock clock
}

// newCounter returns an instantiated Counter, storing the provided metric information
// for reporting later.
func newCounter(interval int64) (*Counter, error) {

	if interval <= 0 {
		return nil, errors.New("interval must be greater than 0")
	}

	return &Counter{
		clock:    &realClock{},
		interval: interval,
		counts:   &sync.Map{},
		mu:       &sync.Mutex{},
	}, nil
}

// Count adds 1 to the running total of this Counter.
func (c *Counter) Count() {

	var zero int64

	count, _ := c.counts.LoadOrStore(c.getKey(), &zero)

	atomic.AddInt64(count.(*int64), 1)
}

// getKey returns a unique key for the current time period using time.Now. The key
// represents the starting time of the period as seconds since epoch.
func (c *Counter) getKey() int64 {
	return c.clock.now().Truncate(time.Second * time.Duration(c.interval)).Unix()
}

// takePoints retrieves any outstanding counts for time intervals that have already
// passed, and removes them from the counter. If an interval is being counted actively
// when this is called, then that won't be retrieved until this is re-called after the
// time period has concluded.
func (c *Counter) takePoints() []*count {

	c.mu.Lock()

	currentFrame := c.getKey()

	completedCounts := make(map[int64]int64)

	c.counts.Range(func(key, value any) bool {

		keyInt := key.(int64)
		valueInt := *value.(*int64)

		if keyInt >= currentFrame {
			return false
		}

		completedCounts[keyInt] = valueInt

		c.counts.Delete(key)

		return true
	})

	c.mu.Unlock()

	response := make([]*count, 0)

	for k, v := range completedCounts {
		response = append(response, &count{
			start: time.Unix(k, 0),
			end:   time.Unix(k+c.interval, 0),
			count: v,
		})
	}

	return response
}
