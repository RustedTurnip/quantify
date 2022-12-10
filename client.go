// Package quantify provides a simplified set of tools for reporting custom
// metrics to Google Cloud Metrics.
package quantify

import (
	"context"
	"path"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

const (
	customMetricRoot = "custom.googleapis.com"

	defaultRefreshInterval = time.Minute
)

// metricCounter defines a wrapper around the Counter unit, tethering it to
// a Metric config.
type metricCounter struct {
	metric  *metricpb.Metric
	counter *Counter
}

// Quantifier implements a client that reports user defined metrics to Google
// Cloud Monitoring.
type Quantifier struct {
	ctx             context.Context
	mu              *sync.Mutex
	stop            chan struct{}
	stopped         chan struct{}
	running         bool
	resourceName    string
	resourceLabels  map[string]string
	client          *monitoring.MetricClient
	counters        []*metricCounter
	errorHandler    func(*Quantifier, error)
	refreshInterval time.Duration
}

// New returns an instantiated Quantifier, or returns an error if instantiation
// fails.
//
// options allow the user to provide custom configurations as a list of Options.
func New(ctx context.Context, options ...Option) (*Quantifier, error) {

	// build Quantifier
	quantifier := &Quantifier{
		ctx:             ctx,
		mu:              &sync.Mutex{},
		stopped:         make(chan struct{}),
		refreshInterval: defaultRefreshInterval,
	}

	for _, option := range options {
		option(quantifier)
	}

	// if quantifier.client isn't supplied with options
	if quantifier.client == nil {

		client, err := monitoring.NewMetricClient(ctx)
		if err != nil {
			return nil, err
		}

		quantifier.client = client
	}

	// if quantifier.resource isn't supplied with options
	if quantifier.resourceName == "" || quantifier.resourceLabels == nil {

		// set to be global resource
		option := OptionWithResourceType(&Global{
			ProjectId: DetectProjectId(),
		})

		// attempt to apply resource
		err := option(quantifier)
		if err != nil {
			return nil, err
		}
	}

	// if quantifier.errorHandler isn't set
	if quantifier.errorHandler == nil {

		// set default behaviour to do nothing
		quantifier.errorHandler = func(r *Quantifier, err error) {}
	}

	go quantifier.run()

	return quantifier, nil
}

// run starts execution of the client providing it isn't already running. Whilst
// running, it will attempt to push recorded data at the interval provided.
//
// run also monitors stop signals and ctc cancelling to cease operations when
// required.
func (q *Quantifier) run() {

	q.mu.Lock()

	if q.running {
		q.mu.Unlock()
		return
	}

	q.running = true
	q.stop = make(chan struct{})
	q.mu.Unlock()

	t := time.NewTicker(q.refreshInterval)

	stop := func() {
		q.mu.Lock()
		q.running = false
		close(q.stop)
		q.mu.Unlock()
	}

	for {
		select {

		// when interval passes, send data
		case <-t.C:
			q.report()

		// when context cancelled, exit immediately
		case <-q.ctx.Done():
			stop()
			return

		// when stop requested, stop gracefully
		case <-q.stop:
			q.report() // flush any remaining counts
			stop()
			return

		}
	}
}

// CreateCounter creates a Counter that can be used to track a tally of
// singular, arbitrary, occurrences.
//
// interval is used to specify how counts should be aggregated, or in other
// words, what level of precision is required when tracking cumulative
// amounts. This value represents seconds.
func (q *Quantifier) CreateCounter(name string, labels map[string]string, interval int64) *Counter {

	mc := &metricCounter{
		metric: &metricpb.Metric{
			Type:   path.Join(customMetricRoot, name),
			Labels: labels,
		},
		counter: newCounter(interval),
	}

	q.counters = append(q.counters, mc)
	return mc.counter
}

// report flushes any metrics that can only be reported periodically,
// like counters.
func (q *Quantifier) report() {

	for _, mc := range q.counters {

		req := &monitoringpb.CreateTimeSeriesRequest{
			Name: "projects/" + q.resourceLabels["project_id"],
			TimeSeries: []*monitoringpb.TimeSeries{
				{
					Metric:     mc.metric,
					MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
					Resource: &monitoredres.MonitoredResource{
						Type:   q.resourceName,
						Labels: q.resourceLabels,
					},
					Points: mc.counter.takePoints(),
				},
			},
		}

		err := q.client.CreateTimeSeries(context.Background(), req)
		if err != nil {
			q.errorHandler(q, err)
		}
	}
}

// Stop can be used to gracefully terminate the Quantifier client. It will attempt
// to push any remaining data that has already been recorded, and then cease
// internal operations.
//
// Note: calling count on any of Quantifier's child counters after this call is made
// won't result in reported metrics as Quantifier will have ceased operations.
func (q *Quantifier) Stop() {

	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return
	}

	// signal stop
	q.stop <- struct{}{}
	q.mu.Unlock()

	// wait for stopped
	<-q.stopped
}
