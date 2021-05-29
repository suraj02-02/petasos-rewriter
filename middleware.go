package main

import (
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

// Middleware returns echo middleware which will inject SpanID and TraceID in response headers.
func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			rsc := trace.RemoteSpanContextFromContext(ctx)
			if !rsc.IsValid() {
				span := trace.SpanFromContext(ctx)
				c.Response().Header().Set(spanIdHeader, span.SpanContext().SpanID().String())
				c.Response().Header().Set(traceIdHeader, span.SpanContext().TraceID().String())
			}
			return next(c)
		}
	}
}
