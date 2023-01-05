package quantify

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
)

func TestGetKey(t *testing.T) {

	tests := []struct {
		name            string
		counterInterval int64
		time            time.Time
		expectedResult  int64
	}{
		{
			name:            "1 Second Interval",
			counterInterval: 1,
			time:            time.Unix(1670678947, 999999999), // 2022-12-10T1:29:07.999999999
			expectedResult:  1670678947,                       // 2022-12-10T1:29:07.0
		},
		{
			name:            "10 Second Interval",
			counterInterval: 10,
			time:            time.Unix(867280356, 123456789), // 1997-06-25T11:12:36.123456789
			expectedResult:  867280350,                       // 1997-06-25T11:12:30.0
		},
		{
			name:            "2 Minute Interval",
			counterInterval: 120,
			time:            time.Unix(1126727272, 5236478), // 2005-09-14T19:47:52.5236478
			expectedResult:  1126727160,                     // 2005-09-14T19:46:00.0
		},
	}

	for _, test := range tests {

		clock := clock.NewMock()
		clock.Set(test.time)

		counter := &Counter{
			clock:    clock,
			interval: test.counterInterval,
		}

		assert.Equalf(t, test.expectedResult, counter.getKey(), "%s: unexpected key", test.name)
	}
}

func TestCount(t *testing.T) {

	tests := []struct {
		name           string
		actions        []func(c *Counter)
		expectedResult int64
	}{
		{
			name: "Single Thread Count",
			actions: []func(c *Counter){
				func(c *Counter) {
					for i := 0; i < 50; i++ {
						c.Count()
					}
				},
			},
			expectedResult: 50,
		},
		{
			name: "Multi-thread Count",
			actions: []func(c *Counter){
				func(c *Counter) {
					wg := &sync.WaitGroup{}

					for i := 0; i < 75; i++ {

						wg.Add(1)

						go func() {
							defer wg.Done()
							for i := 0; i < 10; i++ {
								c.Count()
							}
						}()
					}

					wg.Wait()
				},
			},
			expectedResult: 750,
		},
	}

	for _, test := range tests {

		counter := &Counter{
			clock:  clock.NewMock(),
			counts: &sync.Map{},
			mu:     &sync.Mutex{},
		}

		for _, action := range test.actions {
			action(counter)
		}

		result, _ := counter.counts.Load(counter.getKey())
		assert.Equalf(t, test.expectedResult, *result.(*int64), "%s: unexpected count")
	}
}

func TestTakePoints(t *testing.T) {

	tests := []struct {
		name            string
		counterInterval int64
		startTime       time.Time
		setup           []func(*Counter)
		expectedResult  []*count
	}{
		{
			name:            "Single Thread, Multiple Instances",
			counterInterval: 10,
			startTime:       time.Unix(1670681776, 0), // 2022-10-12T14:16:16.0
			setup: []func(*Counter){

				// count 10
				func(c *Counter) {
					for i := 0; i < 10; i++ {
						c.Count()
					}
				},

				// increment time (10 seconds)
				func(c *Counter) {
					newTime := clock.NewMock()
					newTime.Set(c.clock.Now().Add(time.Second * 10))
					c.clock = newTime
				},

				// count 25
				func(c *Counter) {
					for i := 0; i < 25; i++ {
						c.Count()
					}
				},

				// increment time (10 seconds)
				func(c *Counter) {
					newTime := clock.NewMock()
					newTime.Set(c.clock.Now().Add(time.Second * 10))
					c.clock = newTime
				},
			},
			expectedResult: []*count{
				{
					start: time.Unix(1670681770, 0),
					end:   time.Unix(1670681780, 0),
					count: 10,
				},
				{
					start: time.Unix(1670681780, 0),
					end:   time.Unix(1670681790, 0),
					count: 25,
				},
			},
		},
		{
			name:            "Multi-threaded, Multiple Instances",
			counterInterval: 60,
			startTime:       time.Unix(1670681776, 0), // 2022-10-12T14:16:16.0
			setup: []func(*Counter){

				// count 250
				func(c *Counter) {

					wg := &sync.WaitGroup{}

					for i := 0; i < 25; i++ {

						wg.Add(1)

						go func() {
							defer wg.Done()
							for i := 0; i < 10; i++ {
								c.Count()
							}
						}()

						wg.Wait()
					}
				},

				// increment time (60 seconds)
				func(c *Counter) {
					newTime := clock.NewMock()
					newTime.Set(c.clock.Now().Add(time.Second * 60))
					c.clock = newTime
				},

				// increment 50
				func(c *Counter) {

					wg := &sync.WaitGroup{}

					for i := 0; i < 10; i++ {

						wg.Add(1)

						go func() {
							defer wg.Done()
							for i := 0; i < 5; i++ {
								c.Count()
							}
						}()

						wg.Wait()
					}
				},

				// increment time (60 seconds)
				func(c *Counter) {
					newTime := clock.NewMock()
					newTime.Set(c.clock.Now().Add(time.Second * 60))
					c.clock = newTime
				},
			},
			expectedResult: []*count{
				{
					start: time.Unix(1670681760, 0),
					end:   time.Unix(1670681820, 0),
					count: 250,
				},
				{
					start: time.Unix(1670681820, 0),
					end:   time.Unix(1670681880, 0),
					count: 50,
				},
			},
		},
		{
			name:            "Single Thread, Current Interval",
			counterInterval: 10,
			startTime:       time.Unix(1670681776, 0), // 2022-10-12T14:16:16.0
			setup: []func(*Counter){

				// count 10
				func(c *Counter) {
					for i := 0; i < 10; i++ {
						c.Count()
					}
				},

				// increment time (10 seconds)
				func(c *Counter) {
					newTime := clock.NewMock()
					newTime.Set(c.clock.Now().Add(time.Second * 10))
					c.clock = newTime
				},

				// count 25
				func(c *Counter) {
					for i := 0; i < 25; i++ {
						c.Count()
					}
				},

				// increment time (10 seconds)
				func(c *Counter) {
					newTime := clock.NewMock()
					newTime.Set(c.clock.Now().Add(time.Second * 10))
					c.clock = newTime
				},

				// count 82
				func(c *Counter) {
					for i := 0; i < 82; i++ {
						c.Count()
					}
				},
			},
			expectedResult: []*count{
				{
					start: time.Unix(1670681770, 0),
					end:   time.Unix(1670681780, 0),
					count: 10,
				},
				{
					start: time.Unix(1670681780, 0),
					end:   time.Unix(1670681790, 0),
					count: 25,
				},
			},
		},
	}

	for _, test := range tests {

		clock := clock.NewMock()
		clock.Set(test.startTime)

		counter := &Counter{
			clock:    clock,
			interval: test.counterInterval,
			counts:   &sync.Map{},
			mu:       &sync.Mutex{},
		}

		for _, fn := range test.setup {
			fn(counter)
		}

		// check counts match
		assert.ElementsMatchf(t, test.expectedResult, counter.takePoints(), "%s: unexpected counts response", test.name)

		// check that no counts remain after last takeCounts
		assert.ElementsMatchf(t, make([]*count, 0), counter.takePoints(), "%s: unexpected empty counts response", test.name)
	}

}

func TestCounter_newCounter(t *testing.T) {

	tests := []struct {
		name            string
		interval        int64
		expectedCounter *Counter
		expectedError   error
	}{
		{
			name:     "newCounter - normal interval",
			interval: 10,
			expectedCounter: &Counter{
				clock:    clock.New(),
				interval: 10,
				counts:   &sync.Map{},
				mu:       &sync.Mutex{},
			},
			expectedError: nil,
		},
		{
			name:            "newCounter - zero interval",
			interval:        0,
			expectedCounter: nil,
			expectedError:   errors.New("interval must be greater than 0"),
		},
		{
			name:            "newCounter - negative interval",
			interval:        -50,
			expectedCounter: nil,
			expectedError:   errors.New("interval must be greater than 0"),
		},
	}

	for _, test := range tests {

		counter, err := newCounter(test.interval)

		assert.Equalf(t, test.expectedCounter, counter, "%s failed", test.name)
		assert.Equalf(t, test.expectedError, err, "%s failed", test.name)
	}
}
