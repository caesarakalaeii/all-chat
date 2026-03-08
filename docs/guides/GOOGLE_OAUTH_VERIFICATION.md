# Google OAuth Verification Checklist

This guide addresses the three outstanding issues preventing Google OAuth client approval.

## Issue 1: Incremental Authorization (Datenzugriff und Nutzereinwilligung)

**Problem**: Your OAuth client may not properly support incremental authorization.

**Current Implementation Issue**:
- `services/auth-service/oauth/youtube.go:53` uses `oauth2.ApprovalForce`, which forces re-consent every time
- Google expects apps to request only the scopes they need, when they need them

### Fix: Update YouTube OAuth Implementation

**File**: `services/auth-service/oauth/youtube.go`

Change line 53 from:
```go
return y.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
```

To:
```go
// Remove ApprovalForce to support incremental authorization
// Only request consent when needed (first time or when scopes change)
return y.config.AuthCodeURL(state,
    oauth2.AccessTypeOffline,
    oauth2.SetAuthURLParam("prompt", "consent"), // Only if user needs to grant new scopes
)
```

**Better approach** - Make it conditional:
```go
// GetAuthURL generates the OAuth authorization URL
// forceConsent should only be true when requesting additional scopes
func (y *YouTubeOAuth) GetAuthURL(state string, forceConsent bool) string {
    opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}

    if forceConsent {
        opts = append(opts, oauth2.SetAuthURLParam("prompt", "consent"))
    } else {
        // Use "select_account" to let users choose account without forcing re-consent
        opts = append(opts, oauth2.SetAuthURLParam("prompt", "select_account"))
    }

    return y.config.AuthCodeURL(state, opts...)
}
```

**Required Changes**:

1. Update `YouTubeOAuth.GetAuthURL()` signature to accept `forceConsent bool`
2. Update all callers in handlers to pass `false` for normal login, `true` only when adding new scopes
3. Update `ViewerYouTubeOAuth` similarly

### Supporting Incremental Authorization

To properly support incremental authorization, you should:

1. **Store granted scopes** in the database (you already have token storage)
2. **Check existing scopes** before requesting auth
3. **Only request consent** when new scopes are needed

Example flow:
```go
// In handler
existingScopes := getUserGrantedScopes(userID, "youtube")
requestedScopes := []string{
    "https://www.googleapis.com/auth/youtube.readonly",
    "https://www.googleapis.com/auth/youtube.force-ssl",
}

needsConsent := !hasAllScopes(existingScopes, requestedScopes)
authURL := youtubeOAuth.GetAuthURL(state, needsConsent)
```

---

## Issue 2: Cross-Product Account Protection (Anwendungssicherheit)

**Problem**: Your project is not configured for cross-product account protection.

This refers to Google's **Cross-Account Protection** (RISC/CAEP) which allows Google to notify your app about security events affecting user accounts.

### Fix: Implement RISC Event Receiver

**Step 1: Create RISC Event Handler**

Create new file: `services/auth-service/handlers/risc_handler.go`

```go
package handlers

import (
    "crypto/rsa"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "go.uber.org/zap"
)

// RISCHandler handles Cross-Account Protection (RISC) security events from Google
type RISCHandler struct {
    log           *zap.Logger
    userRepo      UserRepository // Your user repository interface
    googleJWKSURL string
    jwksCache     map[string]*rsa.PublicKey // Cache for Google's public keys
}

// RISCEvent represents a RISC security event from Google
type RISCEvent struct {
    Issuer   string                 `json:"iss"`
    IssuedAt int64                  `json:"iat"`
    JTI      string                 `json:"jti"`
    Audience string                 `json:"aud"`
    Events   map[string]interface{} `json:"events"`
}

func NewRISCHandler(log *zap.Logger, userRepo UserRepository) *RISCHandler {
    return &RISCHandler{
        log:           log,
        userRepo:      userRepo,
        googleJWKSURL: "https://www.googleapis.com/oauth2/v3/certs",
        jwksCache:     make(map[string]*rsa.PublicKey),
    }
}

// HandleSecurityEvent receives and processes RISC security events
func (h *RISCHandler) HandleSecurityEvent(c *gin.Context) {
    // Read the SET (Security Event Token)
    var setToken struct {
        SET string `json:"SET"` // JWT token
    }

    if err := c.ShouldBindJSON(&setToken); err != nil {
        h.log.Error("Failed to parse RISC event", zap.Error(err))
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    // Parse and verify the JWT
    token, err := jwt.Parse(setToken.SET, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }

        // Get the key ID from token header
        kid, ok := token.Header["kid"].(string)
        if !ok {
            return nil, fmt.Errorf("kid header missing")
        }

        // Fetch Google's public key (implement JWKS fetching)
        return h.fetchGooglePublicKey(kid)
    })

    if err != nil || !token.Valid {
        h.log.Error("Invalid RISC token", zap.Error(err))
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
        return
    }

    // Parse claims
    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        h.log.Error("Invalid claims format")
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid claims"})
        return
    }

    // Process events
    events, ok := claims["events"].(map[string]interface{})
    if !ok {
        h.log.Error("No events in RISC token")
        c.JSON(http.StatusBadRequest, gin.H{"error": "no events"})
        return
    }

    // Handle different event types
    for eventType, eventData := range events {
        h.log.Info("Received RISC event",
            zap.String("type", eventType),
            zap.Any("data", eventData),
        )

        switch eventType {
        case "https://schemas.openid.net/secevent/risc/event-type/account-disabled":
            h.handleAccountDisabled(claims, eventData)
        case "https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required":
            h.handleCredentialChangeRequired(claims, eventData)
        case "https://schemas.openid.net/secevent/risc/event-type/sessions-revoked":
            h.handleSessionsRevoked(claims, eventData)
        default:
            h.log.Warn("Unknown RISC event type", zap.String("type", eventType))
        }
    }

    // Acknowledge receipt
    c.Status(http.StatusAccepted)
}

// handleAccountDisabled handles account disabled events
func (h *RISCHandler) handleAccountDisabled(claims jwt.MapClaims, eventData interface{}) {
    subject, _ := claims["sub"].(string)
    if subject == "" {
        return
    }

    // Disable user account and revoke tokens
    h.log.Warn("Google account disabled", zap.String("subject", subject))

    // TODO: Implement
    // 1. Find user by Google subject ID
    // 2. Mark account as disabled
    // 3. Revoke all OAuth tokens
    // 4. Invalidate sessions
}

// handleCredentialChangeRequired handles credential change events
func (h *RISCHandler) handleCredentialChangeRequired(claims jwt.MapClaims, eventData interface{}) {
    subject, _ := claims["sub"].(string)
    if subject == "" {
        return
    }

    h.log.Warn("Google credential change required", zap.String("subject", subject))

    // TODO: Implement
    // 1. Find user by Google subject ID
    // 2. Force re-authentication on next login
    // 3. Optionally revoke current sessions
}

// handleSessionsRevoked handles session revocation events
func (h *RISCHandler) handleSessionsRevoked(claims jwt.MapClaims, eventData interface{}) {
    subject, _ := claims["sub"].(string)
    if subject == "" {
        return
    }

    h.log.Warn("Google sessions revoked", zap.String("subject", subject))

    // TODO: Implement
    // 1. Find user by Google subject ID
    // 2. Revoke all OAuth tokens
    // 3. Force re-authentication
}

// fetchGooglePublicKey fetches Google's public key from JWKS endpoint
func (h *RISCHandler) fetchGooglePublicKey(kid string) (*rsa.PublicKey, error) {
    // TODO: Implement JWKS fetching and caching
    // 1. Check cache first
    // 2. If not cached, fetch from h.googleJWKSURL
    // 3. Parse JWKS and extract public key for kid
    // 4. Cache the key
    // 5. Return the key

    return nil, fmt.Errorf("JWKS fetching not implemented")
}

// HandleConfigurationEndpoint returns RISC configuration
func (h *RISCHandler) HandleConfigurationEndpoint(c *gin.Context) {
    config := gin.H{
        "issuer": "https://your-domain.com",
        "jwks_uri": "https://your-domain.com/.well-known/jwks.json",
        "delivery": map[string]interface{}{
            "delivery_methods_supported": []string{"push"},
        },
        "critical_subject_claims_supported": []string{"sub"},
    }

    c.JSON(http.StatusOK, config)
}
```

**Step 2: Add RISC Routes**

Update `services/auth-service/cmd/main.go`:

```go
// Add after other handlers
riscHandler := handlers.NewRISCHandler(log, userRepo)

// Add RISC endpoints (before protected routes)
router.POST("/.well-known/risc-events", riscHandler.HandleSecurityEvent)
router.GET("/.well-known/risc-configuration", riscHandler.HandleConfigurationEndpoint)
```

**Step 3: Configure in Google Cloud Console**

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select your project
3. Navigate to **APIs & Services** → **Credentials**
4. Select your OAuth 2.0 client
5. In the **Cross-Account Protection** section:
   - Enable Cross-Account Protection
   - Set Security Event Receiver URL: `https://your-domain.com/.well-known/risc-events`
   - Set Configuration URL: `https://your-domain.com/.well-known/risc-configuration`

**Step 4: Store Google Subject ID**

Update your database schema to store Google's subject ID:

```sql
-- Add column to users table
ALTER TABLE users ADD COLUMN google_subject_id VARCHAR(255);
CREATE INDEX idx_users_google_subject_id ON users(google_subject_id);
```

Update user creation to store the subject ID from Google's ID token.

---

## Issue 3: Authorized Domains (Entwickleridentität)

**Problem**: Your application doesn't use authorized domains.

**What are Authorized Domains?**
Authorized domains are the domains where your OAuth consent screen can be displayed. This is a security feature to prevent phishing.

### Fix: Configure Authorized Domains in Google Cloud Console

**Step 1: Add Authorized Domains**

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select your project
3. Navigate to **APIs & Services** → **OAuth consent screen**
4. Click **Edit App**
5. In the **Authorized domains** section, add your domains:
   - Production: `your-domain.com`
   - Development: `localhost` (for testing)

**Step 2: Add Authorized Redirect URIs**

1. Navigate to **APIs & Services** → **Credentials**
2. Select your OAuth 2.0 client ID
3. In **Authorized redirect URIs**, add all your callback URLs:

**Production URIs**:
```
https://your-domain.com/api/v1/auth/youtube/callback
https://your-domain.com/api/v1/auth/viewer/youtube/callback
```

**Development URIs** (keep for testing):
```
http://localhost:8080/api/v1/auth/youtube/callback
http://localhost:8080/api/v1/auth/viewer/youtube/callback
http://localhost:3000/api/v1/auth/youtube/callback
http://localhost:3000/api/v1/auth/viewer/youtube/callback
```

**Step 3: Add Authorized JavaScript Origins**

If your frontend makes direct OAuth requests, add:

```
https://your-domain.com
http://localhost:3000
```

**Step 4: Domain Verification**

Google may require you to verify domain ownership:

1. Go to [Google Search Console](https://search.google.com/search-console)
2. Add your domain
3. Verify ownership using one of:
   - DNS record (recommended for production)
   - HTML file upload
   - HTML meta tag
   - Google Analytics
   - Google Tag Manager

**Step 5: Update Environment Variables**

Make sure your production environment uses the correct domain:

```bash
# Production .env
FRONTEND_URL=https://your-domain.com
CORS_ALLOWED_ORIGINS=https://your-domain.com
WEBSOCKET_ALLOWED_ORIGINS=https://your-domain.com
```

---

## Complete Checklist for Google OAuth Verification

### ✅ Before Submitting for Verification

- [ ] **Issue 1: Incremental Authorization**
  - [ ] Remove `oauth2.ApprovalForce` from code
  - [ ] Implement conditional consent prompt
  - [ ] Test that returning users don't see consent screen unnecessarily
  - [ ] Store and check granted scopes

- [ ] **Issue 2: Cross-Account Protection**
  - [ ] Implement RISC event receiver endpoint
  - [ ] Add RISC configuration endpoint
  - [ ] Store Google subject IDs for users
  - [ ] Test event handling (Google provides test events)
  - [ ] Configure RISC URLs in Google Cloud Console
  - [ ] Implement event handlers (account disabled, credential change, sessions revoked)

- [ ] **Issue 3: Authorized Domains**
  - [ ] Add all production domains to OAuth consent screen
  - [ ] Add all redirect URIs (production and dev)
  - [ ] Add JavaScript origins if needed
  - [ ] Verify domain ownership in Google Search Console
  - [ ] Update environment variables for production

- [ ] **Additional Requirements**
  - [ ] Add privacy policy URL to OAuth consent screen
  - [ ] Add terms of service URL to OAuth consent screen
  - [ ] Add app logo (512x512px)
  - [ ] Complete OAuth consent screen with accurate descriptions
  - [ ] Review and minimize requested scopes
  - [ ] Test OAuth flow in production environment
  - [ ] Prepare demo video showing OAuth flow (Google may request)

### 📝 OAuth Consent Screen Best Practices

**Application name**: Clear, matches your branding
**User support email**: Valid email you monitor
**Developer contact information**: Valid email
**Scopes**: Only request what you absolutely need
**Authorized domains**: All domains where users interact with OAuth

**Required scopes explanation**:
```
youtube.readonly - Required to detect live streams and fetch live chat IDs
youtube.force-ssl - Required to read live chat messages in real-time
userinfo.profile - Required to display user's name and profile picture
```

---

## Testing Your Changes

### Test 1: Incremental Authorization
```bash
# First login should show consent screen
curl -L http://localhost:8080/api/v1/auth/youtube/login

# Subsequent logins should NOT show consent (unless forceConsent=true)
# User should see account selector instead
```

### Test 2: RISC Events
```bash
# Google provides a test endpoint
# Use Google's RISC test tool to send test events
# Verify your endpoint responds with 202 Accepted
# Check logs to confirm event processing
```

### Test 3: Authorized Domains
```bash
# Test OAuth flow from your production domain
# Verify no domain mismatch errors
# Test all redirect URIs work correctly
```

---

## Common Pitfalls

1. **Redirect URI Mismatch**: Ensure exact match (including trailing slash, http vs https)
2. **localhost vs 127.0.0.1**: These are different domains to Google
3. **Port numbers**: Must match exactly (`:8080` vs `:3000`)
4. **HTTPS in production**: Google requires HTTPS for production redirect URIs
5. **Domain verification**: Can take 24-48 hours for propagation

---

## Support Resources

- [Google OAuth 2.0 Documentation](https://developers.google.com/identity/protocols/oauth2)
- [Incremental Authorization](https://developers.google.com/identity/protocols/oauth2/web-server#incrementalAuth)
- [Cross-Account Protection (RISC)](https://developers.google.com/identity/protocols/risc)
- [OAuth Verification Process](https://support.google.com/cloud/answer/9110914)
- [OAuth Consent Screen Setup](https://support.google.com/cloud/answer/6158849)

---

## Next Steps

1. Implement the fixes for all three issues
2. Test thoroughly in development
3. Deploy to production with correct domain configuration
4. Submit OAuth app for verification in Google Cloud Console
5. Respond to any Google reviewer feedback promptly
6. Verification typically takes 1-2 weeks (can be up to 6 weeks)
