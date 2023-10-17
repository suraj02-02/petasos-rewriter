package main

import (
	"strconv"
	"time"

	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

var labelNames = []string{"code", "method", "host", "url"}

type metricRegistry struct {
	TotalRequests             *prometheus.CounterVec
	ServerRequestDuration     *prometheus.HistogramVec
	RequestsWithoutAuthHeader *prometheus.CounterVec
	RequestsWithAuthHeader    *prometheus.CounterVec
}

func provideMetrics(e *echo.Echo) {
	metrics := echo.New()
	metrics.Use(middleware.Logger())
	metrics.Use(middleware.Recover())

	mr := registerMetrics()

	if err := prometheus.Register(mr.TotalRequests); err != nil {
		metrics.Logger.Fatal(err)
	}
	if err := prometheus.Register(mr.ServerRequestDuration); err != nil {
		metrics.Logger.Fatal(err)
	}
	if err := prometheus.Register(mr.RequestsWithoutAuthHeader); err != nil {
		metrics.Logger.Fatal(err)
	}
	if err := prometheus.Register(mr.RequestsWithAuthHeader); err != nil {
		metrics.Logger.Fatal(err)
	}

	metrics.Use(mr.getMiddleware())
	metrics.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	go func() {
		metrics.Logger.Fatal(metrics.Start(":" + viper.GetString(metricsServerPort)))
	}()

	e.Use(mr.getMiddleware())
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

	requestsWithoutAuthHeader := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "server_request_without_auth_header_count",
			Help:      "total requests without authorization header",
		},
		labelNames,
	)

	requestsWithAuthHeader := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "server_request_with_auth_header_count",
			Help:      "total requests with authorization header",
		},
		labelNames,
	)

	return &metricRegistry{
		TotalRequests:             totalRequests,
		ServerRequestDuration:     serverRequestDuration,
		RequestsWithoutAuthHeader: requestsWithoutAuthHeader,
		RequestsWithAuthHeader:    requestsWithAuthHeader,
	}
}

func (mr *metricRegistry) getMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			isCpeRedirectRequest := strings.Contains(c.Request().RequestURI, authHeaderCheckRequestPath)
			isAuthHeaderPresent := len(c.Request().Header.Get("Authorization")) > 0
			c.Request().Header.Set("Accept-Encoding", "identity")

			startTime := time.Now()
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

			if isCpeRedirectRequest {
				if !isAuthHeaderPresent {
					mr.RequestsWithoutAuthHeader.WithLabelValues(values...).Inc()
				} else {
					mr.RequestsWithAuthHeader.WithLabelValues(values...).Inc()
				}
			}
			return err
		}
	}
}
