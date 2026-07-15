#!/bin/bash
# One-command frontend development setup
# Runs: docker up → seed → verify → instructions

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}${BLUE}"
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║   All-Chat Frontend Development - Quick Start Script          ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check if services are already running
if docker-compose -f docker-compose.frontend.yml ps | grep -q "Up"; then
    echo -e "${YELLOW}⚠ Services already running${NC}"
    echo -e "${YELLOW}Do you want to restart? (y/N)${NC}"
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Restarting services...${NC}"
        docker-compose -f docker-compose.frontend.yml down
    else
        echo -e "${YELLOW}Using existing services${NC}"
        echo ""
        ./scripts/verify-frontend-setup.sh
        exit 0
    fi
fi

# Step 1: Start services
echo -e "${BOLD}${BLUE}[1/4] Starting Docker services...${NC}"
docker-compose -f docker-compose.frontend.yml up -d

# Wait for services to be healthy
echo -e "${YELLOW}Waiting for services to be ready (30 seconds)...${NC}"
sleep 5
echo -n "Progress: "
for i in {1..25}; do
    echo -n "█"
    sleep 1
done
echo -e " ${GREEN}✓${NC}"
echo ""

# Step 2: Seed database
echo -e "${BOLD}${BLUE}[2/4] Seeding test data...${NC}"
./scripts/seed-test-data.sh
echo ""

# Step 3: Verify setup
echo -e "${BOLD}${BLUE}[3/4] Verifying setup...${NC}"
./scripts/verify-frontend-setup.sh
echo ""

# Step 4: Display next steps
echo -e "${BOLD}${BLUE}[4/4] Setup complete! 🎉${NC}"
echo ""
echo -e "${BOLD}${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║                    NEXT STEPS                                  ║${NC}"
echo -e "${BOLD}${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}Terminal 1:${NC} Generate test messages"
echo -e "  ${BLUE}make frontend-messages${NC}"
echo -e "  ${BLUE}# or: ./scripts/generate-test-messages.sh${NC}"
echo ""
echo -e "${YELLOW}Terminal 2:${NC} Start frontend development server"
echo -e "  ${BLUE}cd frontend && npm run dev${NC}"
echo ""
echo -e "${YELLOW}Terminal 3:${NC} Monitor WebSocket (optional)"
echo -e "  ${BLUE}cd scripts && npm install && node test-websocket.js${NC}"
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BOLD}Available URLs:${NC}"
echo -e "  Frontend:      ${GREEN}http://localhost:3000${NC}"
echo -e "  API Gateway:   ${GREEN}http://localhost:8080${NC}"
echo -e "  WebSocket:     ${GREEN}ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002${NC}"
echo ""
echo -e "${BOLD}Test Overlay ID:${NC} ${GREEN}00000000-0000-0000-0000-000000000002${NC}"
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BOLD}Useful Commands:${NC}"
echo -e "  ${BLUE}make frontend-verify${NC}     - Check service health"
echo -e "  ${BLUE}make frontend-down${NC}       - Stop services"
echo -e "  ${BLUE}make frontend-reset${NC}      - Complete reset"
echo -e "  ${BLUE}docker-compose -f docker-compose.frontend.yml logs -f${NC} - View logs"
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BOLD}Documentation:${NC}"
echo -e "  ${BLUE}../docs/frontend/FRONTEND_QUICK_START.md${NC} - Quick reference guide"
echo -e "  ${BLUE}../docs/frontend/FRONTEND_DEV_SETUP.md${NC}   - Complete documentation"
echo ""
echo -e "${BOLD}${GREEN}✨ Happy coding! ✨${NC}"
echo ""
