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

## Re-encryption Helper
Create a temporary Go program (or job) that links against the shared crypto package:

```go
package main

import (
        "context"
        "log"
        "os"

        "github.com/caesar/all-chat/shared/crypto"
        "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
        ctx := context.Background()
        pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
        if err != nil {
                log.Fatal(err)
        }
        defer pool.Close()

        cipher, err := crypto.NewAESGCMCipher(os.Getenv("TOKEN_ENCRYPTION_KEY"))
        if err != nil {
                log.Fatal(err)
        }

        rows, err := pool.Query(ctx, "SELECT id, access_token, refresh_token FROM users")
        if err != nil {
                log.Fatal(err)
        }
        defer rows.Close()

        for rows.Next() {
                var id, access, refresh string
                if err := rows.Scan(&id, &access, &refresh); err != nil {
                        log.Fatal(err)
                }

                encAccess, err := cipher.Encrypt(access)
                if err != nil {
                        log.Fatal(err)
                }
                encRefresh, err := cipher.Encrypt(refresh)
                if err != nil {
                        log.Fatal(err)
                }

                if _, err := pool.Exec(ctx, "UPDATE users SET access_token=$2, refresh_token=$3 WHERE id=$1", id, encAccess, encRefresh); err != nil {
                        log.Fatal(err)
                }
        }

        if rows.Err() != nil {
                log.Fatal(rows.Err())
        }

        // Repeat the same loop for youtube_oauth_tokens
}
```

The loop can be extended to update `youtube_oauth_tokens` using the same cipher. After the job runs successfully, redeploy the auth-service and confirm that logins and token refresh operations succeed.
