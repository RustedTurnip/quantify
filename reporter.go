// Package quantify provides a simplified set of tools for reporting custom
// metrics to Google Cloud Metrics.
package quantify

import (
	"context"
	"path"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/robfig/cron/v3"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// Reporter implements a client that reports user defined metrics to Google
// Cloud Monitoring.
type Reporter struct {
	resourceName   string
	resourceLabels map[string]string
	client         *monitoring.MetricClient
	scheduler      *cron.Cron
	counters       []*metricCounter
	errorHandler   func(*Reporter, error)
}

// New returns an instantiated Reporter, or returns an error if instantiation
// fails.
//
// options allow the user to provide custom configurations as a list of Options.
func New(ctx context.Context, options ...Option) (*Reporter, error) {

	c := cron.New(cron.WithSeconds())

	// build Reporter
	reporter := &Reporter{
		scheduler: c,
	}

	for _, option := range options {
		option(reporter)
	}

	// if reporter.client isn't supplied with options
	if reporter.client == nil {

		client, err := monitoring.NewMetricClient(ctx)
		if err != nil {
			return nil, err
		}

		reporter.client = client
	}

	// if reporter.resource isn't supplied with options
	if reporter.resourceName == "" || reporter.resourceLabels == nil {

		// set to be global resource
		option := OptionWithResourceType(&Global{
			ProjectId: DetectProjectId(),
		})

		// attempt to apply resource
		err := option(reporter)
		if err != nil {
			return nil, err
		}
	}

	// if reporter.errorHandler isn't set
	if reporter.errorHandler == nil {

		// set default behaviour to do nothing
		reporter.errorHandler = func(r *Reporter, err error) {}
	}

	// set report schedule
	if _, err := reporter.scheduler.AddFunc(counterSchedule, reporter.report); err != nil {
		panic("bad schedule")
	}

	reporter.scheduler.Start()

	return reporter, nil
}

// CreateCounter creates a Counter that can be used to track a tally of
// singular, arbitrary, occurrences.
func (r *Reporter) CreateCounter(name string, labels map[string]string) *Counter {

	mc := &metricCounter{
		metric: &metricpb.Metric{
			Type:   path.Join(customMetricRoot, name),
			Labels: labels,
		},
		counter: newCounter(),
	}

	r.counters = append(r.counters, mc)
	return mc.counter
}

// report flushes any metrics that can only be reported periodically,
// like counters.
func (r *Reporter) report() {

	now := time.Now()

	// start = 0th second of minute before now
	start := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.Local).Add(time.Minute * -1)
	// end = 59th second of start minute
	end := start.Add(time.Second * 59)

	for _, mc := range r.counters {

		req := &monitoringpb.CreateTimeSeriesRequest{
			Name: "projects/" + r.resourceLabels["project_id"],
			TimeSeries: []*monitoringpb.TimeSeries{
				{
					Metric:     mc.metric,
					MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
					Resource: &monitoredres.MonitoredResource{
						Type:   r.resourceName,
						Labels: r.resourceLabels,
					},
					Points: []*monitoringpb.Point{{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(start),
							EndTime:   timestamppb.New(end),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: mc.counter.fetchAndReset(),
							},
						},
					}},
				},
			},
		}

		err := r.client.CreateTimeSeries(context.Background(), req)
		if err != nil {
			r.errorHandler(r, err)
		}
	}
}

func (r *Reporter) Terminate() {

	r.report()

	ctx := r.scheduler.Stop()
	<-ctx.Done()
}
