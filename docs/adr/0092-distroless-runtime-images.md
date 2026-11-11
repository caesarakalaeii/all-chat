# ADR-0092: Distroless runtime images

**Date**: 2026-11-11
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Every Go service image ran on `alpine:3.24`, and the frontend and discord-bot ran on
`node:26-alpine` and `node:22-alpine`. In production each service needs only its binary
(or `node` and its bundle), CA certificates and tzdata. The alpine runtime stage also
shipped busybox, a shell and `apk`. None of that is used at runtime, but all of it is
attack surface, shows up in image scans, and lets an attacker who gets code execution in a
pod run arbitrary shell commands.

The only runtime uses of the shell were:

- Docker `HEALTHCHECK` instructions calling busybox `wget`. Kubernetes ignores them and runs
  its own `httpGet` probes.
- support-bot downloading `kubectl` with `wget` in its runtime stage.
- youtube-listener-innertube's `preStop` hook, `["/bin/sh", "-c", "sleep 5"]`, in
  caesar-deployment.
- Runbooks that `kubectl exec` into a service pod to run `wget`, `printenv` or `sh`.

## Decision Drivers

- Ship nothing but the service and its runtime: no shell, no package manager.
- Change no behaviour. Ports, env defaults, working directory and UIDs stay the same, so no
  manifest needs a new `securityContext`.
- Keep picking up base-image security fixes on every build, as the floating `alpine:3.24`
  tag did.

## Considered Options

1. **Keep alpine** and remove packages. busybox and `apk` cannot be removed from alpine
   itself.
2. **`scratch`** for the Go services. It has no CA bundle, tzdata or `/etc/passwd`, so every
   image would have to copy them in from the builder, and the Node images would still need a
   base.
3. **Google distroless** (`gcr.io/distroless/static-debian13` for Go,
   `gcr.io/distroless/nodejs<N>-debian13` for Node). Ships CA certificates, tzdata and
   `SSL_CERT_FILE`, no shell, and Node variants with `node` as the entrypoint.

## Decision Outcome

Chosen option: **3, distroless**, on Debian 13 (trixie).

- **Go services** (every `services/*` Go image): the runtime stage is
  `FROM gcr.io/distroless/static-debian13`. Every build is already `CGO_ENABLED=0`, so the
  binaries are static. The build stage stays `golang:<version>-alpine`, because nothing from
  it ships except the binary.
- **support-bot** downloads `kubectl` in the build stage and copies it to
  `/usr/local/bin/kubectl`. Its tools call `exec.Command("kubectl", ...)` directly, with no
  shell, and distroless puts `/usr/local/bin` on `PATH`.
- **frontend**: the runtime stage is `gcr.io/distroless/nodejs26-debian13` with
  `CMD ["server.js"]`. Its build stages move from `node:26-alpine` to `node:26-trixie-slim`.
  The Next.js standalone output ships native packages (sharp) installed for the build
  stage's libc, and a musl build would not load on the glibc runtime.
- **discord-bot** becomes two stages: `npm ci --omit=dev` on `node:22-trixie-slim`, then
  `gcr.io/distroless/nodejs22-debian13` running `CMD ["src/index.js"]`. The runtime has no
  npm, so `npm start` is replaced with what that script runs.
- **Users**: the images set numeric `USER 1000:1000`, or `1001:1001` for the frontend, the
  UIDs the manifests already pin with `runAsUser`. Distroless has no passwd entry for them,
  and the user has to be numeric anyway for `runAsNonRoot` to verify it.
- **Health checks**: the Docker `HEALTHCHECK` instructions are removed. Kubernetes never used
  them, and no compose file waits on a service's health status.
- **Tags**: `static-debian13` and `nodejs<N>-debian13` float within Debian 13, as
  `alpine:3.24` floated within 3.24. Digests are not pinned: distroless republishes often, and
  digest pins would mean a Dependabot PR per image every few days in exchange for
  reproducibility the deploy flow does not use (Keel deploys what `main` builds).

### Not converted

- **migrations** (`allchat-migrations`) stays on alpine. It is a shell script around `psql`
  and `pg_isready`, run as an init container. Moving it means rewriting the runner in Go, a
  separate change.
- **tiktok-listener** and **tiktok-signer** were never alpine; they run on
  `node:*-bookworm-slim`. tiktok-signer needs Chromium, which distroless cannot provide.
- **test/contract/dual-listener** is a test harness that is never deployed.

### Consequences

- Good: no shell, busybox or package manager in any converted image. Image scans report only
  the service's own dependencies and the Debian base packages distroless ships.
- Good: the frontend and discord-bot run on glibc, the same libc as their builds.
- Bad: `kubectl exec <pod> -- sh`, `wget`, `printenv` and similar no longer work. Use
  `kubectl port-forward` plus a local `curl` for HTTP endpoints, the deployment spec for env
  var names, and `kubectl debug <pod> -it --image=busybox:1.37 --target=<container>
  --profile=restricted` for an interactive shell next to the process. The runbooks show
  these forms.
- Bad: a manifest that runs a shell command against these images breaks. The only one,
  youtube-listener-innertube's `preStop`, uses the native `sleep` action (caesar-deployment
  #179), which must be deployed before the images.

## Implementation

- `services/*/Dockerfile`, `frontend/Dockerfile`, `services/discord-bot/Dockerfile`.
- caesar-deployment: `lifecycle.preStop.sleep` for youtube-listener-innertube.
- Runbooks and guides: every `kubectl exec` into a service pod uses one of the forms above.

## Related Decisions

- [ADR-0005](./0005-react-nextjs-frontend.md): the Next.js standalone frontend this image
  runs.
- [ADR-0052](./0052-retiring-euler-stream-for-tiktok-signing.md): why tiktok-signer runs
  Chromium on Debian.
