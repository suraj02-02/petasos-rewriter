module github.com/Equanox/petasos-rewriter

go 1.14

require (
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/benchkram/errz v0.0.0-20180520163740-571a80a661f2
	github.com/getsentry/sentry-go v0.9.0
	github.com/labstack/echo/v4 v4.1.16
	github.com/rs/zerolog v1.19.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.6.1
	github.com/valyala/fasttemplate v1.1.1 // indirect
	go.opentelemetry.io/otel v0.16.0
	go.opentelemetry.io/otel/exporters/stdout v0.16.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.16.0
	go.opentelemetry.io/otel/exporters/trace/zipkin v0.16.0
	go.opentelemetry.io/otel/sdk v0.16.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)
