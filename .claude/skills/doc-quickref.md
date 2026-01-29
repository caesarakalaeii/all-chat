# doc-quickref

Generate quick reference card for a common task.

---

## Usage

```
/doc-quickref <task-name>
```

**Examples**:
- `/doc-quickref deploy-to-staging`
- `/doc-quickref backup-database`
- `/doc-quickref rotate-secrets`
- `/doc-quickref add-metric`

---

## What This Skill Does

1. **Reads existing quick refs** to understand format and style
2. **Interviews user** about task requirements
3. **Generates quick reference card** with:
   - Time estimate and difficulty rating
   - Files to create/modify (explicit paths)
   - Step-by-step checklist
   - Testing commands with expected outputs
   - Common issues with solutions
   - Related documentation links

---

## Instructions for Claude

When this skill is invoked with `/doc-quickref <task-name>`:

### Step 1: Read Existing Quick Refs

**Read 2-3 existing quick refs** to understand format:
- `docs/llm-guides/QUICK-REF-ADD-PLATFORM.md` - Complex multi-step task
- `docs/llm-guides/QUICK-REF-ADD-ENDPOINT.md` - Simple task
- `docs/llm-guides/QUICK-REF-DEBUG-QUOTA.md` - Diagnostic task

**Identify patterns**:
- Time estimate format: "X-Y hours | Difficulty: ⭐⭐ Easy/Moderate/Advanced"
- Goal: One sentence
- Prerequisites: Checkbox list
- Steps: Numbered with subsections
- Common issues: Symptom → Solution format
- Validation: Final checklist

---

### Step 2: Interview User

**Question 1**: "What does this task accomplish?" (Goal)
- One-sentence description

**Question 2**: "How difficult is this task?" (Difficulty)
- ⭐ Easy (30 min - 1 hour)
- ⭐⭐ Moderate (1-3 hours)
- ⭐⭐⭐ Advanced (3+ hours)

**Question 3**: "What are the prerequisites?" (Checklist)
- Required tools, credentials, knowledge
- Services that must be running

**Question 4**: "What are the main steps?" (Step-by-Step)
- Ask user to outline 3-8 major steps
- For each step, collect:
  - Files to create/modify
  - Commands to run
  - What to verify

**Question 5**: "What are common issues?" (Troubleshooting)
- 2-4 things that commonly go wrong
- For each: symptom and solution

---

### Step 3: Generate Quick Reference

**File**: `docs/llm-guides/QUICK-REF-{TASK}.md`

**Slug**: Convert task name to UPPERCASE-WITH-DASHES
- "deploy to staging" → "DEPLOY-TO-STAGING"

**Structure**:

```markdown
# Quick Reference: {Task Title}

**Time Estimate**: {X-Y hours} | **Difficulty**: {⭐⭐⭐ Rating}

**Goal**: {Goal from interview}

---

## Prerequisites

{Checklist from interview}
- [ ] Prerequisite 1
- [ ] Prerequisite 2

---

## Step 1: {Step Name}

{Step description}

**Files to create/modify**:
- `{file-path}` - {What changes}

**Commands**:
\`\`\`bash
# {Description}
command to run

# Expected output:
# what success looks like
\`\`\`

---

## Step 2: {Step Name}

{Similar structure}

---

{Repeat for all steps}

---

## Validation Checklist

{Final verification steps}
- [ ] Validation 1
- [ ] Validation 2

---

## Common Issues & Solutions

### Issue 1: {Issue Name}

**Symptom**: {What user sees}

**Solution**:
1. Step 1
2. Step 2

**File**: `{file-path}:{line}`

---

## Related Documentation

- [{Related Doc}](../{path}) - {Description}
- [{Another Doc}](../{path}) - {Description}

---

## Success Criteria

✅ Task complete when:
1. Criterion 1
2. Criterion 2
```

---

### Step 4: Add Cross-References

**Update CLAUDE.md** if this is a common task:

```markdown
| {Task} | [QUICK-REF-{TASK}.md](./docs/llm-guides/QUICK-REF-{TASK}.md) | ~{N} lines |
```

**Update docs/llm-guides/NAVIGATION.md** if relevant.

---

## Example Invocation

**User**: `/doc-quickref deploy-to-staging`

**Claude**:
1. Reads existing quick refs for format
2. Interviews user:
   - Goal: "Deploy All-Chat to staging Kubernetes cluster"
   - Difficulty: ⭐⭐ Moderate
   - Prerequisites: kubectl configured, staging cluster access, Docker images built
   - Steps:
     1. Build and push Docker images
     2. Apply Kubernetes manifests
     3. Run database migrations
     4. Verify services healthy
   - Common issues: Image pull errors, migration failures
3. Generates `docs/llm-guides/QUICK-REF-DEPLOY-TO-STAGING.md`
4. Includes specific kubectl commands with expected outputs
5. Adds validation checklist (all pods ready, health checks pass)

**Output**: "Created QUICK-REF-DEPLOY-TO-STAGING.md (120 lines)"

---

## Success Criteria

✅ Skill complete when:
1. Quick reference created in docs/llm-guides/
2. Time estimate and difficulty included
3. Prerequisites checklist provided
4. Step-by-step instructions with explicit file paths
5. Commands include expected outputs
6. Common issues documented with solutions
7. Validation checklist included
8. Related documentation linked

---

## Related Documentation

- **Examples**: [docs/llm-guides/](../../docs/llm-guides/) - All existing quick refs
- **Template**: Follow pattern from QUICK-REF-ADD-PLATFORM.md
