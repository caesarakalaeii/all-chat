# Admin Dashboard - Quick Start Guide

## What You Now Have

A fully functional admin dashboard with role-based access control (RBAC).

## How to Access

### Step 1: Wait for Deployment

The CI/CD pipeline is automatically deploying:
- Frontend with admin dashboard
- Auth service with RBAC
- Overlay manager with admin endpoints
- Database migration (adds `is_admin` column)

Check deployment status:
```bash
gh run list --limit 1
```

### Step 2: Grant Yourself Admin Access

Once deployed, run this command to make yourself an admin:

```bash
./scripts/k8s-make-user-admin.sh <your-username>
```

For example, if your username is `caesarlp`:
```bash
./scripts/k8s-make-user-admin.sh caesarlp
```

**Important**: Replace `<your-username>` with your actual username from the platform you used to sign up (Twitch, YouTube, Kick, or TikTok).

### Step 3: Log Out and Log Back In

1. Go to https://your-domain.com
2. Log out (if currently logged in)
3. Log back in using your platform credentials
4. Your new JWT token will now include the admin role

### Step 4: Access Admin Dashboard

Navigate to: `https://your-domain.com/admin`

You should now see:
- **Users** - All users in the system
- **Overlays** - All overlays with source counts
- **Sources** - System-wide source overview

## What Each Section Shows

### Users (`/admin/users`)
- List of all registered users
- Platform connections (Twitch, YouTube, Kick, TikTok)
- User details: ID, username, display name, auth provider
- Platform-specific IDs
- User's overlays

### Overlays (`/admin/overlays`)
- All overlays across all users
- Source counts for each overlay
- Overlay details: name, ID, user ID, creation date
- Connected sources with:
  - Platform and channel information
  - Active/Inactive status
  - Creation dates

### Sources (`/admin/sources`)
- Platform statistics dashboard
- Comprehensive table of all sources
- Filtering by:
  - Search (channel name or ID)
  - Platform (Twitch, YouTube, Kick, TikTok)
  - Status (Active/Inactive)
- Links to parent overlays

## Security

### Who Can Access?

**Only users with `is_admin = TRUE` in the database.**

- New users: **NOT admin** by default
- Must be explicitly granted admin access
- Non-admin users get: `HTTP 403 - Admin access required`

### How to Grant More Admins

```bash
./scripts/k8s-make-user-admin.sh <username>
```

### How to Revoke Admin Access

```bash
kubectl exec -n allchat <postgres-pod> -- psql -U postgres -d allchat \
  -c "UPDATE users SET is_admin = FALSE WHERE username = '<username>';"
```

## Troubleshooting

### "Admin access required" error

**You see this error even though you ran the script?**

**Solution**: Log out and log back in. Your JWT token needs to be regenerated with the admin role.

### Can't find PostgreSQL pod

```bash
# List all pods to find the right one
kubectl get pods -n allchat

# Look for pods with names like:
# - allchat-cluster-1, allchat-cluster-2, etc.
# - postgres-xxxxx
```

### Script permission denied

```bash
chmod +x scripts/k8s-make-user-admin.sh
chmod +x scripts/make-user-admin.sh
```

## API Endpoints

All admin endpoints require:
- `Authorization: Bearer <jwt-token>` header
- JWT token must have "admin" role

**Available endpoints**:
- `GET /api/v1/admin/users` - List all users
- `GET /api/v1/admin/users/:id` - Get specific user
- `GET /api/v1/admin/overlays` - List all overlays
- `GET /api/v1/admin/sources` - List all sources

## Next Steps

1. **Apply the migration** (if not auto-applied):
   ```bash
   kubectl exec -n allchat <postgres-pod> -- psql -U postgres -d allchat \
     < migrations/009_add_admin_role.sql
   ```

2. **Make yourself admin**:
   ```bash
   ./scripts/k8s-make-user-admin.sh <your-username>
   ```

3. **Log out and back in** to get new JWT with admin role

4. **Access the dashboard**: https://your-domain.com/admin

That's it! You now have a secure, production-ready admin dashboard.
