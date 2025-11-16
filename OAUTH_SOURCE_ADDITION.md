# OAuth-Based Source Addition Feature

**Project Repository**: [caesarakalaeii/all-chat](https://github.com/caesarakalaeii/all-chat)

## Overview

This document describes the OAuth-based source addition feature that allows users to add chat sources to overlays by logging in with the platform (Twitch, YouTube, Kick, TikTok) instead of manually entering channel IDs.

## Problem Statement

Previously, users had to manually enter channel IDs when adding sources to overlays. This was problematic because:
- Users logged in with one platform (e.g., Twitch) couldn't easily add sources from another platform (e.g., YouTube)
- Manual ID entry is error-prone and user-unfriendly
- Users need to know their channel ID, which isn't always obvious

## Solution

Enable users to click "Login with [Platform]" to automatically add their channel as a source. The OAuth flow:
1. User clicks "Add YouTube Source" → Triggers OAuth flow with overlay context
2. After successful OAuth, the system automatically detects the user's channel ID
3. The source is created and the user is redirected back to the overlay page

## Architecture

### Backend Components

#### 1. Enhanced OAuth State Model
**File**: `services/auth-service/oauth/state.go`

```go
type OAuthState struct {
    CSRFToken string      // Random string for CSRF protection
    OverlayID string      // Target overlay for source addition (optional)
    Action    OAuthAction // "login" or "add_source"
}
```

The state parameter now carries context about what action to perform after OAuth callback:
- `login`: Regular authentication flow → redirect to `/auth/callback` with JWT
- `add_source`: Add source flow → auto-create source → redirect to overlay page

#### 2. Auth Service Enhancements
**File**: `services/auth-service/handlers/platform_auth_v2.go`

**New Endpoints**:
```
GET /api/v1/auth/:platform/add-source/:overlay_id
```
- Initiates OAuth flow specifically for adding a source
- Stores overlay_id in OAuth state
- Generates platform-specific auth URL (with PKCE for Kick)

**Enhanced Callback Handler**:
```
GET /api/v1/auth/:platform/callback
```
- Decodes state to determine action (login vs add_source)
- If add_source:
  - Calls overlay-manager internal API to create source
  - Redirects to overlay page with success/error status
- If login:
  - Generates JWT and redirects to auth callback page

**Key Features**:
- Validates state with CSRF protection
- Handles PKCE flow for Kick OAuth
- Automatically stores tokens for YouTube, Kick, TikTok in platform-specific tables
- Makes internal HTTP call to overlay-manager with JWT auth

#### 3. Overlay Manager Enhancements
**File**: `services/overlay-manager/handlers/sources.go`

**New Internal Endpoint**:
```
POST /api/v1/internal/overlays/:id/sources/auto
```

Request body:
```json
{
  "platform": "youtube",
  "channel_id": "UCxxx...",
  "channel_name": "Display Name"
}
```

This endpoint:
- Requires JWT authentication (called by auth-service)
- Validates user owns the overlay
- Automatically sets `auth_required` flag for YouTube, Kick, TikTok
- Creates source in database
- Returns created source object

### API Flow

```
┌──────────┐                  ┌──────────────┐                 ┌─────────────────┐
│ Frontend │                  │ Auth Service │                 │ Overlay Manager │
└────┬─────┘                  └──────┬───────┘                 └────────┬────────┘
     │                               │                                  │
     │ 1. GET /auth/youtube/         │                                  │
     │    add-source/:overlay_id     │                                  │
     ├──────────────────────────────>│                                  │
     │                               │                                  │
     │ 2. OAuth URL with state       │                                  │
     │<──────────────────────────────┤                                  │
     │    {action: "add_source",     │                                  │
     │     overlay_id: "uuid"}       │                                  │
     │                               │                                  │
     │ 3. User authorizes on YouTube │                                  │
     ├──────────────────────────────>│                                  │
     │                               │                                  │
     │ 4. YouTube redirects back     │                                  │
     │    with code + state          │                                  │
     ├──────────────────────────────>│                                  │
     │                               │                                  │
     │                               │ 5. Decode state                  │
     │                               │    → action = "add_source"       │
     │                               │                                  │
     │                               │ 6. Exchange code for token       │
     │                               │                                  │
     │                               │ 7. Get user info from YouTube    │
     │                               │                                  │
     │                               │ 8. Get/create user in DB         │
     │                               │                                  │
     │                               │ 9. Generate JWT                  │
     │                               │                                  │
     │                               │ 10. POST /internal/overlays/     │
     │                               │     :id/sources/auto             │
     │                               │     {channel_id, platform, ...}  │
     │                               ├─────────────────────────────────>│
     │                               │                                  │
     │                               │                        11. Validate user
     │                               │                            owns overlay
     │                               │                                  │
     │                               │                        12. Create source
     │                               │                            in database
     │                               │                                  │
     │                               │ 13. Source created               │
     │                               │<─────────────────────────────────┤
     │                               │                                  │
     │ 14. Redirect to overlay page  │                                  │
     │     /overlays/:id?            │                                  │
     │     source_added=youtube      │                                  │
     │<──────────────────────────────┤                                  │
     │                               │                                  │
```

## File Changes

### Created Files

1. **`services/auth-service/oauth/state.go`**
   - OAuth state model with action context
   - Helper functions: `NewLoginState`, `NewAddSourceState`, `Encode`, `Decode`, `Validate`

2. **`services/auth-service/oauth/state_test.go`**
   - Comprehensive tests for OAuth state model
   - Tests encoding, decoding, validation

3. **`services/auth-service/handlers/platform_auth_v2.go`**
   - Enhanced platform auth handler with add-source support
   - Methods: `HandleLogin`, `HandleAddSource`, `HandleCallback`
   - Helper: `addSourceToOverlay` (makes internal HTTP call)

### Modified Files

1. **`services/auth-service/cmd/main.go`**
   - Added `OVERLAY_MANAGER_URL` environment variable
   - Created `PlatformAuthHandlerV2` instance
   - Registered new routes:
     - `/twitch/add-source/:overlay_id`
     - `/youtube/add-source/:overlay_id`
     - `/kick/add-source/:overlay_id`
     - `/tiktok/add-source/:overlay_id`
   - Updated all platform routes to use V2 handler

2. **`services/overlay-manager/handlers/sources.go`**
   - Added `HandleAddSourceAuto` method for internal API
   - Added `RegisterInternalRoutes` method

3. **`services/overlay-manager/cmd/main.go`**
   - Registered internal routes group: `/internal/overlays/:id/sources/auto`

## Environment Variables

### Auth Service
```bash
# Required for internal API calls
OVERLAY_MANAGER_URL=http://localhost:8082  # or http://overlay-manager:8082 in K8s
```

### All Services
No additional environment variables required. Existing OAuth credentials work as-is.

## API Endpoints

### Public Endpoints (No Auth Required)

#### 1. Initiate Add-Source OAuth Flow
```
GET /api/v1/auth/:platform/add-source/:overlay_id
```

**Parameters**:
- `platform`: `twitch`, `youtube`, `kick`, or `tiktok`
- `overlay_id`: UUID of target overlay

**Response**:
```json
{
  "auth_url": "https://accounts.google.com/o/oauth2/v2/auth?..."
}
```

#### 2. OAuth Callback (Enhanced)
```
GET /api/v1/auth/:platform/callback?code=xxx&state=xxx
```

**Behavior**:
- Decodes state to determine action
- If `action=add_source`:
  - Creates source via internal API
  - Redirects to `/overlays/:id?source_added=:platform`
- If `action=login`:
  - Generates JWT
  - Redirects to `/auth/callback#access_token=xxx...`

### Internal Endpoints (JWT Required)

#### 3. Auto-Create Source
```
POST /api/v1/internal/overlays/:id/sources/auto
Authorization: Bearer <JWT>
```

**Request Body**:
```json
{
  "platform": "youtube",
  "channel_id": "UCxxx...",
  "channel_name": "Channel Display Name"
}
```

**Response**: `201 Created`
```json
{
  "id": "uuid",
  "overlay_id": "uuid",
  "platform": "youtube",
  "channel_id": "UCxxx...",
  "channel_name": "Channel Display Name",
  "auth_required": true,
  "is_active": true,
  "created_at": "2025-11-16T...",
  "updated_at": "2025-11-16T..."
}
```

## Frontend Integration (To Be Implemented)

### Current Flow
```tsx
// User manually enters channel ID
<input
  type="text"
  placeholder="Enter channel ID"
  onChange={(e) => setChannelId(e.target.value)}
/>
<button onClick={() => addSource(channelId)}>Add Source</button>
```

### New Flow
```tsx
// User clicks OAuth login button
<button onClick={() => {
  // Get auth URL from backend
  const response = await fetch(
    `/api/v1/auth/youtube/add-source/${overlayId}`
  );
  const { auth_url } = await response.json();

  // Redirect to OAuth
  window.location.href = auth_url;
}}>
  Login with YouTube
</button>

// Manual entry in collapsible "Advanced" section
<Collapsible title="Advanced: Manual ID Entry">
  <input type="text" placeholder="Enter channel ID" />
  <button>Add Manually</button>
</Collapsible>
```

### Redirect Handling
After OAuth callback, user is redirected to:
```
/overlays/:id?source_added=youtube
```

Frontend should:
1. Check for `source_added` query parameter
2. Show success notification
3. Refresh sources list
4. Remove query parameter from URL

## Security Considerations

1. **CSRF Protection**: State parameter includes random CSRF token stored in Redis (10 min TTL)
2. **JWT Authentication**: Internal API call requires valid JWT
3. **Overlay Ownership Validation**: Overlay-manager verifies user owns the overlay before creating source
4. **State Expiration**: OAuth state expires after 10 minutes
5. **PKCE for Kick**: Kick OAuth uses PKCE (Proof Key for Code Exchange) for additional security

## Testing

### Manual Testing

1. **Test Add-Source Flow (YouTube)**:
   ```bash
   # 1. Start services
   make docker-up

   # 2. Create an overlay (get overlay_id from response)
   curl -X POST http://localhost:8080/api/v1/overlays \
     -H "Authorization: Bearer $JWT" \
     -H "Content-Type: application/json" \
     -d '{"name": "Test Overlay"}'

   # 3. Get add-source auth URL
   curl http://localhost:8080/api/v1/auth/youtube/add-source/<overlay_id>

   # 4. Visit the auth_url in browser
   # 5. After OAuth, verify you're redirected to /overlays/:id?source_added=youtube
   # 6. Verify source was created:
   curl http://localhost:8080/api/v1/overlays/<overlay_id>/sources \
     -H "Authorization: Bearer $JWT"
   ```

2. **Test Regular Login Flow (Should Still Work)**:
   ```bash
   curl http://localhost:8080/api/v1/auth/youtube/login
   # Visit auth_url, verify normal login still works
   ```

### Unit Tests

Run tests for OAuth state model:
```bash
cd services/auth-service
go test ./oauth -v
```

## Known Limitations

1. **Channel ID Detection**:
   - For YouTube: Uses Google account ID (works for user's own channel)
   - For Twitch: Uses Twitch user ID
   - For Kick: Uses Kick user ID
   - For TikTok: Uses TikTok open_id
   - **Note**: Users can only add their own channels via OAuth (cannot add other users' channels)

2. **Error Handling**:
   - If source creation fails, user is redirected with `?error=failed_to_add_source`
   - Frontend needs to display appropriate error message

3. **Duplicate Sources**:
   - Database constraint prevents duplicate sources (same overlay + platform + channel)
   - Frontend should handle 409 Conflict gracefully

## Future Enhancements

1. **Support Adding Other Users' Channels**:
   - Add a search/lookup flow after OAuth
   - Allow users to specify which channel to add (if they manage multiple)

2. **Batch Source Addition**:
   - Allow adding multiple sources at once
   - Useful for users with multiple channels

3. **Source Permissions**:
   - Verify user has permission to read channel's chat
   - Show clearer error messages for permission issues

4. **OAuth Token Refresh**:
   - Implement automatic token refresh before expiry
   - Notify users when tokens need re-authorization

## Deployment Notes

### Environment Variables
Ensure `OVERLAY_MANAGER_URL` is set in auth-service:
- Development: `http://localhost:8082`
- Docker Compose: `http://overlay-manager:8082`
- Kubernetes: `http://overlay-manager.default.svc.cluster.local:8082`

### Database
No migrations required. Uses existing schema.

### API Gateway
No changes required. API Gateway already proxies:
- `/api/v1/auth/*` → auth-service
- `/api/v1/overlays/*` → overlay-manager

## Troubleshooting

### "Invalid or expired state" Error
- State expires after 10 minutes
- Redis connection issue
- Check Redis is running: `docker-compose ps redis`

### "Overlay not found" Error
- User doesn't own the overlay
- Overlay ID in URL is invalid
- Check JWT token contains correct user_id

### "Failed to call overlay-manager" Error
- Overlay Manager service is down
- Check `OVERLAY_MANAGER_URL` environment variable
- Verify network connectivity between services

### Source Not Created
- Check overlay-manager logs: `docker-compose logs overlay-manager`
- Verify JWT is valid
- Check database for errors: `docker-compose logs postgres`

## References

- [OAuth 2.0 RFC](https://datatracker.ietf.org/doc/html/rfc6749)
- [PKCE RFC](https://datatracker.ietf.org/doc/html/rfc7636)
- [YouTube OAuth](https://developers.google.com/youtube/v3/guides/authentication)
- [Twitch OAuth](https://dev.twitch.tv/docs/authentication/getting-tokens-oauth)
- [Kick OAuth](https://docs.kick.com/docs/oauth)
- [TikTok OAuth](https://developers.tiktok.com/doc/login-kit-web)
