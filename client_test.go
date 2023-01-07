package quantify

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestQuantifier_countToMetricPointCounter(t *testing.T) {

	tests := []struct {
		name     string
		input    *count
		expected *monitoringpb.Point
	}{
		{
			name: "normal count",
			input: &count{
				start: time.Unix(1672693348, 0), // 2023-01-02 21:02:28
				end:   time.Unix(1672693408, 0), // 2023-01-02 21:03:28
				count: 365,
			},
			expected: &monitoringpb.Point{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{
						Seconds: 1672693348,
						Nanos:   0,
					},
					EndTime: &timestamppb.Timestamp{
						Seconds: 1672693407,
						Nanos:   999000000,
					},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{
						Int64Value: 365,
					},
				},
			},
		},
	}

	for _, test := range tests {
		assert.Equalf(t, test.expected, countToMetricPointProto(test.input), "%s failed", test.name)
	}
}

func TestQuantifier_createTimeSeriesProto(t *testing.T) {

	tests := []struct {
		name        string
		pointsInput []*monitoringpb.Point
		metricInput *metricpb.Metric
		client      *Quantifier
		expected    *monitoringpb.TimeSeries
	}{
		{
			name: "single point, normal",
			pointsInput: []*monitoringpb.Point{
				{
					Interval: &monitoringpb.TimeInterval{
						StartTime: &timestamppb.Timestamp{
							Seconds: 1672693348, // 2023-01-02 21:02:28
							Nanos:   0,
						},
						EndTime: &timestamppb.Timestamp{
							Seconds: 1672693407, // 2023-01-02 21:03:28
							Nanos:   999000000,
						},
					},
					Value: &monitoringpb.TypedValue{
						Value: &monitoringpb.TypedValue_Int64Value{
							Int64Value: 365,
						},
					},
				},
			},
			metricInput: &metricpb.Metric{
				Type: "custom.googleapis.com/test-metric",
				Labels: map[string]string{
					"colour": "red",
				},
			},
			client: &Quantifier{
				resourceName: "global",
				resourceLabels: map[string]string{
					"project_id": "quantify",
				},
			},
			expected: &monitoringpb.TimeSeries{
				Metric: &metricpb.Metric{
					Type: "custom.googleapis.com/test-metric",
					Labels: map[string]string{
						"colour": "red",
					},
				},
				MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
				Resource: &monitoredres.MonitoredResource{
					Type: "global",
					Labels: map[string]string{
						"project_id": "quantify",
					},
				},
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: &timestamppb.Timestamp{
								Seconds: 1672693348, // 2023-01-02 21:02:28
								Nanos:   0,
							},
							EndTime: &timestamppb.Timestamp{
								Seconds: 1672693407, // 2023-01-02 21:03:28
								Nanos:   999000000,
							},
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: 365,
							},
						},
					},
				},
			},
		},
		{
			name: "multiple points, normal",
			pointsInput: []*monitoringpb.Point{
				{
					Interval: &monitoringpb.TimeInterval{
						StartTime: &timestamppb.Timestamp{
							Seconds: 1672693348, // 2023-01-02 21:02:28
							Nanos:   0,
						},
						EndTime: &timestamppb.Timestamp{
							Seconds: 1672693407, // 2023-01-02 21:03:28
							Nanos:   999000000,
						},
					},
					Value: &monitoringpb.TypedValue{
						Value: &monitoringpb.TypedValue_Int64Value{
							Int64Value: 365,
						},
					},
				},
				{
					Interval: &monitoringpb.TimeInterval{
						StartTime: &timestamppb.Timestamp{
							Seconds: 1672693408, // 2023-01-02 21:03:28
							Nanos:   0,
						},
						EndTime: &timestamppb.Timestamp{
							Seconds: 1672693467, // 2023-01-02 21:04:27
							Nanos:   999000000,
						},
					},
					Value: &monitoringpb.TypedValue{
						Value: &monitoringpb.TypedValue_Int64Value{
							Int64Value: 982,
						},
					},
				},
			},
			metricInput: &metricpb.Metric{
				Type: "custom.googleapis.com/test-metric-multiple",
				Labels: map[string]string{
					"colour": "red",
					"shape":  "circle",
				},
			},
			client: &Quantifier{
				resourceName: "gce_instance",
				resourceLabels: map[string]string{
					"project_id":  "quantify",
					"instance_id": "1234567890",
					"zone":        "europe-west1",
				},
			},
			expected: &monitoringpb.TimeSeries{
				Metric: &metricpb.Metric{
					Type: "custom.googleapis.com/test-metric-multiple",
					Labels: map[string]string{
						"colour": "red",
						"shape":  "circle",
					},
				},
				MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
				Resource: &monitoredres.MonitoredResource{
					Type: "gce_instance",
					Labels: map[string]string{
						"project_id":  "quantify",
						"instance_id": "1234567890",
						"zone":        "europe-west1",
					},
				},
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: &timestamppb.Timestamp{
								Seconds: 1672693348, // 2023-01-02 21:02:28
								Nanos:   0,
							},
							EndTime: &timestamppb.Timestamp{
								Seconds: 1672693407, // 2023-01-02 21:03:28
								Nanos:   999000000,
							},
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: 365,
							},
						},
					},
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: &timestamppb.Timestamp{
								Seconds: 1672693408, // 2023-01-02 21:03:28
								Nanos:   0,
							},
							EndTime: &timestamppb.Timestamp{
								Seconds: 1672693467, // 2023-01-02 21:04:27
								Nanos:   999000000,
							},
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: 982,
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		result := test.client.createTimeSeriesProto(test.metricInput, test.pointsInput)
		assert.Equalf(t, test.expected, result, "%s failed", test.name)
	}
}

func TestQuantifier_CreateCounter(t *testing.T) {

	tests := []struct {
		name               string
		client             *Quantifier
		inputName          string
		inputLabels        map[string]string
		inputInterval      int64
		expectedQuantifier *Quantifier
		expectedError      error
	}{
		{
			name: "normal inputs, first counter",
			client: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			inputName: "test_metric",
			inputLabels: map[string]string{
				"colour": "red",
			},
			inputInterval: 10,
			expectedQuantifier: &Quantifier{
				counters: []*metricCounter{
					{
						metric: &metricpb.Metric{
							Type: "custom.googleapis.com/test_metric",
							Labels: map[string]string{
								"colour": "red",
							},
						},
						counter: &Counter{
							interval: 10,
							counts:   &sync.Map{},
							mu:       &sync.Mutex{},
							clock:    clock.New(),
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "normal inputs, appended counter",
			client: &Quantifier{
				counters: []*metricCounter{
					{
						metric: &metricpb.Metric{
							Type: "custom.googleapis.com/test_metric",
							Labels: map[string]string{
								"colour": "red",
							},
						},
						counter: &Counter{
							interval: 10,
							counts:   &sync.Map{},
							mu:       &sync.Mutex{},
							clock:    clock.New(),
						},
					},
				},
			},
			inputName: "test_metric_shape",
			inputLabels: map[string]string{
				"shape": "square",
			},
			inputInterval: 52,
			expectedQuantifier: &Quantifier{
				counters: []*metricCounter{
					{
						metric: &metricpb.Metric{
							Type: "custom.googleapis.com/test_metric",
							Labels: map[string]string{
								"colour": "red",
							},
						},
						counter: &Counter{
							interval: 10,
							counts:   &sync.Map{},
							mu:       &sync.Mutex{},
							clock:    clock.New(),
						},
					},
					{
						metric: &metricpb.Metric{
							Type: "custom.googleapis.com/test_metric_shape",
							Labels: map[string]string{
								"shape": "square",
							},
						},
						counter: &Counter{
							interval: 52,
							counts:   &sync.Map{},
							mu:       &sync.Mutex{},
							clock:    clock.New(),
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "zero interval, first counter",
			client: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			inputName: "test_metric",
			inputLabels: map[string]string{
				"colour": "red",
			},
			inputInterval: 0,
			expectedQuantifier: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			expectedError: errors.New("interval must be greater than 0"),
		},
		{
			name: "negative interval, first counter",
			client: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			inputName: "test_metric",
			inputLabels: map[string]string{
				"colour": "red",
			},
			inputInterval: -10,
			expectedQuantifier: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			expectedError: errors.New("interval must be greater than 0"),
		},
		{
			name: "invalid metric type (name), first counter",
			client: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			inputName: "test_metric!!!",
			inputLabels: map[string]string{
				"colour": "red",
			},
			inputInterval: 60,
			expectedQuantifier: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			expectedError: errors.New("invalid name parameter provided"),
		},
		{
			name: "invalid metric type (name), first counter",
			client: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			inputName: "test_metric",
			inputLabels: map[string]string{
				"@!blah": "red",
			},
			inputInterval: 60,
			expectedQuantifier: &Quantifier{
				counters: make([]*metricCounter, 0),
			},
			expectedError: errors.New("invalid label key provided: @!blah"),
		},
	}

	for _, test := range tests {

		counter, err := test.client.CreateCounter(test.inputName, test.inputLabels, test.inputInterval)

		assert.Equalf(t, test.expectedQuantifier, test.client, "%s failed", test.name)
		assert.Equalf(t, test.expectedError, err, "%s failed", test.name)

		// if counter was created, assert that the counter returned matches the counter stored
		if err == nil {
			assert.Equalf(t, test.client.counters[len(test.client.counters)-1].counter, counter, "%s failed", test.name)
		}
	}
}

func TestQuantifier_runTicker(t *testing.T) {

	tests := []struct {
		name               string
		iterations         int
		expectedIterations int
		terminate          func(quantifier *Quantifier)
	}{
		{
			name:               "runTicker - zero iterations ctx cancelled",
			iterations:         0,
			expectedIterations: 0,
			terminate: func(quantifier *Quantifier) {
				_, cancel := context.WithCancel(quantifier.ctx)
				cancel()
			},
		},
		{
			name:               "runTicker - multiple iterations ctx cancelled",
			iterations:         52,
			expectedIterations: 52,
			terminate: func(quantifier *Quantifier) {
				_, cancel := context.WithCancel(quantifier.ctx)
				cancel()
			},
		},
		{
			name:               "runTicker - zero iterations stopped",
			iterations:         0,
			expectedIterations: 1,
			terminate: func(quantifier *Quantifier) {
				quantifier.Stop()
			},
		},
		{
			name:               "runTicker - multiple iterations stopped",
			iterations:         365,
			expectedIterations: 366,
			terminate: func(quantifier *Quantifier) {
				quantifier.Stop()
			},
		},
	}

	for _, test := range tests {

		// initialise *Quantifier client
		mockClock := clock.NewMock()
		client := &Quantifier{
			clock:           mockClock,
			mu:              &sync.Mutex{},
			ctx:             context.Background(),
			stop:            make(chan struct{}),
			stopped:         make(chan struct{}),
			refreshInterval: time.Second * 10,
			running:         true,
		}

		count := int64(0)
		ticker := client.clock.Ticker(client.refreshInterval)

		// start ticker listener
		go func() {
			client.runTicker(ticker, func() {
				atomic.AddInt64(&count, 1)
			})
		}()

		// tick x times
		for i := 0; i < test.iterations; i++ {
			mockClock.Add(client.refreshInterval)
		}

		// terminate ticker as described by test
		test.terminate(client)

		assert.Equalf(t, test.expectedIterations, int(count), "%s failed", test.name)
	}
}

func TestQuantifier_Stop(t *testing.T) {

	// declared outside so that the pointer value can be shared with client assertion later
	stop := make(chan struct{})
	stopped := make(chan struct{})

	// initialise *Quantifier client
	mockClock := clock.NewMock()
	client := &Quantifier{
		clock:           mockClock,
		mu:              &sync.Mutex{},
		ctx:             context.Background(),
		stop:            stop,
		stopped:         stopped,
		refreshInterval: time.Second * 10,
		running:         true,
	}

	closed := false

	ticker := client.clock.Ticker(client.refreshInterval)

	// start ticker listener
	go func() {
		client.runTicker(ticker, func() {
			closed = true
		})
	}()

	client.Stop()

	// assert that function ran correctly
	assert.True(t, closed)

	// assert that client is in "stopped" state
	expected := &Quantifier{
		clock:           mockClock,
		mu:              &sync.Mutex{},
		ctx:             context.Background(),
		stop:            stop,
		stopped:         stopped,
		refreshInterval: time.Second * 10,
		running:         false,
	}

	assert.Equal(t, expected, client)
}
