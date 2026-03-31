#!/bin/sh

# Run all up-migrations in lexicographic order.
# Skips *_down.sql files.
# Uses ON_ERROR_STOP=0 so idempotent statements (IF NOT EXISTS, ON CONFLICT DO NOTHING)
# that return non-zero exit codes do not abort the run.

for f in $(ls /migrations/[0-9]*.sql | sort); do
    case "$f" in
        *_down.sql)
            echo "Skipping down migration: $f"
            continue
            ;;
    esac
    echo "Running migration: $f"
    PGPASSWORD="$DATABASE_PASSWORD" psql \
        -h "$DATABASE_HOST" \
        -p "$DATABASE_PORT" \
        -U "$DATABASE_USER" \
        -d "$DATABASE_NAME" \
        -v ON_ERROR_STOP=0 \
        -f "$f"
done

echo "All migrations complete"
exit 0
