package backend

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dc_cooling_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dc_cooling_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	modbusCollectTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "dc_cooling_modbus_collect_total",
		Help: "Total number of Modbus collection cycles",
	})

	modbusCollectErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "dc_cooling_modbus_collect_errors_total",
		Help: "Total number of Modbus collection errors",
	})

	modbusCollectDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "dc_cooling_modbus_collect_duration_seconds",
		Help:    "Modbus collection cycle duration",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30},
	})

	pueValue = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "dc_cooling_pue_value",
		Help: "Current PUE value",
	})

	coolingPowerWatts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "dc_cooling_cooling_power_watts",
		Help: "Total cooling power in watts",
	})

	itPowerWatts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "dc_cooling_it_power_watts",
		Help: "IT power in watts",
	})

	alarmsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dc_cooling_alarms_total",
		Help: "Total alarms triggered",
	}, []string{"level"})

	wsConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "dc_cooling_websocket_connections",
		Help: "Current number of WebSocket connections",
	})

	dbQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dc_cooling_db_query_duration_seconds",
		Help:    "Database query duration",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
	}, []string{"query"})
)

func ObserveModbusCollect(duration float64, errCount int) {
	modbusCollectTotal.Inc()
	modbusCollectDuration.Observe(duration)
	modbusCollectErrors.Add(float64(errCount))
}

func SetPUEMetrics(pue, cooling, it float64) {
	pueValue.Set(pue)
	coolingPowerWatts.Set(cooling)
	itPowerWatts.Set(it)
}

func IncAlarms(level int) {
	alarmsTotal.WithLabelValues(strconv.Itoa(level)).Inc()
}

func SetWSConnections(n int) {
	wsConnections.Set(float64(n))
}

func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		route := mux.CurrentRoute(r)
		path := "_unknown_"
		if route != nil {
			if t, err := route.GetPathTemplate(); err == nil {
				path = t
			}
		}

		ww := &responseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(ww.statusCode)).Inc()
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
