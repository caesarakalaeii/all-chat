# doc-adr

Create Architecture Decision Record (ADR) using MADR template.

---

## Usage

```
/doc-adr <decision-title>
```

**Examples**:
- `/doc-adr Message Queue Selection`
- `/doc-adr Frontend Framework Choice`
- `/doc-adr Database Sharding Strategy`

---

## What This Skill Does

1. **Reads ADR index** to determine next ADR number
2. **Interviews user** about the decision using AskUserQuestion
3. **Generates complete ADR** using MADR template
4. **Creates ADR file** in docs/adr/
5. **Updates ADR index** (docs/adr/README.md)

---

## Instructions for Claude

When this skill is invoked with `/doc-adr <decision-title>`:

### Step 1: Determine Next ADR Number

```bash
# Find highest ADR number
ls -1 docs/adr/*.md | grep -E "^docs/adr/[0-9]" | tail -1

# Extract number and increment
# Example: 0006-youtube-quota-tracking.md → next is 0007
```

**Format**: 4-digit number with leading zeros (e.g., `0007`)

---

### Step 2: Interview User (Use AskUserQuestion)

**Question 1**: "What problem does this decision solve?"
- Collect 2-3 sentence problem statement

**Question 2**: "What are the decision drivers?" (multiSelect: true)
- Performance
- Cost
- Team experience
- Operational complexity
- LLM code generation accuracy
- Scalability
- Other (allow custom input)

**Question 3**: "What options did you consider?"
- Ask user to list 2-4 options
- For each option, ask: "What are the pros and cons?"

**Question 4**: "Which option did you choose and why?"
- Collect chosen option
- Collect rationale (2-3 sentences)

**Question 5**: "What are the consequences?"
- Positive consequences (benefits)
- Negative consequences (trade-offs, technical debt)

**Question 6**: "What files are affected?"
- Collect specific file paths
- Migration steps (if applicable)
- Configuration changes (environment variables, etc.)

---

### Step 3: Read MADR Template

Read `docs/adr/README.md` for MADR template structure.

---

### Step 4: Generate ADR

**File**: `docs/adr/<NNNN>-<slug>.md`

**Slug**: Convert title to lowercase-with-dashes
- "Message Queue Selection" → "message-queue-selection"

**Content** (following MADR template):

```markdown
# ADR-<NNNN>: {Title}

**Date**: {Today's date}
**Status**: ✅ Accepted
**Deciders**: {From interview or default to "Development Team"}

## Context and Problem Statement

{Problem statement from interview}

## Decision Drivers

{List from interview}
- {Driver 1}
- {Driver 2}

## Considered Options

{For each option from interview}

1. **{Option A}** - {Description}
   - ✅ Pros: {Pros from interview}
   - ❌ Cons: {Cons from interview}

2. **{Option B}** - {Description}
   - ✅ Pros: ...
   - ❌ Cons: ...

## Decision Outcome

**Chosen**: {Chosen option from interview}

**Rationale**: {Rationale from interview}

## Consequences

### Positive
{Positive consequences from interview}

### Negative
{Negative consequences from interview}

## Implementation

- **Files**: {File paths from interview}
- **Migration**: {Migration steps if applicable}
- **Configuration**: {Environment variables, settings}
- **Timeline**: {Today's date or specified date}

## Related Decisions

{Suggest related ADRs based on topic}
- Links to other ADRs
- Links to architecture docs
```

---

### Step 5: Update ADR Index

**File**: `docs/adr/README.md`

Add new entry to "ADR Index" section:

```markdown
---

### ADR-<NNNN>: {Title}

**Status**: ✅ Accepted
**Date**: {Date}
**Problem**: {One-sentence problem}
**Decision**: {One-sentence decision}
**Impact**: {One-sentence impact}
**→ Read**: [<NNNN>-{slug}.md](./<NNNN>-{slug}.md)
```

---

### Step 6: Suggest Cross-References

**Check** which architecture docs should link to this ADR:
- 00-OVERVIEW.md (if fundamental decision)
- 01-DATA-FLOW.md (if message flow related)
- 02-DEPLOYMENT.md (if deployment related)
- 03-SCALING.md (if performance related)
- 04-OBSERVABILITY.md (if monitoring related)
- 05-SECURITY.md (if security related)

**Suggest updates**:
```
This ADR should be referenced from:
- docs/architecture/{doc}.md (section: {section name})

Would you like me to update these cross-references?
```

---

## Example Invocation

**User**: `/doc-adr Message Queue Selection`

**Claude**:
1. Determines next ADR number: 0007
2. Interviews user:
   - Problem: "Need durable message queue with low latency"
   - Drivers: Performance, Operational complexity, LLM accuracy
   - Options: Kafka, NATS, Redis Streams, RabbitMQ
   - Chosen: Redis Streams
   - Rationale: "Simpler than Kafka, sufficient for Phase 1, LLM-friendly"
3. Generates `docs/adr/0007-message-queue-selection.md`
4. Updates ADR index in docs/adr/README.md
5. Suggests cross-reference in 01-DATA-FLOW.md

**Output**: "Created ADR-0007: Message Queue Selection (380 lines). Updated ADR index."

---

## Success Criteria

✅ Skill complete when:
1. ADR file created with correct number (0001-9999 format)
2. All MADR template sections filled in
3. Options include pros/cons analysis
4. Decision rationale is clear and specific
5. Implementation details include file paths
6. ADR index updated with new entry
7. Cross-reference suggestions provided

---

## Related Documentation

- **ADR Index**: [docs/adr/README.md](../../docs/adr/README.md) - MADR template
- **Examples**: All existing ADRs (0001-0006)
- **MADR Format**: https://adr.github.io/madr/
