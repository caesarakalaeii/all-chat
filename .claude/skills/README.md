# All-Chat Documentation Skills

Custom Claude Code skills for automating documentation tasks.

---

## Available Skills

### /doc-service

**Purpose**: Create or update service README using standardized template

**Usage**: `/doc-service <service-name>`

**What it does**:
- Analyzes service code (cmd/main.go, handlers/, go.mod)
- Extracts environment variables, API endpoints, features
- Generates comprehensive README following template
- Updates existing README with missing sections

**Output**: `services/<service-name>/README.md`

**→ Details**: [doc-service.md](./doc-service.md)

---

### /doc-add-platform

**Purpose**: Generate customized platform integration guide

**Usage**: `/doc-add-platform <platform-name>`

**What it does**:
- Determines platform API type (IRC, WebSocket, HTTP)
- Selects appropriate template (Twitch, Kick, YouTube)
- Generates step-by-step guide with platform-specific code
- Includes decision tree and validation checklist

**Output**: `/tmp/claude-<session>/add-<platform>-guide.md`

**→ Details**: [doc-add-platform.md](./doc-add-platform.md)

---

### /doc-troubleshoot

**Purpose**: Generate troubleshooting guide with diagnostic workflows

**Usage**: `/doc-troubleshoot <issue-category>`

**Categories**: build, connection, quota, websocket, redis, platform-{name}

**What it does**:
- Reads existing troubleshooting docs and service READMEs
- Analyzes logs if provided by user
- Generates guide with symptom → diagnosis → solution workflow
- Includes diagnostic commands with expected outputs

**Output**: `docs/troubleshooting/<category>-issues.md`

**→ Details**: [doc-troubleshoot.md](./doc-troubleshoot.md)

---

### /doc-adr

**Purpose**: Create Architecture Decision Record with full context

**Usage**: `/doc-adr <decision-title>`

**What it does**:
- Determines next ADR number
- Interviews user about decision (problem, options, rationale)
- Generates ADR using MADR template
- Updates ADR index
- Suggests cross-references to architecture docs

**Output**: `docs/adr/<NNNN>-<slug>.md`

**→ Details**: [doc-adr.md](./doc-adr.md)

---

### /doc-quickref

**Purpose**: Generate quick reference card for any task

**Usage**: `/doc-quickref <task-name>`

**What it does**:
- Reads existing quick refs to understand format
- Interviews user about task steps
- Generates task-oriented guide (<200 lines)
- Includes time estimate, prerequisites, validation checklist

**Output**: `docs/llm-guides/QUICK-REF-{TASK}.md`

**→ Details**: [doc-quickref.md](./doc-quickref.md)

---

### /doc-migration

**Purpose**: Create and apply database migrations with automation

**Usage**: `/doc-migration <action> [migration-name]`

**Actions**: create, apply local, apply k8s, rollback, verify

**What it does**:
- Creates migration files with correct numbering
- Generates SQL based on user requirements
- Applies migrations locally or to Kubernetes CNPG cluster
- Grants permissions to application user
- Verifies replication across pods
- Creates rollback migrations

**Output**: `migrations/<NNN>_<name>.sql`

**→ Details**: [doc-migration.md](./doc-migration.md)

---

## Benefits of Using Skills

1. **Consistency**: All documentation follows standardized templates
2. **Speed**: Generate docs in minutes instead of hours
3. **Completeness**: Skills enforce checklist of required sections
4. **Quality**: Automated extraction reduces human error
5. **Maintenance**: Easy to update existing docs with new sections

---

## When to Use Each Skill

| Scenario | Skill to Use |
|----------|--------------|
| Added new microservice | `/doc-service <service-name>` |
| Adding new streaming platform | `/doc-add-platform <platform>` |
| Users reporting common issue | `/doc-troubleshoot <category>` |
| Made architectural decision | `/doc-adr <decision-title>` |
| Documenting common procedure | `/doc-quickref <task-name>` |
| Need to create database migration | `/doc-migration create <name>` |
| Apply migrations to Kubernetes | `/doc-migration apply k8s` |

---

## Skill Development Status

| Skill | Status | File |
|-------|--------|------|
| /doc-service | ✅ Complete | doc-service.md |
| /doc-add-platform | ✅ Complete | doc-add-platform.md |
| /doc-troubleshoot | ✅ Complete | doc-troubleshoot.md |
| /doc-adr | ✅ Complete | doc-adr.md |
| /doc-quickref | ✅ Complete | doc-quickref.md |
| /doc-migration | ✅ Complete | doc-migration.md |

---

## Future Enhancements

**Potential additional skills**:
- `/doc-api-spec` - Generate OpenAPI specification from handlers
- `/doc-metrics` - Document Prometheus metrics for a service
- `/doc-runbook` - Generate operational runbook for incident
- `/doc-test` - Generate test suite from service code

---

## Contributing

To create a new documentation skill:

1. Create skill file: `.claude/skills/doc-{name}.md`
2. Follow structure from existing skills
3. Include clear "Instructions for Claude" section
4. Provide examples and success criteria
5. Update this README with new skill

---

## Related Documentation

- **Service Template**: [docs/development/service-template.md](../../docs/development/service-template.md)
- **ADR Template**: [docs/adr/README.md](../../docs/adr/README.md)
- **Quick Ref Examples**: [docs/llm-guides/](../../docs/llm-guides/)
- **Troubleshooting Examples**: [docs/troubleshooting/](../../docs/troubleshooting/)

---

## Summary

**Total Skills**: 6
**Purpose**: Automate documentation generation and maintenance
**Status**: All skills complete and ready to use

**Using these skills ensures consistency, completeness, and adherence to All-Chat documentation standards.**
