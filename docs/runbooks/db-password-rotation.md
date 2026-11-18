# DB password rotation

Rotate the PostgreSQL `allchat_user` password (the application-level user all Go services use to connect to CNPG cluster `allchat-cluster`). For token/JWT keys see `docs/runbooks/secret-rotation.md`.

Rotate every 90 days, or immediately after a staff change or credential exposure.

## Why manual (not CNPG ManagedRoles)

CNPG issues a single `ALTER ROLE ... PASSWORD ...` atomically with no grace window, and it does not restart application pods, which read `DATABASE_PASSWORD` via secretKeyRef at pod start. So the rotation is owned here: `ALTER ROLE` -> `kubectl patch secret` -> wait for propagation -> rolling restart -> verify. The CNPG Cluster spec in `caesar-deployment/apps/workloads/all-chat/allchat-cluster.yaml` deliberately has no ManagedRoles section.

## Hazards

1. **Never `sops set`/`sops edit`/`sops updatekeys`.** The live K8s Secret `allchat-secrets` has more keys than the SOPS source in the deployment repo; an sops write overwrites it with the stale snapshot and deletes live keys. Use `kubectl patch` for every secret edit.
2. **Never inspect secrets with `-o yaml`/`-o json`.** Putting output in `last-applied-configuration` leaks values via `describe`. Inspect keys only:
   ```bash
   kubectl --context default get secret allchat-secrets -n allchat \
     -o jsonpath='{.data}' | jq 'keys'
   ```
3. **Never rotate `allchat-cluster-secret`.** That is the CNPG bootstrap superuser secret, not the application user.
4. **Capture the OLD password before Step 2** (password manager). If `ALTER ROLE` succeeds and a later step fails, rollback needs it.

## Pre-flight

```bash
kubectl config current-context   # allchat cluster context

PRIMARY=$(kubectl --context default get pods -n allchat \
  -l cnpg.io/cluster=allchat-cluster,cnpg.io/instanceRole=primary \
  -o jsonpath='{.items[0].metadata.name}')
echo "$PRIMARY"

kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat -c "\du allchat_user"     # role exists

kubectl --context default get pods -n allchat -l '!job-name'   # all Running before starting
```

## Rotation

### 1. Generate the new password

```bash
NEW_PASSWORD=$(head -c 32 /dev/urandom | base64 | tr -d '/=+')
echo "length: ${#NEW_PASSWORD}"    # 43
# Save it in the password manager NOW, before ALTER ROLE.
```

### 2. Alter the role

```bash
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "ALTER ROLE allchat_user PASSWORD '$NEW_PASSWORD';"
# Expected: ALTER ROLE
```

Established connections survive; only new connection attempts fail until step 4. A dual-user swap (create `allchat_user_v2`, grant privileges, swap, rename) exists for a mandatory zero-drop window; it also requires updating `DATABASE_USER` and a `DATABASE_USER` drop in every service. Use it only if the brief window is unacceptable.

### 3. Patch the secret (kubectl patch only)

```bash
ENCODED_PASSWORD=$(printf '%s' "$NEW_PASSWORD" | base64)
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"replace\", \"path\": \"/data/database-password\", \"value\": \"${ENCODED_PASSWORD}\"}]"

kubectl --context default get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | jq 'keys' | grep database-password
```

### 4. Wait 60 seconds, then roll the consumers

The wait lets kubelets refresh the secret so restarted pods read the new value.

```bash
sleep 60
DB_SERVICES=(auth-service token-refresh-service twitch-eventsub-listener
  api-gateway overlay-manager share-service message-processor
  source-manager kick-listener tiktok-listener twitch-listener)
for d in "${DB_SERVICES[@]}"; do
  kubectl --context default rollout restart deployment/$d -n allchat
done
for d in "${DB_SERVICES[@]}"; do
  kubectl --context default rollout status deployment/$d -n allchat --timeout=5m
done
```

Before adding a service to the list, confirm it consumes the DB (`kubectl --context default get deployment <name> -n allchat -o jsonpath='{.spec.template.spec.containers[0].env[*].name}'` and look for `DATABASE_*`).

### 5. Verify

```bash
kubectl --context default port-forward -n allchat deploy/auth-service 8081:8081 >/dev/null &
sleep 1; curl -s http://localhost:8081/health/ready; kill $!      # HTTP 200

for d in auth-service overlay-manager token-refresh-service; do
  kubectl --context default logs -n allchat deploy/$d --tail=50 \
    | grep -iE "password authentication failed|allchat_user|pq:.*auth" \
    || echo "$d: no auth failures"
done
```

## Rollback

If pods log `pq: password authentication failed for user "allchat_user"` or CrashLoop from DB auth:

```bash
OLD_PASSWORD="<from password manager>"
kubectl --context default exec -n allchat "$PRIMARY" -- \
  psql -U postgres allchat \
  -c "ALTER ROLE allchat_user PASSWORD '$OLD_PASSWORD';"

OLD_ENCODED=$(printf '%s' "$OLD_PASSWORD" | base64)
kubectl --context default patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"replace\", \"path\": \"/data/database-password\", \"value\": \"${OLD_ENCODED}\"}]"
```

Then re-run the restart loop from step 4 and re-verify.

## Known failure modes

| Symptom | Cause | Mitigation |
|---|---|---|
| Fresh pod auth-fails right after ALTER ROLE | Pod started between ALTER ROLE and the secret patch | Patch the secret immediately after ALTER ROLE; the 60s wait comes after the patch |
| Pod still fails after restart | Stale kubelet secret cache | Restart again; cache has refreshed by then |
| Brief `connection refused` burst during rollout | Pool exhaustion as old pods drain | Transient; investigate if it lasts more than ~2 minutes |
| Restart stalls | PodDisruptionBudget blocks restarts | Check `kubectl --context default get pdb -n allchat` |

The new `database-password` now exists only in the live secret, not in the SOPS source; note it in the secrets drift register.
