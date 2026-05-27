# Quick Reference: Database Migration

**Time Estimate**: 30-60 minutes | **Difficulty**: ⭐⭐ Moderate

**Goal**: Create and apply database schema migrations for All-Chat PostgreSQL database.

---

## Prerequisites

- [ ] PostgreSQL running (local or Kubernetes)
- [ ] Understanding of SQL schema changes needed
- [ ] Backup of production database (if applying to prod)

---

## Step 1: Create Migration File

### 1.1 Find Next Migration Number

```bash
# Find the highest numbered migration
ls -1 migrations/*.sql | tail -1

# Example output: migrations/022_stream_sessions.sql
# Next number: 023
```

### 1.2 Create Migration File

```bash
# Create new migration file
cd migrations
touch 023_your_feature_name.sql
```

**Naming Convention**: `{NUMBER}_{description}.sql`
- Number: 3-digit with leading zeros (001, 002, ..., 023)
- Description: snake_case, descriptive (e.g., `add_user_preferences`)

---

## Step 2: Write Migration SQL

### Basic Structure

```sql
-- Migration: 023_your_feature_name
-- Description: What this migration does and why

-- Create new table
CREATE TABLE IF NOT EXISTS your_table (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    column1 VARCHAR(255) NOT NULL,
    column2 TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_your_table_column1 ON your_table(column1);

-- Add foreign keys if needed
ALTER TABLE your_table ADD CONSTRAINT fk_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Insert default data if needed
INSERT INTO your_table (column1, column2) VALUES ('default', 'value')
ON CONFLICT DO NOTHING;
```

### Common Patterns

**Add Column**:
```sql
ALTER TABLE overlays ADD COLUMN IF NOT EXISTS new_field VARCHAR(100);
```

**Create Index**:
```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_overlays_user_id ON overlays(user_id);
-- CONCURRENTLY allows table to remain accessible during index creation
```

**Create Function/Trigger**:
```sql
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_updated_at
BEFORE UPDATE ON your_table
FOR EACH ROW EXECUTE FUNCTION update_updated_at();
```

---

## Step 3: Test Migration Locally

### 3.1 Backup Database

```bash
# Backup before testing migration
pg_dump postgresql://allchat:allchat_dev_password@localhost:5432/allchat > backup_$(date +%Y%m%d_%H%M%S).sql
```

### 3.2 Apply Migration

```bash
# Run migration
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat -f migrations/023_your_feature_name.sql

# Or use make target
make migrate
```

### 3.3 Verify Schema Changes

```bash
# Connect to database
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat

# Verify table created
\dt your_table

# Verify columns
\d your_table

# Test query
SELECT * FROM your_table LIMIT 1;
```

---

## Step 4: Apply Migration to Kubernetes

### 4.1 Verify Kubernetes Context and Cluster

```bash
# Check current context
kubectl config current-context
# Expected: default (or your cluster context)

# Verify CNPG cluster running
kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster

# Expected output:
# allchat-cluster-1   1/1     Running
# allchat-cluster-2   1/1     Running
# allchat-cluster-3   1/1     Running
```

### 4.2 Apply Migration via stdin (Recommended)

```bash
# Run migration on primary pod using postgres user
cat migrations/023_your_feature_name.sql | \
  kubectl exec -i -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat

# Expected output:
# CREATE TABLE
# CREATE INDEX
# INSERT 0 N (if inserting data)
```

**Why via stdin?**
- CNPG pods have read-only filesystem (cannot `kubectl cp`)
- Stdin method works reliably for all migration types
- No cleanup needed (no temporary files)

**Why postgres user?**
- `postgres` is superuser (can CREATE TABLE, CREATE FUNCTION, etc.)
- Application user `allchat` has limited permissions
- After migration, grant permissions to `allchat`

### 4.3 Grant Permissions to Application User

```bash
# Grant all permissions on new tables to allchat user
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    GRANT SELECT, INSERT, UPDATE, DELETE ON your_table TO allchat;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO allchat;
  "

# Verify permissions
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    SELECT grantee, privilege_type
    FROM information_schema.table_privileges
    WHERE table_name = 'your_table';
  "
```

### 4.4 Verify Migration Applied

```bash
# Check table exists on primary
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "\dt your_table"

# Check table structure
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "\d your_table"

# Verify replicated to standby
kubectl exec -n allchat allchat-cluster-2 -- \
  psql -U postgres allchat -c "\dt your_table"
# Should show same table (automatic replication)
```

---

## Real-World Example: Credit Roll Migrations

### Migration 021: Credit Roll Configs

```bash
# Apply migration
cat migrations/021_credit_roll_configs.sql | \
  kubectl exec -i -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat

# Output:
# CREATE TABLE
# CREATE INDEX
# CREATE FUNCTION
# CREATE TRIGGER
# INSERT 0 52

# Grant permissions
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    GRANT ALL ON credit_roll_configs TO allchat;
  "
```

### Migration 022: Stream Sessions

```bash
# Apply migration
cat migrations/022_stream_sessions.sql | \
  kubectl exec -i -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat

# Output:
# CREATE TABLE
# CREATE INDEX (3x)

# Grant permissions
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    GRANT ALL ON stream_sessions TO allchat;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO allchat;
  "
```

### Verify Both Tables

```bash
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    SELECT tablename FROM pg_tables
    WHERE schemaname = 'public'
    AND tablename IN ('credit_roll_configs', 'stream_sessions')
    ORDER BY tablename;
  "

# Expected:
#      tablename
# ---------------------
#  credit_roll_configs
#  stream_sessions
```

---

## Common Issues & Solutions

### Permission Denied

**Symptom**:
```
ERROR: permission denied for table your_table
```

**Solution**: Run migration as `postgres` user (not `allchat`)
```bash
# Use -U postgres flag
cat migrations/023_migration.sql | \
  kubectl exec -i -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat
```

### Read-Only File System

**Symptom**:
```
tar: Cannot open: Read-only file system
```

**Cause**: Trying to `kubectl cp` to CNPG pod

**Solution**: Use stdin instead
```bash
# Don't do this (fails):
kubectl cp migration.sql allchat/allchat-cluster-1:/tmp/migration.sql

# Do this instead:
cat migration.sql | kubectl exec -i -n allchat allchat-cluster-1 -- psql -U postgres allchat
```

### Table Already Exists

**Symptom**:
```
ERROR: relation "your_table" already exists
```

**Solution**: Use `IF NOT EXISTS` for idempotency
```sql
CREATE TABLE IF NOT EXISTS your_table (...);
ALTER TABLE your_table ADD COLUMN IF NOT EXISTS new_column VARCHAR(100);
```

---

## Best Practices

### Always Use IF NOT EXISTS

```sql
-- Safe to run multiple times
CREATE TABLE IF NOT EXISTS your_table (...);
CREATE INDEX IF NOT EXISTS idx_name ON your_table(column);
ALTER TABLE your_table ADD COLUMN IF NOT EXISTS new_col VARCHAR(100);
```

### Use Transactions

```sql
BEGIN;

CREATE TABLE your_table (...);
CREATE INDEX idx_your_table ON your_table(...);

COMMIT;
-- If any step fails, entire migration rolls back
```

### Add Comments

```sql
-- Migration: 023_add_feature
-- Purpose: Add feature table for new functionality
-- Date: 2026-01-28
-- Author: Development Team

COMMENT ON TABLE your_table IS 'Stores feature data for overlays';
COMMENT ON COLUMN your_table.settings IS 'JSONB settings, schema: {"key": "value"}';
```

---

## Validation Checklist

- [ ] Migration file created with correct number
- [ ] Migration tested on local database
- [ ] Migration uses IF NOT EXISTS for idempotency
- [ ] Indexes created for foreign keys and frequently queried columns
- [ ] Foreign keys enforce referential integrity
- [ ] Migration applied to Kubernetes cluster
- [ ] Permissions granted to `allchat` user
- [ ] Tables verified on primary and replica pods
- [ ] Application code updated to use new schema
- [ ] Service README updated if schema documented

---

## Quick Command Reference

```bash
# Find next migration number
ls -1 migrations/*.sql | tail -1

# Create migration file
touch migrations/023_feature_name.sql

# Test locally
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat -f migrations/023_feature.sql

# Apply to Kubernetes (via stdin with postgres user)
cat migrations/023_feature.sql | \
  kubectl exec -i -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat

# Grant permissions to allchat user
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    GRANT ALL ON your_table TO allchat;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO allchat;
  "

# Verify table created
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "\dt your_table"

# Verify replication
kubectl exec -n allchat allchat-cluster-2 -- \
  psql -U postgres allchat -c "\dt your_table"
```

---

## Related Documentation

- [ADR-0003](../adr/0003-cloudnative-postgres.md) - CloudNativePG decision
- [02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md) - Kubernetes deployment
- [connection-errors.md](../troubleshooting/connection-errors.md) - Database troubleshooting
