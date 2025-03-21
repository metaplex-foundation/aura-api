package metrics

import (
	"strconv"
	"time"

	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	prom "github.com/prometheus/client_golang/prometheus"
)

const (
	apiMethodArg            = "api_method"
	successArg              = "success"
	paymentStatusArg        = "payment_status" // initiated, successful, failed
	backgroundWorkerTypeArg = "background_worker"
)

// See the newMetrics func for proper descriptions and prometheus names!
// In case you add a metric here later, make sure to include it in the
// MetricsList method or you'll going to have a bad time.
var (
	metrics struct {
		// Gauge
		startTime *prometheus.Metric

		// Counter
		apiRequestsTotal        *prometheus.Metric
		cryptoPaymentsProcessed *prometheus.Metric

		// Histogram
		requestExecutionTime          *prometheus.Metric
		backgroundWorkerExecutionTime *prometheus.Metric
	}

	metricList []*prometheus.Metric
)

// Creates and populates a new Metrics struct
// This is where all the prometheus metrics, names and labels are specified
func init() {
	initMetric(&metrics.startTime, newGauge(
		"startTime",
		"start_time",
		"api start time",
	))

	initMetric(&metrics.apiRequestsTotal, newCounterVec(
		"apiRequestsTotal",
		"api_requests_total",
		"Description",
		[]string{apiMethodArg, successArg},
	))

	initMetric(&metrics.cryptoPaymentsProcessed, newCounterVec(
		"cryptoPaymentsProcessed",
		"crypto_payments_processed",
		"Description",
		[]string{paymentStatusArg},
	))

	initMetric(&metrics.requestExecutionTime, newHistogram(
		"requestExecutionTime",
		"request_execution_time",
		"Description",
		[]string{apiMethodArg, successArg},
		[]float64{1, 5, 10, 25, 50, 100, 500, 800, 1000, 2000, 4000, 8000, 15000, 20000, 30000, 50000, 100000},
	))

	initMetric(&metrics.backgroundWorkerExecutionTime, newHistogram(
		"backgroundWorkerExecutionTime",
		"background_worker_execution_time",
		"Description",
		[]string{backgroundWorkerTypeArg},
		[]float64{10, 100, 500, 1000, 2000, 8000, 20000, 50000, 100000, 200000},
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

func IncApiRequestsTotalCnt(method string, success bool) {
	l := prom.Labels{
		apiMethodArg: method,
		successArg:   strconv.FormatBool(success),
	}
	metrics.apiRequestsTotal.MetricCollector.(*prom.CounterVec).With(l).Inc()
}

func IncCryptoPaymentsProcessedTotalCnt(status string) {
	l := prom.Labels{
		paymentStatusArg: status,
	}
	metrics.cryptoPaymentsProcessed.MetricCollector.(*prom.CounterVec).With(l).Inc()
}

func ObserveApiRequestExecutionTime(method string, success bool, d time.Duration) {
	l := prom.Labels{
		apiMethodArg: method,
		successArg:   strconv.FormatBool(success),
	}
	metrics.requestExecutionTime.MetricCollector.(*prom.HistogramVec).With(l).Observe(float64(d.Milliseconds()))
}

func ObserveBackgroundWorkerExecutionTime(worker string, d time.Duration) {
	l := prom.Labels{
		backgroundWorkerTypeArg: worker,
	}
	metrics.backgroundWorkerExecutionTime.MetricCollector.(*prom.HistogramVec).With(l).Observe(float64(d.Milliseconds()))
}

func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			apiMethod := c.Request().URL.Path

			// Process request
			err := next(c)

			// Record metrics after request is processed
			duration := time.Since(start)
			success := c.Response().Status >= 200 && c.Response().Status < 300 && err == nil

			IncApiRequestsTotalCnt(apiMethod, success)

			ObserveApiRequestExecutionTime(apiMethod, success, duration)

			return err
		}
	}
}
