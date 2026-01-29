# doc-service

Create or update service README using standardized template.

---

## Usage

```
/doc-service <service-name>
```

**Examples**:
- `/doc-service auth-service`
- `/doc-service message-processor`
- `/doc-service new-platform-listener`

---

## What This Skill Does

1. **Checks** if `services/<service-name>/README.md` exists
2. **If missing**: Generates new README from template
3. **If exists**: Analyzes for missing sections and offers to update
4. **Reads service code** to extract:
   - Environment variables (from cmd/main.go)
   - API endpoints (from handlers/)
   - Features (from domain packages)
   - Dependencies (from go.mod)
5. **Generates/updates README** with:
   - Complete environment variable table
   - API endpoint documentation with examples
   - Architecture diagram (ASCII art)
   - Testing section
   - Troubleshooting common issues
   - Production considerations

---

## Instructions for Claude

When this skill is invoked with `/doc-service <service-name>`:

### Step 1: Check Existing README

```bash
# Check if README exists
ls services/<service-name>/README.md
```

**If exists**: Ask user if they want to update or regenerate

**If missing**: Proceed with generation

---

### Step 2: Analyze Service Code

**Read these files to understand service**:
1. `services/<service-name>/cmd/main.go` - Extract:
   - Port number (default PORT env var)
   - Environment variables (os.Getenv calls)
   - Dependencies (database, Redis, external APIs)
   - Route registrations

2. `services/<service-name>/go.mod` - Extract:
   - Key dependencies
   - Module name

3. `services/<service-name>/handlers/*.go` - Extract:
   - API endpoints (route definitions)
   - Request/response formats

4. Directory structure:
```bash
ls -R services/<service-name>/
# Identify packages: handlers/, models/, repository/, domain-specific packages
```

---

### Step 3: Use Template

**Read template**: `docs/development/service-template.md`

**Replace template variables**:
- `{SERVICE_NAME}` → Actual service name (e.g., "Auth Service")
- `{PORT}` → Default port from cmd/main.go (e.g., 8081)
- `{PURPOSE}` → One-sentence description
- `{FEATURES}` → List of capabilities extracted from code
- Environment variables table → Extracted from cmd/main.go
- API endpoints → Extracted from handlers/
- Architecture diagram → Generate based on service role in system

---

### Step 4: Generate Sections

**Features Section**:
- Analyze domain packages to understand capabilities
- List 4-6 key features with brief descriptions
- Example: "Multi-platform OAuth (Twitch, YouTube, Kick)"

**Environment Variables**:
Create table with Required vs Optional:
```markdown
### Required
| Variable | Purpose | Example |
|----------|---------|---------|
| DATABASE_HOST | PostgreSQL hostname | localhost |

### Optional
| Variable | Default | Purpose |
|----------|---------|---------|
| PORT | 8081 | HTTP server port |
```

**API Endpoints**:
Document all routes from handlers/:
```markdown
### OAuth Flows

\`\`\`bash
# Initiate Twitch OAuth
GET /api/v1/auth/twitch/authorize
→ Redirects to Twitch authorization page

# OAuth callback
GET /api/v1/auth/twitch/callback?code=...
→ Returns: { "token": "jwt-token", "user": {...} }
\`\`\`
```

**Architecture Diagram**:
Generate ASCII diagram showing service position:
```
Platform OAuth API
  ↓
Auth Service
  ├─ OAuth Handler
  ├─ JWT Generator
  └─ Token Storage
      ↓
  Database (users, oauth_tokens)
```

**Troubleshooting**:
Add 2-3 common issues based on service type:
- Connection services → "Connection refused", "Authentication failed"
- Processing services → "High lag", "Messages not processed"
- API services → "Endpoint returns 500", "Slow response"

---

### Step 5: Write README

Write to `services/<service-name>/README.md` using template structure.

**Sections in order**:
1. Title and one-sentence description
2. Port and status
3. Features
4. Architecture diagram
5. Environment variables
6. Running locally
7. API endpoints (if applicable)
8. Message format / Data schema (if publishes to Redis or database)
9. How it works (internal components)
10. Testing
11. Monitoring (key metrics)
12. Troubleshooting
13. Performance (capacity, scaling)
14. Production considerations
15. Related services
16. Further reading
17. License

---

### Step 6: Validate

**Checklist**:
- [ ] All environment variables documented (check no undocumented os.Getenv calls)
- [ ] All API endpoints listed (check all handler functions)
- [ ] Architecture diagram matches actual code
- [ ] Troubleshooting includes at least 2 common issues
- [ ] Links to related architecture docs and ADRs
- [ ] Production considerations specific to this service

---

## Example Invocation

**User**: `/doc-service auth-service`

**Claude**:
1. Checks services/auth-service/README.md exists ✅
2. Asks: "README exists. Update or regenerate?"
3. User selects "Update"
4. Reads cmd/main.go, handlers/, oauth/, repository/
5. Identifies missing sections: "Troubleshooting", "Production Considerations"
6. Generates new sections following template
7. Updates README with new sections
8. Outputs: "Updated services/auth-service/README.md with missing sections: Troubleshooting, Production Considerations"

---

## Success Criteria

✅ Skill complete when:
1. README generated following standardized template
2. All environment variables documented
3. All API endpoints listed with examples
4. Architecture diagram included
5. Troubleshooting section with common issues
6. Links to related documentation
7. README follows same format as existing service READMEs

---

## Related Documentation

- **Template**: [docs/development/service-template.md](../../docs/development/service-template.md)
- **Examples**: All service READMEs in services/*/README.md
- **Navigation**: [docs/llm-guides/NAVIGATION.md](../../docs/llm-guides/NAVIGATION.md)
