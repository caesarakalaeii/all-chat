# Quick Reference: Create Database Migration

**Time Estimate**: 15-30 minutes | **Difficulty**: ⭐ Easy

**Goal**: Create a new database migration following All-Chat conventions.

---

## Step-by-Step Checklist

### 1. Find Next Migration Number

```bash
cd migrations
ls -1 *.sql | tail -1
# Output: 022_stream_sessions.sql
# Next: 023_your_migration.sql
```

### 2. Create Migration File

```bash
touch migrations/023_your_descriptive_name.sql
```

**Naming Convention**: `NNN_descriptive_snake_case.sql`

### 3. Write Migration SQL

```sql
-- Migration: 023_add_feature_table
-- Purpose: Add support for new feature
-- Date: 2026-01-28

BEGIN;

-- Create new table
CREATE TABLE feature_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID REFERENCES overlays(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    settings JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_feature_items_overlay_id ON feature_items(overlay_id);
CREATE INDEX idx_feature_items_active ON feature_items(is_active) WHERE is_active = true;

-- Add constraints
ALTER TABLE feature_items
    ADD CONSTRAINT check_name_not_empty CHECK (length(name) > 0);

COMMIT;
```

### 4. Test Migration Locally

```bash
# Apply migration
make migrate-up

# Or manually:
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat \
  -f migrations/023_your_migration.sql

# Verify table created
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat \
  -c "\d feature_items"
```

### 5. Create Rollback (Optional)

```bash
touch migrations/023_your_migration_rollback.sql
```

```sql
-- Rollback: 023_add_feature_table
BEGIN;

DROP TABLE IF EXISTS feature_items CASCADE;

COMMIT;
```

---

## Common Patterns

### Add Column to Existing Table

```sql
ALTER TABLE overlays
ADD COLUMN new_field VARCHAR(255) DEFAULT 'default_value';

-- Backfill existing rows if needed
UPDATE overlays SET new_field = 'backfilled_value' WHERE new_field IS NULL;

-- Add NOT NULL constraint after backfill
ALTER TABLE overlays ALTER COLUMN new_field SET NOT NULL;
```

### Add Foreign Key

```sql
ALTER TABLE chat_sources
ADD CONSTRAINT fk_overlay
FOREIGN KEY (overlay_id) REFERENCES overlays(id) ON DELETE CASCADE;
```

### Add Enum Type

```sql
CREATE TYPE platform_type AS ENUM ('twitch', 'youtube', 'kick', 'tiktok');

ALTER TABLE overlay_chat_sources
ALTER COLUMN platform TYPE platform_type USING platform::platform_type;
```

---

## Testing Checklist

- [ ] Migration runs without errors
- [ ] Table/column created with correct types
- [ ] Indexes created
- [ ] Constraints working (try inserting invalid data)
- [ ] Foreign keys enforce referential integrity
- [ ] Default values applied
- [ ] Rollback works (if created)

---

## Kubernetes Migration

**Apply in K8s**:
```bash
# Copy migration to pod
kubectl cp migrations/023_your_migration.sql \
  allchat/allchat-cluster-1:/tmp/migration.sql

# Execute migration
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U allchat -f /tmp/migration.sql

# Verify
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U allchat -c "\d feature_items"
```

---

## Related Documentation

- [CLAUDE.md](../../CLAUDE.md#database-schema) - Database schema overview
- [02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md) - K8s deployment
