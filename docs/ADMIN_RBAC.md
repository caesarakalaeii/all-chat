# Admin Role-Based Access Control (RBAC)

This document explains how admin access control works in All-Chat.

## Overview

All-Chat implements role-based access control (RBAC) using:
- **Database**: `is_admin` column in the `users` table
- **JWT Claims**: `roles` array in JWT tokens (includes "admin" for admin users)
- **Middleware**: `AdminOnly()` middleware that checks for admin role

## Default Behavior

By default, **all new users are created with `is_admin = FALSE`**.

Only users explicitly granted admin privileges can access:
- `/admin/*` routes in the frontend
- `/api/v1/admin/*` API endpoints

## How to Grant Admin Access

### Method 1: Using the Helper Script (Kubernetes)

```bash
# Grant admin access to a user by username
./scripts/k8s-make-user-admin.sh <username>

# Example:
./scripts/k8s-make-user-admin.sh caesarlp
```

This script:
1. Finds the PostgreSQL pod in the `allchat` namespace
2. Updates the user's `is_admin` field to `TRUE`
3. Shows the updated user record

### Method 2: Using the Local Script (Development)

```bash
# For local development with direct database access
./scripts/make-user-admin.sh <username>

# Example:
./scripts/make-user-admin.sh caesarlp
```

### Method 3: Direct SQL (Advanced)

```sql
-- Connect to PostgreSQL
kubectl exec -n allchat <postgres-pod> -- psql -U postgres -d allchat

-- Grant admin access by username
UPDATE users SET is_admin = TRUE WHERE username = 'caesarlp';

-- Or by user ID
UPDATE users SET is_admin = TRUE WHERE id = '550e8400-e29b-41d4-a716-446655440000';

-- Verify
SELECT id, username, display_name, is_admin FROM users WHERE is_admin = TRUE;
```

## JWT Token Structure

When a user logs in, their JWT token includes role information:

```json
{
  "sub": "user-id-here",
  "twitch_id": "12345678",
  "username": "caesarlp",
  "roles": ["user", "admin"],
  "iat": 1700000000,
  "exp": 1700086400,
  "iss": "all-chat"
}
```

- Regular users: `"roles": ["user"]`
- Admin users: `"roles": ["user", "admin"]`

## Middleware Chain

Admin routes use two middleware layers:

```go
admin := router.Group("/admin")
admin.Use(middleware.JWTAuth(jwtSecret))    // 1. Validates JWT token
admin.Use(middleware.AdminOnly())            // 2. Checks for admin role
{
    admin.GET("/users", adminHandler.ListUsers)
    // ...
}
```

1. **JWTAuth**: Validates the token and sets user info + roles in context
2. **AdminOnly**: Checks if "admin" is in the roles array

## HTTP Response Codes

- **200 OK**: Request successful
- **401 Unauthorized**: No valid JWT token provided
- **403 Forbidden**: Valid JWT token but user is not an admin

Example error response for non-admin user:
```json
{
  "error": "Admin access required"
}
```

## Security Best Practices

### 1. Initial Admin Setup

After deploying to production:

```bash
# Make the first admin user
./scripts/k8s-make-user-admin.sh <your-username>
```

### 2. Audit Admin Users

```sql
-- List all admin users
SELECT id, username, display_name, auth_provider, created_at
FROM users
WHERE is_admin = TRUE;
```

### 3. Revoke Admin Access

```sql
-- Remove admin privileges from a user
UPDATE users SET is_admin = FALSE WHERE username = '<username>';
```

### 4. Monitor Admin Actions

All admin endpoints log the action:

```json
{
  "level": "info",
  "msg": "Listed users",
  "count": 42
}
```

Consider implementing audit logging for admin actions in production.

## Protected Admin Endpoints

### Auth Service
- `GET /admin/users` - List all users
- `GET /admin/users/:id` - Get specific user details

### Overlay Manager
- `GET /admin/overlays` - List all overlays
- `GET /admin/sources` - List all sources system-wide

### API Gateway (Proxies)
- `GET /api/v1/admin/users` → auth-service
- `GET /api/v1/admin/users/:id` → auth-service
- `GET /api/v1/admin/overlays` → overlay-manager
- `GET /api/v1/admin/sources` → overlay-manager

## Frontend Access

The admin dashboard at `/admin` will:
1. Check for JWT token in localStorage
2. Make API calls with `Authorization: Bearer <token>` header
3. Display 401 error if not authenticated
4. Display 403 error if authenticated but not admin

## Migration

The admin role feature is added via migration `009_add_admin_role.sql`:

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin) WHERE is_admin = TRUE;
```

To apply the migration:

```bash
# Local development
make migrate

# Kubernetes
kubectl exec -n allchat <postgres-pod> -- psql -U postgres -d allchat < migrations/009_add_admin_role.sql
```

## Testing

### 1. Test Non-Admin Access

```bash
# Login as regular user
TOKEN=$(curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"regularuser","password":"..."}' | jq -r '.access_token')

# Try to access admin endpoint (should get 403)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/users
# Response: {"error":"Admin access required"}
```

### 2. Test Admin Access

```bash
# Make user admin first
./scripts/k8s-make-user-admin.sh adminuser

# Login as admin user
TOKEN=$(curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"adminuser","password":"..."}' | jq -r '.access_token')

# Access admin endpoint (should get 200)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/admin/users
# Response: [{"id":"...","username":"...","is_admin":true,...}]
```

## Troubleshooting

### "Admin access required" error for admin user

**Cause**: User has `is_admin = TRUE` in database but their JWT doesn't have the admin role.

**Solution**: User needs to log out and log back in to get a new JWT with the admin role.

### Migration failed

**Cause**: Migration might have already been applied.

**Solution**: Check if column exists:
```sql
SELECT column_name FROM information_schema.columns
WHERE table_name = 'users' AND column_name = 'is_admin';
```

### Can't access admin dashboard after granting admin

**Cause**: Old JWT token doesn't have admin role.

**Solution**:
1. Clear localStorage: `localStorage.clear()` in browser console
2. Log out and log back in
3. New JWT will include admin role

## Future Enhancements

1. **Multiple Roles**: Extend to support `moderator`, `support`, etc.
2. **Granular Permissions**: Specific permissions per action (read-only admin, etc.)
3. **Audit Logging**: Track all admin actions with timestamps and user info
4. **Admin User Management**: UI to grant/revoke admin access from the dashboard
5. **Session Invalidation**: Force logout when admin status changes
