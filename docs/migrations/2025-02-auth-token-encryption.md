# Auth Token Encryption Migration

The auth service now encrypts OAuth access and refresh tokens at rest. The change introduces the `TOKEN_ENCRYPTION_KEY` environment variable and requires all historical tokens stored in `users.access_token`, `users.refresh_token`, and `youtube_oauth_tokens.(access|refresh)_token` to be re-encrypted before the service is upgraded.

## Prerequisites
1. Generate a 16, 24, or 32-byte encryption key (AES-128/192/256) and store it in the secret store as `TOKEN_ENCRYPTION_KEY`.
2. Ensure `JWT_SECRET` is also present in the secret store—startup now fails fast if either secret is missing.
3. Back up the `users` and `youtube_oauth_tokens` tables so the plaintext values can be restored if a rollback is required.

## Deployment Flow
1. **Prepare secrets** – roll out the new `TOKEN_ENCRYPTION_KEY` secret to the environment but keep the previous build running.
2. **Pause auth-service** – scale the deployment to zero replicas (or disable ingress) so no new tokens are created while the migration runs.
3. **Re-encrypt historical tokens** – run the helper script below once per environment. It reads plaintext tokens, encrypts them with the shared `crypto` package, and writes the ciphertext back.
4. **Deploy the new auth-service image** – once every row has ciphertext, redeploy. The service will decrypt tokens transparently on reads and encrypt on writes.

## Backfill Tool
The repository now includes a purpose-built helper at `services/auth-service/cmd/token-encryption-backfill`. The command scans both the `users` and `youtube_oauth_tokens` tables, encrypts any plaintext `access_token` / `refresh_token` values with the shared AES-GCM helper, and writes the ciphertext back in place.

Usage:

```bash
# Dry-run to confirm how many rows would change
DATABASE_URL=postgres://... TOKEN_ENCRYPTION_KEY=... \
  go run ./services/auth-service/cmd/token-encryption-backfill --dry-run

# Perform the actual update
DATABASE_URL=postgres://... TOKEN_ENCRYPTION_KEY=... \
  go run ./services/auth-service/cmd/token-encryption-backfill
```

Flags:

| Flag | Default | Description |
| --- | --- | --- |
| `--dry-run` | `false` | Log every row that would be updated without writing to the database. |
| `--skip-users` | `false` | Ignore the `users` table (useful for partial rollouts or reruns). |
| `--skip-youtube` | `false` | Ignore the `youtube_oauth_tokens` table. |

The tool is idempotent: it attempts to decrypt each token before re-encrypting it. If the value already decrypts with the provided key it is left untouched, so the command can be rerun safely during staged rollouts. When the command exits successfully, redeploy the auth-service and confirm that logins and token refresh operations succeed. A detailed staging test checklist is available in `docs/migrations/2025-02-auth-token-encryption-staging-test-plan.md`.

## Rollback Plan
1. **Detect the issue** – if logins start failing or you detect corrupted tokens after the upgrade, immediately stop accepting new writes by scaling the auth-service deployment to zero replicas (or disabling ingress) to prevent additional encrypted tokens from being created.
2. **Restore the plaintext backup** – revert the `users` and `youtube_oauth_tokens` tables from the snapshot that was captured during the migration prerequisites. This restores the original plaintext tokens so the previous build can keep working.
3. **Redeploy the prior build** – roll back to the last known-good auth-service release that did not require encrypted tokens. Remove `TOKEN_ENCRYPTION_KEY` from the deployment (or point it to the legacy dummy value) so the downgraded service does not attempt to decrypt data.
4. **Validate** – confirm that logins and token refresh flows succeed again. Once stability is restored you can re-run the staging test plan, address the root cause, and attempt the migration again.
