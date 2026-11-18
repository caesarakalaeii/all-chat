# Admin

The admin dashboard lives at `/admin` in the frontend. Access is
role-based: admin is a flag, not a tier system.

- **Database**: `is_admin` column on the `users` table (migration
  `009_add_admin_role.sql`, partial index on `is_admin = TRUE`). New users
  default to `FALSE`.
- **JWT**: login issues a `roles` array; admin users carry `"user", "admin"`.
- **Middleware**: `AdminOnly()` in `shared/middleware/auth.go` checks the
  admin role. Admin surfaces are session-only: a personal access token
  (`allchat_pat_`) is refused even when its owner is an admin.

Responses: `401` without a valid JWT, `403` with one but no admin role
(`{"error": "Admin access required"}`).

## Granting admin access

Kubernetes (the `allchat` namespace):

```bash
./scripts/k8s-make-user-admin.sh <username>
```

Local development:

```bash
./scripts/make-user-admin.sh <username>
```

The JWT only changes on re-login: after granting, log out and back in
before opening `/admin`. Revoke with

```bash
kubectl exec -n allchat <postgres-pod> -- psql -U postgres -d allchat \
  -c "UPDATE users SET is_admin = FALSE WHERE username = '<username>';"
```

If you keep running the migration by hand, apply `migrations/009_add_admin_role.sql`
via `make migrate` (local) or the same `kubectl exec` pattern; if it already
ran, re-running is a no-op.

## Endpoints

All admin API calls go through the gateway at `/api/v1/admin/*`, which
applies `JWTAuthWithRevocation` and `AdminOnly` before proxying
(`services/api-gateway/cmd/main.go`, admin route group). Frontend sections
(`/admin/users`, `/admin/overlays`, `/admin/sources`) read from these.

**auth-service** (`/admin`, group in `cmd/main.go`):

- Users: list/search, `GET /admin/users/:id`, impersonate, ban/unban,
  list banned
- `GET /admin/stats` dashboard stats
- Viewers: list, session activity, ban/unban, set viewer premium
- Cosmetics catalog: frames and flairs (list/create/delete)

**overlay-manager** (`cmd/main.go`):

- `GET /admin/overlays`, `GET /admin/overlays/:id/sources`
- `GET /admin/sources` (system-wide, filterable)
- `GET /admin/user-overlays/:id`
- Maintenance windows (list/create/delete)

**share-service** (`handlers/admin*.go`, separate admin groups):

- Premium grants, beta-tester roles, ambassadors (ADR-0041, ADR-0020)
- Feature gates (`GET /api/v1/admin/feature-gates`,
  `PATCH /feature-gates/:key`)
- Localization review queue and export (ADR-0063)

## Troubleshooting

- **"Admin access required" after granting**: the JWT predates the grant.
  Log out, clear `localStorage`, log back in.
- Column exists but no access: check
  `SELECT is_admin FROM users WHERE username = '...';` and re-login.
