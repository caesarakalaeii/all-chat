# Google OAuth Verification

The client-side fixes Google required are in the code already:

- Incremental authorization: YouTube OAuth uses `prompt=select_account`,
  not `ApprovalForce` (`services/auth-service/oauth/youtube.go`).
- Cross-Account Protection (RISC): event and configuration endpoints are
  mounted (`/.well-known/risc-events`, `/.well-known/risc-configuration`,
  handler in `services/auth-service/handlers/risc_handler.go`).

What remains is configuration in Google Cloud Console, repeated per
environment:

1. **OAuth consent screen**: add the production and development domains
   under "Authorized domains"; verify domain ownership through Google
   Search Console.
2. **Credentials**: add the authorized redirect URIs for the YouTube
   streamer and viewer callback routes:
   - `https://<domain>/api/v1/auth/youtube/callback`
   - `https://<domain>/api/v1/auth/viewer/youtube/callback`
   plus the localhost equivalents (`:8080` for services, `:3000` for the
   frontend) for development.
3. **RISC receiver**: set the Security Event Receiver URL to
   `https://<domain>/.well-known/risc-events` in the Google Cloud Console
   (APIs & Services > Credentials, under the OAuth client's security
   settings).
4. **Production env**: `FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`,
   `WEBSOCKET_ALLOWED_ORIGINS` must name the production domain; the wildcard
   is rejected with cookie auth.

Verification review typically takes Google one to two weeks.
