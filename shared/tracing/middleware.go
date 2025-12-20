package tracing

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// GinMiddleware returns a Gin middleware that traces HTTP requests
func GinMiddleware(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName)

	return func(c *gin.Context) {
		// Extract context from incoming request headers
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)

		// Start span
		spanName := c.Request.Method + " " + c.FullPath()
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethod(c.Request.Method),
				semconv.HTTPTarget(c.Request.URL.Path),
				semconv.HTTPRoute(c.FullPath()),
				attribute.String("http.scheme", c.Request.URL.Scheme),
				attribute.String("http.host", c.Request.Host),
				attribute.String("http.user_agent", c.Request.UserAgent()),
			),
		)
		defer span.End()

		// Inject span context into Gin context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Record response status
		status := c.Writer.Status()
		span.SetAttributes(semconv.HTTPStatusCode(status))

		// Mark span as error if status >= 400
		if status >= 400 {
			span.SetStatus(codes.Error, c.Errors.String())
			if len(c.Errors) > 0 {
				span.RecordError(c.Errors.Last().Err)
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}

// AddSpanAttribute adds a custom attribute to the current span
func AddSpanAttribute(c *gin.Context, key string, value string) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		span.SetAttributes(attribute.String(key, value))
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(c *gin.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// RecordError records an error in the current span
func RecordError(c *gin.Context, err error) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
