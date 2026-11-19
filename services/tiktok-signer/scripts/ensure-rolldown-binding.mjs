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
 * Repairs the rolldown native binding npm picked, when it cannot serve this Node binary.
 *
 * vitest 4 bundles rolldown, whose binding is a native module. npm picks it from the *OS*
 * libc: on Alpine `ldd` reports musl, so it installs the musl build. When `node` itself is
 * glibc-linked (e.g. supplied by Nix on an Alpine host) loading that binding dies with
 * "invalid ELF header", and vitest fails at startup before collecting a single test —
 * including when it is invoked directly as `npx vitest run`, which is how CI gates and
 * editors run single files.
 *
 * The gnu binding is already an optional dependency in the lockfile at the same version, so
 * this fetches that package into node_modules (`npm install` refuses it outright with
 * "notsup Actual libc: musl"; `npm pack` bypasses that gate without touching the lockfile)
 * and drops the .node file at *both* the gnu and the musl path. The musl copy is what makes
 * a bare `npx vitest` work: rolldown's loader picks its candidate from the OS libc too, so
 * it only ever looks at the musl path unless NAPI_RS_NATIVE_LIBRARY_PATH points elsewhere.
 *
 * Everything here is a no-op on a normal machine: it runs only once the fault has actually
 * been detected, and it never fails the install — a broken repair must not stop `npm ci`.
 */

import { spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const MODULES = join(import.meta.dirname, '..', 'node_modules');
const GNU_DIR = join(MODULES, '@rolldown', 'binding-linux-x64-gnu');
const GNU_BINDING = join(GNU_DIR, 'rolldown-binding.linux-x64-gnu.node');
const MUSL_DIR = join(MODULES, '@rolldown', 'binding-linux-x64-musl');
const MUSL_BINDING = join(MUSL_DIR, 'rolldown-binding.linux-x64-musl.node');

/**
 * Returns the libc this Node binary is actually linked against, which is not necessarily
 * the one the surrounding OS advertises. Node reports a glibc version only for glibc builds.
 */
export function runtimeLibc() {
  try {
    return process.report.getReport().header.glibcVersionRuntime ? 'glibc' : 'musl';
  } catch {
    return null;
  }
}

/**
 * Makes a loadable rolldown binding available, and returns its path — or null when nothing
 * needed doing (the common case) or when the repair could not be completed.
 */
export function ensureRolldownBinding() {
  // Only the glibc-node-on-musl-OS combination is broken; anything else npm got right.
  if (process.platform !== 'linux' || process.arch !== 'x64' || runtimeLibc() !== 'glibc') {
    return null;
  }
  if (!existsSync(MUSL_DIR)) {
    return null;
  }

  if (!existsSync(GNU_BINDING)) {
    let version;
    try {
      // Match the rolldown that is actually installed, so the ABI lines up.
      ({ version } = createRequire(import.meta.url)('rolldown/package.json'));
    } catch {
      return null;
    }

    const spec = `@rolldown/binding-linux-x64-gnu@${version}`;
    console.warn(
      `[rolldown] node is glibc-linked but the musl binding was installed; fetching ${spec}.`
    );

    const scratch = mkdtempSync(join(MODULES, '.rolldown-gnu-'));
    try {
      const packed = spawnSync('npm', ['pack', spec, '--pack-destination', scratch], {
        stdio: 'inherit',
        shell: process.platform === 'win32'
      });

      if (packed.status !== 0) {
        return null;
      }

      mkdirSync(GNU_DIR, { recursive: true });
      const extracted = spawnSync(
        'tar',
        [
          'xzf',
          join(scratch, `rolldown-binding-linux-x64-gnu-${version}.tgz`),
          '-C',
          GNU_DIR,
          '--strip-components=1'
        ],
        { stdio: 'inherit' }
      );

      if (extracted.status !== 0 || !existsSync(GNU_BINDING)) {
        return null;
      }
    } finally {
      rmSync(scratch, { recursive: true, force: true });
    }
  }

  // rolldown's own loader resolves the musl path on this OS, so park a working binding there
  // too: that is what lets `npx vitest run` succeed with no environment variable set.
  try {
    copyFileSync(GNU_BINDING, MUSL_BINDING);
  } catch {
    // The NAPI_RS_NATIVE_LIBRARY_PATH route below still works without it.
  }

  return GNU_BINDING;
}

// `npm ci` runs this as postinstall: never let a failed repair fail the install.
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    ensureRolldownBinding();
  } catch (error) {
    console.warn(`[rolldown] binding repair skipped: ${error.message}`);
  }
}
