package main

import (
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"
)

// Middleware returns echo middleware which will inject
// SpanID and TraceID in response headers, will be creating
// context based logger and will be injecting trace information
// in  sentry scope
func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			ctx := req.Context()
			rsc := trace.RemoteSpanContextFromContext(ctx)
			span := trace.SpanFromContext(ctx)
			spanId, traceId := span.SpanContext().SpanID().String(), span.SpanContext().TraceID().String()
			if !rsc.IsValid() && span.SpanContext().IsValid() {
				c.Response().Header().Set(spanIdHeader, spanId)
				c.Response().Header().Set(traceIdHeader, traceId)
			}

			// Creating context based loggger
			logger := log.With().Str(traceIdHeader, traceId).Str(spanIdHeader, spanId).Logger()
			ctx = logger.WithContext(ctx)
			request := req.WithContext(ctx)
			c.SetRequest(request)

			// Adding trace information in sentry scope.
			sentry.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetExtras(map[string]interface{}{"span_id": spanId, "trace_id": traceId, "X-TENANT-ID": req.Header.Get("X-TENANT-ID"), "X-Webpa-Device-Name": req.Header.Get("X-Webpa-Device-Name")})
			})

			return next(c)
		}
	}
}