{
  # Keep this line accurate and one line long: `nix flake metadata` prints it,
  # and it is the first thing a cold agent learns about the repo.
  description = "all-chat -- multi-platform live chat overlay: Go microservices + Next.js frontend. Run `nix flake show` for the command map.";

  # nixpkgs is the only input, on purpose. flake-utils would buy exactly one
  # thing here -- eachDefaultSystem -- and the canonical block below already
  # provides that as `forAllSystems`, keeping the system list in this file
  # instead of in a second input's copy of it.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    # `...` rather than a closed { self, nixpkgs }: adding a second input later
    # would otherwise fail with "called with unexpected argument '<name>'".
    #
    # `self` is mandatory. The canonical block's rootPreamble names this flake's
    # own source through it, and a flake whose outputs omit `self` will not
    # evaluate against that block at all.
    { self, nixpkgs, ... }:
    let
      lib = nixpkgs.lib;

      # Cosmetic: the interactive dev-shell banner and nothing else.
      repoName = "all-chat";

      # ======================================================================
      # PER-REPO BLOCK 1 -- the toolchain
      # ======================================================================
      # Everything the commands below need. `checks.toolchain` realises this
      # closure, so a typo'd attr name fails at the flake gate instead of
      # surfacing as "command not found" halfway through a task.
      #
      # Explicit `pkgs.foo`, never `with pkgs; [ ... ]`: when an attr disappears
      # in a nixpkgs bump, `with` reports a bare undefined identifier with no
      # hint of which set it came from, and the name is not greppable.
      #
      # Every attr here is forced on all three systems in the canonical block's
      # `systems` list, so all of them must exist on aarch64-darwin too. They do
      # under the current lock -- that is what `nix flake check --all-systems`
      # checks.
      toolchain = pkgs: [
        # ---- Go: services/* and shared/* (see the module walk in block 4) ----
        # Pinned by major. Under the current lock `pkgs.go_1_26` and the
        # unpinned `pkgs.go` are literally the same derivation (go-1.26.5, same
        # store path), so the pin is free today and stops a nixpkgs bump from
        # moving the compiler under the repo without a lock change.
        #
        # 1.26.5 satisfies every module the verbs walk: all 22 go.mod files
        # under services/ and shared/ declare `go 1.25.6` or newer. The Go
        # service Dockerfiles are less uniform -- they range from
        # golang:1.25-alpine to golang:1.26.5-alpine -- and CI's actions/setup-go
        # pins 1.25.12. GOTOOLCHAIN below is what keeps that spread honest
        # instead of silently downloading a fourth Go.
        pkgs.go_1_26
        pkgs.gopls
        # CONTRIBUTING.md asks for `golangci-lint run` before a PR. It is NOT
        # wired into `dev-lint` (see block 4) -- there is no .golangci.yml
        # tracked anywhere in the repo, so it would run its own default linter
        # set and report findings no CI job gates on. Shipped here so an agent
        # can run it deliberately.
        pkgs.golangci-lint

        # ---- Node 22: frontend/ plus the JS/TS services ----
        # 22, not 24/26, because every actions/setup-node step in
        # .github/workflows/ pins `node-version: '22'`, and frontend/README.md
        # names "Node.js 22+" as the floor. frontend/Dockerfile builds on
        # node:26-alpine and nothing in the repo explains that gap, so this
        # shell follows CI. npm ships INSIDE this derivation (bin/npm, bin/npx)
        # -- never add an npm attr beside it.
        pkgs.nodejs_22
        # For ad-hoc `tsc` at the prompt. The verbs deliberately do NOT use it:
        # `dev-build` runs each project's own node_modules/.bin/tsc, which is
        # the version its package-lock.json pins.
        pkgs.typescript

        # ---- clients for the local infrastructure the repo talks to ----
        # psql + pg_isready: the Makefile's `migrate` target,
        # scripts/run-migrations.sh, scripts/seed-test-data.sh and
        # scripts/verify-frontend-setup.sh all shell out to them. 16 matches
        # postgres:16-alpine in docker-compose.frontend.yml and in
        # deployments/k8s/base/postgres/deployment.yaml.
        pkgs.postgresql_16
        # redis-cli, used by scripts/generate-test-messages.sh,
        # scripts/verify-frontend-setup.sh and scripts/chaos-test-phase5.sh.
        pkgs.redis
        # `make docker-up` and `make frontend-dev` shell out to `docker-compose`.
        # This is the compose CLIENT only -- the docker DAEMON is a host service
        # and no flake can supply it. On NixOS enable virtualisation.docker.
        pkgs.docker-compose
        # scripts/k8s-make-user-admin.sh, plus the kubectl invocations in
        # deployments/ansible/.
        pkgs.kubectl

        # ---- shell conveniences ----
        # The Makefile is the repo's documented entrypoint (`make docker-up`,
        # `make migrate`, `make frontend-dev`), and scripts/chaos-test-phase5.sh
        # pipes through jq. git is here so an agent inside `nix develop` can
        # commit without leaving the shell -- no verb and no part of the
        # canonical anchor shells out to it.
        pkgs.git
        pkgs.jq
        pkgs.gnumake
      ];

      # ======================================================================
      # PER-REPO BLOCK 2 -- libraries that get dlopened, not linked
      # ======================================================================
      # npm prebuilds carry .so files that are dlopened at runtime, so neither
      # patchelf nor the nix linker ever sees them and NixOS has no /usr/lib for
      # them to find. frontend/package-lock.json pins both families that do this
      # here: @next/swc-* and @img/sharp-*. stdenv.cc.cc.lib supplies libstdc++,
      # which is the one that breaks first. Keep this list minimal --
      # LD_LIBRARY_PATH is a blunt instrument.
      #
      # This fixes shared libraries only. A prebuilt *executable* still needs a
      # real ELF interpreter at /lib64/ld-linux-x86-64.so.2, which is a host
      # setting (environment.ldso / programs.nix-ld.enable) that no project
      # flake can supply. That is why the Playwright browsers the frontend a11y
      # suite downloads are not usable out of the box here -- see block 4.
      #
      # Linux-only attrs are safe in this list: the canonical block only forces
      # it behind `pkgs.stdenv.hostPlatform.isLinux`.
      nativeLibs = pkgs: [
        pkgs.stdenv.cc.cc.lib
        pkgs.zlib
      ];

      # ======================================================================
      # PER-REPO BLOCK 3 -- constant environment variables
      # ======================================================================
      # Constants only. Anything that must READ an existing value
      # (LD_LIBRARY_PATH) or UNSET something (SOURCE_DATE_EPOCH) is handled by
      # the canonical block instead.
      #
      # This attrset is applied to BOTH surfaces -- the dev shell and every
      # `nix run` wrapper -- so a command cannot behave differently depending on
      # how it was invoked.
      envVars = pkgs: {
        # Measured, with only Go on PATH and a `go 1.99` directive in go.mod:
        #   GOTOOLCHAIN=local -> "go: go.mod requires go >= 1.99 (running go
        #                         1.26.5; GOTOOLCHAIN=local)"
        #   GOTOOLCHAIN=auto  -> "go: downloading go1.99.0 (linux/amd64)"
        # The second is a network fetch in the middle of a build. If a module
        # ever outruns nixpkgs, bump flake.lock -- do not unset this.
        GOTOOLCHAIN = "local";
        # Keeps `go build` from stamping VCS metadata, which is what produces
        # "error obtaining VCS status" in worktrees and in checkouts owned by
        # another uid. Verified that a flag the subcommand does not know is
        # ignored rather than fatal: `go vet ./...` and `go mod tidy` both exit
        # 0 with this in GOFLAGS. Never put -mod=vendor or -mod=mod here.
        GOFLAGS = "-buildvcs=false";
        # Every Go service Dockerfile (18 of them) builds with CGO_ENABLED=0 and
        # no file in the tree contains `import "C"`, so this matches what ships.
        # It is also load-bearing here: writeShellApplication puts ONLY
        # runtimeInputs on PATH and this toolchain has no C compiler, so with
        # cgo left at its default a plain `go build` of a package importing
        # "net" dies with
        #   cgo: C compiler "gcc" not found: exec: "gcc": executable file not
        #   found in $PATH
        # -- reproduced in a scratch module against exactly this Go derivation,
        # and fixed by this line. Setting it beats shipping gcc for a repo that
        # never needs one, and keeps the dev shell and `nix run` identical.
        CGO_ENABLED = "0";
        # Opts out of Next.js telemetry collection, so `dev-build` and `dev-run`
        # do not emit the first-run telemetry notice into captured output.
        NEXT_TELEMETRY_DISABLED = "1";
        # npm's funding banner, off, for the same reason.
        NPM_CONFIG_FUND = "false";
      };

      # ======================================================================
      # PER-REPO BLOCK 4 -- the command map
      # ======================================================================
      # THE single source of truth. It generates `apps` (so `nix run .#test`
      # works), the `dev-*` wrappers on PATH inside the shell, and `dev-help`.
      # Nothing is written twice, so `nix flake show` can never disagree with
      # what `dev-test` actually runs.
      #
      # `text` is bash under `set -euo pipefail`, shellcheck'd at BUILD time.
      # Rules, all of them enforced by the canonical block's contract: anchor
      # every path at $REPO_ROOT rather than a bare `.`; call
      # `need_writable_checkout` first in anything that writes; and pass the
      # batch/non-interactive flag to anything that could prompt, because there
      # is no tty under `nix run` and a prompt hangs until the agent's timeout.
      # That last rule is why the frontend suite below is invoked as
      # `vitest --run` and NOT via `npm test`: frontend/package.json maps `test`
      # to bare `vitest`, which is watch mode and never returns.
      #
      # Deliberately NOT covered by any verb, so the map stays honest:
      #   * scripts/quick-start-frontend.sh (`make frontend-quick`) asks a
      #     `read -r` question, so it can only be run by a human.
      #   * the `storybook` vitest project (frontend/vitest.config.ts) and
      #     frontend/tests/e2e/ drive a real Chromium through Playwright. Those
      #     downloaded browsers are FHS-linked binaries that need a host ldso
      #     (see block 2), and the nixpkgs alternative,
      #     playwright-driver.browsers, has a 2.30 GB closure (2304223264 bytes,
      #     per `nix path-info -S` against cache.nixos.org) -- which does not
      #     belong in a shell every agent enters. Run those gates in CI.
      #   * frontend eslint (`npm run lint`) and prettier: the repo's own
      #     frontend-quality.yml is `workflow_dispatch`-only. `dev-lint`
      #     therefore gates on the a11y ESLint config instead, whose job
      #     ("A11y lint + token contrast") IS in main's required status checks.
      #   * marketing/, scripts/ and the root package.json. They are tracked
      #     Node projects, but no workflow names them either -- the Node legs
      #     below cover exactly the four projects in security-scan.yml's
      #     npm-audit matrix.
      commands =
        pkgs:
        let
          # Every service under services/ and every package set under shared/ is
          # its own Go module -- 22 go.mod files, wired together with relative
          # `replace` directives and NO committed go.work (it is gitignored;
          # only go.work.sum is tracked). So there is no root package pattern
          # that reaches them: `go build ./...` at the top level fails with
          # "pattern ./...: directory prefix . does not contain main module or
          # its selected dependencies". Every Go verb therefore walks the
          # modules one at a time, the way the Makefile's per-service targets
          # and the CI test matrix do. Defined once here rather than pasted into
          # four command texts.
          #
          # test/ is excluded on purpose: no CI job runs `go test` on it (the
          # only workflow that names those modules is security-scan.yml, which
          # runs govulncheck), and two of them --
          # test/contract/deletion and test/contract/lifecycle -- drive
          # testcontainers and so want a Docker daemon. `dev-fmt` still covers
          # test/, because gofmt needs nothing.
          #
          # Failures are COLLECTED into $dev_failures and reported by devReport
          # at the very end of a verb; nothing aborts the walk. In a repo with
          # this many modules, one that needs something the machine lacks -- a
          # Docker daemon for the testcontainers suites, an npm ci that has not
          # run -- must not hide the state of the other twenty-one.
          #
          # `< <(find ...)` rather than `find ... | while`, because a pipeline
          # would put the loop in a subshell and the accumulator would not
          # survive it.
          eachGoModule = cmd: ''
            cd "$REPO_ROOT"
            dev_failures=""
            dev_modules=0
            while IFS= read -r gomod; do
              dev_modules=$((dev_modules + 1))
              mod="''${gomod%/go.mod}"
              # services/auth-service/shared/tracing is a go.mod + go.sum with
              # no .go file anywhere under it -- the one such module in the
              # tree. `go vet ./...` there exits 1 with "no packages to vet", so
              # that single empty directory would red every Go verb. Skip
              # loudly instead.
              if [ -z "$(find "$mod" -name '*.go' -print -quit)" ]; then
                printf '==> %s (no Go files, skipped)\n' "$mod"
                continue
              fi
              printf '==> %s\n' "$mod"
              if ! ( cd "$mod" && ${cmd} ); then
                dev_failures="$dev_failures $mod"
              fi
            done < <(find services shared -type f -name go.mod \
              -not -path "*/node_modules/*" -not -path "*/vendor/*" | sort)
            # Belt and braces behind the canonical anchor: a walk that inspected
            # NOTHING must never fall through to devReport and exit 0. That
            # green -- a whole repo's worth of go vet reported as a pass without
            # a single file read -- is the one outcome an agent cannot detect
            # from the outside.
            if [ "$dev_modules" -eq 0 ]; then
              echo "no Go module found under $REPO_ROOT/{services,shared} -- refusing to report success" >&2
              exit 1
            fi
          '';

          # Every Node leg is guarded on the exact binary it is about to run,
          # not on the mere existence of a node_modules directory, because the
          # two come apart: an npm ci that did not finish leaves the directory
          # there without the .bin entry, and bash's diagnostic for running a
          # path that does not exist is a bare "No such file or directory" with
          # exit 127 -- no mention of npm. A missing tool is a loud SKIPPED
          # rather than a failure, so `dev-test` is still worth running before
          # `dev-setup` has fetched anything; skipping SILENTLY is what would
          # make the verb a liar. `dev-run` is the exception: it is nothing but
          # the frontend, so it hard-fails.
          nodeLeg = ''
            node_bin() {
              if [ -x "$REPO_ROOT/$1/node_modules/.bin/$2" ]; then
                return 0
              fi
              echo "SKIPPED $1: $2 is not installed under $REPO_ROOT -- run 'dev-setup' (needs network)" >&2
              return 1
            }
            note_failure() {
              dev_failures="$dev_failures $1"
            }
          '';

          # Ends a verb: names everything that failed and sets the exit code.
          # It must be the LAST line -- an earlier `exit` (or a leg that aborts
          # under `set -e`) would swallow the summary.
          devReport = ''
            if [ -n "''${dev_failures:-}" ]; then
              echo "FAILED:''${dev_failures}" >&2
              exit 1
            fi
          '';
        in
        {
          setup = {
            description = "(network) go mod download every Go module, npm ci the frontend and the three Node services";
            text = ''
              # npm ci writes node_modules INTO the tree, so the read-only store
              # snapshot is a dead end -- say so once here instead of failing
              # four times.
              need_writable_checkout
              ${eachGoModule "go mod download"}
              ${nodeLeg}
              for proj in frontend services/discord-bot services/support-bot services/tiktok-listener; do
                printf '==> npm ci %s\n' "$proj"
                ( cd "$REPO_ROOT/$proj" && npm ci ) || note_failure "$proj"
              done
              ${devReport}
            '';
          };

          build = {
            description = "go build every Go module under services/ and shared/, then the Next.js frontend and tiktok-listener";
            text = ''
              ${eachGoModule ''go build ./... "$@"''}
              ${nodeLeg}
              if node_bin frontend next; then
                printf '==> next build frontend\n'
                ( cd "$REPO_ROOT/frontend" && npm run build ) || note_failure frontend
              fi
              # services/discord-bot and services/support-bot have no `build`
              # script in package.json; tiktok-listener's is `tsc`.
              if node_bin services/tiktok-listener tsc; then
                printf '==> tsc services/tiktok-listener\n'
                ( cd "$REPO_ROOT/services/tiktok-listener" && npm run build ) \
                  || note_failure services/tiktok-listener
              fi
              ${devReport}
            '';
          };

          test = {
            description = "go test -short every Go module under services/ and shared/ (some need Docker), then the Node suites";
            text = ''
              # `-short` is what the PR matrix in build-and-push.yml runs. It
              # does NOT make this hermetic: eight modules under services/
              # import testcontainers and only services/payment-service guards
              # on testing.Short(). Measured on a host with no Docker daemon
              # reachable, five of them fail on "failed to create Docker
              # provider" and land in the summary:
              #   FAILED: services/auth-service services/moderation-service
              #           services/overlay-manager services/share-service
              #           services/twitch-eventsub-listener
              # nix cannot supply a daemon, which is exactly why the failures
              # are collected and named at the end instead of aborting the
              # walk. Treat that list as a floor, not a ceiling: a second run
              # on the same host added services/youtube-listener-innertube on a
              # timing-sensitive TestPoller_TransientError. Read the summary the
              # verb prints, not this comment.
              ${eachGoModule ''go test -short ./... "$@"''}
              ${nodeLeg}
              # `--project unit --run`, never `npm test`: package.json maps
              # `test` to bare `vitest`, which is watch mode and would hang
              # forever with no tty, and the `storybook` project drives a real
              # browser (see the note above).
              if node_bin frontend vitest; then
                printf '==> vitest --project unit frontend\n'
                ( cd "$REPO_ROOT/frontend" \
                  && "$REPO_ROOT/frontend/node_modules/.bin/vitest" --project unit --run ) \
                  || note_failure frontend
              fi
              # Both of these map `test` to `vitest run`, which does terminate;
              # the binary is still invoked directly so the verb does not depend
              # on that staying true.
              for proj in services/support-bot services/tiktok-listener; do
                if node_bin "$proj" vitest; then
                  printf '==> vitest run %s\n' "$proj"
                  ( cd "$REPO_ROOT/$proj" && "$REPO_ROOT/$proj/node_modules/.bin/vitest" run ) \
                    || note_failure "$proj"
                fi
              done
              ${devReport}
            '';
          };

          lint = {
            description = "go vet every Go module under services/ and shared/, then the frontend a11y ESLint gate";
            text = ''
              ${eachGoModule ''go vet ./... "$@"''}
              ${nodeLeg}
              # The same eslint config, suppressions file and glob as the
              # "A11y lint + token contrast" job in
              # .github/workflows/frontend-a11y.yml, which is one of main's
              # required status checks and is ratcheted by
              # eslint.a11y.suppressions.json, so it is expected to be green.
              # (The job reaches eslint through npx; this runs the same pinned
              # binary directly.)
              # The repo's other frontend lint gates are not wired in (see the
              # note in block 4).
              if node_bin frontend eslint; then
                printf '==> eslint (a11y) frontend\n'
                ( cd "$REPO_ROOT/frontend" \
                  && "$REPO_ROOT/frontend/node_modules/.bin/eslint" \
                    -c eslint.a11y.config.mjs \
                    --suppressions-location eslint.a11y.suppressions.json \
                    'src/**/*.tsx' ) \
                  || note_failure frontend
              fi
              ${devReport}
            '';
          };

          fmt = {
            description = "gofmt -l -w the given Go paths (default: services/ shared/ test/ in the repo)";
            text = ''
              # -w REWRITES files, so this is the verb that must not be allowed
              # to guess: from the read-only store snapshot there is nothing
              # legitimate to write to.
              need_writable_checkout
              if [ "$#" -gt 0 ]; then
                # Explicit arguments are forwarded exactly as given, so they
                # resolve against the caller's cwd the way gofmt's own arguments
                # do. That is the one place cwd SHOULD matter: the caller named
                # the target.
                gofmt -l -w "$@"
              else
                # The default is the repo, never the cwd.
                #
                # These three directories are named individually rather than
                # handing gofmt "$REPO_ROOT", because a walk from the root would
                # also descend into frontend/node_modules and into whatever
                # build output a contributor is carrying. `git ls-files '*.go'`
                # reaches exactly services/, shared/ and test/ and nothing else.
                #
                # This IS a repo-wide rewrite, and the tree carries 106
                # gofmt-drifted files that predate this flake (measured with
                # `gofmt -l services shared test`), so an argument-less run is a
                # large diff on purpose. -l names every file it rewrote; scope
                # it with a path (`dev-fmt services/api-gateway`) when that diff
                # does not belong in your PR.
                gofmt -l -w "$REPO_ROOT/services" "$REPO_ROOT/shared" "$REPO_ROOT/test"
              fi
            '';
          };

          run = {
            description = "start the Next.js dev server on :3000 (backend: `make frontend-dev`)";
            text = ''
              # `next dev` writes .next/ into the tree and needs an installed
              # node_modules, neither of which the store snapshot can offer.
              need_writable_checkout
              if [ ! -x "$REPO_ROOT/frontend/node_modules/.bin/next" ]; then
                echo "frontend is not installed -- run 'dev-setup' (needs network) first" >&2
                exit 1
              fi
              cd "$REPO_ROOT/frontend"
              # frontend/package.json maps `dev` to `next dev -p 3000`, hence
              # the port in the description. One unconditional form, because a
              # trailing bare `--` is not passed on: measured against the npm
              # inside pkgs.nodejs_22 (10.9.8), `npm run <script> --` reaches
              # the script with argc=0, exactly like `npm run <script>`, while
              # `npm run <script> -- -p 4000` reaches it with argc=2.
              npm run dev -- "$@"
            '';
          };
        };

      # ======================================================================
      # PER-REPO BLOCK 5 -- repo-specific checks
      # ======================================================================
      # The canonical `anchoring` check proves the MECHANISM (rootPreamble and
      # guardPreamble) behaves. It does not prove that THIS repo's verbs call
      # it. `dev-fmt` is the verb that rewrites files, so it is the one worth
      # pinning, and gofmt needs no network -- which is what makes this probe
      # runnable inside the `nix flake check` sandbox at all. (`dev-lint` cannot
      # be probed the same way: `go vet` would want to download modules.)
      #
      # Both halves matter. A guard that refused everything would pass the
      # refusal half and leave every mutating verb in the repo dead, so the
      # probe also proves dev-fmt still does its job in a real checkout.
      extraChecks = pkgs: {
        fmtAnchoring =
          pkgs.runCommand "fmt-anchoring-check"
            {
              nativeBuildInputs = lib.attrValues (wrappers pkgs);
            }
            ''
              set -euo pipefail

              # ---- a foreign tree: dev-fmt must refuse, and touch nothing ----
              # The decoy carries a Go file and a flake.nix, which is everything
              # a marker-file anchor would need to be fooled. It is deliberately
              # NOT inside the checkout below: a directory inside a checkout is
              # part of that checkout, and the anchor treats it that way.
              mkdir decoy
              printf 'package main\nfunc  main( ){ }\n' > decoy/decoy.go
              printf '{\n  description = "not all-chat";\n  outputs = _: { };\n}\n' > decoy/flake.nix
              cp -r decoy decoy.orig
              if ( cd decoy && dev-fmt ) > fmt-refusal.log 2>&1; then
                echo "dev-fmt succeeded in a foreign tree; it must refuse" >&2
                cat fmt-refusal.log >&2
                exit 1
              fi
              diff -r decoy decoy.orig

              # ---- a real checkout: dev-fmt must adopt it and rewrite ----
              cp -r ${lib.escapeShellArg "${self}"} checkout
              chmod -R u+w checkout
              mkdir -p checkout/services/zz-fmt-probe
              printf 'package probe\nfunc  Probe( ){ }\n' \
                > checkout/services/zz-fmt-probe/probe.go
              ( cd checkout && dev-fmt )
              if ! grep -q 'func Probe() {}' checkout/services/zz-fmt-probe/probe.go; then
                echo "dev-fmt did not reformat a drifted file in its own checkout" >&2
                cat checkout/services/zz-fmt-probe/probe.go >&2
                exit 1
              fi

              # ...and the run inside the checkout must not have reached out of
              # it into the decoy sitting next door.
              diff -r decoy decoy.orig

              touch "$out"
            '';
      };

      # >>>>> BEGIN CANONICAL MACHINERY v1 <<<<<
      # ======================================================================
      # Everything from the BEGIN sentinel above to the END sentinel on the last
      # line of this file is fleet-canonical text: the same bytes in every repo
      # that carries this flake style. That is a checkable claim, not a boast --
      #
      #   sed -n '/BEGIN CANONICAL MACHINERY v1/,$p' flake.nix | sha256sum
      #
      # prints the same digest in every repo, or one of them has been edited.
      # (`,$p`, not a range ending on the END sentinel: a range whose closing
      # pattern were spelled out here would terminate on this very comment.)
      # Nothing here names a repository, a language, a tool or a project file.
      # If you find such a name below, it is contamination: the fix is to move
      # it into the per-repo section above, never to special-case it here.
      #
      # This region READS exactly these names from the per-repo section:
      #   nixpkgs  self  lib  repoName  toolchain  nativeLibs  envVars
      #   commands  extraChecks
      # and DEFINES exactly these:
      #   systems  forAllSystems  ldPreamble  rootPreamble  guardPreamble
      #   wrappers  helpFor  anchorCheck
      # plus the four flake outputs apps / devShells / checks / formatter.
      # Anything else in scope is invisible to it. The types of those eight
      # inputs, and the shell variables this region exports into command texts,
      # are specified in INTERFACE.md, which travels with this block.
      #
      # To change behaviour here you change it in every repo at once and bump
      # the version in both sentinels. A local edit is a bug by construction:
      # the digest above stops matching, and -- because rootPreamble anchors on
      # flake.nix byte-identity -- an edited working tree also stops being
      # recognised by wrappers built from the previous revision.
      # ======================================================================

      # ---- systems policy: decided once for the whole fleet ----
      #
      # Read this list as "evaluated on three, built on one". That is what was
      # measured, and it is all it means:
      #   * `nix flake check --all-systems` passes, so every output attribute
      #     below EVALUATES on all three systems.
      #   * only x86_64-linux has ever been BUILT. The machine this was verified
      #     on has no aarch64 emulation -- no binfmt handler, and `extra-
      #     platforms` is x86-only -- so aarch64 cannot be built there at all.
      # It is not a statement that anything works on aarch64. Do not upgrade it
      # into one in a README.
      #
      # Evaluating all three is still worth its seconds, because the failure it
      # catches is an eval-time failure: a `pkgs.<attr>` that exists on Linux
      # and not on darwin (`stdenv.cc.cc.lib` is the usual one) throws during
      # evaluation, and `nix flake check` without --all-systems checks only the
      # current system and sails straight past it.
      #
      # x86_64-darwin is deliberately absent. nixpkgs 26.11 replaced that whole
      # attribute set with a `throw`. genAttrs is lazy, so plain `nix develop`
      # on Linux would not notice -- it detonates later, on the --all-systems
      # run this policy requires. Add it back only against a separate
      # nixpkgs-26.05-darwin input.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      # Stand-in for flake-utils.lib.eachDefaultSystem. Passes `pkgs` rather
      # than a system string, because that is what every call site wants, and
      # keeps the system list in this file rather than in a second input's
      # hardcoded copy of it.
      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # Prepend, never assign: a host LD_LIBRARY_PATH may be carrying something
      # the user needs, and clobbering it breaks binaries they launch from here.
      # Linux only -- on darwin the loader variable is DYLD_*, and exporting a
      # Linux-shaped value there is at best useless.
      #
      # `&&` short-circuits in Nix, so on darwin `nativeLibs pkgs` is never
      # forced. That is load-bearing for the systems policy above: it is what
      # lets a repo list Linux-only attrs in nativeLibs and still evaluate on
      # aarch64-darwin. Do not reorder the two operands.
      ldPreamble =
        pkgs:
        lib.optionalString (pkgs.stdenv.hostPlatform.isLinux && nativeLibs pkgs != [ ]) ''
          export LD_LIBRARY_PATH="${lib.makeLibraryPath (nativeLibs pkgs)}''${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
        '';

      # Every command gets $SRC_ROOT and $REPO_ROOT. `nix run` and `nix develop`
      # both start in whatever directory they were invoked from, and no verb may
      # act on that directory -- these two are what it acts on instead.
      #
      # $SRC_ROOT is this flake's own source, snapshotted into the store when
      # the flake was evaluated. It is the one anchor that is always available:
      # `nix run /path/to/repo#lint` tells the running program nothing whatever
      # about /path/to/repo (flake refs are location-independent by design, and
      # there is no $FLAKE_DIR to read), so without `self` a wrapper invoked
      # that way has literally no way to name the repo it belongs to. Two
      # limitations worth knowing: it is read-only, being a store path, and in a
      # git checkout it contains only TRACKED files.
      #
      # $REPO_ROOT is the writable checkout when the caller is standing in one,
      # and $SRC_ROOT when they are not. Three things this deliberately is NOT:
      #
      #   * NOT `pwd`. A fallback to the caller's directory is how `fmt`
      #     rewrites a stranger's source tree and how `lint` prints "all checks
      #     passed" having read none of this repo.
      #   * NOT `git rev-parse --show-toplevel`. Run from inside some OTHER git
      #     repo it cheerfully answers with THAT repo's top level. It also needs
      #     git on PATH and a .git directory, so it fails on an export and in
      #     any wrapper whose toolchain omits git.
      #   * NOT an inherited $REPO_ROOT from the environment. The dev shell
      #     EXPORTS this variable, so honouring it would mean that running
      #     `nix run /path/to/B#fmt` from inside repo A's dev shell points B's
      #     formatter at A. An explicit path argument is how a caller overrides
      #     a verb's target; an ambient variable is how they do it by accident.
      #
      # Instead: walk up from $PWD and take the first ancestor that IS this
      # repo, proved by carrying a byte-identical flake.nix. A single tracked
      # filename, a marker directory, or a set of them is not proof -- sibling
      # repos in a fleet share those, and a decoy can be built to carry any list
      # of names you care to publish. The whole flake.nix is what distinguishes
      # repos, because description, toolchain and command map all differ, so the
      # whole flake.nix is what gets compared. Compared with bash's own
      # `$(<file)` rather than cmp or sha256sum, so the check depends on no
      # package at all -- pure builtins, correct even in a wrapper whose PATH
      # carries nothing but the repo's own toolchain.
      #
      # Consequence worth knowing: edit flake.nix and the dev-* wrappers in an
      # already-open `nix develop` stop recognising the tree, because they were
      # built from the previous flake.nix. That is a stale shell telling you so
      # -- re-enter it. `nix run` re-evaluates every time and never sees this.
      rootPreamble = ''
        SRC_ROOT=${lib.escapeShellArg "${self}"}
        export SRC_ROOT

        _dev_find_root() {
          local dir ref
          ref=$(<"$SRC_ROOT/flake.nix") || return 1
          dir=$(
            unset CDPATH
            cd -P -- "''${1:-.}" 2>/dev/null && pwd
          ) || return 1
          while [ -n "$dir" ]; do
            if [ -f "$dir/flake.nix" ] && [ "$(<"$dir/flake.nix")" = "$ref" ]; then
              printf '%s\n' "$dir"
              return 0
            fi
            dir=''${dir%/*}
          done
          return 1
        }

        REPO_ROOT="$(_dev_find_root "$PWD" || printf '%s\n' "$SRC_ROOT")"
        export REPO_ROOT
      '';

      # Wrappers only, not the shellHook -- an interactive shell has no business
      # carrying this function around. Any command text that writes files calls
      # it first, and it is the reason a mutating verb can fail loudly instead
      # of falling back to "well, the cwd then".
      #
      # The test is $REPO_ROOT != $SRC_ROOT, i.e. "rootPreamble found a real
      # checkout", not a permission or a store-path-prefix test. Both of those
      # answer a narrower question: a checkout may be read-only for unrelated
      # reasons, and a store path is not the only tree we must refuse to write.
      guardPreamble = ''
        need_writable_checkout() {
          if [ "$REPO_ROOT" != "$SRC_ROOT" ]; then
            return 0
          fi
          echo "''${0##*/}: this command rewrites files, so it needs a writable" >&2
          echo "checkout of this repo -- and standing in $PWD there is none: no" >&2
          echo "parent directory carries this flake's flake.nix. The only tree in" >&2
          echo "reach is the read-only store snapshot $SRC_ROOT, and rewriting" >&2
          echo "$PWD instead is exactly the bug this guard exists to prevent." >&2
          echo "cd into the repo (or \`nix develop\` it), or pass an explicit path." >&2
          exit 1
        }
      '';

      # One derivation per command, reused by both `apps` and the dev shell, so
      # the two can never diverge. `dev-` prefixed because a bare `test` binary
      # earlier on PATH would shadow the POSIX shell builtin and quietly break
      # every script in the repo that uses it.
      #
      # writeShellApplication, not writeShellScriptBin: it runs shellcheck at
      # BUILD time and sets `set -euo pipefail`, so an unquoted $@ or a silently
      # ignored failure is a `nix flake check` failure rather than a surprise in
      # front of an agent.
      wrappers =
        pkgs:
        lib.mapAttrs (
          name: cmd:
          pkgs.writeShellApplication {
            name = "dev-${name}";
            runtimeInputs = toolchain pkgs;
            runtimeEnv = envVars pkgs;
            meta.description = cmd.description;
            text = ''
              ${rootPreamble}
              ${guardPreamble}
              ${ldPreamble pkgs}
              ${cmd.text}
            '';
          }
        ) (commands pkgs);

      # `dev-help` is generated from the same attrset as everything else, so it
      # cannot describe a verb that does not exist or miss one that does. No
      # runtimeInputs: printing the map must work with nothing installed.
      helpFor =
        pkgs:
        let
          cmds = commands pkgs;
          names = lib.attrNames cmds;
          width = lib.foldl' (a: n: lib.max a (builtins.stringLength n)) 0 names;
          pad = n: n + lib.concatStrings (lib.genList (_: " ") (width - builtins.stringLength n));
          line = n: c: "  dev-${pad n}  ${c.description}";
        in
        pkgs.writeShellApplication {
          name = "dev-help";
          meta.description = "print this repo's command map (works offline)";
          text = ''
            cat <<'EOF'
            ${lib.concatStringsSep "\n" (lib.mapAttrsToList line cmds)}
            EOF
          '';
        };

      # The regression gate for rootPreamble and guardPreamble, which are the
      # two pieces of this flake that can silently damage a tree that is not
      # this repo. It tests the MECHANISM, not any verb, which is precisely what
      # makes it fleet-generic: it needs to know nothing about what this repo
      # does, only that the anchor resolves and the guard refuses.
      #
      # The decoy is a real directory carrying a real flake.nix that differs.
      # Marker-file anchors pass a decoy like this -- that is the whole point of
      # the probe -- and so does any anchor that trusts `pwd`. Probe 2 is the
      # other half, and without it a guard that refused everything would score a
      # perfect pass: a tree that IS byte-identical must still be adopted, or
      # every mutating verb in the repo is dead. Probe 3 pins the subdirectory
      # case, which is the normal one for an agent working inside a repo.
      #
      # A per-repo probe that drives the actual verbs is strictly better and
      # cannot live here -- it has to know which verb writes and which needs a
      # network. INTERFACE.md shows how to add one via `extraChecks`.
      anchorCheck =
        pkgs:
        pkgs.runCommand "anchor-check" { } ''
          set -euo pipefail

          # The two preambles under test, verbatim, in a file the probes source.
          # A quoted heredoc, so every $ below is the bash the wrappers see.
          cat > preamble.sh <<'CANONICAL_PREAMBLE_EOF'
          ${rootPreamble}
          ${guardPreamble}
          CANONICAL_PREAMBLE_EOF

          mkdir decoy
          printf '{\n  description = "a different repo";\n  outputs = _: { };\n}\n' > decoy/flake.nix
          printf 'do not touch me\n' > decoy/victim.txt
          cp -r decoy decoy.orig

          # ---- probe 1: a foreign tree must not be adopted ----
          if ! ( cd decoy && . ../preamble.sh && [ "$REPO_ROOT" = "$SRC_ROOT" ] ); then
            echo "anchor adopted a directory that is not this repo" >&2
            exit 1
          fi
          # In a subshell: need_writable_checkout ends in `exit`, which would
          # otherwise take this whole build down instead of failing a condition.
          if ( cd decoy && . ../preamble.sh && need_writable_checkout ) > guard.log 2>&1; then
            echo "need_writable_checkout accepted a tree that is not this repo" >&2
            exit 1
          fi
          if ! diff -r decoy decoy.orig; then
            echo "the probes modified the foreign tree" >&2
            exit 1
          fi

          # ---- probe 2: a byte-identical checkout must be adopted ----
          cp -r ${lib.escapeShellArg "${self}"} checkout
          chmod -R u+w checkout
          if ! ( cd checkout && . ../preamble.sh &&
                 [ "$REPO_ROOT" = "$(pwd -P)" ] && need_writable_checkout ); then
            echo "anchor refused a byte-identical checkout of this repo" >&2
            exit 1
          fi

          # ---- probe 3: from a subdirectory, still the checkout root ----
          mkdir -p checkout/probe3/deeper
          if ! ( cd checkout/probe3/deeper && . ../../../preamble.sh &&
                 [ "$REPO_ROOT" = "$(cd -P ../.. && pwd)" ] ); then
            echo "anchor did not walk up to the checkout root from a subdirectory" >&2
            exit 1
          fi

          touch "$out"
        '';
    in
    {
      # `nix flake show` -- the discovery entrypoint, and deliberately the whole
      # machine-facing contract: every app carries a meta.description, which
      # `nix flake show` prints inline and `nix flake show --json` exposes at
      # .apps.<system>.<name>.description. Pure evaluation, so an agent gets the
      # entire command map in one cheap call without reading a README.
      #
      # Do NOT invent a top-level output for this (`agentManifest`, `probeThing`
      # ...). Nix answers with `warning: unknown flake output '<name>'` on every
      # single `nix flake check`, forever.
      apps = forAllSystems (
        pkgs:
        lib.mapAttrs (name: cmd: {
          type = "app";
          program = "${(wrappers pkgs).${name}}/bin/dev-${name}";
          meta.description = cmd.description;
        }) (commands pkgs)
      );

      # `nix develop` -- the toolchain, plus a dev-<verb> for every app.
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = toolchain pkgs ++ lib.attrValues (wrappers pkgs) ++ [ (helpFor pkgs) ];

          env = envVars pkgs;

          # Natively-compiled extension modules are routinely built at -O0,
          # where glibc's _FORTIFY_SOURCE stops being a warning and becomes a
          # hard error.
          hardeningDisable = [ "fortify" ];

          shellHook = ''
            # mkShell inherits SOURCE_DATE_EPOCH=315532800 (1980-01-01) from
            # stdenv, and any wheel or zip built in here then dies with "ZIP does
            # not support timestamps before 1980".
            unset SOURCE_DATE_EPOCH

            # $REPO_ROOT and $SRC_ROOT are exported here as a convenience for
            # the human at the prompt. Every wrapper re-resolves them from
            # scratch and none of them reads these, on purpose: a stale value
            # exported by one repo's shell must never steer another repo's verb.
            ${rootPreamble}
            ${ldPreamble pkgs}

            # Nothing networked, nothing stateful and nothing interactive above
            # this line, and nothing below it either. No environment
            # bootstrapping, no dependency installation, no `read`, no
            # `exec $SHELL`. Bootstrapping in the hook makes a cold
            # `nix develop -c <anything>` start downloading before it runs
            # anything, on EVERY invocation -- the exact failure an unattended
            # agent cannot diagnose. That is what a `setup` verb is for.

            # The banner is interactive-only, and this guard is load-bearing:
            # shellHook output lands on the STDOUT of `nix develop -c <cmd>`, so
            # an unguarded echo corrupts anything parsing it
            # (`nix develop -c cat x.json | jq` fails to parse). $- is the only
            # reliable discriminator here -- it lacks `i` for `nix develop -c`
            # and has it at an interactive prompt. Do not test $PS1 (unset in
            # both) or $IN_NIX_SHELL (set in both). >&2 is the second layer, for
            # the case where a caller runs us on a pty.
            case $- in
              *i*) echo "${repoName} dev shell -- 'dev-help' for the command map" >&2 ;;
            esac
          '';
        };
      });

      # `nix flake check` -- honest by construction, and the only gate this
      # style has. `toolchain` realises the whole toolchain closure (so a typo'd
      # or currently-broken attr fails here, not halfway through a task) and
      # builds every wrapper, which runs shellcheck over every command text.
      # `anchoring` is the regression test described above.
      #
      # Repo-specific checks go in `extraChecks`, never here. They may not
      # shadow either canonical name: silently replacing `anchoring` with
      # something weaker is the exact failure this whole file exists to make
      # impossible, so a collision is an eval error with both names in it.
      #
      # NEVER add a check that always passes. An agent reads "all checks
      # passed!" as a signal, and a fake check makes `nix flake check` a liar.
      checks = forAllSystems (
        pkgs:
        let
          canonical = {
            toolchain =
              pkgs.runCommand "toolchain-check"
                {
                  nativeBuildInputs = toolchain pkgs ++ lib.attrValues (wrappers pkgs) ++ [ (helpFor pkgs) ];
                }
                ''
                  set -euo pipefail
                  dev-help > help.txt

                  # A while-read over a heredoc rather than `for x in <list>`,
                  # which is a bash syntax error when the list is empty -- and a
                  # repo with no verbs yet is a legitimate state.
                  while IFS= read -r verb; do
                    [ -n "$verb" ] || continue
                    command -v "dev-$verb" > /dev/null || {
                      echo "dev-$verb is not on PATH" >&2
                      exit 1
                    }
                    grep -q -- "dev-$verb" help.txt || {
                      echo "dev-$verb is missing from the dev-help map" >&2
                      exit 1
                    }
                  done <<'CANONICAL_VERBS_EOF'
                  ${lib.concatStringsSep "\n" (lib.attrNames (commands pkgs))}
                  CANONICAL_VERBS_EOF

                  touch "$out"
                '';
            anchoring = anchorCheck pkgs;
          };
          extra = extraChecks pkgs;
          clash = lib.intersectLists (lib.attrNames canonical) (lib.attrNames extra);
        in
        if clash != [ ] then
          throw "extraChecks must not redefine canonical checks: ${lib.concatStringsSep ", " clash}"
        else
          canonical // extra
      );

      # `nix fmt` -- formats the *Nix* in this repo; project code gets a `fmt`
      # verb. nixfmt-tree (the treefmt wrapper) rather than bare nixfmt, because
      # bare nixfmt tries to parse every path handed to it and fails on non-Nix
      # files. This file ships already formatted, so `nix fmt` is a no-op rather
      # than a diff across the fleet.
      #
      # This is the one verb here NOT anchored to $REPO_ROOT, and it cannot be:
      # `nix fmt` is nix's own verb, and nix -- not this flake -- decides which
      # paths the formatter receives, passing the cwd when the user names none.
      # A wrapper that overrode them would break `nix fmt path/to/one/file.nix`,
      # and it cannot tell that "." apart from the default. So `nix fmt` formats
      # where you stand, by design; the `fmt` verb is the anchored one.
      formatter = forAllSystems (pkgs: pkgs.nixfmt-tree);
    };
}
# >>>>> END CANONICAL MACHINERY v1 <<<<<
