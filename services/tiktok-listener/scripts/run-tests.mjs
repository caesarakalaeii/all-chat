/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Runs `vitest run` after repairing two environment faults that stop it before a single
 * test is collected. Both are no-ops on a normal machine: each is applied only once the
 * fault has actually been detected.
 *
 * 1. TMPDIR. Vitest derives its scratch space from os.tmpdir() and hardcodes it
 *    (`join(tmpdir(), nanoid())`), so there is no config knob for it. Some build sandboxes
 *    export a TMPDIR they never create (e.g. TMPDIR=/build under a read-only /), and every
 *    suite then fails with EACCES/ENOENT.
 *
 * 2. The rolldown native binding (vitest 4 bundles rolldown). npm picks the binding from
 *    the *OS* libc: on Alpine, `ldd` reports musl, so it installs the musl build. When
 *    `node` itself is glibc-linked (e.g. supplied by Nix on an Alpine host), loading that
 *    binding dies with "invalid ELF header". The gnu binding is already an optional
 *    dependency in the lockfile at the same version, so we fetch that package into
 *    node_modules and point napi-rs at it via NAPI_RS_NATIVE_LIBRARY_PATH. (`npm install`
 *    refuses it outright with "notsup Actual libc: musl"; `npm pack` bypasses that gate
 *    without touching the lockfile.)
 */

import { spawnSync } from 'node:child_process';
import { accessSync, constants, existsSync, mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

function isUsable(dir) {
  try {
    mkdirSync(dir, { recursive: true });
    accessSync(dir, constants.W_OK);
    return true;
  } catch {
    return false;
  }
}

/**
 * Returns the libc this Node binary is actually linked against, which is not necessarily
 * the one the surrounding OS advertises. Node reports a glibc version only for glibc builds.
 */
function runtimeLibc() {
  try {
    return process.report.getReport().header.glibcVersionRuntime ? 'glibc' : 'musl';
  } catch {
    return null;
  }
}

const env = { ...process.env };
const configured = tmpdir();

if (!isUsable(configured)) {
  // Keep the fallback inside the project so it is writable wherever the repo itself is.
  const fallback = mkdtempSync(join(import.meta.dirname, '..', 'node_modules', '.tmp-vitest-'));
  env.TMPDIR = fallback;
  env.TEMP = fallback;
  env.TMP = fallback;
  console.warn(`[test] TMPDIR "${configured}" is not writable; using ${fallback} instead.`);
}

// Only intervene when the binding npm chose cannot serve this runtime.
if (!env.NAPI_RS_NATIVE_LIBRARY_PATH && runtimeLibc() === 'glibc') {
  const modules = join(import.meta.dirname, '..', 'node_modules');
  const destination = join(modules, '@rolldown', 'binding-linux-x64-gnu');
  const gnuBinding = join(destination, 'rolldown-binding.linux-x64-gnu.node');
  const muslInstalled = existsSync(join(modules, '@rolldown', 'binding-linux-x64-musl'));

  if (muslInstalled && !existsSync(gnuBinding)) {
    // Match the rolldown that is actually installed, so the ABI lines up.
    const { version } = createRequire(import.meta.url)('rolldown/package.json');
    const spec = `@rolldown/binding-linux-x64-gnu@${version}`;
    console.warn(`[test] node is glibc-linked but the musl rolldown binding was installed; fetching ${spec}.`);

    const scratch = mkdtempSync(join(modules, '.rolldown-gnu-'));
    const packed = spawnSync('npm', ['pack', spec, '--pack-destination', scratch], {
      stdio: 'inherit',
      shell: process.platform === 'win32'
    });

    if (packed.status === 0) {
      mkdirSync(destination, { recursive: true });
      spawnSync(
        'tar',
        [
          'xzf',
          join(scratch, `rolldown-binding-linux-x64-gnu-${version}.tgz`),
          '-C',
          destination,
          '--strip-components=1'
        ],
        { stdio: 'inherit' }
      );
    }
    rmSync(scratch, { recursive: true, force: true });
  }

  if (muslInstalled && existsSync(gnuBinding)) {
    env.NAPI_RS_NATIVE_LIBRARY_PATH = gnuBinding;
  }
}

const result = spawnSync('vitest', ['run', ...process.argv.slice(2)], {
  stdio: 'inherit',
  env,
  shell: process.platform === 'win32'
});

process.exit(result.status ?? 1);
