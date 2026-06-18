#!/bin/bash
# Start (or control) the public test-stream generator.
#
# The message-processor publishes fake chat, poll votes (1/2/3/4) and platform
# events to a single fixed test overlay (migration 058). Point an external tool
# at the returned ws_url to evaluate the WebSocket feed.
#
# Usage:
#   scripts/start-test-stream.sh                      # start with defaults
#   scripts/start-test-stream.sh start 120 8 0.5 10   # duration rate voteRatio eventEveryN
#   scripts/start-test-stream.sh status
#   scripts/start-test-stream.sh stop

set -e

MESSAGE_PROCESSOR_URL=${MESSAGE_PROCESSOR_URL:-http://localhost:8087}
ACTION=${1:-start}

case "$ACTION" in
  start)
    DURATION=${2:-60}
    RATE=${3:-5}
    VOTE_RATIO=${4:-0.4}
    EVENT_EVERY_N=${5:-12}
    curl -fsS -X POST "$MESSAGE_PROCESSOR_URL/public/test-stream/start" \
      -H "Content-Type: application/json" \
      -d "{\"duration_seconds\":$DURATION,\"rate_per_second\":$RATE,\"vote_ratio\":$VOTE_RATIO,\"event_every_n\":$EVENT_EVERY_N}"
    ;;
  stop)
    curl -fsS -X POST "$MESSAGE_PROCESSOR_URL/public/test-stream/stop"
    ;;
  status)
    curl -fsS "$MESSAGE_PROCESSOR_URL/public/test-stream/status"
    ;;
  *)
    echo "Unknown action: $ACTION (use start|stop|status)" >&2
    exit 1
    ;;
esac
echo
