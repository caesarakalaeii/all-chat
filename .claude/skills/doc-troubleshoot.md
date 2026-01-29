# doc-troubleshoot

Generate troubleshooting guide with decision tree and diagnostic steps.

---

## Usage

```
/doc-troubleshoot <issue-category>
```

**Categories**:
- `build` - Compilation, Docker build, startup errors
- `connection` - PostgreSQL, Redis connection issues
- `quota` - YouTube API quota issues
- `websocket` - WebSocket connection/disconnection issues
- `redis` - Redis Streams, Pub/Sub issues
- `platform-{name}` - Platform-specific issues (e.g., platform-twitch, platform-kick)

**Examples**:
- `/doc-troubleshoot build`
- `/doc-troubleshoot platform-twitch`
- `/doc-troubleshoot redis`

---

## What This Skill Does

1. **Reads decision tree** for overall triage context
2. **Reads relevant service READMEs** for common issues
3. **Analyzes logs** (if provided by user) to identify patterns
4. **Generates troubleshooting guide** with:
   - Symptom descriptions
   - Root cause analysis
   - Step-by-step diagnostic commands (with expected outputs)
   - Quick fixes and long-term solutions
   - File references for code fixes

---

## Instructions for Claude

When this skill is invoked with `/doc-troubleshoot <category>`:

### Step 1: Identify Category

Map category to service/component:
- `build` → All services (Go compilation, Docker)
- `connection` → Database, Redis infrastructure
- `quota` → YouTube Listener
- `websocket` → API Gateway
- `redis` → Message Processor, API Gateway, all publishers
- `platform-twitch` → Twitch Listener
- `platform-youtube` → YouTube Listener
- `platform-kick` → Kick Listener
- `platform-tiktok` → TikTok Listener

---

### Step 2: Read Existing Troubleshooting Docs

**Always read**:
- `docs/troubleshooting/decision-tree.md` - Overall triage context

**Read category-specific** (if exists):
- `docs/troubleshooting/build-errors.md` (for build category)
- `docs/troubleshooting/connection-errors.md` (for connection category)
- `docs/troubleshooting/youtube-quota-exceeded.md` (for quota category)
- `docs/troubleshooting/websocket-disconnects.md` (for websocket category)
- `docs/troubleshooting/twitch-irc-issues.md` (for platform-twitch)

**Read service READMEs**:
- Relevant service README troubleshooting section
- Related service READMEs

---

### Step 3: Analyze Logs (If Provided)

**If user provides logs**, analyze for patterns:
- Error messages (extract error types)
- Frequency of errors (once vs repeated)
- Timestamps (when did it start?)
- Context (what was happening before error?)

**Ask user**:
```
Question: "Do you have error logs to share?"
Options:
1. Yes, I'll paste them
2. No, just general troubleshooting
```

If yes, ask user to paste logs, then analyze.

---

### Step 4: Generate Troubleshooting Guide

**Output file**: `docs/troubleshooting/<category>-issues.md`

**Structure**:

```markdown
# Troubleshooting: {Category Title}

{Brief description of issue category}

---

## Common Issue 1: {Issue Name}

**Symptom**: {What user sees/experiences}

**Diagnosis**:
\`\`\`bash
# Check {something}
command to run

# Expected output:
# what success looks like
\`\`\`

**Cause**: {Root cause explanation}

**Solution**:
1. Step 1 with specific command
2. Step 2 with specific command
3. Step 3 with specific command

**Quick Fix** (if applicable):
\`\`\`bash
# One-liner to fix immediately
command
\`\`\`

**Long-Term Fix** (if applicable):
{Permanent solution to prevent recurrence}

**File**: `{service-name}/{package}/{file}.go:{line}`

---

## Common Issue 2: {Issue Name}

{Same structure as Issue 1}

---

## Diagnostic Commands Reference

{Category-specific commands for quick diagnosis}

---

## Related Documentation

- [decision-tree.md](./decision-tree.md) - High-level triage
- [{Service} README](../../services/{service}/README.md) - Service documentation
- [QUICK-REF-{TOPIC}.md](../llm-guides/QUICK-REF-{TOPIC}.md) - Related quick ref
```

---

### Step 5: Populate Issues

**For each common issue**:

1. **Extract from existing docs**:
   - Service README troubleshooting sections
   - Existing troubleshooting guides
   - Decision tree branches

2. **Add diagnostic commands**:
   - kubectl commands (if Kubernetes-related)
   - Database queries (if data-related)
   - Redis commands (if Redis-related)
   - curl commands (if API-related)

3. **Include expected outputs**:
   - Show what success looks like
   - Show what failure looks like
   - Help user interpret results

4. **Provide file references**:
   - Specific file paths where issue might occur
   - Line numbers if known (from code analysis)

---

## Example Invocation

**User**: `/doc-troubleshoot platform-twitch`

**Claude**:
1. Reads decision-tree.md (Twitch issues section)
2. Reads twitch-listener/README.md (troubleshooting section)
3. Identifies common issues:
   - OAuth token invalid
   - Bot not joining channels
   - Rate limit hit (20 JOIN/10s)
   - Messages not received
4. Generates `docs/troubleshooting/twitch-irc-issues.md` with:
   - Each issue with symptom, diagnosis, solution
   - Diagnostic commands (kubectl logs, curl, redis-cli)
   - File references (irc/client.go, channels/manager.go)
5. Outputs: "Created twitch-irc-issues.md with 4 common issues and diagnostic workflows."

**Generated file**: ~150 lines

---

## Success Criteria

✅ Skill complete when:
1. Troubleshooting guide created in docs/troubleshooting/
2. At least 3 common issues documented
3. Each issue has: symptom, diagnosis, cause, solution
4. Diagnostic commands include expected outputs
5. File references point to specific code locations
6. Links to related documentation included
7. Follows standardized troubleshooting format

---

## Related Documentation

- **Decision Tree**: [docs/troubleshooting/decision-tree.md](../../docs/troubleshooting/decision-tree.md)
- **Examples**: Existing troubleshooting guides (build-errors.md, connection-errors.md, etc.)
- **Service READMEs**: services/*/README.md (troubleshooting sections)
