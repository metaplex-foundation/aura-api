package metrics

import "github.com/labstack/echo-contrib/prometheus"

const (
	gaugeMetricType        = "gauge"
	gaugeVecMetricType     = "gauge_vec"
	counterMetricType      = "counter"
	counterVecMetricType   = "counter_vec"
	histogramVecMetricType = "histogram_vec"
)

func newGauge(id, name, description string) *prometheus.Metric {
	return &prometheus.Metric{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        gaugeMetricType,
	}
}
func newGaugeVec(id, name, description string, labels []string) *prometheus.Metric {
	return &prometheus.Metric{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        gaugeVecMetricType,
		Args:        labels,
	}
}

func newCounter(id, name, description string) *prometheus.Metric {
	return &prometheus.Metric{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        counterMetricType,
	}
}
func newCounterVec(id, name, description string, labels []string) *prometheus.Metric { //nolint:unparam
	return &prometheus.Metric{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        counterVecMetricType,
		Args:        labels,
	}
}

func newHistogram(id, name, description string, labels []string, buckets []float64) *prometheus.Metric {
	return &prometheus.Metric{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        histogramVecMetricType,
		Args:        labels,
		Buckets:     buckets,
	}
}
