// Package quantify provides a simplified set of tools for reporting custom
// metrics to Google Cloud Metrics.
package quantify

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/benbjohnson/clock"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// see https://cloud.google.com/monitoring/api/metrics_gcp for more info on
	// metric roots.
	//
	// as this client is designed for custom metrics, this root is non-configurable
	// (see https://cloud.google.com/monitoring/custom-metrics#identifier).
	customMetricRoot = "custom.googleapis.com"

	defaultRefreshInterval = time.Minute

	resourceLabelKeyProjectId = "project_id"

	projectPathPrefix = "projects"
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
	clock           clock.Clock
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
		clock:           clock.New(),
		mu:              &sync.Mutex{},
		stopped:         make(chan struct{}),
		refreshInterval: defaultRefreshInterval,
	}

	for _, option := range options {
		err := option(quantifier)
		if err != nil {
			return nil, err
		}
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
		option := OptionWithResourceType(&ResourceGlobal{
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

	q.runTicker(q.clock.Ticker(q.refreshInterval), func() {
		q.report(false)
	})
}

// runTicker starts a blocking operation that will call the provided function (fn)
// at the configured Quantifier.refreshInterval.
//
// The function will cease when a stop signal is received (Quantifier.Stop) or when
// the Quantifier.ctx is cancelled.
func (q *Quantifier) runTicker(t *clock.Ticker, fn func()) {

	stop := func() {
		q.mu.Lock()
		q.running = false
		close(q.stop)
		q.stopped <- struct{}{}
		q.mu.Unlock()
	}

	for {
		select {

		// when interval passes, send data
		case <-t.C:
			fn()

		// when context cancelled, exit immediately
		case <-q.ctx.Done():
			stop()
			return

		// when stop requested, stop gracefully
		case <-q.stop:
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
//
// CreateCounter will return an error if the provided name does not match
// Google's Metric_Type specification, or if any of the provided label keys
// under the labels parameter do not match Google's requirements. Refer to
// this link for more information:
// https://cloud.google.com/monitoring/api/v3/naming-conventions
func (q *Quantifier) CreateCounter(name string, labels map[string]string, interval int64) (*Counter, error) {

	if !isMetricTypeValid(name) {
		return nil, fmt.Errorf("invalid name parameter provided")
	}

	for key := range labels {
		if !isMetricLabelKeyValid(key) {
			return nil, fmt.Errorf("invalid label key provided: %s", key)
		}
	}

	counter, err := newCounter(interval)
	if err != nil {
		return nil, err
	}

	mc := &metricCounter{
		metric: &metricpb.Metric{
			Type:   path.Join(customMetricRoot, name),
			Labels: labels,
		},
		counter: counter,
	}

	q.counters = append(q.counters, mc)
	return mc.counter, nil
}

// report flushes any metrics that can only be reported periodically,
// like counters.
//
// current is used to specify the inclusion of any current intervals
// within the tracked counters.
func (q *Quantifier) report(current bool) {

	// each request must only have one point per counter, this multidimensional array
	// tracks a single point from each counter as multiple points can be submitted as
	// long as they are from different counters.
	series := make([][]*monitoringpb.TimeSeries, 0)

	for _, mc := range q.counters {

		pointCount := 0

		// generate request
		for _, point := range mc.counter.takePoints(current) {

			// if series[pointCount] is out of bounds
			if len(series) <= pointCount {
				series = append(series, make([]*monitoringpb.TimeSeries, 0))
			}

			// split points out so only on point per metric per request
			series[pointCount] = append(series[pointCount], q.createTimeSeriesProto(mc.metric, countToMetricPointProto(point)))
			pointCount++
		}
	}

	// send requests
	for _, series := range series {
		err := q.client.CreateTimeSeries(context.Background(), q.createCreateTimeSeriesRequestProto(series))
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

	q.terminate()

	// flush any remaining counts
	q.report(true)
}

// terminate is the underlying close function used when the client needs to be stopped.
func (q *Quantifier) terminate() {

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

// countToMetricPointProto converts a count into a monitoringpb.Point.
//
// note: the duration between the start and end times must be greater than
// 2 milliseconds for a valid Point as countToMetricPointProto will take 1
// millisecond from the end time.
func countToMetricPointProto(count *count) *monitoringpb.Point {
	return &monitoringpb.Point{
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(count.start),

			// minus millisecond because: "The new start time must be at least a
			// millisecond after the end time of the previous interval."
			EndTime: timestamppb.New(count.end.Add(time.Millisecond * -1)),
		},
		Value: &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: count.count,
			},
		},
	}
}

// getGcpProjectPath takes a project id and returns the expected GCP project path.
func getGcpProjectPath(projectId string) string {
	return path.Join(projectPathPrefix, projectId)
}

// createTimeSeriesProto compiles a list of monitoringpb.TimeSeries protos
// (one per provided point) that can be submitted to Google Cloud Monitoring
// within a monitoringpb.CreateTimeSeriesRequest.
func (q *Quantifier) createTimeSeriesProto(metric *metricpb.Metric, point *monitoringpb.Point) *monitoringpb.TimeSeries {

	return &monitoringpb.TimeSeries{
		Metric:     metric,
		MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
		Resource: &monitoredres.MonitoredResource{
			Type:   q.resourceName,
			Labels: q.resourceLabels,
		},
		Points: []*monitoringpb.Point{
			point,
		},
	}
}

// createCreateTimeSeriesRequestProto compiles a monitoringpb.CreateTimeSeriesRequest proto
// within the Quantifiers project scope with the provided []*monitoringpb.TimeSeries.
func (q *Quantifier) createCreateTimeSeriesRequestProto(series []*monitoringpb.TimeSeries) *monitoringpb.CreateTimeSeriesRequest {
	return &monitoringpb.CreateTimeSeriesRequest{
		Name:       getGcpProjectPath(q.resourceLabels[resourceLabelKeyProjectId]),
		TimeSeries: series,
	}
}
