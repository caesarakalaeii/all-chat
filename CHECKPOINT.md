# Checkpoint: Viewer Message Sending Feature

**Date**: 2025-12-19
**Branch**: main
**Status**: In Progress - OAuth implementation complete, message sending endpoint pending

## Feature Overview

Implementing a feature that allows viewers to send messages to streamer chats through All-Chat at `allch.at/chat/{streamer}`. Viewers authenticate with their platform (starting with Twitch) and send messages using their own account, avoiding the need for moderation tools.

## ✅ Completed Work

### 1. Database Schema (Migration 011)

**Files Created**:
- `migrations/011_viewer_authentication.sql`
- `migrations/011_viewer_authentication_down.sql`

**Tables Added**:
- `viewer_sessions`: Stores viewer OAuth sessions with encrypted tokens
  - Supports Twitch, YouTube, Kick, TikTok (platform field)
  - Includes rate limiting fields (message_count_1min, message_count_1hour)
  - Tracks token expiration for refresh logic

- `viewer_message_history`: Audit log of all messages sent through All-Chat
  - Links to viewer_session_id and streamer_user_id
  - Tracks success/failure with error messages
  - Records timestamp and platform information

### 2. Backend - Auth Service

**New Files Created**:
- `services/auth-service/models/viewer.go`
  - ViewerSession, ViewerMessageLog, ViewerAuthResponse, ViewerInfo, ViewerJWTClaims

- `services/auth-service/repository/viewer_repository.go`
  - Full CRUD operations for viewer sessions
  - Token encryption support (uses same cipher as UserRepository)
  - Rate limit update methods
  - Message logging functionality

- `services/auth-service/oauth/viewer_twitch.go`
  - ViewerTwitchOAuth provider with `user:write:chat` scope
  - Extends base TwitchOAuth with chat write permissions

- `services/auth-service/handlers/viewer_auth.go`
  - HandleTwitchLogin: Initiates OAuth flow with optional streamer context
  - HandleTwitchCallback: Completes OAuth, encrypts tokens, generates JWT
  - HandleMe: Returns viewer info (protected route)
  - HandleLogout: Deletes viewer session (protected route)

**Modified Files**:
- `services/auth-service/cmd/main.go`
  - Added viewerTwitchOAuth initialization (line 169-171)
  - Added viewerRepo initialization (line 173-174)
  - Added viewerAuthHandler initialization (line 179)
  - Added viewer auth routes (line 226-228)
  - Added viewer protected routes (line 245-251)

### 3. API Gateway

**Modified Files**:
- `services/api-gateway/cmd/main.go`
  - Added viewer OAuth proxy routes (line 194-196):
    - `GET /api/v1/auth/viewer/twitch/login`
    - `GET /api/v1/auth/viewer/twitch/callback`

### 4. Architecture Decisions

**OAuth Flow**:
1. Viewer clicks login on `/chat/{streamer}` page
2. Frontend calls `GET /api/v1/auth/viewer/twitch/login?streamer={username}`
3. Backend returns OAuth URL with state stored in Redis
4. Twitch redirects to callback with code
5. Backend exchanges code for tokens (with `user:write:chat` scope)
6. Backend encrypts tokens, stores in `viewer_sessions` table
7. Backend generates JWT with viewer claims (is_viewer=true)
8. Backend redirects to frontend with JWT token
9. Frontend stores JWT and uses it for message sending

**JWT Claims Structure**:
```json
{
  "session_id": "uuid",
  "platform": "twitch",
  "platform_user_id": "12345",
  "username": "viewer_username",
  "is_viewer": true,
  "exp": 1234567890,
  "iat": 1234567890
}
```

**Rate Limiting Design**:
- 20 messages per minute (message_count_1min)
- 100 messages per hour (message_count_1hour)
- Counters stored in database (viewer_sessions table)
- Reset timestamps tracked (rate_limit_reset_1min, rate_limit_reset_1hour)

## 🚧 Pending Work

### 1. Send Message Endpoint (NEXT TASK)

**Create New Service or Add to API Gateway**:

Option A: Create `services/chat-sender/` service
- Dedicated microservice for message sending
- Easier to scale independently
- Clear separation of concerns

Option B: Add to API Gateway as handler
- Simpler deployment (fewer services)
- Direct access to viewer JWT validation
- Suitable for initial implementation

**Recommended Approach**: Start with Option B, migrate to Option A if needed

**Implementation Steps**:
1. Create Twitch API client function to send messages
   - Use Helix API `POST /helix/chat/messages`
   - Requires: broadcaster_id, sender_id, message, access_token
   - Handle API errors and rate limits

2. Create message sending handler
   - File: `services/api-gateway/handlers/chat_send.go`
   - Validate viewer JWT (middleware)
   - Check rate limits (query viewer_sessions)
   - Decrypt viewer access token
   - Call Twitch API
   - Update rate limit counters
   - Log to viewer_message_history
   - Return success/error response

3. Add rate limiting logic
   - Helper function: `checkRateLimit(viewerSession) (allowed bool, error)`
   - Update counters: `updateRateLimitCounters(sessionID, timestamp)`
   - Reset logic: if current_time > reset_time, reset counter to 0

4. Wire up in API Gateway
   - Route: `POST /api/v1/chat/send`
   - Middleware: JWTAuth (check is_viewer claim)
   - Request body: `{"streamer_username": "...", "message": "..."}`

### 2. Streamer Info Endpoint

**Purpose**: Frontend needs to know which platforms are active for a streamer

**Endpoint**: `GET /api/v1/streamers/{username}`

**Response**:
```json
{
  "username": "caesarlp",
  "display_name": "CaesarLP",
  "platforms": [
    {
      "platform": "twitch",
      "channel_id": "12345",
      "channel_name": "caesarlp",
      "is_live": true
    },
    {
      "platform": "youtube",
      "channel_id": "UC...",
      "channel_name": "CaesarLP",
      "is_live": false
    }
  ]
}
```

**Implementation**:
- Query `users` table to get user by username
- Query `overlay_chat_sources` to get active sources
- Optionally check live status (future enhancement)

### 3. Frontend Implementation

**New Page**: `frontend/app/chat/[streamer]/page.tsx`

**Components Needed**:
- ChatDisplay: Reuse WebSocket logic from overlay display
  - Connect to `ws://localhost:8080/ws/overlay/{overlay_id}` or create new viewer endpoint
  - Display aggregated messages from all platforms

- LoginButton: Trigger viewer OAuth flow
  - Redirect to `/api/v1/auth/viewer/twitch/login?streamer={username}`
  - Handle callback and store JWT token

- MessageInput: Text input + send button
  - Validate message length
  - Show rate limit status
  - Call `POST /api/v1/chat/send`

- PlatformSelector: If streamer has multiple platforms
  - Let viewer choose which platform to send to
  - Default to their logged-in platform

**Auth Flow Pages**:
- `/chat/auth-success?token={jwt}&streamer={username}`: Store JWT, redirect to chat
- `/chat/auth-error?error={message}`: Display error message

### 4. Testing & Migration

**Steps**:
1. Run database migration:
   ```bash
   make migrate-up
   # Or manually: psql ... < migrations/011_viewer_authentication.sql
   ```

2. Test OAuth flow:
   - Visit `/api/v1/auth/viewer/twitch/login?streamer=test`
   - Complete Twitch OAuth
   - Verify JWT token generated
   - Check `viewer_sessions` table populated

3. Test message sending (once implemented):
   - Send message with valid JWT
   - Verify message appears in Twitch chat
   - Check rate limits enforced
   - Verify `viewer_message_history` logging

4. Test error cases:
   - Invalid JWT
   - Expired token
   - Rate limit exceeded
   - Invalid streamer username
   - Streamer not streaming on platform

## Important Notes

### Environment Variables Required

**Existing** (already in use):
- `TWITCH_CLIENT_ID`: Used for both streamer and viewer OAuth
- `TWITCH_CLIENT_SECRET`: Used for both streamer and viewer OAuth
- `JWT_SECRET`: Used for both streamer and viewer JWTs
- `TOKEN_ENCRYPTION_KEY`: Used for encrypting viewer tokens
- `FRONTEND_URL`: Used to build OAuth redirect URLs

**No New Variables Required**: Reusing existing Twitch credentials with different scopes

### Security Considerations

1. **Token Encryption**: Viewer tokens are encrypted using same AES cipher as streamer tokens
2. **JWT Separation**: Viewer JWTs have `is_viewer: true` claim to distinguish from streamer tokens
3. **Rate Limiting**: Database-backed rate limits prevent abuse
4. **Audit Logging**: All messages logged to `viewer_message_history` table
5. **Token Expiry**: Viewer tokens checked for expiration (refresh logic needed in future)

### Twitch API Details

**Send Message Endpoint**:
```
POST https://api.twitch.tv/helix/chat/messages
Headers:
  Authorization: Bearer {viewer_access_token}
  Client-Id: {client_id}
  Content-Type: application/json
Body:
{
  "broadcaster_id": "12345",  // Streamer's Twitch user ID
  "sender_id": "67890",        // Viewer's Twitch user ID
  "message": "Hello from All-Chat!"
}
```

**Required Scope**: `user:write:chat` (already configured in ViewerTwitchOAuth)

**Rate Limits**: Twitch has its own rate limits, but our database rate limits are more conservative

### Future Enhancements

1. **Token Refresh Logic**: Automatically refresh expired viewer tokens
2. **YouTube Message Sending**: Implement YouTube Live Chat message sending
3. **Kick Message Sending**: Implement Kick message sending via WebSocket
4. **Message History**: Show viewer's own message history
5. **Live Status**: Show which platforms streamer is currently live on
6. **Viewer Preferences**: Save preferred platform, message history settings

## File Summary

### New Files (6)
- migrations/011_viewer_authentication.sql
- migrations/011_viewer_authentication_down.sql
- services/auth-service/models/viewer.go
- services/auth-service/repository/viewer_repository.go
- services/auth-service/oauth/viewer_twitch.go
- services/auth-service/handlers/viewer_auth.go

### Modified Files (2)
- services/auth-service/cmd/main.go
- services/api-gateway/cmd/main.go

### Files to Create (Next Session)
- services/api-gateway/handlers/chat_send.go (or services/chat-sender/)
- services/api-gateway/handlers/streamer_info.go
- frontend/app/chat/[streamer]/page.tsx
- frontend/app/chat/auth-success/page.tsx
- frontend/app/chat/auth-error/page.tsx

## Next Session TODO

1. **Run Migration**: Apply migration 011 to database
2. **Implement Send Message Endpoint**: Create Twitch API integration
3. **Add Rate Limiting**: Implement counter checks and updates
4. **Test OAuth Flow**: Verify end-to-end authentication works
5. **Build Frontend**: Create chat page with login and message sending

## Questions to Consider

1. **WebSocket for Chat Display**: Should viewers use same WebSocket endpoint as overlays, or create viewer-specific endpoint?
2. **Service Architecture**: Keep message sending in API Gateway or create dedicated chat-sender service?
3. **Live Status Check**: Should we query Twitch/YouTube APIs to show live status, or just show configured platforms?
4. **Message Formatting**: Should we support emotes, mentions, or just plain text initially?

## Git Status

All changes staged and ready to commit. No conflicts expected.
