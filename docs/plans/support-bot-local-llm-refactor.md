# Support-bot refactor: local LLM + scoped access model

- **Date:** 2026-07-10
- **Branch:** `refactor/support-bot-local-llm-scoped-agent`
- **Status:** implemented in the worktree (builds, `go vet`, `go test`, `govulncheck` clean); pending review + companion caesar-deployment PR.

## Motivation

The Discord support bot (`services/support-bot`) was a TypeScript service that answered
questions by shelling out to the `claude` CLI (`execa('claude', ['-p', …])`) authenticated
with a `CLAUDE_CODE_OAUTH_TOKEN` — the "Claude workaround" to avoid API charges. It had one
fixed tool set and parsed text markers (`PROPOSE_ISSUE:`, `INFRA_VERDICT:`, `STORE_MEMORY:`)
out of the model output.

Two problems drove this refactor:

1. **Deprecate the Claude workaround.** We now run a locally hosted, OpenAI-compatible LLM,
   so the bot should call it directly (`POST {BASE_URL}/v1/chat/completions` with native
   tool calling) instead of driving the `claude` CLI subprocess.
2. **Two access modes.** A single fixed capability set is wrong. We want:
   - **support** — cannot write code, read-only cluster access, must not disclose raw log
     lines / secrets / internal hostnames.
   - **admin** — may write code, do code reviews, and push to the cluster; only DB writes
     and secret reads are off-limits. Admin is restricted to repo maintainers via a Discord
     UID allow-list.

The design follows a standard secure tool-gateway pattern: a code-enforced access scope,
fail-closed tool gating, credential isolation, prompt-injection hygiene, and an agentic
tool loop.

## Decisions

| Decision | Choice | Why |
|---|---|---|
| Language/runtime | **Go** (`bwmarrin/discordgo`) | Aligns with the rest of the all-chat backend (ADR-0001 standard Go layout, zap, pgx, gin, health probes). |
| LLM protocol | **OpenAI-compatible + native tool calling** | Near-universal local-server interface (vLLM/Ollama/LM Studio/llama.cpp); `tool_choice: auto`, `choices[].message.tool_calls[]`. Base URL + model are config. |
| Cluster write for admin | **GitHub write only** (per maintainer clarification) | Admin "push to cluster" = open a PR against the manifests; the GitOps/Keel pipeline deploys. **No new K8s write RBAC.** |
| Scope of change | **Full rewrite** in place | Replaces the TS service entirely; TS files removed. |

## Current-state facts that shaped the design

- **Live cluster RBAC is already read-only** (`support-bot-cluster-reader`: get/list/watch
  pods, events, deployments, replicasets, pod metrics). That is exactly "support" today.
- The SA also has `support-bot-secret-patcher` (get+patch `allchat-secrets`) — an operational
  ability, *not* an LLM tool. The LLM never gets a secret-read tool in either mode.
- Real owner is `caesarakalaeii`; image `ghcr.io/caesarakalaeii/allchat-support-bot:main`;
  lead dev UID `198569499228766208`; guild `1441528700259663884`; client `1485959612334346342`.

## Target architecture

Standard Go layout under `services/support-bot/`:

```
cmd/main.go            entry: config → logger → db(memory) → llm client → registry → bot → health
config/                env → Config (LOCAL_LLM_*, admin allow-list, etc.)
access/                Mode {support, admin}; Policy.ModeFor(uid) from allow-list  ← the scope decision
llm/                   OpenAI-compatible client (types, client, errors, mask)
agent/                 Run loop, loop detector, scope-aware prompt builder
tool/                  Tool interface + ToolCtx + mixins (BothModes/AdminOnly/Disabled) + Registry (choke point)
tools/                 read_file/glob/grep, kubectl(read-only), grafana logs/metrics, github, github_write, memory
ghclient/              thin GitHub REST client
grafana/               thin Grafana Loki/Prometheus client
ghsafe/                protected-branch + repo deny-list + param validation
sanitize/              prompt-injection hygiene + boundary tags + scaffold stripping
redact/                code-level leak redactor + log summarizer
memory/                bot_memories repository (pgx) + tag extraction
moderation/            cross-channel-spam detector
discord/               discordgo adapter: mode resolution, loop, redaction, threading, delivery
handlers/              /health/live, /health/ready
```

Request flow: Discord message / `/support` → resolve `access.Mode` from the UID allow-list →
scope-aware system prompt + `SanitizeForPrompt`d user turn → `agent.Run` (LLM → `tool_calls`
→ `registry.Dispatch` fail-closed by mode → tool results → repeat until stop / cap) →
`StripInternalScaffolds` + (support) `redact.Redact` on the final answer → deliver in a
Discord thread; ping lead dev on any side-effecting action.

### Access matrix (enforced in code, `registry.Dispatch`)

| Tool | support | admin |
|---|---|---|
| `read_file`, `glob`, `grep` (repo, path-jailed) | ✅ redacted | ✅ |
| `kubectl` (get/describe/logs/events/top only) | ✅ logs summarized | ✅ raw |
| `grafana_logs` / `grafana_metrics` | ✅ logs summarized | ✅ raw |
| `github` (get_issue / comment / create_issue) | ✅ | ✅ |
| `github_write` (push_file / open_pr / review / close_issue) | ❌ | ✅ |
| `memory` (recall / store / update) | ✅ | ✅ |
| DB write / secret read | ❌ (no such tool) | ❌ (no such tool) |

`access.ModeSupport` is the zero value, so a forgotten assignment fails closed to
read-only+redacted, never to admin.

## Security invariants

- **Scope is set by code, never by prompt.** `access.Policy.ModeFor(discordUID)` reads a
  trusted config allow-list; `ToolCtx.Mode` is immutable per request; no message/tool/memory
  content can change it.
- **Fail-closed dispatch.** Unknown tool name, tool not allowed in mode, or any tool error →
  a boundary-wrapped error tool result; the tool is never invoked. Admin tools are not even
  advertised to the model in support mode (`Registry.Defs(mode)`).
- **Two-layer leak redaction** (support mode): `redact.Redactor` runs over every tool output
  in `Dispatch` **and** over the final answer before Discord. Log-bearing tools additionally
  *summarize* (counts + normalized patterns) rather than emitting raw lines — a stack frame
  survives token redaction, so it must never leave the tool raw.
- **Prompt-injection hygiene.** `SanitizeForPrompt` strips control/format/bidi/zero-width
  chars + fullwidth homoglyphs and escapes markup on anything echoed into the prompt; tool
  output is fenced in `<tool_output>` boundary tags with breakout neutralization;
  `StripInternalScaffolds` removes internal tags from model output before delivery.
- **kubectl safety.** Fixed read-only action allow-list mapped to verbs; no write action
  exists; `exec.CommandContext("kubectl", argv...)` with an arg slice (never a shell string);
  namespace pinned before any positional; positionals validated and flag-injection rejected;
  the `secrets` resource is blocked; proxy env stripped; RBAC is the real backstop.
- **GitHub write safety** (admin). Never push to a protected branch (main/master/release-…);
  all changes land on a feature branch + PR; repo deny-list + allow-list; param validation
  (reject `..`, `//`, `?`, `#`, `%`, `@`, CRLF); secret-strip on outbound titles/bodies/
  comments; edit/close restricted to bot-authored objects.
- **No secrets to the LLM.** Upstream error bodies (LLM, GitHub, kubectl stderr, Grafana) are
  credential-masked/first-line-only and logged, never inserted into the conversation.

## Local LLM integration

`llm.Client` is a non-streaming OpenAI-compatible client: `POST {BASE_URL}/v1/chat/completions`
with `model`, `messages`, `tools`, `tool_choice` defaulted to auto. Function-call arguments
are a JSON-encoded string in both directions; tool results are sent back as `role:"tool"`
messages keyed by `tool_call_id`. Bearer auth is optional (local servers often need none) and
API keys are validated as printable ASCII to prevent header injection. Retries use capped
(30 s) exponential backoff + jitter on 408/425/429/5xx, honoring `Retry-After`; redirects are
disabled (SSRF). End-of-turn = `finish_reason == stop` and no tool calls.

## What changed

- **Removed:** all TypeScript (`src/**`, `package.json`, `package-lock.json`, `tsconfig.json`,
  `vitest.config.ts`) and the Node Dockerfile.
- **Added:** the Go service above, a Go multi-stage Dockerfile (bundles `kubectl`),
  `README.md`, `.env.example`, and unit tests (10 packages).
- **CI:** moved `services/support-bot` from the npm-audit matrix + `test-node` job to the
  `govulncheck` matrix (`security-scan.yml`), the Go PR test matrix (`build-and-push.yml`,
  and deleted `test-node`), and the nightly Go test matrix (`nightly-tests.yml`).
- **Manifests:** rewrote `deployments/k8s/base/support-bot/deployment.yaml` (drop
  `CLAUDE_CODE_OAUTH_TOKEN`, add `LOCAL_LLM_*` + `SUPPORT_BOT_ADMIN_DISCORD_IDS` +
  `GITHUB_BOT_LOGIN`, add HTTP health probes, keep the read-only SA for kubectl).

## Companion caesar-deployment changes (separate PR)

`apps/workloads/all-chat/support-bot-deployment.yaml` must be updated to match:

1. Remove the `CLAUDE_CODE_OAUTH_TOKEN` env and the `register-slash-commands` initContainer
   (slash-command registration is now in-process on startup).
2. Add env: `LOCAL_LLM_BASE_URL`, `LOCAL_LLM_MODEL`, `LOCAL_LLM_API_KEY` (optional secret),
   `SUPPORT_BOT_ADMIN_DISCORD_IDS`, `GITHUB_BOT_LOGIN`, `KUBE_NAMESPACE=allchat`,
   `PORT=8094`, `GIN_MODE=release`, `HOME=/tmp`.
3. Replace the `ps aux | grep node` liveness exec probe with httpGet `/health/live` +
   readiness `/health/ready` on port 8094.
4. Keep the repo-clone initContainers and the `run-migrations` initContainer (bot_memories).
5. `support-bot-rbac.yaml` is unchanged (cluster access stays read-only). Reassess whether the
   `support-bot-secret-patcher` (get+patch `allchat-secrets`) role is still needed by any
   operational job; the bot itself no longer requires it.

**New secret key** (add to the sealed `allchat-secrets`, do not `sops set` blind — see
secrets-drift note): `SUPPORT_BOT_LLM_API_KEY` (optional; omit if the local server is
unauthenticated).

## Testing

Unit tests cover the security-critical units: access-mode resolution + zero-value safety,
registry fail-closed dispatch (unknown/denied/redacted/wrapped), sanitize (invisible strip,
markup + homoglyph escape, boundary neutralization, scaffold stripping), redact (secrets,
internal topology, log summarization, stack-trace detection), ghsafe (protected branch, repo
deny-list, param validation), kubectl arg building (write actions rejected, secrets blocked,
namespace pinned first), path-jail escape rejection, the LLM client (tool-call round-trip,
retry, credential masking, bad-key rejection) via `httptest`, the agent loop (tool execution,
admin-tool denial in support mode, max-iterations, loop-detector), and the spam detector.
`go build`, `go vet`, `gofmt`, `go test -race ./...`, and `govulncheck ./...` (0 reachable
vulns) all pass.

### Post-implementation adversarial review

A 4-dimension review (scope/leak, Go correctness, tool safety, discord/loop), with each
finding independently verified, surfaced 7 defects — all fixed and regression-tested:

1. **(high) LLM client data race** — `Client.lastMasked` was per-request state on the
   shared client; concurrent `Chat` calls (different channels + slash commands) raced and
   could cross-contaminate masked error bodies. Fixed: the masked body is now a local
   threaded through the retry loop; the client holds no per-request state (verified under
   `-race`).
2. **(medium) support-mode leak via `kubectl describe`/`events`** — only `logs` was
   summarized; describe/events dumped raw env values + `Last State` stack traces that the
   token redactor did not catch. Fixed: `Redact()` now strips stack traces globally, and
   support mode summarizes logs/describe/events (get/top stay tabular + redacted).
3. **(medium) `grep` followed file symlinks** out of the jail. Fixed: symlink entries are
   skipped during the walk.
4. **(medium) `read_file` symlink jail escape** — lexical check only. Fixed: the
   symlink-resolved real path is re-verified within the root.
5. **(medium) `/support` dropped answer chunks** past the first when thread creation
   failed. Fixed: the remainder is delivered in-channel.
6. **(medium) per-channel ordering overstated** + **(low) unbounded queue map**. Fixed:
   `keyedMutex` replaced by a per-key serial queue that preserves FIFO among enqueued
   tasks and deletes idle keys; comments corrected to promise mutual exclusion + enqueue
   FIFO (not a global gateway receive-order guarantee).

## Rollout ordering

1. Merge this all-chat PR → CI builds `ghcr.io/caesarakalaeii/allchat-support-bot:main`.
2. Add the `SUPPORT_BOT_LLM_API_KEY` secret (if used) and confirm the local LLM service is
   reachable in-cluster at the configured `LOCAL_LLM_BASE_URL`.
3. Merge the caesar-deployment PR → ArgoCD syncs the new env/probes; Keel rolls the image.
4. Verify: `/support` answers as support (redacted), an allow-listed maintainer gets admin
   tools, and `github_write` opens a PR (never a push to `main`).

## Residual risks / follow-ups

- **`LOCAL_LLM_BASE_URL` / `LOCAL_LLM_MODEL` are placeholders** in the manifests — set to the
  real in-cluster model service before deploy.
- **`push_file` content is not auto-redacted** (redacting code risks corrupting it); it relies
  on admin trust + PR review + GitHub secret scanning. Human text (titles/bodies/comments) is
  redacted.
- **Loop-detector canonicalization** normalizes argument whitespace, not JSON key order
  (avoids untyped decoding); adequate in practice since models emit stable key order.
- **Multi-file code changes** use one `push_file` per file; a tree/commit batch API is a
  possible future enhancement.
- Consider promoting the redaction regexes and prompt-injection sanitizer into `shared/` if
  another service needs them.
- **Short-form internal hostnames** (`service.namespace`, no `.svc.cluster.local`) in free
  text are a best-effort residual — they are indistinguishable from `file.ext` without
  false positives, so the redactor does not target them; `.svc.cluster.local`, pod IPs,
  secrets, and stack traces are covered.
- **`botThreads` map** (thread IDs the bot manages) still grows for the process lifetime;
  low severity and bounded by Keel rollouts. Prune on thread-archive events if it matters.
