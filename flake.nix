{
  # Keep this line accurate and one line long: `nix flake metadata` prints it,
  # and it is the first thing a cold agent learns about the repo.
  description = "all-chat -- multi-platform live chat overlay: Go microservices + Next.js frontend. Run `nix flake show` for the command map.";

  # nixpkgs is the only input, on purpose.
  #
  # flake-utils would buy exactly one thing here -- eachDefaultSystem -- which is
  # the three-line genAttrs below. In exchange it costs a second lock node, a
  # second upstream that can break, and a hardcoded system list this repo cannot
  # edit. That list is currently broken: it still contains x86_64-darwin, which
  # now throws (see `systems` below).
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    # `...` rather than a closed { self, nixpkgs }: adding a second input later
    # would otherwise fail with "called with unexpected argument '<name>'".
    #
    # `self` is destructured because rootPreamble needs to name this flake's own
    # source -- the tree nix has already copied into the store -- as the last
    # resort when the caller is standing outside any checkout of this repo.
    { self, nixpkgs, ... }:
    let
      lib = nixpkgs.lib;

      # x86_64-darwin is deliberately absent -- nixpkgs 26.11 replaced that whole
      # attribute set with a `throw`. genAttrs is lazy, so plain `nix develop` on
      # Linux would not notice; it detonates on `nix flake check --all-systems`.
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      # Stand-in for flake-utils.lib.eachDefaultSystem. Passes `pkgs` rather than
      # a system string, because that is what every call site below wants.
      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # ======================================================================
      # PER-REPO BLOCK 1 -- the toolchain
      # ======================================================================
      # Everything the commands below need. `nix flake check` realises this
      # closure, so a typo'd attr name fails at the flake gate instead of
      # surfacing as "command not found" halfway through a task.
      #
      # Explicit `pkgs.foo`, never `with pkgs; [ ... ]`: when an attr disappears
      # in a nixpkgs bump, `with` reports a bare undefined identifier with no
      # hint of which set it came from, and the name is not greppable.
      toolchain = pkgs: [
        # ---- Go: services/* and shared/* (see the module list in block 4) ----
        # Bare `pkgs.go` is the documented exception to pin-by-major: there is no
        # major-pinned Go alias in nixpkgs. Every go.mod here asks for 1.25.6 or
        # newer and the service Dockerfiles build on golang:1.26.5-alpine, so the
        # nixpkgs Go (1.26.x) is the right side of every directive. GOTOOLCHAIN
        # below keeps that honest instead of silently downloading another Go.
        pkgs.go
        pkgs.gopls
        # CONTRIBUTING.md asks for `golangci-lint run` before a PR. It is NOT
        # wired into `dev-lint` (see block 4) -- there is no .golangci.yml in the
        # repo, so it would run its own default linter set and report findings CI
        # never gated on. Shipped here so an agent can run it deliberately.
        pkgs.golangci-lint

        # ---- Node 22: frontend/ (Next.js) plus the JS/TS services ----
        # 22, not 24/26, because that is what every CI job pins
        # (actions/setup-node node-version: '22') and 22 is the declared floor in
        # frontend/README.md. frontend/Dockerfile's node:26 is documented there
        # as a "stay current" choice, not a requirement. npm ships INSIDE this
        # derivation -- never add an npm attr beside it.
        pkgs.nodejs_22
        pkgs.typescript

        # ---- clients for the local infrastructure the repo talks to ----
        # psql + pg_isready: `make migrate`, scripts/run-migrations.sh,
        # scripts/seed-test-data.sh. 16 matches postgres:16-alpine in
        # docker-compose.frontend.yml and the CNPG cluster in production.
        pkgs.postgresql_16
        # redis-cli, used by the seed/verify/chaos scripts.
        pkgs.redis
        # `make docker-up` / `make frontend-dev` drive these compose files. This
        # is the compose CLIENT only -- the docker DAEMON is a host service and
        # no flake can supply it. On NixOS enable virtualisation.docker.
        pkgs.docker-compose
        # scripts/k8s-make-user-admin.sh and the deployment docs.
        pkgs.kubectl

        # ---- present in every repo in the fleet ----
        pkgs.git
        pkgs.jq
        pkgs.gnumake
      ];

      # ======================================================================
      # PER-REPO BLOCK 2 -- libraries that get dlopened, not linked
      # ======================================================================
      # npm prebuilds (the SWC binaries Next.js pulls, sharp-style addons) carry
      # .so files that are dlopened at runtime, so neither patchelf nor the nix
      # linker ever sees them and NixOS has no /usr/lib for them to find.
      # stdenv.cc.cc.lib supplies libstdc++, which is the one that breaks first.
      # Keep this list minimal -- LD_LIBRARY_PATH is a blunt instrument.
      #
      # This fixes shared libraries only. A prebuilt *executable* still needs a
      # real ELF interpreter at /lib64/ld-linux-x86-64.so.2, which is a host
      # setting (environment.ldso / programs.nix-ld.enable) that no project flake
      # can supply. That is exactly why the Playwright browsers the frontend a11y
      # suite downloads are not usable out of the box here -- see block 4.
      nativeLibs = pkgs: [
        pkgs.stdenv.cc.cc.lib
        pkgs.zlib
      ];

      # ======================================================================
      # PER-REPO BLOCK 3 -- constant environment variables
      # ======================================================================
      # Only values that are constants belong here. Anything that must READ an
      # existing value (LD_LIBRARY_PATH), UNSET something (SOURCE_DATE_EPOCH) or
      # touch the work tree goes in the shellHook further down.
      #
      # This attrset is applied to BOTH surfaces -- the dev shell and every
      # `nix run` wrapper -- so a command cannot behave differently depending on
      # how it was invoked.
      envVars = pkgs: {
        # Without this, a `go 1.99` directive in any go.mod under services/ or
        # shared/ makes Go fetch another toolchain mid-build over the network. With
        # it you get a legible "go.mod requires go >= X (running go Y;
        # GOTOOLCHAIN=local)". If a module ever outruns nixpkgs, bump
        # flake.lock -- do not unset this.
        GOTOOLCHAIN = "local";
        # Avoids "error obtaining VCS status" in worktrees and agent checkouts
        # owned by another uid. Unknown-to-command flags in GOFLAGS are ignored,
        # so vet/test/mod tidy still work. Never put -mod=vendor/-mod=mod here.
        GOFLAGS = "-buildvcs=false";
        # Every service Dockerfile builds with `CGO_ENABLED=0` and no package in
        # the tree imports "C", so this only matches what ships. It is also
        # load-bearing here: writeShellApplication puts ONLY runtimeInputs on
        # PATH, so the stdenv C compiler that `nix develop` happens to provide is
        # absent under `nix run`, and cgo-enabled builds of the stdlib then fail
        # with `cgo: C compiler "gcc" not found` -- reproduced on `nix run .#lint`
        # before this line existed. Setting it beats shipping gcc for a repo that
        # never needs it, and keeps both surfaces identical.
        CGO_ENABLED = "0";
        # Next.js phones home on every build otherwise, and the prompt it prints
        # the first time is pure noise in an agent's captured output.
        NEXT_TELEMETRY_DISABLED = "1";
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
      # Rules: always a quoted "$@"; anchor EVERY path at $REPO_ROOT (see the
      # resolver below -- no bare `.`, no bare relative path, and no `pwd`
      # fallback, so a verb reads and writes the same tree no matter where it
      # was invoked from, and only paths the caller passes explicitly may be
      # cwd-relative); and pass the batch/non-interactive flag to anything that
      # could prompt -- there is no tty under `nix run`, so a prompt hangs
      # until the agent's timeout. That
      # last rule is why the frontend suite below is invoked as
      # `vitest --run` and NOT via `npm test`: frontend/package.json maps `test`
      # to bare `vitest`, which is WATCH mode and never returns.
      #
      # Deliberately NOT covered by any verb, so the map stays honest:
      #   * scripts/quick-start-frontend.sh (`make frontend-quick`) asks a
      #     `read -r` question, so it can only be run by a human.
      #   * the storybook vitest project and tests/e2e/ need a real Chromium
      #     from `npx playwright install`; those downloads are FHS-linked
      #     binaries that need a host ldso (see block 2), and the nixpkgs
      #     alternative (playwright-driver.browsers) is ~2.3 GB, which does not
      #     belong in a shell every agent enters. Run the a11y browser gates in
      #     CI, or install the browsers by hand in a shell that has nix-ld.
      #   * frontend eslint (`npm run lint`) and prettier: the repo's own
      #     frontend-quality.yml is workflow_dispatch-only because of
      #     pre-existing lint debt, and `gofmt`/prettier drift already exists in
      #     the tree. `dev-lint` therefore gates on the a11y ESLint config,
      #     which IS a required check on main, and `dev-fmt` is Go-only: it
      #     rewrites the repo's Go trees, and the ~100 pre-existing drifted
      #     files come with it unless you hand it a narrower path. Frontend
      #     formatting stays `npm run format` inside frontend/, as
      #     CONTRIBUTING.md says.
      commands =
        pkgs:
        let
          # Every service under services/ and every package set under shared/ is
          # its own Go module, wired together with relative `replace` directives
          # and NO committed go.work (it is gitignored; only go.work.sum is
          # tracked). So there is no root package pattern that reaches them --
          # `go build ./...` from the top level sees nothing -- and every Go verb
          # walks the modules the way the Makefile and the CI matrix do. Defined
          # once here rather than pasted into four command texts.
          #
          # test/ is excluded on purpose: those contract modules are
          # //go:build integration harnesses that want Docker and a migrated
          # Postgres, and CI runs them in dedicated jobs, not in the PR matrix.
          #
          # Failures are COLLECTED into $dev_failures and reported by devReport
          # at the very end of a verb; nothing aborts the walk. That mirrors CI,
          # where every module and every Node project is its own job (and the
          # matrices that could stop early set `fail-fast: false`): in a repo
          # this size, one module that needs something the machine lacks -- a
          # Docker daemon for the testcontainers suites, an npm ci that has not
          # run -- must not hide the state of the other twenty.
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
              # no .go file anywhere under it (a leftover module path that CI
              # still scans). `go vet ./...` there exits 1 with "no packages to
              # vet", so one empty directory would red every Go verb --
              # reproduced on `nix run .#lint`. Skip loudly instead.
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
            # Belt and braces behind the root resolution above: a walk that
            # inspected NOTHING must never fall through to devReport and exit 0.
            # That green -- 20 modules' worth of go vet reported as a pass
            # without a single file read -- is what the old `|| pwd` produced,
            # and it is the one outcome an agent cannot detect from the outside.
            if [ "$dev_modules" -eq 0 ]; then
              echo "no Go module found under $REPO_ROOT/{services,shared} -- refusing to report success" >&2
              exit 1
            fi
          '';

          # Every Node leg is guarded on the exact binary it is about to run,
          # not on the mere existence of a node_modules directory: an
          # interrupted `npm ci` leaves the directory in place but empty, and
          # the leg then dies with a bare "No such file or directory" and exit
          # 127 -- reproduced against a half-finished install. A missing tool is
          # a loud SKIPPED rather than a failure, so `dev-test` is still worth
          # running before `dev-setup` has fetched anything; skipping SILENTLY
          # is what would make the verb a liar. `dev-run` is the exception: it
          # is nothing but the frontend, so it hard-fails.
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
          # under `set -e`) would swallow the summary, which is exactly the bug
          # a run with a broken frontend leg exposed here.
          devReport = ''
            if [ -n "''${dev_failures:-}" ]; then
              echo "FAILED:''${dev_failures}" >&2
              exit 1
            fi
          '';
        in
        {
          setup = {
            description = "(network) go mod download every Go module, npm ci every Node project";
            text = ''
              # npm ci writes node_modules INTO the tree, so a snapshot root is
              # a dead end -- say so once here instead of failing four times.
              dev_require_checkout
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
            description = "go build every service module, then the Next.js frontend and tiktok-listener";
            text = ''
              ${eachGoModule ''go build ./... "$@"''}
              ${nodeLeg}
              if node_bin frontend next; then
                printf '==> next build frontend\n'
                ( cd "$REPO_ROOT/frontend" && npm run build ) || note_failure frontend
              fi
              if node_bin services/tiktok-listener tsc; then
                printf '==> tsc services/tiktok-listener\n'
                ( cd "$REPO_ROOT/services/tiktok-listener" && npm run build ) \
                  || note_failure services/tiktok-listener
              fi
              ${devReport}
            '';
          };

          test = {
            description = "go test -short every module (some suites need Docker), then the Node suites";
            text = ''
              # `-short` is what the PR matrix runs. It does NOT make this
              # hermetic: several services start a real Postgres through
              # testcontainers even under -short, and those suites fail with
              # "failed to create Docker provider" when no daemon is reachable --
              # verified on a host without one, where five modules failed that
              # way and the rest passed. nix cannot supply a daemon, which is
              # exactly why the failures are collected and named at the end
              # instead of aborting the walk.
              ${eachGoModule ''go test -short ./... "$@"''}
              ${nodeLeg}
              # `--project unit --run`, never `npm test`: package.json maps
              # `test` to bare `vitest`, which is WATCH mode and would hang
              # forever with no tty, and the `storybook` project drives a real
              # browser (see the note above).
              if node_bin frontend vitest; then
                printf '==> vitest --project unit frontend\n'
                ( cd "$REPO_ROOT/frontend" \
                  && "$REPO_ROOT/frontend/node_modules/.bin/vitest" --project unit --run ) \
                  || note_failure frontend
              fi
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
            description = "go vet every service module, then the frontend a11y ESLint gate";
            text = ''
              ${eachGoModule ''go vet ./... "$@"''}
              ${nodeLeg}
              # The same invocation as the a11y-static job in
              # .github/workflows/frontend-a11y.yml -- a required check on main,
              # ratcheted by eslint.a11y.suppressions.json, so it is expected to
              # be green. The repo's other frontend lint gates are not wired in
              # (see the note in block 4).
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
              # to guess: from the read-only snapshot there is nothing
              # legitimate to write to.
              dev_require_checkout
              if [ "$#" -gt 0 ]; then
                # Explicit arguments are forwarded exactly as given, so they
                # resolve against the caller's cwd the way gofmt's own
                # arguments do. That is the one place cwd SHOULD matter: the
                # caller named the target.
                gofmt -l -w "$@"
              else
                # The default is the repo, never the cwd. It used to be `.`,
                # which meant `nix run /path/to/all-chat#fmt` from anywhere
                # rewrote whatever tree the caller stood in -- verified against
                # a scratch directory outside the repo, whose .go file came
                # back reformatted.
                #
                # These three directories are named individually rather than
                # handing gofmt "$REPO_ROOT", because a walk from the root also
                # descends into frontend/node_modules -- which ships a vendored
                # .go file of its own -- and into whatever build output a
                # contributor is carrying. `git ls-files '*.go'` reaches only
                # services/, shared/ and test/ today.
                #
                # This IS a repo-wide rewrite, and the tree carries ~100
                # gofmt-drifted files that predate this flake, so an
                # argument-less run is a large diff on purpose. -l names every
                # file it rewrote; scope it with a path
                # (`dev-fmt services/api-gateway`) when that diff does not
                # belong in your PR.
                gofmt -l -w "$REPO_ROOT/services" "$REPO_ROOT/shared" "$REPO_ROOT/test"
              fi
            '';
          };

          run = {
            description = "start the Next.js dev server on :3000 (backend: `make frontend-dev`)";
            text = ''
              # `next dev` writes .next/ into the tree and needs an installed
              # node_modules, neither of which a snapshot root can offer.
              dev_require_checkout
              if [ ! -x "$REPO_ROOT/frontend/node_modules/.bin/next" ]; then
                echo "frontend is not installed -- run 'dev-setup' (needs network) first" >&2
                exit 1
              fi
              cd "$REPO_ROOT/frontend"
              # The `--` separator is only added when there is something to
              # forward: `npm run dev --` with nothing after it still hands a
              # bare `--` to `next dev`.
              if [ "$#" -eq 0 ]; then
                npm run dev
              else
                npm run dev -- "$@"
              fi
            '';
          };
        };

      # ======================================================================
      # GENERIC MACHINERY -- byte-identical across the fleet, do not edit
      # ======================================================================

      # Prepend, never assign: a host LD_LIBRARY_PATH may be carrying something
      # the user needs, and clobbering it breaks binaries they launch from here.
      # Linux only -- on darwin the loader variable is DYLD_*, and exporting a
      # Linux-shaped value there is at best useless.
      ldPreamble =
        pkgs:
        lib.optionalString (pkgs.stdenv.hostPlatform.isLinux && nativeLibs pkgs != [ ]) ''
          export LD_LIBRARY_PATH="${lib.makeLibraryPath (nativeLibs pkgs)}''${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
        '';

      # Every command gets $REPO_ROOT, and every verb below anchors on it.
      # `nix run` and `nix develop` both start in whatever directory they were
      # invoked from, so a bare relative path (or a bare `.`) does not mean "this
      # repo", it means "whatever tree the caller happened to stand in".
      #
      # The `|| pwd` this preamble used to end in was exactly that bug, and both
      # halves of it were reproduced, not theorised:
      #   * `nix run /path/to/all-chat#fmt` from an unrelated directory resolved
      #     REPO_ROOT to that directory and `gofmt -l -w .` REWROTE the files in
      #     it. A verb that mutates a tree it was never pointed at is the worst
      #     failure mode in this file.
      #   * `nix run /path/to/all-chat#lint` from the same place walked zero Go
      #     modules (`find services shared` printed "No such file or directory"
      #     into the void), skipped the Node leg and exited 0. The flake-URL form
      #     is precisely what CI and a cold agent use, so the surface that lies
      #     was the surface everything automated hits.
      #
      # Resolution order. A candidate only counts if it carries what the verbs
      # walk -- services/, shared/ and go.work.sum -- so the root can never be
      # some unrelated git repo that merely happens to be the cwd:
      #   1. the git top level of the caller's cwd, so the WORKING TREE wins and
      #      uncommitted edits are what gets linted and formatted;
      #   2. $REPO_ROOT from the environment: the dev shell exports it, so a
      #      wrapper run from /tmp inside that shell still means that checkout,
      #      and `REPO_ROOT=/path/to/all-chat nix run <url>#fmt` is the escape
      #      hatch when there is no checkout at the cwd at all;
      #   3. this flake's own source, which nix has already copied into the
      #      store. It is a complete tracked-file snapshot of the very flake ref
      #      the caller named, so the read-only verbs (build, test, lint) behave
      #      identically from anywhere instead of inspecting nothing. It is also
      #      read-only, which dev_require_checkout below turns into a clear
      #      refusal for the verbs that write.
      # There is no fourth tier: never `pwd`, never `.`.
      #
      # Naming ${self} costs one thing, stated so nobody has to discover it:
      # every wrapper now embeds that store path, so editing ANY tracked file
      # changes the source hash and the six wrappers are rebuilt (shellcheck and
      # all) on the next `nix run` or `nix develop`. Measured at ~4 s for a whole
      # shell entry, against a `nix run` that already copies the tree into the
      # store on every invocation. A verb that silently operates on the wrong
      # tree is not worth four seconds.
      rootPreamble = ''
        dev_is_repo_root() {
          [ -n "''${1:-}" ] && [ -d "$1/services" ] && [ -d "$1/shared" ] && [ -f "$1/go.work.sum" ]
        }
        dev_repo_root() {
          local top
          top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
          if dev_is_repo_root "$top"; then
            printf '%s\n' "$top"
          elif dev_is_repo_root "''${REPO_ROOT:-}"; then
            printf '%s\n' "$REPO_ROOT"
          elif dev_is_repo_root "${self}"; then
            # Loud, because the caller has to know WHICH tree was inspected --
            # a snapshot cannot contain gitignored state (node_modules above
            # all), so the Node legs will report SKIPPED here.
            echo "note: no all-chat checkout at or above $PWD -- using this flake's own source, ${self} (read-only)" >&2
            printf '%s\n' "${self}"
          else
            echo "cannot locate an all-chat checkout: neither $PWD, nor \$REPO_ROOT, nor ${self} has services/, shared/ and go.work.sum" >&2
            return 1
          fi
        }
        # Command substitution in an assignment carries its own exit status, so
        # `set -e` aborts the verb here if nothing resolved. Nothing downstream
        # has to re-check.
        REPO_ROOT="$(dev_repo_root)"
        export REPO_ROOT

        # Called first by every verb that WRITES into the tree (setup, fmt,
        # run). Tier 3 above is a store path, and store paths are read-only by
        # design: refuse with an actionable message instead of burying the
        # caller in "permission denied" from twenty different tools. Only ever
        # invoked from a wrapper's command text, never interactively.
        dev_require_checkout() {
          case "$REPO_ROOT" in
            ${builtins.storeDir}/*)
              echo "this verb writes to the tree, and REPO_ROOT is the read-only flake source ($REPO_ROOT)." >&2
              echo "run it from inside an all-chat checkout, or name one: REPO_ROOT=/path/to/all-chat <this command>" >&2
              exit 1
              ;;
          esac
        }
      '';

      # One derivation per command, reused by both `apps` and the dev shell, so
      # the two can never diverge. `dev-` prefixed because a bare `test` binary
      # earlier on PATH would shadow the POSIX shell builtin and quietly break
      # every script in the repo that uses it.
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
              ${ldPreamble pkgs}
              ${cmd.text}
            '';
          }
        ) (commands pkgs);

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
    in
    {
      # `nix flake show` -- the discovery entrypoint, and deliberately the whole
      # machine-facing contract: every app carries a meta.description, which
      # `nix flake show` prints inline and `nix flake show --json` exposes at
      # .apps.<system>.<name>.description. Pure evaluation, so an agent gets the
      # entire command map in one cheap call without reading a README.
      #
      # Do NOT invent a top-level output for this (`agentManifest`, ...): Nix
      # answers `warning: unknown flake output '<name>'` on every flake check.
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

          # node-gyp addons compile at -O0, where glibc's _FORTIFY_SOURCE
          # becomes a hard error instead of a warning.
          hardeningDisable = [ "fortify" ];

          shellHook = ''
            # mkShell inherits SOURCE_DATE_EPOCH=315532800 (1980-01-01) from
            # stdenv, and any zip or npm pack built in here then dies with "ZIP
            # does not support timestamps before 1980".
            unset SOURCE_DATE_EPOCH

            ${rootPreamble}
            ${ldPreamble pkgs}

            # Nothing networked, nothing stateful and nothing interactive above
            # this line, and nothing below it either. No `npm ci`, no
            # `go mod download`, no `read`, no `exec $SHELL`. Bootstrapping here
            # would make a cold `nix develop -c go build ./...` start
            # downloading before it runs anything, on EVERY invocation, and fail
            # outright in a sandbox. That is what `dev-setup` is for.

            # The banner is interactive-only, and this guard is load-bearing:
            # shellHook output lands on the STDOUT of `nix develop -c <cmd>`, so
            # an unguarded echo corrupts anything parsing it. $- is the only
            # reliable discriminator -- it lacks `i` for `nix develop -c` and has
            # it at an interactive prompt. Do not test $PS1 (unset in both) or
            # $IN_NIX_SHELL (set in both). >&2 is the second layer, for a caller
            # that runs us on a pty.
            case $- in
              *i*) echo "all-chat dev shell -- 'dev-help' for the command map" >&2 ;;
            esac
          '';
        };
      });

      # `nix flake check` -- honest by construction. It realises the toolchain
      # closure (so a typo'd or currently-broken attr fails here) and builds
      # every wrapper, which runs shellcheck over every command text. NEVER add
      # a check that always passes: an agent reads "all checks passed!" as a
      # signal, and a fake check makes `nix flake check` a liar.
      checks = forAllSystems (pkgs: {
        toolchain =
          pkgs.runCommand "toolchain-check"
            {
              nativeBuildInputs = toolchain pkgs ++ lib.attrValues (wrappers pkgs);
            }
            ''
              for verb in ${lib.escapeShellArgs (lib.attrNames (commands pkgs))}; do
                command -v "dev-$verb" > /dev/null || {
                  echo "dev-$verb is not on PATH" >&2
                  exit 1
                }
              done
              touch "$out"
            '';
      });

      # `nix fmt` -- formats the *Nix* in this repo; project code is `dev-fmt`.
      # nixfmt-tree (the treefmt wrapper) rather than bare nixfmt, because bare
      # nixfmt tries to parse every path handed to it and fails on non-Nix files.
      formatter = forAllSystems (pkgs: pkgs.nixfmt-tree);
    };
}
