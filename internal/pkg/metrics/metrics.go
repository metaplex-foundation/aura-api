package metrics

import (
	"time"

	"github.com/labstack/echo-contrib/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
)

// See the NewMetrics func for proper descriptions and prometheus names!
// In case you add a metric here later, make sure to include it in the
// MetricsList method or you'll going to have a bad time.
var (
	metrics struct {
		// Gauge
		startTime *prometheus.Metric
	}

	metricList []*prometheus.Metric
)

// Creates and populates a new Metrics struct
// This is where all the prometheus metrics, names and labels are specified
func init() {
	// Gauge
	initMetric(&metrics.startTime, newGauge(
		"startTime",
		"start_time",
		"api start time",
	))
}

func initMetric(dest **prometheus.Metric, metric *prometheus.Metric) {
	*dest = metric
	metricList = append(metricList, metric)
}

// Needed by echo-contrib so echo can register and collect these metrics
func MetricList() []*prometheus.Metric {
	return metricList
}

func InitStartTime() {
	metrics.startTime.MetricCollector.(prom.Gauge).Set(float64(time.Now().UTC().Unix()))
}
