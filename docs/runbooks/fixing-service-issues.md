# Fixing a service issue in production

Fix path: verify the fix locally, push, let CI/CD and Keel deploy, watch the rollout. Never apply an image change to the cluster directly.

## Diagnose

```bash
kubectl -n allchat get pods -l app=<service> -o wide
kubectl -n allchat describe pod <pod>       # events: probes, OOMKilled, scheduling
kubectl -n allchat logs <pod> --tail=100    # add --previous for a crashed pod
kubectl -n allchat get hpa                  # autoscaling state
kubectl -n allchat top pods -l app=<service>
```

Common causes: changed upstream API formats (unmarshal errors), lost DB/Redis or external API connectivity, missing config, resources (OOM/throttling), app bugs (panics, deadlocks).

## Fix

1. Change the code locally, build it first:

   ```bash
   cd services/<service> && go build -o /tmp/<service>-test ./cmd
   ```

2. Commit (no AI attribution, no gitmoji, Conventional Commits) and push the branch; open a PR and merge. CI builds the image at ghcr.io and Keel rolls it out.
3. Watch:

   ```bash
   kubectl -n allchat rollout status deployment/<service>
   kubectl -n allchat logs -f deployment/<service>
   ```

4. Verify health:

   ```bash
   kubectl -n allchat get pods -l app=<service>
   kubectl -n allchat port-forward -n allchat deployment/<service> 8080:8080 >/dev/null &
   sleep 1; curl -s http://localhost:8080/health/ready; kill $!
   ```

## Rollback

```bash
kubectl -n allchat rollout history deployment/<service>
kubectl -n allchat rollout undo deployment/<service>
# or to a revision:
kubectl -n allchat rollout undo deployment/<service> --to-revision=<N>
kubectl -n allchat rollout status deployment/<service>
```
