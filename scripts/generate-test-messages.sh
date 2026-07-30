#!/bin/bash
# Generate test messages for frontend development
# Publishes mock chat messages to Redis for the test overlay

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
REDIS_HOST=${REDIS_HOST:-localhost}
REDIS_PORT=${REDIS_PORT:-6379}
MESSAGE_PROCESSOR_URL=${MESSAGE_PROCESSOR_URL:-http://localhost:8087}
API_KEY=${MESSAGE_PROCESSOR_API_KEY:-dev-frontend-key}
TEST_OVERLAY_ID=${TEST_OVERLAY_ID:-"00000000-0000-0000-0000-000000000002"}

# Message generation parameters
MESSAGE_INTERVAL=${MESSAGE_INTERVAL:-3}  # seconds between messages
MESSAGE_COUNT=${MESSAGE_COUNT:-0}        # 0 = infinite

# Sample messages
MESSAGES=(
    "Hello from Twitch! PogChamp"
    "This is a test message Kappa"
    "Testing the overlay LUL"
    "Can you see this? 👀"
    "Frontend development is fun! 🚀"
    "Lorem ipsum dolor sit amet"
    "Another test message here"
    "Chat is working great!"
    "WebSocket connection active ✅"
    "All systems operational 💯"
    "Testing emotes and formatting"
    "Long message test: Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
    "Multiple emojis test: 😀 😃 😄 😁 😆 😅"
    "Special characters: !@#$%^&*()"
    "Unicode test: こんにちは 你好 مرحبا"
)

# Sample usernames
USERNAMES=(
    "TestUser1"
    "FrontendDev"
    "ChatTester"
    "StreamViewer"
    "DevBot"
    "OverlayTest"
    "MessageGen"
    "TestAccount"
)

# Platforms
PLATFORMS=("twitch" "youtube")

echo -e "${BLUE}=== All-Chat Test Message Generator ===${NC}"
echo -e "Target Overlay: ${GREEN}$TEST_OVERLAY_ID${NC}"
echo -e "Message Interval: ${GREEN}${MESSAGE_INTERVAL}s${NC}"
echo -e "Message Count: ${GREEN}$([ $MESSAGE_COUNT -eq 0 ] && echo "infinite" || echo $MESSAGE_COUNT)${NC}"
echo -e "Message Processor: ${GREEN}$MESSAGE_PROCESSOR_URL${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
echo ""

# Check if curl is available
if ! command -v curl &> /dev/null; then
    echo -e "${RED}Error: curl is required but not installed${NC}"
    exit 1
fi

# Redis check is a nicety only (messages go through the processor's HTTP API);
# don't hard-require redis-cli on the host.
if command -v redis-cli &> /dev/null; then
    echo -e "${YELLOW}Testing Redis connection...${NC}"
    if redis-cli -h $REDIS_HOST -p $REDIS_PORT ping > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Redis connection OK${NC}"
    else
        echo -e "${YELLOW}⚠ Cannot reach Redis at $REDIS_HOST:$REDIS_PORT (continuing; the processor connects to Redis itself)${NC}"
    fi
    echo ""
fi

# Test Message Processor connection
echo -e "${YELLOW}Testing Message Processor connection...${NC}"
if ! curl -s -f "$MESSAGE_PROCESSOR_URL/health/live" > /dev/null 2>&1; then
    echo -e "${RED}Error: Cannot connect to Message Processor at $MESSAGE_PROCESSOR_URL${NC}"
    echo -e "${YELLOW}Make sure message-processor is running${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Message Processor connection OK${NC}"
echo ""

# Function to generate a random message
generate_message() {
    local platform=${PLATFORMS[$RANDOM % ${#PLATFORMS[@]}]}
    local username=${USERNAMES[$RANDOM % ${#USERNAMES[@]}]}
    local message_text=${MESSAGES[$RANDOM % ${#MESSAGES[@]}]}

    # Create JSON payload (schema: mockMessageRequest in
    # services/message-processor/cmd/main.go — overlay_id + text required)
    local json_payload=$(cat <<EOF
{
    "overlay_id": "$TEST_OVERLAY_ID",
    "platform": "$platform",
    "channel_id": "test_channel_123",
    "user_id": "user_$(($RANDOM % 1000))",
    "username": "$username",
    "display_name": "$username",
    "text": "$message_text",
    "color": "#$(printf '%06X' $(($RANDOM % 16777216)))"
}
EOF
)

    # Send to Message Processor mock endpoint
    local response=$(curl -s -X POST "$MESSAGE_PROCESSOR_URL/internal/mock-messages" \
        -H "Content-Type: application/json" \
        -H "X-Internal-Token: $API_KEY" \
        -d "$json_payload" \
        -w "\n%{http_code}")

    local http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" = "200" ] || [ "$http_code" = "201" ] || [ "$http_code" = "202" ]; then
        echo -e "${GREEN}✓${NC} [$platform] ${BLUE}$username${NC}: $message_text"
        return 0
    else
        echo -e "${RED}✗ Failed to send message (HTTP $http_code)${NC}"
        return 1
    fi
}

# Main loop
counter=0
while true; do
    counter=$((counter + 1))

    generate_message $counter

    # Exit if we've reached the message count
    if [ $MESSAGE_COUNT -gt 0 ] && [ $counter -ge $MESSAGE_COUNT ]; then
        echo ""
        echo -e "${GREEN}✓ Sent $counter messages${NC}"
        exit 0
    fi

    # Wait before next message
    sleep $MESSAGE_INTERVAL
done
