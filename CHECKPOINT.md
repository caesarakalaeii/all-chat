# OAuth Source Addition Feature - Checkpoint

**Date**: 2025-11-16
**Status**: ✅ WORKING - Minor UI improvement pending deployment

## Current Status

### ✅ Completed & Working in Production (allch.at)

The OAuth-based source addition feature is **fully functional** and deployed to the cluster. Users can now click "Login with YouTube/Twitch/Kick/TikTok" to automatically add their streaming channels to overlays.

**Verified Working:**
- Multi-platform OAuth flow (Twitch, YouTube, Kick, TikTok)
- Account linking (links YouTube to existing Twitch account)
- Source auto-creation after OAuth
- User stays authenticated and redirects to overlay page
- Success notifications (via query parameters)
- Both Twitch and YouTube sources display correctly

### 🚀 Successfully Deployed Components

**Backend Services (all running in k3d cluster):**
- ✅ Auth Service - OAuth state management, account linking, add-source endpoints
- ✅ Overlay Manager - Internal auto-create source API
- ✅ API Gateway - Routes for add-source endpoints

**Frontend:**
- ✅ OAuth login buttons (4 platforms with icons)
- ✅ Advanced manual entry (collapsible section)
- ✅ Success/error notifications
- ✅ OAuth callback handling with redirect_to support
- ✅ GitHub project link in navbar

### 🔧 Pending Deployment

**Minor UI Improvement (committed, build in progress):**
- Display `channel_name` (e.g., "Caesar LP") instead of `channel_id` (e.g., "101802728631468199113")
- Type definition updated: `ChatSource` now includes optional `channel_name` field
- Commits: `3777e23` and `a07796e`

**To deploy:**
```bash
# Wait for GitHub Actions build to complete
gh run list --limit 1

# Once successful, restart frontend
kubectl rollout restart deployment/frontend -n allchat
kubectl rollout status deployment/frontend -n allchat
```

## Architecture Summary

### OAuth Flow with Account Linking

```
1. User clicks "Login with YouTube" on overlay page
   ↓
2. Frontend calls /api/v1/auth/youtube/add-source/:overlay_id
   - Sends JWT token in Authorization header
   - Backend extracts user_id from JWT
   ↓
3. Backend generates OAuth URL with enhanced state:
   {
     "csrf_token": "...",
     "overlay_id": "...",
     "user_id": "a81170f6-...",  // Current user ID
     "action": "add_source"
   }
   ↓
4. User authorizes on YouTube
   ↓
5. YouTube redirects to /api/v1/auth/youtube/callback
   ↓
6. Backend:
   - Decodes state → finds user_id
   - Links YouTube account to existing user (account linking!)
   - Calls overlay-manager internal API to create source
   - Generates JWT for existing user
   ↓
7. Redirects to /auth/callback with JWT + redirect_to=/overlays/:id
   ↓
8. Frontend stores JWT and redirects to overlay page
   ↓
9. Success! User sees both Twitch and YouTube sources
```

### Key Fixes Applied

1. **API Gateway Routing** - Added add-source endpoint routes
2. **OVERLAY_MANAGER_URL** - Set to `http://overlay-manager:8082` in auth-service deployment
3. **Internal API Path** - Changed from `/api/v1/internal/...` to `/internal/...`
4. **Account Linking** - Implemented `linkPlatformToUser` method
5. **JWT Authentication** - Made add-source endpoints require JWT
6. **State Management** - Added user_id to OAuth state for linking context
7. **Redirect Fix** - Route through /auth/callback to preserve authentication

## Database Schema (No Migration Needed)

The existing schema already supports multiple platform IDs per user:

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    twitch_id VARCHAR(50) UNIQUE,        -- Can be NULL
    google_id VARCHAR(50) UNIQUE,        -- Can be NULL
    kick_id VARCHAR(255) UNIQUE,         -- Can be NULL
    tiktok_open_id VARCHAR(255) UNIQUE,  -- Can be NULL
    ...
)
```

Account linking works by updating these nullable fields on the same user record.

## Files Modified

### Backend
- `services/auth-service/oauth/state.go` - Added user_id field
- `services/auth-service/oauth/state_test.go` - Updated tests
- `services/auth-service/handlers/platform_auth_v2.go` - Account linking logic
- `services/auth-service/cmd/main.go` - Protected add-source routes
- `services/overlay-manager/handlers/sources.go` - Internal auto-create endpoint
- `services/overlay-manager/cmd/main.go` - Registered internal routes
- `services/api-gateway/cmd/main.go` - Added add-source route proxying
- `deployments/k8s/base/auth-service/deployment.yaml` - OVERLAY_MANAGER_URL env var

### Frontend
- `frontend/src/app/overlays/[id]/page.tsx` - OAuth buttons + JWT in requests
- `frontend/src/app/auth/callback/page.tsx` - redirect_to handling
- `frontend/src/lib/types/overlay.ts` - Added channel_name to ChatSource

### Documentation
- `OAUTH_SOURCE_ADDITION.md` - Complete feature documentation
- `README.md` - GitHub link
- `CHECKPOINT.md` - This file

## Environment Variables

### Auth Service (Kubernetes)
```yaml
- name: OVERLAY_MANAGER_URL
  value: "http://overlay-manager:8082"
```

Applied via: `kubectl patch deployment auth-service -n allchat`

## Testing Results

### Playwright Test (13:36 UTC)
- ✅ User logged in with Twitch
- ✅ Clicked "Login with YouTube" button
- ✅ Completed Google OAuth flow
- ✅ YouTube account linked to Twitch user
- ✅ YouTube source auto-created
- ✅ Redirected to overlay page (not home page)
- ✅ Both sources visible: YouTube + Twitch

**Screenshot**: `.playwright-mcp/oauth-source-addition-success.png`

## Known Issues

### ✅ RESOLVED
1. ~~Connection refused to overlay-manager~~ - Fixed with OVERLAY_MANAGER_URL env var
2. ~~404 from overlay-manager~~ - Fixed internal API path
3. ~~Overlay not found error~~ - Fixed with account linking
4. ~~Redirect to home page~~ - Fixed by routing through /auth/callback
5. ~~Separate user accounts per platform~~ - Fixed with account linking

### 🔄 In Progress
1. **UI: Channel ID vs Name** - Committed, build in progress
   - Currently shows: `101802728631468199113`
   - Will show: `Caesar LP` (or display name)
   - Commits: `3777e23`, `a07796e`

## Next Steps (When Resuming)

1. **Check build status:**
   ```bash
   gh run list --limit 1
   ```

2. **If build succeeded, deploy frontend:**
   ```bash
   kubectl rollout restart deployment/frontend -n allchat
   kubectl rollout status deployment/frontend -n allchat
   ```

3. **Test channel name display:**
   - Navigate to https://allch.at/overlays/8a314647-5638-4d65-9784-80c341190b60
   - Verify YouTube source shows "Caesar LP" instead of ID

4. **Optional enhancements:**
   - Add account unlinking capability
   - Show which platforms are linked in user profile
   - Better error messages for OAuth failures
   - Support for users with multiple channels per platform

## Git Commits (Latest First)

```
a07796e - fix(types): add channel_name to ChatSource type
3777e23 - fix(ui): display channel name instead of ID in source list
97d6b96 - feat(oauth): implement multi-platform account linking
5f92471 - fix(oauth): correct overlay-manager internal API URL path
a69e048 - fix(oauth): preserve auth and redirect to overlay after source addition
8cc0250 - (previous commits before OAuth feature)
```

## API Endpoints

### Public Endpoints
```
GET /api/v1/auth/:platform/login
GET /api/v1/auth/:platform/callback
```

### Protected Endpoints (JWT Required)
```
GET /api/v1/auth/:platform/add-source/:overlay_id
POST /api/v1/internal/overlays/:id/sources/auto
```

## Kubernetes Deployment Commands Used

```bash
# Restart services
kubectl rollout restart deployment/auth-service -n allchat
kubectl rollout restart deployment/overlay-manager -n allchat
kubectl rollout restart deployment/frontend -n allchat

# Check status
kubectl rollout status deployment/auth-service -n allchat
kubectl get pods -n allchat -l app=auth-service

# View logs
kubectl logs -n allchat deployment/auth-service --tail=50
kubectl logs -n allchat deployment/overlay-manager --tail=50

# Set environment variable
kubectl patch deployment auth-service -n allchat --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/env/-",
  "value": {"name": "OVERLAY_MANAGER_URL", "value": "http://overlay-manager:8082"}}]'
```

## Success Metrics

- ✅ Feature implemented and tested end-to-end
- ✅ All services building successfully
- ✅ Deployed to production (allch.at)
- ✅ Account linking working (multi-platform support)
- ✅ User experience smooth (OAuth + auto-create)
- ✅ Documentation complete

## References

- **GitHub Repo**: https://github.com/caesarakalaeii/all-chat
- **Live Site**: https://allch.at
- **Feature Docs**: OAUTH_SOURCE_ADDITION.md
- **Project Docs**: CLAUDE.md, GETTING_STARTED.md
