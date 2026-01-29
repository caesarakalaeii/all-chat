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
- Run `make fmt` before committing (gofmt)
- Run `make lint` to check with golangci-lint
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
<type>: <subject>

<body>

Co-Authored-By: Claude Sonnet 4.5 (1M context) <noreply@anthropic.com>
```

**Types**: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

**Examples**:
```
feat: Add Kick OAuth support to auth-service

Implements Kick OAuth 2.0 authorization flow with token storage
and refresh logic.

Co-Authored-By: Claude Sonnet 4.5 (1M context) <noreply@anthropic.com>
```

---

## Pull Request Process

### Before Submitting

- [ ] Code compiles: `make build`
- [ ] Tests pass: `make test`
- [ ] Code formatted: `make fmt`
- [ ] Lint passes: `make lint`
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
- **Security concerns**: Email security@example.com (private disclosure)

---

## License

By contributing, you agree that your contributions will be licensed under the project's existing license.

---

## Thank You!

Your contributions make All-Chat better for everyone! 🎉
