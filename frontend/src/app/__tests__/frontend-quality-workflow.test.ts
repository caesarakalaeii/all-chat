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
 * Contract for `.github/workflows/frontend-quality.yml`.
 *
 * This workflow was `workflow_dispatch`-only for as long as the frontend
 * carried lint debt. Now that `eslint . --max-warnings 0` and
 * `prettier --check .` are clean it gates pull requests, and three properties
 * of that arrangement are load-bearing enough that a silent regression would
 * cost a repo-wide outage rather than a red check:
 *
 *  1. NO top-level `paths:` filter. A workflow gated that way is skipped
 *     ENTIRELY on a backend-only PR, so its required contexts never report and
 *     the PR sits permanently "Expected"/blocked — see ADR-0033 / PR #577. The
 *     `changes` + per-job `if:` shim gives present-but-skipped instead, which
 *     branch protection treats as passing. `frontend-a11y.yml` already carries
 *     the same shim for the same reason.
 *
 *  2. EVERY job sets `timeout-minutes`. Without it a job inherits GitHub's
 *     6-hour default, and on 2026-08-19 two jobs hung ~4 hours on PR #729. A
 *     hang is worse than a failure: the context never reports, so the PR blocks
 *     with no signal at all.
 *
 *  3. Chromatic NEVER runs on `pull_request`. It needs
 *     `secrets.CHROMATIC_PROJECT_TOKEN`, whose existence cannot be verified
 *     from the repo. If that secret is absent, an unguarded Chromatic step
 *     fails every PR and blocks the repo.
 *
 * These are asserted against the parsed YAML rather than by grep so that a
 * restructure (renaming a job, moving a step) still has to keep the guarantees
 * rather than merely keep the strings.
 */

import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import { parse } from 'yaml'

interface WorkflowStep {
  name?: string
  uses?: string
  run?: string
  with?: Record<string, unknown>
}

interface WorkflowJob {
  name?: string
  needs?: string | string[]
  if?: string
  'timeout-minutes'?: number
  steps?: WorkflowStep[]
}

interface Workflow {
  // `on` is the YAML 1.1 boolean `true` once parsed unless quoted; the `yaml`
  // package used here is YAML 1.2, where `on` stays a string. Accept both so
  // the test pins the workflow, not the parser's version dialect.
  on?: Record<string, unknown>
  true?: Record<string, unknown>
  jobs: Record<string, WorkflowJob>
}

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..')
const workflowPath = path.join(repoRoot, '.github/workflows/frontend-quality.yml')

const workflow = parse(readFileSync(workflowPath, 'utf8')) as Workflow
const triggers = workflow.on ?? workflow.true ?? {}

function job(id: string): WorkflowJob {
  const found = workflow.jobs[id]
  expect(found, `expected a job named "${id}" in frontend-quality.yml`).toBeDefined()
  return found
}

function steps(id: string): WorkflowStep[] {
  return job(id).steps ?? []
}

/** The shell commands a job runs, joined, for substring assertions. */
function runScript(id: string): string {
  return steps(id)
    .map((step) => step.run ?? '')
    .join('\n')
}

describe('frontend-quality workflow triggers', () => {
  it('runs on pull_request so the static gates actually gate', () => {
    expect(Object.keys(triggers)).toContain('pull_request')
  })

  it('keeps workflow_dispatch so the slow jobs stay manually runnable', () => {
    expect(Object.keys(triggers)).toContain('workflow_dispatch')
  })

  it('declares no top-level paths filter, which would skip the whole workflow', () => {
    // ADR-0033 / PR #577: a `paths:`-gated workflow never reports its contexts
    // on a backend-only PR, leaving branch protection blocked forever.
    const pullRequest = (triggers.pull_request ?? {}) as Record<string, unknown>
    expect(pullRequest).not.toHaveProperty('paths')
    expect(pullRequest).not.toHaveProperty('paths-ignore')
  })
})

describe('frontend-quality skip shim', () => {
  it('detects frontend changes with a paths-filter instead of a top-level filter', () => {
    const filter = steps('changes').find((step) => step.uses?.startsWith('dorny/paths-filter'))
    expect(filter, 'expected a dorny/paths-filter step in the changes job').toBeDefined()
    const filters = String(filter?.with?.filters ?? '')
    expect(filters).toContain('frontend/**')
    // The workflow must re-run itself when edited, or a broken edit lands green.
    expect(filters).toContain('.github/workflows/frontend-quality.yml')
  })

  it('needs a full history for the paths-filter to diff against the base', () => {
    const checkout = steps('changes').find((step) => step.uses?.startsWith('actions/checkout'))
    expect(checkout?.with?.['fetch-depth']).toBe(0)
  })

  it('skips the static job when nothing frontend-relevant changed', () => {
    // Present-but-skipped, which branch protection treats as passing.
    expect(job('static').needs).toContain('changes')
    expect(job('static').if).toContain("needs.changes.outputs.frontend == 'true'")
  })
})

describe('frontend-quality static gates', () => {
  it('fails the build on any ESLint warning', () => {
    // --max-warnings 0 is the whole point of the gate: without it a warning
    // ratchet silently reopens.
    expect(runScript('static')).toContain('eslint . --max-warnings 0')
  })

  it('checks formatting and types', () => {
    expect(runScript('static')).toContain('prettier --check .')
    expect(runScript('static')).toContain('tsc --noEmit')
  })

  it('runs no browser or secret-dependent step, so it is safe on every PR', () => {
    const staticSteps = steps('static')
    expect(staticSteps.some((step) => step.uses?.startsWith('chromaui/'))).toBe(false)
    expect(JSON.stringify(staticSteps)).not.toContain('secrets.')
  })
})

describe('frontend-quality slow jobs', () => {
  it('holds the Chromatic job to workflow_dispatch, since its token may not exist', () => {
    const chromatic = Object.entries(workflow.jobs).filter(([, definition]) =>
      (definition.steps ?? []).some((step) => step.uses?.startsWith('chromaui/'))
    )
    expect(chromatic.length, 'expected exactly one job to run Chromatic').toBe(1)
    for (const [id, definition] of chromatic) {
      expect(definition.if, `job "${id}" must be dispatch-gated`).toContain(
        "github.event_name == 'workflow_dispatch'"
      )
    }
  })

  it('keeps the build and bundle analysis off pull requests too', () => {
    // These are slow rather than dangerous, but they were never part of the
    // "fast static gates" this workflow was armed for.
    expect(job('quality').if).toContain("github.event_name == 'workflow_dispatch'")
  })
})

describe('frontend-quality timeouts', () => {
  it('bounds every job, because a hang blocks a PR harder than a failure', () => {
    // 2026-08-19: two jobs inherited the 6-hour default and hung ~4 hours on
    // PR #729, reporting nothing at all.
    for (const [id, definition] of Object.entries(workflow.jobs)) {
      expect(definition['timeout-minutes'], `job "${id}" must set timeout-minutes`).toBeTypeOf(
        'number'
      )
    }
  })
})
