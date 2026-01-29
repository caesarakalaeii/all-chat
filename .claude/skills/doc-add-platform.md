# doc-add-platform

Generate customized quick reference guide for adding a new streaming platform.

---

## Usage

```
/doc-add-platform <platform-name>
```

**Examples**:
- `/doc-add-platform rumble`
- `/doc-add-platform facebook-gaming`
- `/doc-add-platform dlive`

---

## What This Skill Does

Generates a **customized platform integration guide** based on the platform's API characteristics:
1. **Analyzes platform API** (IRC vs WebSocket vs HTTP polling)
2. **Selects appropriate template** (Twitch, Kick, or YouTube listener)
3. **Generates step-by-step guide** with platform-specific code snippets
4. **Creates decision tree** for implementation approach
5. **Provides validation checklist** with test commands

---

## Instructions for Claude

When this skill is invoked with `/doc-add-platform <platform-name>`:

### Step 1: Research Platform API

**Ask user** (use AskUserQuestion tool):
```
Question: "What type of API does {platform} use for live chat?"
Options:
1. IRC (Internet Relay Chat)
2. WebSocket (real-time connection)
3. HTTP Polling (REST API)
4. Other/Unknown
```

Based on answer, select template:
- **IRC** → Use twitch-listener as template
- **WebSocket** → Use kick-listener as template
- **HTTP Polling** → Use youtube-listener as template
- **Unknown** → Provide decision tree to help user determine

---

### Step 2: Read Base Quick Reference

Read `docs/llm-guides/QUICK-REF-ADD-PLATFORM.md` as foundation.

---

### Step 3: Read Template Service

Based on API type:
- **IRC**: Read `services/twitch-listener/` (irc/, channels/, publisher/)
- **WebSocket**: Read `services/kick-listener/` (websocket/, channels/, publisher/)
- **HTTP**: Read `services/youtube-listener/` (youtube/, streams/, channels/)

---

### Step 4: Generate Customized Guide

**Output file**: `/tmp/claude-<session>/add-<platform>-guide.md`

**Sections**:

1. **Decision Summary**:
   ```markdown
   # Adding {Platform} Support to All-Chat

   **Platform**: {Platform}
   **API Type**: {IRC|WebSocket|HTTP}
   **Template**: {twitch|kick|youtube}-listener
   **Estimated Time**: 6-8 hours
   ```

2. **Platform-Specific Prerequisites**:
   ```markdown
   ## Prerequisites

   - [ ] {Platform} API documentation reviewed
   - [ ] {Platform} developer account created
   - [ ] API credentials obtained (if required)
   - [ ] Example chat message structure documented
   ```

3. **Step-by-Step Guide** (customize from QUICK-REF-ADD-PLATFORM.md):
   - Copy template service files
   - Update service name and ports
   - Implement {Platform}-specific client logic
   - Create normalizer for message format
   - Database migration
   - Kubernetes deployment
   - Testing checklist

4. **Platform-Specific Code Snippets**:

   For **IRC** platforms:
   ```go
   // Example from twitch-listener
   client := irc.NewClient(username, oauthToken)
   client.OnConnect(func() {
       client.Join("#channel")
   })
   client.OnPrivateMessage(func(msg) {
       publisher.Publish(msg)
   })
   ```

   For **WebSocket** platforms:
   ```go
   // Example from kick-listener
   wsClient := websocket.Dial(url)
   wsClient.Subscribe("chatrooms.{id}")
   wsClient.OnMessage(func(event) {
       publisher.Publish(event)
   })
   ```

   For **HTTP** platforms:
   ```go
   // Example from youtube-listener
   poller := NewPoller(apiClient)
   poller.Start(func(messages) {
       for _, msg := range messages {
           publisher.Publish(msg)
       }
   })
   ```

5. **Platform-Specific Normalizer**:
   ```go
   // services/message-processor/normalizer/{platform}_normalizer.go
   func Parse{Platform}Message(rawMsg map[string]interface{}) (*models.UnifiedMessage, error) {
       // Extract platform-specific fields
       // Map to unified format
       // Return unified message
   }
   ```

6. **Testing Commands**:
   ```bash
   # Build service
   go build ./services/{platform}-listener/cmd

   # Test connection
   # (platform-specific test based on API type)

   # Check Redis Stream
   redis-cli XREAD COUNT 10 STREAMS chat:raw 0
   ```

7. **Validation Checklist** (from template, customized):
   - [ ] Service builds and runs
   - [ ] Connects to {Platform} API successfully
   - [ ] Messages published to Redis Streams
   - [ ] Normalizer converts to unified format
   - [ ] End-to-end test passes
   - [ ] Documentation updated

---

### Step 5: Output Guide

**Write guide** to temporary location:
```
/tmp/claude-<session>/add-<platform>-guide.md
```

**Notify user**:
```
Generated customized guide for adding {Platform} support:
- Template: {template}-listener
- API Type: {type}
- File: /tmp/claude-<session>/add-{platform}-guide.md

Next steps:
1. Review the guide
2. Gather API credentials for {Platform}
3. Follow step-by-step instructions
4. Estimated time: 6-8 hours
```

---

## Success Criteria

✅ Skill complete when:
1. Platform API type identified (IRC/WebSocket/HTTP)
2. Appropriate template selected
3. Customized guide generated with platform-specific code
4. Decision tree included for implementation approach
5. Testing commands specific to platform API
6. Validation checklist complete

---

## Example Output

For `/doc-add-platform rumble` (WebSocket-based):

**Generated guide includes**:
- Template choice: kick-listener (WebSocket)
- Rumble WebSocket connection code (adapted from Kick Pusher client)
- Rumble message format → unified format normalizer
- Specific Rumble API endpoints and authentication
- Testing with actual Rumble chat room
- Kubernetes deployment for rumble-listener

**File location**: `/tmp/claude-<session>/add-rumble-guide.md` (~180 lines)

---

## Related Documentation

- **Base Guide**: [docs/llm-guides/QUICK-REF-ADD-PLATFORM.md](../../docs/llm-guides/QUICK-REF-ADD-PLATFORM.md)
- **Templates**: services/twitch-listener/, services/kick-listener/, services/youtube-listener/
- **Template README**: [docs/development/service-template.md](../../docs/development/service-template.md)
