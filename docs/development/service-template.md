# Service Template - README.md

This template should be used when creating README.md files for All-Chat microservices.

**Replace `{SERVICE_NAME}`, `{PORT}`, `{PURPOSE}` etc. with actual values.**

---

# {SERVICE_NAME}

{One-sentence description of what this service does}

**Port**: {PORT}
**Status**: ✅ Production Ready | 🧪 Beta | 🚧 Development

---

## Features

- **Feature 1**: Description
- **Feature 2**: Description
- **Feature 3**: Description
- **Feature 4**: Description

---

## Architecture

```
{ASCII diagram showing service position in system}

Example:
External API
  ↓
Service Component 1
  ↓
Service Component 2
  ↓
Database/Redis/Next Service
```

---

## Environment Variables

### Required

```bash
# {Category 1}
VAR_NAME=value
VAR_NAME_2=value

# Database (if applicable)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis (if applicable)
REDIS_HOST=localhost
REDIS_PORT=6379
```

### Optional

```bash
# Server configuration
PORT={DEFAULT_PORT}
LOG_LEVEL=info  # debug, info, warn, error

# Feature toggles
FEATURE_ENABLED=true

# OpenTelemetry tracing
OTEL_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317

# Application
APP_VERSION=dev
ENVIRONMENT=development
```

---

## Running Locally

### Prerequisites

- Go 1.25+
- PostgreSQL with all-chat schema (if service uses database)
- Redis (if service uses Redis)
- {Other dependencies}

### Development

```bash
# Set environment variables
export VAR_NAME=value
export DATABASE_HOST=localhost
export REDIS_HOST=localhost

# Run the service
cd services/{service-name}
go run ./cmd

# Or build and run
go build -o {service-name} ./cmd
./{service-name}
```

### With Docker Compose

```bash
# Start all dependencies
make docker-up

# {SERVICE_NAME} starts automatically
# Check logs
docker-compose logs -f {service-name}
```

---

## API Endpoints

### {Feature Category 1}

```bash
# {Endpoint description}
GET /api/v1/{resource}
→ Returns: { "data": [...] }

# {Endpoint description}
POST /api/v1/{resource}
Body: { "field": "value" }
→ Returns: { "id": "uuid", ... }

# {Endpoint description}
PUT /api/v1/{resource}/:id
Body: { "field": "updated_value" }
→ Returns: { "id": "uuid", ... }

# {Endpoint description}
DELETE /api/v1/{resource}/:id
→ Returns: { "success": true }
```

### Health Checks

```bash
# Liveness probe (always returns 200 if service running)
GET /health/live

# Readiness probe (checks dependencies: DB, Redis, etc.)
GET /health/ready

# Detailed status (optional, service-specific)
GET /status
```

**Example Response** (`/status`):
```json
{
  "status": "running",
  "feature": {
    "metric1": 42,
    "metric2": "value"
  }
}
```

### Metrics

```bash
# Prometheus metrics endpoint
GET /metrics
```

**Key Metrics**:
- `{service}_operations_total{result="success|error"}` - Operation counts
- `{service}_duration_seconds` - Operation latency
- `{service}_active_resources_total` - Active resources (connections, jobs, etc.)

---

## Message Format / Data Schema

**If service publishes to Redis Streams**:

### Input Format

```json
{
  "field1": "value1",
  "field2": "value2"
}
```

### Output Format

```json
{
  "id": "uuid",
  "field1": "processed_value1",
  "timestamp": "2026-01-28T10:00:00Z"
}
```

**If service uses database**:

### Database Tables

**{table_name} Table**:
```sql
CREATE TABLE {table_name} (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    field1 VARCHAR(255) NOT NULL,
    field2 TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

---

## How It Works

{Detailed explanation of service internals}

### {Component 1}

**File**: `{service-name}/{package}/{file}.go`

{Description of what this component does}

```go
// Example code snippet showing key logic
func KeyFunction() {
    // ...
}
```

### {Component 2}

**File**: `{service-name}/{package}/{file}.go`

{Description}

---

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Run specific package tests
go test ./{package} -v
```

**Test Coverage**: {XX}%

---

## Monitoring

### Key Metrics to Monitor

```promql
# {Metric category 1}
rate({service}_operations_total[5m])

# {Metric category 2}
histogram_quantile(0.95, rate({service}_duration_seconds_bucket[5m]))
```

### Alerts

**{Alert Name}**:
```yaml
alert: {AlertName}
expr: {expression}
for: 5m
severity: warning|critical
```

---

## Troubleshooting

### {Common Issue 1}

**Symptom**: {Description of what user sees}

**Diagnosis**:
```bash
# Check {something}
command to run

# Expected output:
# what success looks like
```

**Cause**: {Root cause explanation}

**Solution**:
1. Step 1
2. Step 2
3. Step 3

**File**: `{service-name}/{package}/{file}.go:{line}`

---

### {Common Issue 2}

**Symptom**: {Description}

**Solutions**:
1. Solution 1
2. Solution 2

**File**: `{service-name}/{package}/{file}.go:{line}`

---

## Performance

### Capacity

**Per Replica**:
- **Throughput**: {Number} operations/second
- **CPU**: {Range} (varies with load)
- **Memory**: {Range}

**Bottlenecks**:
1. {Bottleneck description}
2. {Bottleneck description}

### Scaling Guidelines

| Load | Replicas | CPU Total | Memory Total |
|------|----------|-----------|--------------|
| {Low} | {N} | {X} | {Y} |
| {Medium} | {N} | {X} | {Y} |
| {High} | {N} | {X} | {Y} |

---

## Production Considerations

1. **{Consideration 1}**: {Description and recommendation}
2. **{Consideration 2}**: {Description and recommendation}
3. **{Consideration 3}**: {Description and recommendation}
4. **{Consideration 4}**: {Description and recommendation}

---

## Related Services

- **{Service A}**: {Relationship description}
- **{Service B}**: {Relationship description}
- **{Service C}**: {Relationship description}

---

## Further Reading

- **[{Architecture Doc}](../../docs/architecture/{file}.md)** - {Description}
- **[{ADR}](../../docs/adr/{file}.md)** - {Description}
- **[{Quick Ref}](../../docs/llm-guides/{file}.md)** - {Description}
- **External Docs**: {URL}

---

## License

Copyright © 2025 All-Chat. All rights reserved.
