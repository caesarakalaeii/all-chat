# Auth Token Encryption – Staging Test Plan

This runbook verifies the token-encryption rollout in a non-production environment before it reaches production. Execute every step sequentially so we can detect regressions early and prove that both the backfill tool and the updated auth-service behave correctly.

## 1. Pre-checklist
- [ ] Confirm the staging database has fresh snapshots of the `users` and `youtube_oauth_tokens` tables. Store them outside of the primary cluster so they are available if we need to roll back.
- [ ] Ensure the staging secrets manager contains both `JWT_SECRET` and the new `TOKEN_ENCRYPTION_KEY` entries. The key must be 16/24/32 bytes.
- [ ] Export `DATABASE_URL` pointing at staging and `TOKEN_ENCRYPTION_KEY` locally so the CLI can connect without additional prompts.
- [ ] Scale the auth-service deployment down to zero replicas (or disable ingress) to prevent new tokens from being written while the backfill runs.

## 2. Dry-run the backfill
1. From the repo root, run:
   ```bash
   DATABASE_URL=$STAGING_DATABASE_URL TOKEN_ENCRYPTION_KEY=$STAGING_TOKEN_KEY \
     go run ./services/auth-service/cmd/token-encryption-backfill --dry-run
   ```
2. Verify the output reports the number of `users` / `youtube_oauth_tokens` rows that would be updated.
3. Inspect a few sample rows manually to confirm they still contain plaintext tokens (`SELECT id, access_token FROM users LIMIT 5;`).

## 3. Execute the backfill
1. Re-run the tool without `--dry-run` and capture the logs. Example:
   ```bash
   DATABASE_URL=$STAGING_DATABASE_URL TOKEN_ENCRYPTION_KEY=$STAGING_TOKEN_KEY \
     go run ./services/auth-service/cmd/token-encryption-backfill
   ```
2. Wait for the summary lines (`processed=… updated=…`) and confirm `rows.Err()` did not report failures.
3. Spot-check the database:
   - `SELECT access_token FROM users LIMIT 5;` – tokens should now look like base64 strings that begin with random data.
   - `SELECT user_id, access_token FROM youtube_oauth_tokens LIMIT 5;` – values should also be base64.

## 4. Deploy the new auth-service build
1. Update the staging manifests/Helm release/Compose stack so the auth-service pods receive the `TOKEN_ENCRYPTION_KEY` env var.
2. Deploy the latest container image that contains the encryption-aware repository changes.
3. Confirm the pods start successfully and no startup log contains `JWT_SECRET`/`TOKEN_ENCRYPTION_KEY must be set` errors.

## 5. Functional verification
- [ ] Perform a Twitch login via the staging frontend and verify the auth callback succeeds.
- [ ] Trigger a token refresh (e.g., revoke the Twitch token so the service requests a new one) and ensure it is persisted.
- [ ] Link a YouTube account so `youtube_oauth_tokens` receives an entry; confirm the listener can still poll data.
- [ ] Run `SELECT COUNT(*) FROM users WHERE access_token = ''` to ensure no rows lost their tokens.
- [ ] Tail the auth-service logs for AES/GCM errors or database failures.

## 6. Observability checks
- [ ] Confirm no new alerts fired in staging (authentication latency, error-rate, or listener failures).
- [ ] Ensure dashboards that read from `users` / `youtube_oauth_tokens` still show data (counts remain stable).

## 7. Rollback rehearsal (optional but recommended)
- [ ] Practice restoring the staging snapshots into a scratch schema to validate the backup files.
- [ ] Re-run the backfill tool after the restore to ensure it behaves as expected if we must retry in production.

## 8. Exit criteria
Only promote the change when:
1. The backfill tool updated 100% of rows without errors.
2. Manual spot-checks confirm the ciphertext decrypts at runtime (e.g., logins/refresh flows succeed).
3. Observability dashboards remain healthy for at least one hour after redeploying the auth-service.
4. The rollback rehearsal succeeded so we know we can recover the plaintext values if needed.
