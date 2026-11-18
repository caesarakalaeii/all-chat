# ADR-0092: Distroless runtime images

**Date**: 2026-11-11
**Status**: Accepted
**Revised**: 2026-11-15
**Deciders**: caesarakalaeii

## Context

Every Go service image ran on `alpine:3.24`, and the frontend and discord-bot on
`node:26-alpine`/`node:22-alpine`. Runtime needs are only the binary (or `node` and its
bundle), CA certificates and tzdata; the alpine stage additionally shipped busybox, a
shell and `apk`, unused at runtime but attack surface for anyone with code execution in a
pod. The only shell uses were Docker `HEALTHCHECK`s (ignored by Kubernetes), support-bot
downloading `kubectl`, youtube-listener-innertube's `preStop` (`["/bin/sh", "-c", "sleep
5"]`), and runbooks `kubectl exec`-ing `wget`/`printenv`/`sh`.

Constraints: ship nothing but the service and its runtime (no shell, no package manager);
change no behaviour (ports, env defaults, working directory and UIDs stay the same, so no
manifest needs a new `securityContext`); keep picking up base-image security fixes on
every build as the floating `alpine:3.24` tag did.

## Considered options

- Keep alpine: busybox and `apk` cannot be removed from alpine itself.
- `scratch` for the Go services: no CA bundle, tzdata or `/etc/passwd` shipped; every image
  would copy them from the builder, and Node still needs a base.
- Google distroless on Debian 13 (trixie) (`gcr.io/distroless/static-debian13` for Go,
  `gcr.io/distroless/nodejs<N>-debian13` for Node): ships CA certificates, tzdata and
  `SSL_CERT_FILE`, no shell, Node variants with `node` as the entrypoint. Chosen.

## Decision

- **Go services** (every `services/*` image): runtime `FROM
  gcr.io/distroless/static-debian13`; builds are already `CGO_ENABLED=0` so the binaries
  are static, and the builder stays `golang:<version>-alpine` because nothing from it
  ships except the binary.
- **support-bot** copies `kubectl` (downloaded in the build stage) to `/usr/local/bin`;
  its tools `exec.Command("kubectl", ...)` directly, with no shell. Since ADR-0093 it also
  ships the pinned Claude Code binary, which links glibc, so support-bot alone runs
  `distroless/cc-debian13` instead of `static-debian13`.
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
  `alpine:3.24` did. Digests are left unpinned: distroless republishes often and Keel
  deploys what `main` builds anyway.

## Not converted

- **migrations** (`allchat-migrations`) stays on alpine. It is a shell script around `psql`
  and `pg_isready`, run as an init container. Moving it means rewriting the runner in Go, a
  separate change.
- **tiktok-listener** and **tiktok-signer** were never alpine; they run on
  `node:*-bookworm-slim`. tiktok-signer needs Chromium, which distroless cannot provide.
- **test/contract/dual-listener** is a test harness that is never deployed.

## Consequences

- No shell, busybox or package manager in any converted image; scans report only the
  service's dependencies and the Debian base packages. The frontend and discord-bot run on
  glibc, the same libc as their builds.
- `kubectl exec <pod> -- sh`/`wget`/`printenv` no longer work: use `kubectl port-forward`
  plus local `curl`, the deployment spec for env names, and `kubectl debug <pod> -it
  --image=busybox:1.37 --target=<container> --profile=restricted` for an interactive
  shell next to the process (the runbooks show these forms).
- A manifest running a shell command against these images breaks; the only one,
  youtube-listener-innertube's `preStop`, uses the native `sleep` action (caesar-
  deployment), which must deploy before the images.

## Implementation

`services/*/Dockerfile`, `frontend/Dockerfile`, `services/discord-bot/Dockerfile`;
caesar-deployment `lifecycle.preStop.sleep` for youtube-listener-innertube; every runbook
and guide's `kubectl exec` uses one of the forms above.

## Related

- [ADR-0005](./0005-react-nextjs-frontend.md): the Next.js standalone frontend this image
  runs.
- [ADR-0052](./0052-retiring-euler-stream-for-tiktok-signing.md): why tiktok-signer runs
  Chromium on Debian.
