#!/bin/sh

# Run all up-migrations in lexicographic order.
# Skips *_down.sql files.
# Uses a PostgreSQL advisory lock (ID 8439271) to serialize concurrent runs
# across multiple init containers (7 deployments may start simultaneously).
#
# History: an earlier version of this script silently swallowed connection
# failures because it (1) had no DB-readiness wait and (2) ended with a
# hardcoded `exit 0`. When the CNPG primary wasn't ready at pod startup,
# psql failed with "Connection refused", the init container was marked
# Completed, the new app code rolled out, and the missing migration broke
# production at runtime (e.g. migration 049_overlay_tts_configs).
#
# Three current safeguards address that class of failure:
#   1. We pg_isready-poll the DB for up to ~120 s before any psql work.
#   2. We propagate psql's exit code instead of unconditional `exit 0`.
#   3. ON_ERROR_STOP stays 0 (per-statement) because not every migration is
#      idempotent for CREATE INDEX / CREATE TRIGGER on re-runs — a fatal
#      connection failure still produces a non-zero psql exit code, which
#      this script now respects. Future cleanup: make all migrations
#      idempotent and flip ON_ERROR_STOP=1.

set -u

: "${DATABASE_HOST:?must be set}"
: "${DATABASE_PORT:?must be set}"
: "${DATABASE_USER:?must be set}"
: "${DATABASE_NAME:?must be set}"
: "${DATABASE_PASSWORD:?must be set}"

# ---- Step 1: wait for the DB to be reachable -------------------------------
# pg_isready returns 0 = accepting connections, 1 = rejecting (auth/db wrong),
# 2 = not responding (still booting), 3 = bad params. We retry on 2 (transient)
# and surface the others as fatal. This is the safety net that prevents the
# "init container exits 0 even though psql could not connect" silent failure.
WAIT_TIMEOUT_SECONDS=${MIGRATION_WAIT_TIMEOUT_SECONDS:-120}
WAIT_INTERVAL=2
elapsed=0
echo "Waiting up to ${WAIT_TIMEOUT_SECONDS}s for ${DATABASE_HOST}:${DATABASE_PORT} to accept connections..."
while :; do
    PGPASSWORD="$DATABASE_PASSWORD" pg_isready \
        -h "$DATABASE_HOST" \
        -p "$DATABASE_PORT" \
        -U "$DATABASE_USER" \
        -d "$DATABASE_NAME" \
        -q
    rc=$?
    case "$rc" in
        0) break ;;
        2) ;; # not responding yet — keep retrying
        *)
            echo "ERROR: pg_isready returned ${rc} (1 = rejecting connection, 3 = bad params); aborting." >&2
            exit "$rc"
            ;;
    esac
    if [ "$elapsed" -ge "$WAIT_TIMEOUT_SECONDS" ]; then
        echo "ERROR: database did not become ready within ${WAIT_TIMEOUT_SECONDS}s; aborting." >&2
        exit 1
    fi
    echo "  not ready (rc=${rc}), sleeping ${WAIT_INTERVAL}s..."
    sleep "$WAIT_INTERVAL"
    elapsed=$((elapsed + WAIT_INTERVAL))
done
echo "Database is accepting connections (after ${elapsed}s)."

# ---- Step 2: build the batch SQL ------------------------------------------
BATCH=$(mktemp)
trap 'rm -f "$BATCH"' EXIT

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

# ---- Step 3: run the batch and propagate psql's exit code ------------------
echo "Starting migrations (acquiring advisory lock)..."
PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "$DATABASE_PORT" \
    -U "$DATABASE_USER" \
    -d "$DATABASE_NAME" \
    -v ON_ERROR_STOP=0 \
    -f "$BATCH"
psql_rc=$?

if [ "$psql_rc" -ne 0 ]; then
    echo "ERROR: psql exited with code ${psql_rc}; migrations may be incomplete. Init container will fail so Kubernetes restarts the pod." >&2
    exit "$psql_rc"
fi

echo "All migrations applied successfully."
exit 0
