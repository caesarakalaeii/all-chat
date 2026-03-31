---
phase: quick
plan: 260331-gqi
type: execute
wave: 1
depends_on: []
files_modified:
  - migrations/Dockerfile
  - scripts/run-migrations.sh
  - .github/workflows/build-and-push.yml
  # caesar-deployment repo:
  - ../caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml
  - ../caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml
  - ../caesar-deployment/apps/workloads/all-chat/share-service-deployment.yaml
  - ../caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml
  - ../caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml
  - ../caesar-deployment/apps/workloads/all-chat/source-controller-deployment.yaml
  - ../caesar-deployment/apps/workloads/all-chat/message-processor-deployment.yaml
autonomous: true
requirements: []
must_haves:
  truths:
    - "Database migrations run automatically before any service starts after a deployment"
    - "New migration files added to migrations/ are automatically picked up on next deploy"
    - "Migration image is built and pushed by CI when migration files change"
  artifacts:
    - path: "migrations/Dockerfile"
      provides: "Migration Docker image definition"
    - path: "scripts/run-migrations.sh"
      provides: "Shell script that runs all up-migrations in order"
    - path: ".github/workflows/build-and-push.yml"
      provides: "CI pipeline including migrations image build"
  key_links:
    - from: ".github/workflows/build-and-push.yml"
      to: "migrations/Dockerfile"
      via: "Docker build-and-push for migrations image"
    - from: "K8s init containers"
      to: "ghcr.io/caesarakalaeii/allchat-migrations:main"
      via: "initContainers image reference"
---

<objective>
Automate database migrations so they run before services start in Kubernetes, eliminating the risk of downtime from missed manual migration runs.

Purpose: Currently no production migration automation exists (except support-bot's inline init container). Services can deploy with code that expects new tables/columns that don't exist yet.
Output: Migration Docker image, CI build pipeline, init containers on all DB-using service deployments.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/quick/260331-gqi-automate-database-migrations-to-prevent-/260331-gqi-RESEARCH.md
@CLAUDE.md

Key facts from research:
- 66 SQL files in migrations/, all idempotent (IF NOT EXISTS, ON CONFLICT DO NOTHING)
- Duplicate number prefixes (004, 005, 007, 008, 009) but no conflicts — different tables
- _down.sql files must be skipped
- ON_ERROR_STOP=0 is the established pattern (support-bot precedent)
- DB credentials: allchat-config ConfigMap + allchat-secrets Secret (database-password key)
- Keel auto-deploys on image push to main — migration image gets same treatment
- 14 services reference DATABASE_HOST but key DB-writing services are: auth-service, overlay-manager, share-service, source-manager, token-refresh-service, source-controller, message-processor
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create migration Docker image and runner script</name>
  <files>migrations/Dockerfile, scripts/run-migrations.sh</files>
  <action>
Create `scripts/run-migrations.sh`:
- Iterate over `/migrations/[0-9]*.sql` files in lexicographic order
- Skip any file matching `*_down.sql` pattern (case filter)
- For each file, run it via `PGPASSWORD="$DATABASE_PASSWORD" psql -h "$DATABASE_HOST" -p "$DATABASE_PORT" -U "$DATABASE_USER" -d "$DATABASE_NAME" -v ON_ERROR_STOP=0 -f "$f"`
- Echo each filename before running for logging
- Echo "All migrations complete" at end
- Use `#!/bin/sh` (not bash — alpine)
- set -e is NOT used (ON_ERROR_STOP=0 means psql can exit non-zero for idempotent re-runs)
- Exit 0 at the end explicitly so the init container always succeeds

Create `migrations/Dockerfile`:
- Base: `postgres:16-alpine` (has psql built-in)
- COPY all `migrations/` SQL files into `/migrations/` inside the image
- COPY `scripts/run-migrations.sh` to `/run-migrations.sh`
- RUN chmod +x /run-migrations.sh
- ENTRYPOINT ["/run-migrations.sh"]

Note: The Dockerfile context will be the repo root (same as all other services), so paths are relative to repo root. The Dockerfile itself lives at `migrations/Dockerfile`.

Verify the image builds locally:
```bash
docker build -f migrations/Dockerfile -t allchat-migrations:test .
```
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && docker build -f migrations/Dockerfile -t allchat-migrations:test . 2>&1 | tail -5</automated>
  </verify>
  <done>migrations/Dockerfile builds successfully, scripts/run-migrations.sh exists with correct logic, image contains all 66 SQL files and the runner script</done>
</task>

<task type="auto">
  <name>Task 2: Add migrations image to CI build pipeline and K8s init containers</name>
  <files>.github/workflows/build-and-push.yml, ../caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml, ../caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml, ../caesar-deployment/apps/workloads/all-chat/share-service-deployment.yaml, ../caesar-deployment/apps/workloads/all-chat/source-manager-deployment.yaml, ../caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml, ../caesar-deployment/apps/workloads/all-chat/source-controller-deployment.yaml, ../caesar-deployment/apps/workloads/all-chat/message-processor-deployment.yaml</files>
  <action>
**CI Pipeline (.github/workflows/build-and-push.yml):**

1. Add `migrations` to the `dorny/paths-filter` filters:
```yaml
migrations:
  - 'migrations/**'
  - 'scripts/run-migrations.sh'
```

2. Add `migrations` to the detect-changes job outputs:
```yaml
migrations: ${{ steps.changes.outputs.migrations }}
```

3. Add a `migrations` entry to the build-and-push matrix:
```yaml
- name: migrations
  changed: ${{ needs.detect-changes.outputs.migrations }}
  path: migrations
```

The existing workflow already handles `context: .` (repo root) and `file: ./${{ matrix.service.path }}/Dockerfile`, so `migrations/Dockerfile` with context `.` works perfectly. The image will be tagged `ghcr.io/caesarakalaeii/allchat-migrations:main` (following the existing `$REGISTRY/$IMAGE_PREFIX-$name` pattern).

**K8s Deployments (caesar-deployment repo):**

Add an `initContainers` block to each of these 7 DB-using service deployments:
- auth-service-deployment.yaml
- overlay-manager-deployment.yaml
- share-service-deployment.yaml
- source-manager-deployment.yaml
- token-refresh-service-deployment.yaml
- source-controller-deployment.yaml
- message-processor-deployment.yaml

The init container block (add under `spec.template.spec`, BEFORE the `containers` array):
```yaml
initContainers:
  - name: run-migrations
    image: ghcr.io/caesarakalaeii/allchat-migrations:main
    envFrom:
      - configMapRef:
          name: allchat-config
    env:
      - name: DATABASE_PASSWORD
        valueFrom:
          secretKeyRef:
            name: allchat-secrets
            key: database-password
```

Add Keel annotations to the init container image so Keel auto-updates it. Actually, Keel watches the Deployment's container images — it already handles init containers if the image matches the policy pattern. The existing `keel.sh/policy: force` + `keel.sh/trigger: poll` annotations on each deployment will auto-update the init container image when a new `allchat-migrations:main` tag is pushed.

Do NOT modify support-bot-deployment.yaml — it already has its own inline migration init container and does not use the shared migrations.

Do NOT add init containers to services that don't write to the database (api-gateway, emote-service, twitch-listener, youtube-listener, youtube-listener-innertube, kick-listener, tiktok-listener, discord-listener, twitch-eventsub-listener, discord-bot, frontend).
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && grep -c "migrations" .github/workflows/build-and-push.yml && cd /home/moersener/Hobby/caesar-deployment && grep -l "allchat-migrations" apps/workloads/all-chat/*-deployment.yaml | wc -l</automated>
  </verify>
  <done>CI workflow builds migrations image on migrations/ changes; 7 K8s deployments have init containers referencing ghcr.io/caesarakalaeii/allchat-migrations:main with correct DB credentials from ConfigMap and Secret</done>
</task>

</tasks>

<verification>
1. `docker build -f migrations/Dockerfile -t allchat-migrations:test .` succeeds from repo root
2. `grep -c "run-migrations" ../caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml` returns 1
3. CI workflow has `migrations` in detect-changes outputs AND in build matrix
4. No init container added to support-bot (already has its own), api-gateway, emote-service, or any listener service
5. Runner script skips _down.sql files and uses ON_ERROR_STOP=0
</verification>

<success_criteria>
- Migration Docker image builds from migrations/Dockerfile with all 66 SQL files
- Runner script runs up-migrations in order, skips _down files, uses ON_ERROR_STOP=0, exits 0
- GitHub Actions builds and pushes migrations image when migrations/ or scripts/run-migrations.sh change
- 7 DB-writing service deployments have init containers that run migrations before app starts
- Keel auto-deploys new migration image versions (same policy as other services)
</success_criteria>

<output>
After completion, create `.planning/quick/260331-gqi-automate-database-migrations-to-prevent-/260331-gqi-SUMMARY.md`
</output>
