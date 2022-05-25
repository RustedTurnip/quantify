package cloud_metrics

import (
	"context"
	"path"
	"sync"
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

// metricCounter defines a wrapper around the counter unit, tethering it to
// a Metric config.
type metricCounter struct {
	metric  *metricpb.Metric
	counter *counter
}

// reporter implements a client that reports user defined metrics to Google
// Cloud Monitoring.
type reporter struct {
	resourceType   ResourceType
	resourceConfig map[string]string
	client         *monitoring.MetricClient
	scheduler      *cron.Cron
	counters       *sync.Map
	onReportError  func(err error)
}

// New returns an instantiated reporter, or returns an error if instantiation
// fails.
//
// The resourceType parameter takes a ResourceType to define how metrics should be
// reported to Google Cloud.
//
// client is the MetricClient required to interface with Google Cloud Monitoring.
//
// onReportError is a required error handler to tell the reporter how it should handle an
// error when an attempt at reporting metrics fails. Metrics aren't necessarily reported
// when they are initially recorded, which is why this parameter is required.
//
// options allow the user to provide custom configurations as a list of Options.
func New(resourceType ResourceType, client *monitoring.MetricClient, onReportError func(error), options ...Option) (*reporter, error) {

	c := cron.New(cron.WithSeconds())

	// build reporter
	reporter := &reporter{
		resourceType:   resourceType,
		resourceConfig: make(map[string]string),
		client:         client,
		scheduler:      c,
		counters:       &sync.Map{},
		onReportError:  onReportError,
	}

	for _, option := range options {
		if err := option(reporter); err != nil {
			return nil, err
		}
	}

	// fetch required ResourceType's fields
	err := reporter.configure()
	if err != nil {
		return nil, err
	}

	// set report schedule
	if _, err := reporter.scheduler.AddFunc(counterSchedule, reporter.report); err != nil {
		panic("bad schedule")
	}

	reporter.scheduler.Start()

	return reporter, nil
}

// CreateCounter creates a counter that can be used to track a tally of
// singular, arbitrary, occurrences.
func (r *reporter) CreateCounter(name string, labels map[string]string) *counter {

	mc := &metricCounter{
		metric: &metricpb.Metric{
			Type:   path.Join(customMetricRoot, name),
			Labels: labels,
		},
		counter: newCounter(),
	}

	r.counters.LoadOrStore(name, mc)
	return mc.counter
}

// report flushes any metrics that can only be reported periodically,
// like counters.
func (r *reporter) report() {

	now := time.Now()

	// start = 0th second of minute before now
	start := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.Local).Add(time.Minute * -1)
	// end = 59th second of start minute
	end := start.Add(time.Second * 59)

	r.counters.Range(func(key, value any) bool {

		mc := value.(*metricCounter)

		req := &monitoringpb.CreateTimeSeriesRequest{
			Name: "projects/" + r.resourceConfig["project_id"],
			TimeSeries: []*monitoringpb.TimeSeries{{
				Metric:     mc.metric,
				MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
				Resource: &monitoredres.MonitoredResource{
					Type:   string(r.resourceType),
					Labels: r.resourceConfig,
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
			}},
		}

		err := r.client.CreateTimeSeries(context.Background(), req)
		if err != nil {
			r.onReportError(err)
		}

		return true
	})
}

func (r *reporter) Terminate() {

	r.report()

	ctx := r.scheduler.Stop()
	<-ctx.Done()
}
