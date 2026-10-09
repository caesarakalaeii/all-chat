# tiktok-relay

Repeats one TikTok request from a US region: the live lookup by handle
(`/api-live/user/room/?uniqueId=…`). Accounts hosted by TikTok's US entity
answer that lookup with `user_not_found` from our EU cluster, while the room ID
it returns works from the EU (`check_alive`, the WebSocket). tiktok-listener
asks this relay only after TikTok answers `user_not_found`, and then connects
by room ID as usual. It is not deployed to the cluster; it runs on Google Cloud
Run in `us-east1`.

## API

`GET /v1/live?handle=<tiktok handle>` with header `X-Relay-Token: <token>`.

```json
{ "found": true, "live": true, "status": 2, "roomId": "7694394629390748430", "userId": "6864719847335576582" }
{ "found": false, "code": 19881007, "message": "user_not_found" }
```

The token is not sent as `Authorization: Bearer`: Cloud Run's frontend treats
any bearer token there as a Google ID token and rejects the request before it
reaches the service. Handles are lowercased and must match `[a-z0-9._]{2,24}`,
so the relay cannot be pointed at any other URL. 401 without a valid token, 400
for a malformed handle, 502 when TikTok does not answer.

## Deploy

From this directory, with the token in `RELAY_TOKEN`:

```bash
gcloud run deploy tiktok-relay \
  --project <gcp-project> --region us-east1 --source . \
  --allow-unauthenticated \
  --set-env-vars RELAY_TOKEN="$RELAY_TOKEN" \
  --memory 256Mi --cpu 1 --concurrency 20 \
  --min-instances 0 --max-instances 2 --timeout 15
```

`--allow-unauthenticated` is deliberate: access control is the token check in
`server.mjs`, not Cloud Run IAM. `--max-instances 2` caps scaling and cost. A
redeploy without `--set-env-vars` keeps the existing token. The same token goes
into the cluster as `TIKTOK_US_RELAY_TOKEN` on tiktok-listener.

## Test

```bash
npm test
```
