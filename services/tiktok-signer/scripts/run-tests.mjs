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
 * 2. The rolldown native binding (vitest 4 bundles rolldown), repaired by
 *    ./ensure-rolldown-binding.mjs — which `npm ci` already runs as postinstall, so this is
 *    normally a second no-op. See that file for what the fault is and why it is repaired
 *    there rather than here: `npx vitest run <file>` has to work on its own too.
 */

import { spawnSync } from 'node:child_process';
import { accessSync, constants, mkdirSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { ensureRolldownBinding } from './ensure-rolldown-binding.mjs';

function isUsable(dir) {
  try {
    mkdirSync(dir, { recursive: true });
    accessSync(dir, constants.W_OK);
    return true;
  } catch {
    return false;
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
if (!env.NAPI_RS_NATIVE_LIBRARY_PATH) {
  let repaired = null;
  try {
    repaired = ensureRolldownBinding();
  } catch (error) {
    console.warn(`[test] rolldown binding repair skipped: ${error.message}`);
  }

  if (repaired) {
    env.NAPI_RS_NATIVE_LIBRARY_PATH = repaired;
  }
}

const result = spawnSync('vitest', ['run', ...process.argv.slice(2)], {
  stdio: 'inherit',
  env,
  shell: process.platform === 'win32'
});

process.exit(result.status ?? 1);
