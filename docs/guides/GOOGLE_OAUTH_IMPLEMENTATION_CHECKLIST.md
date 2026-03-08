# Google OAuth Verification - Implementation Checklist

## ✅ Completed Code Changes

### Issue #1: Incremental Authorization - FIXED ✅
- **File**: `services/auth-service/oauth/youtube.go:51-58`
- **Change**: Removed `oauth2.ApprovalForce`, now uses `prompt=select_account`
- **Result**: Users won't be forced to re-consent on every login

- **File**: `services/auth-service/oauth/viewer_youtube.go:42-49`
- **Change**: Same fix for viewer OAuth flow
- **Result**: Viewer authentication also supports incremental authorization

### Issue #2: Cross-Account Protection (RISC) - IMPLEMENTED ✅
- **New File**: `services/auth-service/handlers/risc_handler.go`
  - Handles security events from Google
  - Processes account-disabled, credential-change, session-revoked events
  - Automatically revokes tokens when Google reports security issues

- **Updated**: `services/auth-service/cmd/main.go:199-204`
  - Added RISC handler initialization
  - Added 3 new endpoints:
    - `POST /.well-known/risc-events` - Receives security events
    - `GET /.well-known/risc-configuration` - Configuration endpoint
    - `GET /.well-known/jwks.json` - Public key endpoint

- **Database**: Uses existing `google_id` column (stores Google subject ID)

### Build Status
```bash
cd services/auth-service && go build
```
✅ Build successful - no compilation errors

---

## 🔧 Required Google Cloud Console Configuration

### STEP 1: Configure OAuth Consent Screen
**Location**: [Google Cloud Console](https://console.cloud.google.com/) → APIs & Services → OAuth consent screen

1. Click **"Edit App"**
2. Scroll to **"Authorized domains"**
3. Add your production domain:
   ```
   your-domain.com
   ```
   (Replace with your actual domain - do NOT include http:// or https://)

4. Add development domains (optional):
   ```
   localhost
   ```

5. Add **Privacy Policy URL** (required for verification):
   ```
   https://your-domain.com/privacy
   ```

6. Add **Terms of Service URL** (required for verification):
   ```
   https://your-domain.com/terms
   ```

7. Upload **Application Logo** (recommended):
   - 512x512 pixels
   - PNG or JPG format

8. Click **"Save and Continue"**

### STEP 2: Configure Authorized Redirect URIs
**Location**: Google Cloud Console → APIs & Services → Credentials → Your OAuth 2.0 Client ID

Click **"Edit"** on your OAuth client, then add these URIs:

**Production URIs** (CRITICAL - must match exactly):
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

**Important Notes**:
- URIs must match EXACTLY (including trailing slash, protocol, port)
- `http://` only allowed for localhost
- Production must use `https://`

### STEP 3: Configure Cross-Account Protection (RISC)
**Location**: Google Cloud Console → APIs & Services → Credentials → Your OAuth 2.0 Client ID

1. Scroll to **"Cross-Account Protection"** section
2. Enable Cross-Account Protection
3. Set **Security Event Receiver URL**:
   ```
   https://your-domain.com/.well-known/risc-events
   ```

4. Set **Configuration URL**:
   ```
   https://your-domain.com/.well-known/risc-configuration
   ```

5. Click **"Save"**

**Testing RISC**:
- Google provides a testing tool at: https://risc-test.google.com/
- You can send test events to verify your implementation

### STEP 4: Verify Domain Ownership (Required)
**Location**: [Google Search Console](https://search.google.com/search-console)

1. Click **"Add Property"**
2. Enter your domain: `https://your-domain.com`
3. Verify ownership using one of these methods:

**Option A: DNS Record (Recommended for production)**
```
TXT record: google-site-verification=xxxxxxxxxxxxxxxxxxxx
```

**Option B: HTML File Upload**
- Download verification file
- Upload to your website root
- Access: `https://your-domain.com/google[hash].html`

**Option C: HTML Meta Tag**
- Add meta tag to your homepage `<head>` section

4. Click **"Verify"**

---

## 🚀 Deployment Steps

### 1. Deploy Updated Code

**Option A: Docker Compose (Development)**
```bash
# Rebuild and restart auth-service
docker-compose build auth-service
docker-compose up -d auth-service

# Check logs
docker-compose logs -f auth-service
```

**Option B: Kubernetes (Production)**
```bash
# Build new image
docker build -t your-registry/auth-service:latest services/auth-service/

# Push to registry
docker push your-registry/auth-service:latest

# Update deployment
kubectl rollout restart deployment/auth-service -n allchat

# Check status
kubectl get pods -n allchat -l app=auth-service
kubectl logs -f -n allchat -l app=auth-service
```

### 2. Update Environment Variables

Make sure your production environment has:
```bash
FRONTEND_URL=https://your-domain.com
YOUTUBE_CLIENT_ID=your_client_id.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-your_secret
```

### 3. Verify RISC Endpoints

Test that the endpoints are accessible:

```bash
# Test configuration endpoint
curl https://your-domain.com/.well-known/risc-configuration

# Expected response:
{
  "issuer": "https://your-domain.com",
  "jwks_uri": "https://your-domain.com/.well-known/jwks.json",
  "delivery": {
    "delivery_methods_supported": ["push"]
  },
  "critical_subject_claims_supported": ["sub"]
}

# Test JWKS endpoint
curl https://your-domain.com/.well-known/jwks.json

# Expected response:
{
  "keys": []
}
```

### 4. Test OAuth Flow

**Test incremental authorization**:
1. Go to your app
2. Click "Login with YouTube"
3. First time: Should see consent screen (normal)
4. Logout and login again
5. Second time: Should see account selector, NOT consent screen ✅

---

## 📋 Submit for Google OAuth Verification

### Prerequisites Checklist

Before submitting, ensure:

- [ ] All three issues are resolved (incremental auth, RISC, authorized domains)
- [ ] OAuth consent screen is complete with privacy policy and terms
- [ ] All redirect URIs are configured correctly
- [ ] Authorized domains are added and verified
- [ ] RISC endpoints are deployed and accessible
- [ ] Domain ownership is verified in Google Search Console
- [ ] Production environment is using HTTPS
- [ ] Application logo is uploaded (512x512px)

### Submission Process

1. Go to: [Google Cloud Console](https://console.cloud.google.com/)
2. Navigate to: **APIs & Services** → **OAuth consent screen**
3. Click: **"Publish App"** or **"Submit for Verification"**
4. Fill out the verification form:
   - **App name**: Your app name
   - **App description**: Clear description of what your app does
   - **Privacy policy URL**: `https://your-domain.com/privacy`
   - **Terms of service URL**: `https://your-domain.com/terms`
   - **Authorized domains**: Your verified domain
   - **App logo**: 512x512px image

5. **Scope Justification**: Explain why you need each scope
   ```
   youtube.readonly - Required to detect when streamers go live and fetch live chat IDs
   youtube.force-ssl - Required to read live chat messages in real-time
   userinfo.profile - Required to display user's name and profile picture
   ```

6. **Demo Video** (may be required):
   - Record a screen capture showing:
     - User clicks "Login with YouTube"
     - OAuth consent screen appears
     - User grants permissions
     - App receives data and displays it
     - Show where scopes are used in the app

7. Click **"Submit"**

### Expected Timeline
- **Initial Review**: 1-2 weeks
- **Follow-up Questions**: Respond within 7 days
- **Total Time**: 2-6 weeks on average

### Common Verification Issues

**Issue**: "Redirect URI mismatch"
- **Fix**: Ensure URIs in code match Google Cloud Console exactly

**Issue**: "Domain not verified"
- **Fix**: Complete domain verification in Google Search Console

**Issue**: "Privacy policy not accessible"
- **Fix**: Ensure privacy policy page is publicly accessible (no login required)

**Issue**: "Scope justification unclear"
- **Fix**: Provide specific examples of how each scope is used

---

## 🧪 Testing Recommendations

### Test 1: Incremental Authorization
```bash
# Expected: Users should only see consent screen once
1. Login with YouTube (first time) → Consent screen ✅
2. Logout
3. Login with YouTube (second time) → Account selector only ✅
```

### Test 2: RISC Event Handling
```bash
# Use Google's RISC test tool
1. Go to: https://risc-test.google.com/
2. Enter your endpoint: https://your-domain.com/.well-known/risc-events
3. Send test event
4. Check logs for event processing
5. Verify tokens are revoked in database
```

### Test 3: Domain Configuration
```bash
# All redirect URIs should work
curl -I https://your-domain.com/api/v1/auth/youtube/callback
# Should return 200 or 302, not 404
```

---

## 🆘 Troubleshooting

### "redirect_uri_mismatch" Error
**Cause**: Redirect URI doesn't match Google Cloud Console configuration

**Fix**:
1. Check the full URI in your browser's address bar
2. Copy it exactly (including protocol, port, path)
3. Add it to Google Cloud Console → Credentials → Authorized redirect URIs

### RISC Events Not Received
**Cause**: Endpoint not accessible or returning errors

**Check**:
```bash
# Test endpoint is reachable
curl -X POST https://your-domain.com/.well-known/risc-events \
  -H "Content-Type: application/json" \
  -d '{"test": "event"}'

# Should return 202 Accepted
```

**Fix**:
- Ensure auth-service is running
- Check firewall allows HTTPS traffic
- Verify no reverse proxy issues
- Check logs: `kubectl logs -f -l app=auth-service`

### Domain Verification Fails
**Cause**: DNS record not propagated or incorrect

**Fix**:
```bash
# Check DNS record
dig TXT your-domain.com

# Wait 24-48 hours for DNS propagation
# Try verification again
```

---

## 📝 Summary

**Changes Made**:
1. ✅ Removed `ApprovalForce` from YouTube OAuth (both streamer and viewer flows)
2. ✅ Implemented RISC security event handler
3. ✅ Added 3 RISC endpoints to auth-service
4. ✅ Code compiles successfully
5. ✅ Ready for deployment

**Next Steps**:
1. 🔧 Configure Google Cloud Console (domains, redirect URIs, RISC)
2. 🔐 Verify domain ownership
3. 🚀 Deploy updated code to production
4. 🧪 Test all OAuth flows
5. 📤 Submit for Google OAuth verification

**Estimated Time to Complete**:
- Google Cloud Console configuration: 30 minutes
- Domain verification: 1-24 hours (DNS propagation)
- Deployment: 15 minutes
- Testing: 30 minutes
- **Total**: 2-3 hours (excluding Google review time)
