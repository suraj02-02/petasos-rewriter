package main

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

var labelNames = []string{"code", "method", "host", "url"}

type metricRegistry struct {
	TotalRequests         *prometheus.CounterVec
	ServerRequestDuration *prometheus.HistogramVec
}

func provideMetrics(e *echo.Echo) {
	mr := registerMetrics()

	if err := prometheus.Register(mr.TotalRequests); err != nil {
		e.Logger.Fatal(err)
	}
	if err := prometheus.Register(mr.ServerRequestDuration); err != nil {
		e.Logger.Fatal(err)
	}

	e.Use(mr.getMiddleware())
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
}

func registerMetrics() *metricRegistry {
	namespace := viper.GetString("metricsOptions.namespace")
	if len(namespace) == 0 {
		namespace = "xmidt"
	}
	subsystem := viper.GetString("metricsOptions.subsystem")
	if len(subsystem) == 0 {
		subsystem = "petasos_rewriter"
	}

	totalRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "server_request_count",
			Help:      "total incoming HTTP requests",
		},
		labelNames,
	)

	serverRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "server_request_duration_seconds",
			Help:      "tracks incoming request durations in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 1.5, 2, 2.5, 3},
		},
		labelNames,
	)

	return &metricRegistry{
		TotalRequests:         totalRequests,
		ServerRequestDuration: serverRequestDuration,
	}
}

func (mr *metricRegistry) getMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			startTime := time.Now()
			c.Request().Header.Set("Accept-Encoding", "identity")
			err := next(c)
			elapsed := time.Since(startTime).Seconds()
			status := strconv.Itoa(c.Response().Status)

			values := make([]string, len(labelNames))
			values[0] = status
			values[1] = c.Request().Method
			values[2] = c.Request().Host
			values[3] = c.Path()

			mr.TotalRequests.WithLabelValues(values...).Inc()
			mr.ServerRequestDuration.WithLabelValues(values...).Observe(elapsed)
			return err
		}
	}
}
