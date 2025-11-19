# Credit Roll OAuth Flow - Progressive Scope Requests

## Philosophy: Opt-In Feature with Progressive OAuth

**Key Principle**: Don't ask for credit roll scopes until the user explicitly enables the feature.

## OAuth Scope Strategy

### Initial Login (Existing Chat Overlay Feature)
When users first sign up for All-Chat, we ONLY request scopes needed for chat:

**Twitch**:
```
(No scopes needed - uses bot OAuth for IRC)
```

**YouTube**:
```
https://www.googleapis.com/auth/youtube.readonly  # Read live chat
```

**Kick**:
```
(No OAuth needed - public WebSocket)
```

**TikTok**:
```
user.info.basic
user.info.profile
```

**Result**: Users can immediately use chat overlay without extensive permissions.

---

### Enabling Credit Roll Feature (Progressive OAuth)

When user navigates to "Credit Roll" settings and clicks "Enable":

#### Step 1: Show Feature Explanation
```
┌────────────────────────────────────────────────┐
│ Enable Hollywood Credit Rolls                  │
│                                                │
│ Automatically create end-of-stream credits    │
│ with today's followers, subs, bits, and clips.│
│                                                │
│ 📋 This feature will track:                   │
│  • New followers                               │
│  • Subscribers (new + resubs)                  │
│  • Bits/cheers                                 │
│  • Raids                                       │
│  • Unique chatters                             │
│                                                │
│ 🔒 Additional Permissions Needed:             │
│  • Read subscription events                    │
│  • Read follower information                   │
│  • Read bits/cheer events                      │
│                                                │
│ [Cancel] [Grant Permissions & Enable]         │
└────────────────────────────────────────────────┘
```

#### Step 2: Request Additional Scopes (Per Platform)

**Twitch - Additional Scopes**:
```
channel:read:subscriptions      # Track subs/resubs
moderator:read:followers        # Track new followers
bits:read                       # Track bits/cheers
```

**YouTube - Additional Scopes**:
```
https://www.googleapis.com/auth/yt-analytics.readonly
https://www.googleapis.com/auth/youtube.channel-memberships.creator
```

**Kick**:
```
(No additional scopes - WebSocket provides events)
```

**TikTok**:
```
(TBD - depends on Live Events API requirements)
```

#### Step 3: OAuth Flow
```
User clicks "Grant Permissions & Enable"
    ↓
Redirect to Twitch OAuth:
  /auth/twitch/credit-roll?state={csrf_token}
    ↓
Twitch shows permissions dialog:
  "All-Chat wants to:
   - View your subscriber list
   - View your follower list
   - View bits/cheer events"
    ↓
User approves
    ↓
Redirect back to All-Chat:
  /auth/twitch/credit-roll/callback?code=...&state=...
    ↓
Exchange code for token (with new scopes)
    ↓
Update user's access token in database
    ↓
Enable credit roll feature for user
    ↓
Start EventSub collector automatically
```

## Database Schema for Scope Tracking

### Option 1: Add Feature Flag to Users Table

```sql
ALTER TABLE users ADD COLUMN credit_roll_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN credit_roll_scopes_granted BOOLEAN DEFAULT FALSE;
```

### Option 2: Separate Feature Enablement Table (Recommended)

```sql
CREATE TABLE user_feature_flags (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    credit_roll_enabled BOOLEAN DEFAULT FALSE,
    credit_roll_scopes_granted BOOLEAN DEFAULT FALSE,
    credit_roll_enabled_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

**Benefits**:
- Cleaner separation of concerns
- Easy to add more features later
- Can track when features were enabled

## Implementation in Auth Service

### New OAuth Endpoints

```go
// GET /auth/twitch/credit-roll
// Redirects to Twitch OAuth with additional scopes
func (h *AuthHandler) InitiateCreditRollOAuth(c *gin.Context) {
    // Generate state token
    state := generateState()

    // Build OAuth URL with ADDITIONAL scopes
    scopes := []string{
        "channel:read:subscriptions",
        "moderator:read:followers",
        "bits:read",
    }

    authURL := buildTwitchOAuthURL(scopes, state, "/credit-roll")

    c.Redirect(302, authURL)
}

// GET /auth/twitch/credit-roll/callback
// Handle OAuth callback for credit roll feature
func (h *AuthHandler) CreditRollOAuthCallback(c *gin.Context) {
    code := c.Query("code")
    state := c.Query("state")

    // Validate state, exchange code for token
    token, err := exchangeCode(code)

    // Update user's access token (now includes new scopes)
    user := getCurrentUser(c)
    user.AccessToken = encryptToken(token.AccessToken)
    user.RefreshToken = encryptToken(token.RefreshToken)

    // Enable credit roll feature
    enableCreditRollFeature(user.ID)

    // Start EventSub collector automatically
    startEventSubCollector(user.ID, user.TwitchID, token.AccessToken)

    c.Redirect(302, "/settings/credit-roll?success=true")
}
```

### Frontend Flow

```javascript
// Credit Roll Settings Page
function CreditRollSettings() {
  const { user } = useAuth();
  const [enabled, setEnabled] = useState(user.credit_roll_enabled);

  const handleEnable = async () => {
    if (!user.credit_roll_scopes_granted) {
      // Redirect to OAuth flow
      window.location.href = '/auth/twitch/credit-roll';
    } else {
      // Scopes already granted, just enable feature
      await fetch('/api/v1/credit-roll/enable', { method: 'POST' });
      setEnabled(true);
    }
  };

  return (
    <div>
      <h2>Hollywood Credit Rolls</h2>
      {!enabled ? (
        <button onClick={handleEnable}>
          Enable Credit Rolls
        </button>
      ) : (
        <div>
          ✅ Credit rolls enabled!
          <CreditRollConfiguration />
        </div>
      )}
    </div>
  );
}
```

## User Experience Flow

### First-Time User (Chat Only)
```
1. Sign up with Twitch → Basic/no scopes
2. Create chat overlay → Works immediately
3. (Never sees credit roll feature unless they look for it)
```

### User Enables Credit Rolls
```
1. Navigate to "Settings" → "Features" → "Credit Rolls"
2. See feature description and required permissions
3. Click "Enable Credit Rolls"
4. Redirect to Twitch OAuth (additional scopes)
5. Approve permissions
6. Redirect back → Feature enabled
7. Configure preferences (one time)
8. Add overlay URL to OBS
9. Done - works forever!
```

### Existing User (Already Has Chat)
```
1. See banner: "New Feature: Hollywood Credit Rolls!"
2. Click "Learn More"
3. See explanation + permission requirements
4. Choose to enable or dismiss
5. If enable → OAuth flow (doesn't affect chat overlay)
```

## Scope Management Strategy

### Incremental Re-Authentication
When re-requesting OAuth with additional scopes:
- Twitch preserves existing scopes
- New scopes are ADDED to existing permissions
- Users see: "All-Chat wants to ALSO access: [new scopes]"
- Approving doesn't revoke existing chat access

### Fallback Behavior
If user denies additional scopes:
- Credit roll feature stays disabled
- Chat overlay continues working normally
- User can retry enabling credit roll later

## Migration Path for Existing Users

For users who already use All-Chat for chat overlays:

```sql
-- All existing users default to credit_roll_enabled = FALSE
-- They continue using chat without interruption
-- Only when they explicitly enable credit roll do we request new scopes
```

**Banner/Notification**:
```
╔════════════════════════════════════════════════╗
║ 🎬 New Feature: Hollywood Credit Rolls        ║
║                                                ║
║ Show your followers, subs, and bits in        ║
║ beautiful end-of-stream credits!              ║
║                                                ║
║ [Learn More] [Enable Now] [Dismiss]           ║
╚════════════════════════════════════════════════╝
```

## Implementation Checklist

- [ ] Add `credit_roll_enabled` to users table (or user_feature_flags)
- [ ] Create `/auth/twitch/credit-roll` OAuth endpoint (additional scopes)
- [ ] Create `/auth/twitch/credit-roll/callback` handler
- [ ] Add "Enable Credit Rolls" button in frontend settings
- [ ] Auto-start EventSub collector when user enables feature
- [ ] Show permission explanation before OAuth redirect
- [ ] Add feature announcement banner for existing users
- [ ] Update privacy policy with new scope usage

## Security Considerations

### Principle of Least Privilege
- Only request scopes when feature is enabled
- Clear explanation of what each scope is used for
- Users can disable feature and revoke scopes later

### Token Storage
- Store access tokens encrypted (already implemented)
- Refresh tokens before expiry
- Handle revocation gracefully (feature disables automatically)

### Audit Log
Track feature enablement:
```sql
CREATE TABLE feature_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    feature_name VARCHAR(50),  -- 'credit_roll'
    action VARCHAR(50),         -- 'enabled', 'disabled', 'scopes_granted'
    timestamp TIMESTAMP DEFAULT NOW()
);
```

## Future: Multi-Platform Incremental OAuth

Same pattern for other platforms:

**YouTube Credit Rolls**:
- Initial: `youtube.readonly` (chat only)
- Credit Roll: + `yt-analytics.readonly` + `channel-memberships.creator`

**Kick**:
- No additional scopes needed (WebSocket provides everything)

**TikTok**:
- Initial: `user.info.basic`
- Credit Roll: + TikTok Live Events API scopes (TBD)

## Summary

✅ **Default Experience**: Users get chat overlay with minimal scopes
✅ **Opt-In**: Credit roll feature requires explicit enablement
✅ **Progressive OAuth**: Additional scopes requested only when needed
✅ **Non-Intrusive**: Existing users aren't interrupted
✅ **Clear Communication**: Explain what permissions are needed and why

This approach maximizes trust and adoption while maintaining security best practices.
