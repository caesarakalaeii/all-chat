# OpenTelemetry Tracing Package

Shared package for distributed tracing across All-Chat microservices using OpenTelemetry.

## Features

- ✅ OTLP gRPC exporter (compatible with Jaeger, Tempo, etc.)
- ✅ Automatic HTTP request tracing via Gin middleware
- ✅ Context propagation across services
- ✅ Configurable sampling
- ✅ Service metadata (name, version, environment)

## Quick Start

### 1. Initialize Tracer

In your service's `main.go`:

```go
import (
    "github.com/caesar/all-chat/shared/tracing"
    "go.uber.org/zap"
)

func main() {
    log := logger.NewLogger("my-service", "info")

    // Configure tracing
    tracingCfg := tracing.Config{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    getEnv("ENVIRONMENT", "development"),
        OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
        Enabled:        getEnv("OTEL_ENABLED", "true") == "true",
    }

    // Initialize tracer
    shutdown, err := tracing.InitTracer(tracingCfg, log)
    if err != nil {
        log.Fatal("Failed to initialize tracer", zap.Error(err))
    }
    defer shutdown(context.Background())

    // Rest of your service initialization...
}
```

### 2. Add Middleware

Add the tracing middleware to your Gin router:

```go
router := gin.New()
router.Use(tracing.GinMiddleware("my-service"))
```

### 3. Manual Instrumentation

For custom spans within your code:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

func MyHandler(c *gin.Context) {
    tracer := otel.Tracer("my-service")
    ctx, span := tracer.Start(c.Request.Context(), "custom-operation")
    defer span.End()

    // Add custom attributes
    span.SetAttributes(
        attribute.String("user.id", userID),
        attribute.String("platform", "twitch"),
    )

    // Your business logic here
}
```

### 4. Helper Functions

Use helper functions for common operations:

```go
// Add attribute to current span
tracing.AddSpanAttribute(c, "user.id", userID)

// Add event to current span
tracing.AddSpanEvent(c, "token.refreshed",
    attribute.String("platform", "twitch"))

// Record error
if err != nil {
    tracing.RecordError(c, err)
    return
}
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_ENABLED` | Enable/disable tracing | `true` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | `localhost:4317` |
| `ENVIRONMENT` | Deployment environment | `development` |

## Running Jaeger Locally

```bash
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest
```

Access Jaeger UI at: http://localhost:16686

## Trace Context Propagation

The middleware automatically propagates trace context across services via HTTP headers using W3C Trace Context standard.

When making HTTP requests to other services:

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Use instrumented HTTP client
client := &http.Client{
    Transport: otelhttp.NewTransport(http.DefaultTransport),
}
```

## Best Practices

1. **Always defer span.End()** to ensure spans are completed
2. **Use meaningful span names** (e.g., "POST /overlays", "refresh-token")
3. **Add relevant attributes** (user IDs, platform, overlay IDs)
4. **Record errors** using `span.RecordError(err)`
5. **Keep spans focused** - one logical operation per span
6. **Use child spans** for sub-operations

## Example: Complete Service Setup

```go
package main

import (
    "context"
    "github.com/caesar/all-chat/shared/logger"
    "github.com/caesar/all-chat/shared/tracing"
    "github.com/gin-gonic/gin"
)

func main() {
    // Initialize logger
    log := logger.NewLogger("api-gateway", "info")

    // Initialize tracing
    tracingCfg := tracing.Config{
        ServiceName:    "api-gateway",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        OTLPEndpoint:   "jaeger:4317",
        Enabled:        true,
    }

    shutdown, err := tracing.InitTracer(tracingCfg, log)
    if err != nil {
        log.Fatal("Failed to initialize tracer", zap.Error(err))
    }
    defer shutdown(context.Background())

    // Setup Gin with tracing
    router := gin.New()
    router.Use(tracing.GinMiddleware("api-gateway"))
    router.Use(gin.Recovery())

    // Register routes
    router.GET("/health", handleHealth)

    // Start server
    router.Run(":8080")
}
```

## Troubleshooting

### Traces not appearing in Jaeger

1. Check OTLP endpoint is correct: `telnet localhost 4317`
2. Verify `OTEL_ENABLED=true` environment variable
3. Check service logs for tracer initialization errors
4. Ensure Jaeger is running with OTLP enabled

### High cardinality warnings

Avoid using high-cardinality values in span names (like UUIDs). Use span attributes instead:

```go
// ❌ Bad - high cardinality span name
span := tracer.Start(ctx, "process-overlay-" + overlayID)

// ✅ Good - attribute
span := tracer.Start(ctx, "process-overlay")
span.SetAttributes(attribute.String("overlay.id", overlayID))
```

## Resources

- [OpenTelemetry Go Docs](https://opentelemetry.io/docs/instrumentation/go/)
- [OpenTelemetry Specification](https://opentelemetry.io/docs/specs/otel/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
