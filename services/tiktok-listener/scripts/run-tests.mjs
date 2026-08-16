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
 * Runs `vitest run` after making sure the temp directory it is pointed at actually exists.
 *
 * Vitest derives its scratch space from os.tmpdir() — i.e. TMPDIR — and hardcodes it
 * (`join(tmpdir(), nanoid())`), so there is no config knob for it. Some build sandboxes
 * export a TMPDIR they never create (e.g. TMPDIR=/build under a read-only /), and every
 * suite then fails with EACCES/ENOENT before a single test is collected.
 *
 * On a normal machine TMPDIR is fine and this is a no-op; we only substitute a writable
 * fallback when the configured directory cannot be used.
 */

import { spawnSync } from 'node:child_process';
import { accessSync, constants, mkdirSync, mkdtempSync } from 'node:fs';
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

const result = spawnSync('vitest', ['run', ...process.argv.slice(2)], {
  stdio: 'inherit',
  env,
  shell: process.platform === 'win32'
});

process.exit(result.status ?? 1);
