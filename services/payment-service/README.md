# payment-service

Grants all-chat **premium** (`users.is_premium`) from **Patreon** subscriptions to
all-chat's own campaign (ADR-0018).

A user already logged into all-chat connects their Patreon account; if they are an
active patron of all-chat's campaign at or above a configurable tier, they receive
premium. Webhooks plus a reconcile job keep entitlement current and revoke it when a
subscription lapses.

## Role in the system: a second writer of `users.is_premium`

`users.is_premium` is a **derived column** with two independent inputs (ADR-0018):

```
is_premium = (users.premium_admin_override IS TRUE)
             OR (users.premium_admin_override IS NULL AND <active premium_subscriptions row>)
```

- The **admin** endpoint (`share-service`, `POST /api/v1/admin/premium/users/:id`)
  writes `users.premium_admin_override` (tri-state).
- **payment-service** writes `premium_subscriptions` (Patreon state).
- Both call `shared/premium.RecomputePremium(userID)`, which re-derives and writes
  `users.is_premium`. Recompute is a pure function of current rows, so admin comps
  survive subscription lapses and payment never clobbers an admin decision.

All existing readers (`shared/middleware/premium.go`, `moderation-service`) are
unchanged — they keep reading `users.is_premium`.

## Endpoints

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `GET`  | `/api/v1/payment/patreon/connect` | user JWT | Start OAuth; returns `{auth_url}` |
| `GET`  | `/api/v1/payment/patreon/callback` | one-time Redis state | OAuth callback; links account, grants premium |
| `GET`  | `/api/v1/payment/status` | user JWT | `{connected, status, tier_id, cents, renews_at, is_premium}` |
| `DELETE` | `/api/v1/payment/patreon/connection` | user JWT | Unlink Patreon; revoke subscription-derived premium |
| `POST` | `/api/v1/webhooks/patreon` | Patreon HMAC | `members:create/update/delete` |
| `GET`  | `/health/live`, `/health/ready`, `/metrics` | none | Ops |

## Webhook signature (the one deviation from the Twitch template)

Patreon signs webhooks with **HMAC-MD5 of the raw body**, hex-encoded, in the
`X-Patreon-Signature` header — NOT the HMAC-SHA256+timestamp scheme used by Twitch
EventSub. The raw body must be read before any JSON decode. See
`patreon/webhook.go` (`VerifyWebhookSignature`).

## Subscription status

`patreon.SubscriptionStatusFor` maps Patreon's `patron_status` +
`currently_entitled_amount_cents` to our status. A row grants premium iff its status
is `active`. Patreon keeps a patron `active_patron` with entitled cents during its own
payment-retry window, so honoring those fields respects Patreon's grace period — we
keep no separate grace timer.

| Patreon | status | premium |
|---------|--------|---------|
| `active_patron`, cents ≥ `PATREON_MIN_TIER_CENTS` | `active` | yes |
| `active_patron`, cents below threshold | `expired` | no |
| `declined_patron` | `declined` | no |
| `former_patron` | `former` | no |
| no membership to our campaign | `none` | no |

## Configuration

| Env | Required | Default | Notes |
|-----|----------|---------|-------|
| `PATREON_CLIENT_ID` / `PATREON_CLIENT_SECRET` | yes | – | OAuth client (secret) |
| `PATREON_CAMPAIGN_ID` | yes | – | all-chat's campaign id |
| `PATREON_REDIRECT_URL` | yes | – | `https://<host>/api/v1/payment/patreon/callback` |
| `PATREON_WEBHOOK_SECRET` | yes | – | webhook HMAC key (secret) |
| `PATREON_MIN_TIER_CENTS` | no | `500` | qualifying threshold |
| `PAYMENT_RECONCILE_INTERVAL` | no | `6h` | reconcile cadence |
| `PATREON_TOKEN_REFRESH_BUFFER` | no | `24h` | refresh tokens expiring within this window |
| `PAYMENT_RECONCILE_BATCH_SIZE` | no | `500` | connections re-queried per pass |
| `JWT_SECRET_V1` | yes | – | validates user JWTs |
| `TOKEN_ENCRYPTION_KEY_V1` | yes | – | encrypts stored Patreon tokens |
| `DATABASE_*`, `REDIS_*`, `FRONTEND_URL` | – | localhost | standard |

## Deployment

Single replica (the reconcile job must not run concurrently). Manifest lives in the
deploy repo (`caesar-deployment/apps/workloads/all-chat/payment-service-deployment.yaml`).
Routed through the API gateway under `/api/v1/payment` and `/api/v1/webhooks/patreon`.
