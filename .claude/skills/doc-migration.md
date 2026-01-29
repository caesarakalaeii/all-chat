# doc-migration

Create and apply database migrations with automated validation and deployment.

---

## Usage

```
/doc-migration <action> [migration-name]
```

**Actions**:
- `create <name>` - Create new migration file
- `apply local` - Apply all pending migrations locally
- `apply k8s` - Apply all pending migrations to Kubernetes
- `rollback <number>` - Generate rollback for migration
- `verify` - Verify migrations applied correctly

**Examples**:
- `/doc-migration create add_user_preferences`
- `/doc-migration apply local`
- `/doc-migration apply k8s`
- `/doc-migration rollback 023`
- `/doc-migration verify`

---

## What This Skill Does

### Action: create

1. **Finds next migration number** (e.g., 023)
2. **Interviews user** about schema changes using AskUserQuestion
3. **Generates migration SQL** based on user requirements
4. **Creates migration file** in `migrations/` directory
5. **Optionally creates rollback** migration

### Action: apply local

1. **Checks local PostgreSQL** connection
2. **Applies all migrations** in order
3. **Verifies** each migration succeeded
4. **Reports** which migrations were applied

### Action: apply k8s

1. **Verifies Kubernetes context** (default)
2. **Finds CNPG primary pod** (allchat-cluster-1)
3. **Applies migrations** via stdin with postgres user
4. **Grants permissions** to allchat user
5. **Verifies replication** to standby pods

### Action: rollback

1. **Reads migration file** to understand changes
2. **Generates reverse SQL** (DROP TABLE, ALTER TABLE DROP COLUMN, etc.)
3. **Creates rollback file**: `{number}_rollback_{name}.sql`

### Action: verify

1. **Checks all migration files** exist in database
2. **Compares schema** between local and Kubernetes
3. **Reports** any missing migrations

---

## Instructions for Claude

### Action: create

When invoked with `/doc-migration create <name>`:

**Step 1: Find Next Migration Number**
```bash
ls -1 migrations/*.sql | tail -1 | grep -oE "[0-9]+" | head -1
# Extract number, increment by 1, format with leading zeros
```

**Step 2: Interview User (AskUserQuestion)**

```
Question 1: "What type of schema change?"
Options:
- Create new table
- Add column to existing table
- Add index
- Add foreign key
- Create function/trigger
- Multiple changes
```

**If "Create new table"**:
```
Question 2: "Table name?" (text input)
Question 3: "What columns?" (text input)
  Example: "id UUID, name VARCHAR(255), settings JSONB, created_at TIMESTAMP"

Question 4: "Foreign keys?" (text input)
  Example: "overlay_id references overlays(id)"

Question 5: "Indexes?" (text input)
  Example: "overlay_id, created_at DESC"
```

**Step 3: Generate Migration SQL**

```sql
-- Migration: {NUMBER}_{name}
-- Description: {Generated from user input}

CREATE TABLE IF NOT EXISTS {table_name} (
    {columns from user input},
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
{Generate CREATE INDEX for each specified column}

-- Foreign keys
{Generate ALTER TABLE ADD CONSTRAINT for each FK}

-- Trigger for updated_at (if table has updated_at column)
CREATE TRIGGER trigger_update_{table_name}_updated_at
BEFORE UPDATE ON {table_name}
FOR EACH ROW EXECUTE FUNCTION update_updated_at();
```

**Step 4: Write Migration File**

Write to `migrations/{NUMBER}_{name}.sql`

**Step 5: Ask About Rollback**

```
Question: "Create rollback migration?"
Options:
- Yes, create rollback file
- No, skip rollback
```

If yes, create `migrations/{NUMBER}_rollback_{name}.sql`:
```sql
-- Rollback: {NUMBER}_{name}

DROP TABLE IF EXISTS {table_name} CASCADE;
```

---

### Action: apply local

When invoked with `/doc-migration apply local`:

**Step 1: Check PostgreSQL Connection**
```bash
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat -c "SELECT 1"
```

**Step 2: Apply Each Migration**

For each `.sql` file in `migrations/` (in order):
```bash
echo "Applying migrations/001_initial_schema.sql..."
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat \
  -f migrations/001_initial_schema.sql

# Check exit code
if [ $? -eq 0 ]; then
    echo "✅ Migration 001 applied successfully"
else
    echo "❌ Migration 001 failed"
    exit 1
fi
```

**Step 3: Report Summary**

```
Applied migrations:
✅ 001_initial_schema.sql
✅ 002_add_youtube_support.sql
✅ 003_add_kick_support.sql
...
✅ 023_your_new_migration.sql

Total: 23 migrations applied successfully
```

---

### Action: apply k8s

When invoked with `/doc-migration apply k8s`:

**Step 1: Verify Kubernetes Context**
```bash
kubectl config current-context
# Must be: default

kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster
# Must show 3 running pods
```

**Step 2: Confirm with User**

```
⚠️  WARNING: About to apply migrations to Kubernetes cluster (context: default)

Migrations to apply:
- migrations/023_new_feature.sql

Continue?
Options:
- Yes, apply migrations
- No, cancel
```

**Step 3: Apply Each Migration**

For each migration:
```bash
echo "Applying migrations/023_new_feature.sql to allchat-cluster-1..."

cat migrations/023_new_feature.sql | \
  kubectl exec -i -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat

# Capture output and check for errors
```

**Step 4: Grant Permissions**

Extract table names from migration SQL, grant permissions:
```bash
# Parse CREATE TABLE statements to find table names
# For each table:
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    GRANT ALL ON {table_name} TO allchat;
  "

# Grant sequence permissions
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO allchat;
  "
```

**Step 5: Verify Replication**

```bash
# Check primary
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "\dt {table_name}"

# Check replica
kubectl exec -n allchat allchat-cluster-2 -- \
  psql -U postgres allchat -c "\dt {table_name}"

# Check replication lag
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "SELECT * FROM pg_stat_replication;"
# Lag should be 0 or <1 MB
```

---

### Action: rollback

When invoked with `/doc-migration rollback <number>`:

**Step 1: Read Migration File**
```bash
cat migrations/{number}_*.sql
# Parse to understand what was created
```

**Step 2: Generate Reverse SQL**

```sql
-- Rollback: {NUMBER}_{name}

-- Reverse CREATE TABLE
DROP TABLE IF EXISTS {table_name} CASCADE;

-- Reverse ALTER TABLE ADD COLUMN
ALTER TABLE {table} DROP COLUMN IF EXISTS {column};

-- Reverse CREATE INDEX
DROP INDEX IF EXISTS {index_name};

-- Reverse INSERT
DELETE FROM {table} WHERE {condition};
```

**Step 3: Write Rollback File**

Write to `migrations/{NUMBER}_rollback_{name}.sql`

**Step 4: Ask User to Test**

```
Created rollback migration: migrations/{NUMBER}_rollback_{name}.sql

⚠️  WARNING: Test rollback on local database first before applying to production.

Test commands:
# Backup first
pg_dump ... > backup.sql

# Apply rollback
psql ... -f migrations/{NUMBER}_rollback_{name}.sql

# Verify tables dropped
psql ... -c "\dt {table_name}"
```

---

### Action: verify

When invoked with `/doc-migration verify`:

**Step 1: List All Migration Files**
```bash
ls -1 migrations/*.sql | grep -v rollback | sort
```

**Step 2: Check Which Are Applied**

For each migration, check if tables exist:
```bash
# Extract table names from CREATE TABLE statements
grep "CREATE TABLE" migrations/001_initial_schema.sql

# Check if table exists in database
kubectl exec -n allchat allchat-cluster-1 -- \
  psql -U postgres allchat -c "\dt {table_name}"
```

**Step 3: Report Status**

```
Migration Status:
✅ 001_initial_schema.sql (applied)
✅ 002_add_youtube_support.sql (applied)
✅ 003_add_kick_support.sql (applied)
...
❌ 023_new_feature.sql (NOT applied)

Recommendation: Run /doc-migration apply k8s
```

---

## Example Workflow

### Complete Migration Workflow

```bash
# 1. Create migration
/doc-migration create add_user_preferences

# Claude asks:
# - What type of change? → Create new table
# - Table name? → user_preferences
# - Columns? → user_id UUID, theme VARCHAR, font_size INT
# - Foreign keys? → user_id references users(id)
# - Indexes? → user_id

# Claude generates: migrations/023_add_user_preferences.sql

# 2. Test locally
/doc-migration apply local

# Claude:
# ✅ Applied 023_add_user_preferences.sql successfully

# 3. Apply to Kubernetes
/doc-migration apply k8s

# Claude:
# ⚠️  WARNING: Apply to cluster (context: default)?
# [User confirms]
# ✅ Applied migration to allchat-cluster-1
# ✅ Granted permissions to allchat user
# ✅ Verified replication to standby pods

# 4. Verify
/doc-migration verify

# Claude:
# ✅ All 23 migrations applied successfully
```

---

## Success Criteria

✅ Skill complete when:
1. Migration file created with correct numbering
2. SQL follows best practices (IF NOT EXISTS, transactions, comments)
3. Migration tested locally before Kubernetes deployment
4. Permissions granted to application user after applying
5. Replication verified across all CNPG pods
6. Migration documented in service README if schema change is significant

---

## Related Documentation

- **Quick Ref**: [QUICK-REF-DATABASE-MIGRATION.md](../../docs/llm-guides/QUICK-REF-DATABASE-MIGRATION.md)
- **ADR**: [ADR-0003](../../docs/adr/0003-cloudnative-postgres.md) - CloudNativePG
- **Deployment**: [02-DEPLOYMENT.md](../../docs/architecture/02-DEPLOYMENT.md)
