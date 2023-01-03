package quantify

import "regexp"

const (
	// reMetricLabelKey provides the maximum length of a Google Cloud Metric_Type
	//
	// see: https://cloud.google.com/monitoring/api/v3/naming-conventions
	maxLengthMetricType = 200

	// reMetricLabelKey provides the maximum length of a Google Cloud Metric label key
	//
	// see: https://cloud.google.com/monitoring/api/v3/naming-conventions
	maxLengthMetricLabelKey = 100
)

var (
	// reMetricType provides the permissible pattern for Google Cloud Metric_Types.
	//
	// see: https://cloud.google.com/monitoring/api/v3/naming-conventions
	reMetricType = regexp.MustCompile("^[a-zA-Z0-9]((\\/[a-zA-Z0-9])?[a-zA-Z0-9\\._]*)*$")

	// reMetricLabelKey provides the pattern for Google Cloud Metric label keys
	//
	// see: https://cloud.google.com/monitoring/api/v3/naming-conventions
	reMetricLabelKey = regexp.MustCompile("^[a-z][a-z0-9\\_]*$")
)

// validateMetricType asserts whether the provided string is a valid Google Cloud
// Metric_Type according to their guidance:
// https://cloud.google.com/monitoring/api/v3/naming-conventions
func isMetricTypeValid(metricType string) bool {

	if !reMetricType.Match([]byte(metricType)) {
		return false
	}

	if len(metricType) > maxLengthMetricType {
		return false
	}

	return true
}

// validateMetricType asserts whether the provided string is a valid Google Cloud
// Metric Label Key according to their guidance:
// https://cloud.google.com/monitoring/api/v3/naming-conventions
func isMetricLabelKeyValid(metricLabelKey string) bool {

	if !reMetricLabelKey.Match([]byte(metricLabelKey)) {
		return false
	}

	if len(metricLabelKey) > maxLengthMetricLabelKey {
		return false
	}

	return true
}
