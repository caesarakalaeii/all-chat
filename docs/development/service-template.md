# Service README Template

Use this when writing `services/<name>/README.md`. Fill in only the sections that apply to the service; drop the rest rather than leaving placeholders.

---

# {Service Name}

One-sentence description of what the service does.

Port: {PORT}

## How it works

Short prose: what the service consumes (or serves), what it publishes or writes, and on what interval or trigger. Show the input/output stream or message fields if the service reads Redis Streams (`chat:raw`) or Pub/Sub (`overlay:{id}`).

Key files:
- `{package}/{file}.go` - entry point / main loop
- `{package}/...` - the rest

## Environment variables

Required:
- `DATABASE_HOST/PORT/NAME/USER/PASSWORD` (if DB-backed)
- `REDIS_HOST/PORT` (if Redis-backed)
- service-specific variables

Optional: `PORT`, `LOG_LEVEL`, tracing (`OTEL_ENABLED`, `OTEL_EXPORTER_OTLP_ENDPOINT`).

## API endpoints

List routes with a one-line description each, including `GET /health/live` (always 200) and `GET /health/ready` (checks DB/Redis). Add `GET /metrics` if Prometheus metrics are exported.

## Database tables

Name the tables the service owns and point at `migrations/` for the schema. If another service also writes a table, say which one owns the columns.

## Running locally

```bash
cd services/{service-name}
go run ./cmd
```

With the full stack: `make docker-up` from the repo root, then `docker-compose logs -f {service-name}`.

## Testing

```bash
go test ./... -v          # in the service directory
go test ./... -cover
```

## Troubleshooting

2-3 concrete symptoms readers actually hit, each with the command that diagnoses it and the fix.
