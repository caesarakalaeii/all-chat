# Google OAuth Verification - Quick TODO List

## ✅ COMPLETED (Code Changes)

All code changes are done and tested:
- ✅ Fixed incremental authorization (removed `ApprovalForce`)
- ✅ Implemented RISC security event handler
- ✅ Added 3 RISC endpoints to auth-service
- ✅ Code builds successfully

## 🔧 TODO: Google Cloud Console (30 minutes)

### 1. OAuth Consent Screen → Authorized Domains
**URL**: https://console.cloud.google.com/ → APIs & Services → OAuth consent screen → Edit App

- [ ] Add authorized domain: `your-domain.com` (replace with your actual domain)
- [ ] Add privacy policy URL
- [ ] Add terms of service URL
- [ ] Upload 512x512px app logo (optional but recommended)

### 2. Credentials → Redirect URIs
**URL**: https://console.cloud.google.com/ → APIs & Services → Credentials → Edit OAuth Client

- [ ] Add production redirect URIs:
  - `https://your-domain.com/api/v1/auth/youtube/callback`
  - `https://your-domain.com/api/v1/auth/viewer/youtube/callback`

### 3. Credentials → Cross-Account Protection (RISC)
**URL**: Same page as step 2, scroll down to "Cross-Account Protection"

- [ ] Enable Cross-Account Protection
- [ ] Set Security Event Receiver URL: `https://your-domain.com/.well-known/risc-events`
- [ ] Set Configuration URL: `https://your-domain.com/.well-known/risc-configuration`

### 4. Verify Domain Ownership
**URL**: https://search.google.com/search-console

- [ ] Add property: `your-domain.com`
- [ ] Verify using DNS TXT record (recommended) or HTML file

## 🚀 TODO: Deploy & Test (1-2 hours)

### 5. Deploy Updated Code

```bash
# Rebuild and deploy auth-service
docker-compose build auth-service
docker-compose up -d auth-service

# Or for Kubernetes:
kubectl rollout restart deployment/auth-service -n allchat
```

### 6. Test RISC Endpoints

```bash
curl https://your-domain.com/.well-known/risc-configuration
# Should return JSON with issuer, jwks_uri, etc.

curl https://your-domain.com/.well-known/jwks.json
# Should return {"keys": []}
```

### 7. Test OAuth Flow

- [ ] Login with YouTube (first time) → Should see consent screen
- [ ] Logout and login again → Should see account selector only (NO consent screen)

### 8. Test RISC Events (Optional)

- [ ] Go to https://risc-test.google.com/
- [ ] Send test event to your endpoint
- [ ] Check logs to confirm event was received and processed

## 📤 TODO: Submit for Verification (15 minutes)

**URL**: https://console.cloud.google.com/ → APIs & Services → OAuth consent screen → Publish App

### 9. Fill Out Verification Form

- [ ] Complete app description
- [ ] Explain scope usage:
  - `youtube.readonly` - Detect live streams and fetch live chat IDs
  - `youtube.force-ssl` - Read live chat messages in real-time
  - `userinfo.profile` - Display user's name and profile picture
- [ ] Submit for review

### 10. Wait for Google Review

- **Timeline**: 1-6 weeks
- **Action**: Respond promptly to any Google reviewer questions
- **Status**: Check email and Google Cloud Console for updates

---

## 📚 Full Documentation

- **Implementation Checklist**: `docs/guides/GOOGLE_OAUTH_IMPLEMENTATION_CHECKLIST.md`
- **Detailed Guide**: `docs/guides/GOOGLE_OAUTH_VERIFICATION.md`

---

## ⚠️ Important Notes

1. **Production domain required**: Replace `your-domain.com` with your actual domain
2. **HTTPS required**: All production redirect URIs must use HTTPS
3. **Exact URI match**: Redirect URIs must match exactly (including port, path, trailing slash)
4. **DNS propagation**: Domain verification may take 24-48 hours

---

## 🆘 Need Help?

If you encounter issues:
1. Check logs: `docker-compose logs -f auth-service` or `kubectl logs -f -l app=auth-service`
2. Review troubleshooting section in `GOOGLE_OAUTH_IMPLEMENTATION_CHECKLIST.md`
3. Test each endpoint individually with curl
4. Verify all environment variables are set correctly

---

**Status**: Ready to configure Google Cloud Console and deploy! 🚀
