# Contributing to All-Chat

Thank you for contributing to All-Chat! This guide will help you submit high-quality pull requests.

---

## Quick Start

1. **Fork the repository** on GitHub
2. **Clone your fork**: `git clone https://github.com/YOUR_USERNAME/all-chat.git`
3. **Create a branch**: `git checkout -b feature/your-feature-name`
4. **Make changes** (see [Development Guidelines](#development-guidelines))
5. **Run tests**: `make test`
6. **Submit PR** (see [Pull Request Process](#pull-request-process))

---

## Development Guidelines

### Code Style

**Go**:
- Run `gofmt -w` (or your editor's format-on-save) before committing
- Run `golangci-lint run` if installed (no `make lint` target yet)
- Follow [Standard Go Layout](./docs/adr/0001-standard-go-layout.md)
- Use structured logging (Zap) with context

**TypeScript/React**:
- Use TypeScript (no plain JavaScript)
- Follow React hooks patterns
- Use Next.js App Router conventions

### Testing

**Required**:
- [ ] Unit tests for new functions
- [ ] Integration tests if adding new service
- [ ] All tests pass: `make test`
- [ ] Coverage not decreased

**Test Patterns**:
```go
// Good: Table-driven tests
func TestFeature(t *testing.T) {
    tests := []struct{
        name string
        input string
        want string
    }{
        {"case 1", "input1", "output1"},
        {"case 2", "input2", "output2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Feature(tt.input)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Commit Messages

**Format**:
```
<type>(<optional scope>): <subject>

<body>
```

**Types**: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

**Examples**:
```
feat(auth): add Kick OAuth support to auth-service

Implements Kick OAuth 2.0 authorization flow with token storage
and refresh logic.
```

---

## Filing an Agent Task

Some issues are written to be picked up and implemented by an autonomous agent
(Caterpillar) rather than by a person. Use the **Agent task** issue template
(`.github/ISSUE_TEMPLATE/agent_task.md`); it carries the full contract as
comments, so you do not have to remember it.

The two things that make an issue claimable:

1. **A fenced `agent` block** in the body, listing the repos and the acceptance
   commands. Without it the issue is rejected with a comment, because a task
   with no machine-checkable acceptance criteria can never be marked done.
   ````
   ```agent
   repos:
     - caesarakalaeii/all-chat
   acceptance:
     - "cd services/foo && go test -short ./..."
   ```
   ````
2. **The `agent` label**, added *last*. Intake polls frequently, so an issue
   that carries the label before you have finished writing it can be claimed
   mid-edit.

Labels Caterpillar manages on its own: `agent-wip` (a runner holds the task)
and `needs-human` (the agent parked on a question and is waiting on you).

Two rules worth internalising before writing one:

- **Acceptance commands must fail on today's code.** The supervisor runs them,
  not the agent, and a gate that already passes proves nothing. Verify each one
  locally, in both directions, before you add the label. Equally, never gate on
  something that is *already red* on `main` — that makes the task unsatisfiable
  no matter what the agent does.
- **Verify every file:line and every premise you cite.** A stale line number
  sends the agent to the wrong place. A refuted premise makes it implement a fix
  for a problem that does not exist. If part of a report turns out to be wrong,
  or already fixed, say so in the issue in as many words.

See #724 and #728 for worked examples.

---

## Pull Request Process

### Before Submitting

- [ ] Code compiles: `make build`
- [ ] Tests pass: `make test`
- [ ] Code formatted: `gofmt -w .` (frontend: `cd frontend && npm run format`)
- [ ] Lint passes: `golangci-lint run` (frontend: `cd frontend && npm run lint`)
- [ ] Commit messages follow format
- [ ] Documentation updated (if applicable)

### PR Description Template

```markdown
## Summary
- Brief description of changes (1-3 bullet points)

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Test Plan
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manually tested locally
- [ ] Tested in staging environment

## Related Issues
Closes #123

## Screenshots (if applicable)
[Add screenshots for UI changes]
```

### Review Process

1. **Automated checks** run (CI pipeline)
2. **Code review** by maintainer
3. **Feedback addressed** (update PR branch)
4. **Approval** by 1+ maintainer
5. **Merge** (squash and merge preferred)

---

## Code Review Guidelines

### What Reviewers Look For

**Functionality**:
- [ ] Code does what PR description claims
- [ ] No unintended side effects
- [ ] Edge cases handled

**Quality**:
- [ ] Follows Standard Go Layout conventions
- [ ] Tests cover new code
- [ ] Error handling is appropriate
- [ ] Logging is informative (not verbose)

**Security**:
- [ ] No hardcoded secrets
- [ ] Input validation on all user inputs
- [ ] SQL queries use parameterized statements
- [ ] OAuth flows follow best practices

**Performance**:
- [ ] No obvious performance issues
- [ ] Database queries optimized (indexes used)
- [ ] No N+1 query patterns

### Giving Feedback

**Good feedback**:
- ✅ Specific and actionable
- ✅ Suggests alternative approaches
- ✅ Cites documentation or examples

**Bad feedback**:
- ❌ "This looks wrong" (not specific)
- ❌ "I don't like this" (not actionable)
- ❌ Style nitpicks without rationale

---

## Documentation Standards

### When to Update Documentation

**Always update** when:
- ✅ Adding new service (create service README)
- ✅ Adding new API endpoint (update service README)
- ✅ Changing architecture (create ADR)
- ✅ Adding environment variable (update service README + .env.example)

**Consider updating** when:
- ⚠️ Fixing bug that was hard to diagnose (add to troubleshooting guide)
- ⚠️ Implementing optimization (update performance docs)

### Documentation Format

**Service READMEs**: Follow template in [docs/development/service-template.md](./docs/development/service-template.md)

**Quick Reference Cards**: Task-oriented, <200 lines, step-by-step checklists

**Architecture Docs**: Comprehensive, diagrams, examples, cross-references

**ADRs**: Use MADR template from [docs/adr/README.md](./docs/adr/README.md)

---

## Questions?

- **Technical questions**: File a GitHub issue
- **Documentation unclear**: Submit PR to improve it
- **Security concerns**: Email all.chat.support@gmail.com (private disclosure)

---

## License

By contributing, you agree that your contributions will be licensed under the project's existing license.

---

## Thank You!

Your contributions make All-Chat better for everyone! 🎉
