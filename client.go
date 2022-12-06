// Package quantify provides a simplified set of tools for reporting custom
// metrics to Google Cloud Metrics.
package quantify

import (
	"context"
	"path"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/robfig/cron/v3"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

const (
	customMetricRoot = "custom.googleapis.com"
	counterSchedule  = "0 * * * * *"
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
	ctx            context.Context
	resourceName   string
	resourceLabels map[string]string
	client         *monitoring.MetricClient
	scheduler      *cron.Cron
	counters       []*metricCounter
	errorHandler   func(*Quantifier, error)
}

// New returns an instantiated Quantifier, or returns an error if instantiation
// fails.
//
// options allow the user to provide custom configurations as a list of Options.
func New(ctx context.Context, options ...Option) (*Quantifier, error) {

	c := cron.New(cron.WithSeconds())

	// build Quantifier
	quantifier := &Quantifier{
		ctx:       ctx,
		scheduler: c,
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

	// set report schedule
	if _, err := quantifier.scheduler.AddFunc(counterSchedule, quantifier.report); err != nil {
		panic("bad schedule")
	}

	quantifier.scheduler.Start()

	return quantifier, nil
}

// CreateCounter creates a Counter that can be used to track a tally of
// singular, arbitrary, occurrences.
func (q *Quantifier) CreateCounter(name string, labels map[string]string) *Counter {

	mc := &metricCounter{
		metric: &metricpb.Metric{
			Type:   path.Join(customMetricRoot, name),
			Labels: labels,
		},
		counter: newCounter(60),
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

func (q *Quantifier) Terminate() {

	q.report()

	ctx := q.scheduler.Stop()
	<-ctx.Done()
}