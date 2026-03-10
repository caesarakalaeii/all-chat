#!/bin/bash
# Seed test data for frontend development
# Creates a test user, overlay, and chat sources

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-allchat}
DB_USER=${DB_USER:-allchat}
DB_PASSWORD=${DB_PASSWORD:-dev_password_123}

# Test data
TEST_USER_ID="00000000-0000-0000-0000-000000000001"
TEST_OVERLAY_ID="00000000-0000-0000-0000-000000000002"
TEST_TWITCH_SOURCE_ID="00000000-0000-0000-0000-000000000003"
TEST_YOUTUBE_SOURCE_ID="00000000-0000-0000-0000-000000000004"

echo -e "${BLUE}=== All-Chat Frontend Test Data Seeder ===${NC}"
echo ""

# Wait for database to be ready
echo -e "${YELLOW}Waiting for PostgreSQL...${NC}"
until PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c '\q' 2>/dev/null; do
  echo "PostgreSQL is unavailable - sleeping"
  sleep 2
done
echo -e "${GREEN}✓ PostgreSQL is ready${NC}"
echo ""

# Create test user
echo -e "${YELLOW}Creating test user...${NC}"
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME <<EOF
-- Create test user (mock Twitch OAuth)
INSERT INTO users (
    id,
    twitch_id,
    username,
    display_name,
    profile_image_url,
    access_token,
    refresh_token,
    token_expires_at
) VALUES (
    '$TEST_USER_ID'::uuid,
    'test_twitch_12345',
    'teststreamer',
    'Test Streamer',
    'https://static-cdn.jtvnw.net/jtv_user_pictures/test.png',
    'mock_access_token_encrypted',
    'mock_refresh_token_encrypted',
    NOW() + INTERVAL '30 days'
) ON CONFLICT (id) DO UPDATE SET
    username = EXCLUDED.username,
    display_name = EXCLUDED.display_name;
EOF
echo -e "${GREEN}✓ Test user created (ID: $TEST_USER_ID)${NC}"
echo ""

# Create test overlay
echo -e "${YELLOW}Creating test overlay...${NC}"
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME <<EOF
-- Create test overlay
INSERT INTO overlays (
    id,
    user_id,
    name,
    description,
    is_active
) VALUES (
    '$TEST_OVERLAY_ID'::uuid,
    '$TEST_USER_ID'::uuid,
    'Frontend Test Overlay',
    'Auto-generated overlay for frontend development and testing',
    TRUE
) ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    is_active = EXCLUDED.is_active;
EOF
echo -e "${GREEN}✓ Test overlay created (ID: $TEST_OVERLAY_ID)${NC}"
echo ""

# Create chat sources
echo -e "${YELLOW}Creating chat sources...${NC}"
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME <<EOF
-- Twitch source
INSERT INTO overlay_chat_sources (
    id,
    overlay_id,
    platform,
    channel_id,
    channel_name,
    auth_required,
    is_active
) VALUES (
    '$TEST_TWITCH_SOURCE_ID'::uuid,
    '$TEST_OVERLAY_ID'::uuid,
    'twitch',
    'teststreamer',
    'TestStreamer',
    FALSE,
    TRUE
) ON CONFLICT (overlay_id, platform, channel_id) DO UPDATE SET
    channel_name = EXCLUDED.channel_name,
    is_active = EXCLUDED.is_active;

-- YouTube source
INSERT INTO overlay_chat_sources (
    id,
    overlay_id,
    platform,
    channel_id,
    channel_name,
    auth_required,
    is_active
) VALUES (
    '$TEST_YOUTUBE_SOURCE_ID'::uuid,
    '$TEST_OVERLAY_ID'::uuid,
    'youtube',
    'UCtest12345',
    'Test YouTube Channel',
    TRUE,
    TRUE
) ON CONFLICT (overlay_id, platform, channel_id) DO UPDATE SET
    channel_name = EXCLUDED.channel_name,
    is_active = EXCLUDED.is_active;
EOF
echo -e "${GREEN}✓ Chat sources created (Twitch + YouTube)${NC}"
echo ""

# Display summary
echo -e "${BLUE}=== Test Data Summary ===${NC}"
echo -e "User ID:           ${GREEN}$TEST_USER_ID${NC}"
echo -e "Username:          ${GREEN}teststreamer${NC}"
echo -e "Overlay ID:        ${GREEN}$TEST_OVERLAY_ID${NC}"
echo -e "Overlay Name:      ${GREEN}Frontend Test Overlay${NC}"
echo -e "Twitch Source ID:  ${GREEN}$TEST_TWITCH_SOURCE_ID${NC}"
echo -e "YouTube Source ID: ${GREEN}$TEST_YOUTUBE_SOURCE_ID${NC}"
echo ""
echo -e "${GREEN}✓ Test data seeding complete!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo -e "1. Run the message generator: ${BLUE}./scripts/generate-test-messages.sh${NC}"
echo -e "2. Connect your overlay WebSocket to: ${BLUE}ws://localhost:8080/ws/overlay/$TEST_OVERLAY_ID${NC}"
echo -e "3. Start your frontend: ${BLUE}cd frontend && npm run dev${NC}"
