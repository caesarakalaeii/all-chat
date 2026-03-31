#!/bin/sh

# Run all up-migrations in lexicographic order.
# Skips *_down.sql files.
# Uses a PostgreSQL advisory lock (ID 8439271) to serialize concurrent runs
# across multiple init containers (7 deployments may start simultaneously).
# Uses ON_ERROR_STOP=0 so idempotent statements (IF NOT EXISTS, ON CONFLICT DO NOTHING)
# that return non-zero exit codes do not abort the run.

PSQL="PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME"

# Build a single SQL script that:
# 1. Acquires an advisory lock (blocks until available)
# 2. Runs all up-migration files via \i
# 3. Releases the lock
BATCH=$(mktemp)

echo "SELECT pg_advisory_lock(8439271);" > "$BATCH"

for f in $(ls /migrations/[0-9]*.sql | sort); do
    case "$f" in
        *_down.sql)
            echo "Skipping down migration: $f"
            continue
            ;;
    esac
    echo "\\echo Running migration: $f" >> "$BATCH"
    echo "\\i $f" >> "$BATCH"
done

echo "SELECT pg_advisory_unlock(8439271);" >> "$BATCH"
echo "\\echo All migrations complete" >> "$BATCH"

echo "Starting migrations (acquiring advisory lock)..."
PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "$DATABASE_PORT" \
    -U "$DATABASE_USER" \
    -d "$DATABASE_NAME" \
    -v ON_ERROR_STOP=0 \
    -f "$BATCH"

rm -f "$BATCH"
exit 0
