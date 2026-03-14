---
phase: 26
slug: enforcement-quality-gates
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-14
---

# Phase 26 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | ESLint CLI, TypeScript CLI, Storybook Vitest, GitHub Actions |
| **Config file** | `frontend/eslint.config.mjs`, `frontend/.prettierrc`, `frontend/.husky/pre-commit` |
| **Quick run command** | `cd frontend && npx eslint . --max-warnings 0` |
| **Full suite command** | `cd frontend && npx eslint . --max-warnings 0 && npx tsc --noEmit && npx storybook test --ci` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npx eslint . --max-warnings 0`
- **After every plan wave:** Run full suite (ESLint + tsc + Storybook tests)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------|-------------------|--------|
| ESLint config | 01 | 1 | ENFORCE-01, ENFORCE-03 | lint | `cd frontend && npx eslint . --max-warnings 0` | ⬜ pending |
| Prettier config | 01 | 1 | ENFORCE-02 | format | `cd frontend && npx prettier --check .` | ⬜ pending |
| Husky pre-commit | 02 | 1 | ENFORCE-04 | manual | Verify hook blocks bad commit | ⬜ pending |
| CI workflow | 03 | 2 | ENFORCE-05 | ci | GitHub Actions run passes | ⬜ pending |
| Chromatic CI | 03 | 2 | ENFORCE-06 | ci | Chromatic step exits 0 on first run | ⬜ pending |
| Bundle analyzer | 04 | 2 | ENFORCE-09 | build | `ANALYZE=true npm run build` generates report | ⬜ pending |
| Performance test | 05 | 3 | ENFORCE-08 | vitest | `cd frontend && npx storybook test --ci` | ⬜ pending |
| Migration guide | 06 | 3 | ENFORCE-07 | manual | File exists, content accurate | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements — this phase IS the infrastructure setup, no test scaffolding needed before implementation.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Pre-commit hook blocks bad commit | ENFORCE-04 | Requires local git commit attempt | Temporarily add `className="gray-500"` to a file, attempt git commit, verify hook rejects it |
| Chromatic baseline established | ENFORCE-06 | Requires Chromatic project setup + first-run approval | Run `npx chromatic --project-token=<token>`, verify stories uploaded |
| Marketplace migration guide accurate | ENFORCE-07 | Content review required | Read MARKETPLACE_MIGRATION_GUIDE.md, verify class names match current events.css |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
