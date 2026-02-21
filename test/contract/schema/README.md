# Golden File Capture Tool

This directory contains tooling for capturing golden files from the official YouTube listener and validating schema compatibility between the official listener and the InnerTube listener.

## Overview

Golden files are JSON snapshots of RawChatMessage output from the official youtube-listener. They serve as the ground truth for schema validation tests that ensure the InnerTube listener produces identical output format.

## Prerequisites

1. **Built official youtube-listener binary**:
   ```bash
   cd services/youtube-listener
   go build
   ```

2. **Redis running**:
   ```bash
   docker run -d -p 6379:6379 redis:7-alpine
   # OR
   make docker-up  # from project root
   ```

3. **Active YouTube live stream**: Find streams with active chat at https://www.youtube.com/trending

## Usage

### Building the capture tool

```bash
cd test/contract/schema
go mod tidy
go build -o capture golden_capture.go
```

### Capturing golden files

```bash
./capture -stream-url https://www.youtube.com/watch?v=VIDEO_ID -duration 10m
```

**Options**:
- `-stream-url` (required): YouTube live stream URL
- `-duration`: Capture duration (default: 5m)
- `-output-dir`: Where to save golden files (default: ./golden)
- `-listener-binary`: Path to official youtube-listener (default: ../../../services/youtube-listener/youtube-listener)
- `-redis-host`: Redis host:port (default: localhost:6379)
- `-redis-password`: Redis password (optional)

### Recommended workflow

1. **Capture from 5-10 different streams** to get variety:
   - High-volume streams (100+ messages/min)
   - Low-volume streams (5-10 messages/min)
   - Streams with Super Chats and memberships
   - Streams with different languages

2. **Run until you have 100+ golden files** across different message types:
   - Text messages: 50+ files
   - Super Chats: 10+ files
   - Memberships (joined/milestone): 5+ files
   - Super Stickers: 5+ files

3. **Example capture session**:
   ```bash
   # Stream 1: High volume gaming stream (10 minutes)
   ./capture -stream-url https://www.youtube.com/watch?v=XXXXX -duration 10m

   # Stream 2: Music stream with Super Chats (10 minutes)
   ./capture -stream-url https://www.youtube.com/watch?v=YYYYY -duration 10m

   # Stream 3: Lower volume tech stream (5 minutes)
   ./capture -stream-url https://www.youtube.com/watch?v=ZZZZZ -duration 5m

   # Check progress
   ls golden/*.json | wc -l
   ls golden/*/text_message*.json | wc -l
   ls golden/*/super_chat*.json | wc -l
   ```

### Golden file naming convention

Files are saved as: `{stream_name}_{message_type}_{sequence}.json`

**Examples**:
- `VIDEO_ID_text_message_001.json` - First text message
- `VIDEO_ID_super_chat_001.json` - First Super Chat
- `VIDEO_ID_member_joined_001.json` - First membership join
- `VIDEO_ID_member_milestone_001.json` - First membership milestone
- `VIDEO_ID_super_sticker_001.json` - First Super Sticker

### What gets captured

Each golden file contains a complete RawChatMessage JSON object with:
- `message_id`: Unique identifier
- `platform`: Always "youtube"
- `channel_id`: YouTube channel ID
- `stream_id`: Live stream video ID
- `user_id`: YouTube user channel ID
- `username`: Display name
- `text`: Message text (or event description)
- `timestamp`: UTC timestamp
- `tags`: YouTube-specific metadata
- `event_type`: (optional) "super_chat", "member_joined", etc.
- `event_data`: (optional) Event-specific payload

## Running schema validation tests

After capturing golden files:

```bash
cd test/contract/schema
go test -v
```

See `schema_test.go` for test implementation details.

## Regenerating golden files

When the schema legitimately changes (e.g., new required fields added):

1. Rebuild the official listener with schema changes
2. Delete existing golden files: `rm golden/*.json`
3. Re-run capture tool to generate new golden files
4. Update tests with `-update` flag: `go test -update`

## Troubleshooting

**"youtube-listener binary not found"**:
- Build the listener first: `cd services/youtube-listener && go build`

**"Failed to connect to Redis"**:
- Verify Redis is running: `redis-cli ping`
- Check Redis host/port matches your configuration

**"No messages captured"**:
- Verify the stream has active chat (not all streams have chat enabled)
- Try a different stream from /trending
- Check youtube-listener logs for authentication errors

**"Only text messages, no events"**:
- Events (Super Chats, memberships) are rarer than regular messages
- Try high-profile streams with more engaged audiences
- Increase capture duration to 15-30 minutes

## Notes

- Golden files are **gitignored** initially (large binary data)
- For CI/CD, commit a representative sample (~20 files) covering all message types
- Capture tool automatically cleans up the youtube-listener subprocess on exit
- Messages are acknowledged in Redis to prevent reprocessing
