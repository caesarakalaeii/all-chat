#!/usr/bin/env node
/**
 * Make `npx vitest` and `npx tsc` work from the repo root by pointing the root
 * node_modules at the frontend's install.
 *
 * The frontend owns the only real JS toolchain in this repo, and the documented
 * way to install it is `npm ci --prefix frontend`, which populates
 * frontend/node_modules and deliberately leaves the repo root untouched. But
 * several tooling and CI invocations run from the repo root, e.g.
 *
 *   npx vitest --project unit --run frontend/src/hooks/__tests__/...
 *
 * With no root node_modules, npx cannot resolve vitest locally, so it downloads
 * an unrelated copy into ~/.npm/_npx. That copy has no native bindings for this
 * host and dies before a single test runs with "Cannot find native binding".
 *
 * Rather than duplicating a second full toolchain at the root -- which would
 * double install time and let the two versions drift -- this script symlinks the
 * few entries the root entry points need into node_modules/, so the root
 * resolves the exact same packages the frontend already installed:
 *
 *   .bin/vitest, .bin/tsc  what npx executes
 *   vitest, vite           so root vitest.config.ts can import 'vitest/config'
 *   jsdom                  the environment the hook tests request, loaded by
 *                          whichever vitest package is running
 *
 * It runs from the frontend's postinstall hook, which npm executes even for
 * `npm ci --prefix frontend`, so the root is provisioned by the same install
 * command the docs already tell people to run.
 *
 * Symlinks, not copies, so there is exactly one installed version on disk and
 * `npm ci` in frontend/ cannot leave the root pointing at something stale.
 * node_modules/ is gitignored, so none of this is committed.
 *
 * Best-effort by design: any failure is reported and swallowed, because
 * provisioning a convenience entry point must never be able to fail `npm ci`.
 */
import { existsSync, mkdirSync, lstatSync, unlinkSync, rmSync, symlinkSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repoRoot = resolve(frontendDir, '..')
const frontendModules = join(frontendDir, 'node_modules')
const rootModules = join(repoRoot, 'node_modules')

// Packages the root needs to resolve, and the bin stubs npx looks for.
const packages = ['vitest', 'vite', 'jsdom']
const binaries = ['vitest', 'tsc']

/**
 * Point `linkPath` at `target` using a relative symlink, replacing whatever is
 * already there. Skips a real (non-symlink) directory: that means someone ran a
 * genuine `npm install` at the root, and their install wins over ours.
 */
function link(linkPath, target) {
  if (existsSync(linkPath) || isBrokenSymlink(linkPath)) {
    const existing = lstatSync(linkPath)
    if (!existing.isSymbolicLink()) {
      if (existing.isDirectory()) return false
      unlinkSync(linkPath)
    } else {
      rmSync(linkPath, { recursive: true, force: true })
    }
  }
  mkdirSync(dirname(linkPath), { recursive: true })
  symlinkSync(relative(dirname(linkPath), target), linkPath)
  return true
}

/** existsSync follows symlinks, so a dangling one reads as absent. */
function isBrokenSymlink(path) {
  try {
    lstatSync(path)
    return true
  } catch {
    return false
  }
}

// Nothing to point at means the frontend install has not finished (or this is a
// production image that never installs devDependencies); leave the root alone.
if (existsSync(frontendModules)) {
  const linked = []
  for (const name of packages) {
    const target = join(frontendModules, name)
    if (!existsSync(target)) continue
    try {
      if (link(join(rootModules, name), target)) linked.push(name)
    } catch (err) {
      console.warn(`[link-root-toolchain] could not link ${name}: ${err.message}`)
    }
  }
  for (const name of binaries) {
    const target = join(frontendModules, '.bin', name)
    if (!existsSync(target)) continue
    try {
      link(join(rootModules, '.bin', name), target)
    } catch (err) {
      console.warn(`[link-root-toolchain] could not link .bin/${name}: ${err.message}`)
    }
  }
  if (linked.length > 0) {
    console.log(`[link-root-toolchain] root entry points now resolve ${linked.join(', ')}`)
  }
}
