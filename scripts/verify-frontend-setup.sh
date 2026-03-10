#!/bin/bash
# Verify frontend development environment is ready

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Frontend Development Environment Verification ===${NC}"
echo ""

# Check if docker-compose is running
echo -e "${YELLOW}Checking Docker services...${NC}"
if ! docker-compose -f docker-compose.frontend.yml ps | grep -q "Up"; then
    echo -e "${RED}✗ Services not running${NC}"
    echo -e "${YELLOW}Run: make frontend-dev${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker services running${NC}"

# Check PostgreSQL
echo -e "${YELLOW}Checking PostgreSQL...${NC}"
if ! PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat -c '\q' 2>/dev/null; then
    echo -e "${RED}✗ PostgreSQL not accessible${NC}"
    exit 1
fi
echo -e "${GREEN}✓ PostgreSQL accessible${NC}"

# Check Redis
echo -e "${YELLOW}Checking Redis...${NC}"
if ! redis-cli -h localhost -p 6379 ping > /dev/null 2>&1; then
    echo -e "${RED}✗ Redis not accessible${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Redis accessible${NC}"

# Check API Gateway
echo -e "${YELLOW}Checking API Gateway...${NC}"
if ! curl -s -f http://localhost:8080/health/live > /dev/null 2>&1; then
    echo -e "${RED}✗ API Gateway not responding${NC}"
    exit 1
fi
echo -e "${GREEN}✓ API Gateway healthy${NC}"

# Check Overlay Manager
echo -e "${YELLOW}Checking Overlay Manager...${NC}"
if ! curl -s -f http://localhost:8082/health/live > /dev/null 2>&1; then
    echo -e "${RED}✗ Overlay Manager not responding${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Overlay Manager healthy${NC}"

# Check Message Processor
echo -e "${YELLOW}Checking Message Processor...${NC}"
if ! curl -s -f http://localhost:8087/health/live > /dev/null 2>&1; then
    echo -e "${RED}✗ Message Processor not responding${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Message Processor healthy${NC}"

# Check for test data
echo -e "${YELLOW}Checking test data...${NC}"
TEST_OVERLAY_ID="00000000-0000-0000-0000-000000000002"
OVERLAY_COUNT=$(PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat -t -c "SELECT COUNT(*) FROM overlays WHERE id = '$TEST_OVERLAY_ID'::uuid;" 2>/dev/null | xargs)

if [ "$OVERLAY_COUNT" != "1" ]; then
    echo -e "${YELLOW}⚠ Test overlay not found${NC}"
    echo -e "${YELLOW}Run: make frontend-seed${NC}"
else
    echo -e "${GREEN}✓ Test overlay exists${NC}"
fi

# Check chat sources
SOURCE_COUNT=$(PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat -t -c "SELECT COUNT(*) FROM overlay_chat_sources WHERE overlay_id = '$TEST_OVERLAY_ID'::uuid;" 2>/dev/null | xargs)

if [ "$SOURCE_COUNT" -lt "1" ]; then
    echo -e "${YELLOW}⚠ No chat sources found${NC}"
    echo -e "${YELLOW}Run: make frontend-seed${NC}"
else
    echo -e "${GREEN}✓ Chat sources configured ($SOURCE_COUNT sources)${NC}"
fi

echo ""
echo -e "${GREEN}=== Environment Ready! ===${NC}"
echo ""
echo -e "${BLUE}Available endpoints:${NC}"
echo -e "  API Gateway:        ${GREEN}http://localhost:8080${NC}"
echo -e "  Overlay Manager:    ${GREEN}http://localhost:8082${NC}"
echo -e "  Message Processor:  ${GREEN}http://localhost:8087${NC}"
echo -e "  WebSocket:          ${GREEN}ws://localhost:8080/ws/overlay/$TEST_OVERLAY_ID${NC}"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo -e "  1. Generate messages: ${YELLOW}make frontend-messages${NC}"
echo -e "  2. Start frontend:    ${YELLOW}cd frontend && npm run dev${NC}"
echo ""
