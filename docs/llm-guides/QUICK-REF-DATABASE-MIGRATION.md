# Quick Reference: Database Migration

Goal: create and apply a PostgreSQL schema migration.

## Create the migration

```bash
# Next number = highest existing + 1
ls -1 migrations/*.sql | tail -5
```

Naming: `migrations/{NNN}_{description}.sql`, 3-digit number, snake_case description. Create the matching idempotent rollback file `{NNN}_{description}_down.sql` if you can drop what you create (see the `_down.sql` pairs in `migrations/`).

Notes from the repo's actual migrations:
- Files run from `001_initial_schema.sql` upward; some numbers have multiple files (e.g. two `004_*` files), and `_down.sql` files for rollback.
- Migrations must be **idempotent** (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`): services run the full set on pod restart and the runner does not track applied migrations. Example: `migrations/128_media_upload_feature_gate.sql`.
- Use `BEGIN; ... COMMIT;`, add a header comment saying what the migration does and why, use parameterized/identifier quoting (no user-input string concatenation), and `CREATE INDEX CONCURRENTLY` where the table is large.

## Test locally

```bash
make migrate    # applies migrations/001_initial_schema.sql to the local DB
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat \
  -f migrations/023_your_feature_name.sql   # apply your new file directly

# Verify
psql ... -c '\d your_table'
```

`make migrate-down` does nothing automated: rollback is manual.

## Apply in Kubernetes (CNPG cluster, namespace `allchat`)

CNPG pods have read-only filesystems, so pipe the file via stdin, as `postgres` superuser, on the primary pod:

```bash
kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster   # check primary is Running
cat migrations/023_your_feature_name.sql | \
  kubectl exec -i -n allchat allchat-cluster-1 -- psql -U postgres allchat
```

Then grant the application user (`allchat`) permissions on new tables and sequences, and verify the schema with `psql -U postgres allchat -c '\d your_table'`.

## Troubleshooting

- Permission denied on new tables: you forgot the `GRANT` above.
- "Table already exists": migration is not idempotent; add `IF NOT EXISTS`.
- Connection failures: see [docs/troubleshooting/connection-errors.md](../troubleshooting/connection-errors.md).
