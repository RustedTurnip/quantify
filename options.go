package quantify

import (
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
)

// Option defines a function for supplying the Quantifier constructor with certain
// configurations.
type Option func(*Quantifier) error

// OptionWithCloudMetricsClient allows a cloud_metrics Client, which has been
// manually configured, to be supplied to the client instead of using the default
// configuration.
func OptionWithCloudMetricsClient(client *monitoring.MetricClient) Option {
	return func(quantifier *Quantifier) error {
		quantifier.client = client
		return nil
	}
}

// OptionWithResourceType allows a Resource other than the default to be provided
// which will govern how the metric is filed in Google Cloud Monitoring.
func OptionWithResourceType(resource Resource) Option {
	return func(quantifier *Quantifier) error {

		resourceLabels, err := flatten(resource)
		if err != nil {
			return err
		}

		value, ok := resourceLabels[resourceLabelKeyProjectId]
		if !ok || value == "" {
			return fmt.Errorf("missing required %s resource label", resourceLabelKeyProjectId)
		}

		quantifier.resourceLabels = resourceLabels
		quantifier.resourceName = resource.GetName()

		return nil
	}
}

// OptionWithErrorHandler allows a way for internal error handling to be defined
// externally to the library, for example if errors need to be logged, or if the
// program should be terminated in the event of an error.
func OptionWithErrorHandler(fn func(*Quantifier, error)) Option {
	return func(quantifier *Quantifier) error {
		quantifier.errorHandler = fn
		return nil
	}
}

// OptionWithRefreshInterval allows a way to specify how regularly metrics should
// be pushed to Google Cloud. This does not affect how counts are aggregated.
func OptionWithRefreshInterval(interval time.Duration) Option {
	return func(q *Quantifier) error {
		q.refreshInterval = interval
		return nil
	}
}
