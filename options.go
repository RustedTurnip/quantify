package cloud_metrics

import (
	"errors"

	monitoring "cloud.google.com/go/monitoring/apiv3"
)

// Option defines a function for supplying the Reporter constructor with certain
// configurations.
type Option func(*Reporter) error

// OptionWithCloudMetricsClient allows a cloud_metrics Client, which has been
// manually configured, to be supplied to the client instead of using the default
// configuration.
func OptionWithCloudMetricsClient(client *monitoring.MetricClient) Option {
	return func(reporter *Reporter) error {
		reporter.client = client
		return nil
	}
}

// OptionWithResourceType allows a Resource other than the default to be provided
// which will govern how the metric is filed in Google Cloud Monitoring.
func OptionWithResourceType(resource Resource) Option {
	return func(reporter *Reporter) error {

		reporter.resourceName = resource.GetName()

		resourceLabels, err := flatten(resource)
		if err != nil {
			return err
		}

		value, ok := resourceLabels["project_id"]
		if !ok || value == "" {
			return errors.New("missing required project_id resource label")
		}

		reporter.resourceLabels = resourceLabels
		return nil
	}
}

// OptionWithErrorHandler allows a way for internal error handling to be defined
// externally to the library, for example if errors need to be logged, or if the
// program should be terminated in the event of an error.
func OptionWithErrorHandler(fn func(*Reporter, error)) Option {
	return func(r *Reporter) error {
		r.errorHandler = fn
		return nil
	}
}
