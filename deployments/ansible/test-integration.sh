#!/bin/bash
set -e

# Integration test script for All-Chat in Kubernetes
# Tests multi-platform message flow (Twitch + YouTube)

NAMESPACE="allchat"
API_URL="http://localhost:8080"

echo "========================================="
echo "All-Chat Integration Tests"
echo "========================================="
echo "Namespace: $NAMESPACE"
echo "API URL: $API_URL"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check port-forward is active
if ! lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1; then
  echo -e "${RED}✗${NC} Port 8080 not forwarded. Run './port-forward.sh' first"
  exit 1
fi

echo -e "${GREEN}✓${NC} Port forward active"
echo ""

# Test 1: Health Checks
echo "========================================="
echo "Test 1: Health Checks"
echo "========================================="

services=(
  "8080:API Gateway"
  "8086:YouTube Listener"
  "8088:Source Controller"
)

for svc in "${services[@]}"; do
  port=$(echo "$svc" | cut -d':' -f1)
  name=$(echo "$svc" | cut -d':' -f2)

  if curl -s -f "http://localhost:$port/health/live" > /dev/null; then
    echo -e "${GREEN}✓${NC} $name health check passed"
  else
    echo -e "${RED}✗${NC} $name health check failed"
    exit 1
  fi
done
echo ""

# Test 2: Status Endpoints
echo "========================================="
echo "Test 2: Status Endpoints"
echo "========================================="

echo "YouTube Listener status:"
YOUTUBE_STATUS=$(curl -s "http://localhost:8086/status")
echo "$YOUTUBE_STATUS" | jq '.'
echo ""

STREAM_COUNT=$(echo "$YOUTUBE_STATUS" | jq -r '.streams.active_count // 0')
QUOTA_USED=$(echo "$YOUTUBE_STATUS" | jq -r '.quota.used // 0')
echo -e "Active streams: $STREAM_COUNT"
echo -e "Quota used today: $QUOTA_USED"
echo ""

echo "Source Controller status:"
SOURCE_STATUS=$(curl -s "http://localhost:8088/status")
echo "$SOURCE_STATUS" | jq '.'
echo ""

TOTAL_SOURCES=$(echo "$SOURCE_STATUS" | jq -r '.registry.total_sources // 0')
LEADER_COUNT=$(echo "$SOURCE_STATUS" | jq -r '.leadership.leader_count // 0')
echo -e "Total sources: $TOTAL_SOURCES"
echo -e "Leader streams: $LEADER_COUNT"
echo ""

# Test 3: Database Connectivity
echo "========================================="
echo "Test 3: Database Connectivity"
echo "========================================="

# Check if postgres is accessible from inside cluster
POSTGRES_TEST=$(kubectl run -it --rm psql-test \
  --image=postgres:16-alpine \
  -n "$NAMESPACE" \
  --restart=Never \
  -- psql postgresql://allchat:allchat_dev_password@postgres:5432/allchat -c "SELECT 1;" 2>&1 || true)

if echo "$POSTGRES_TEST" | grep -q "1 row"; then
  echo -e "${GREEN}✓${NC} PostgreSQL connection successful"
else
  echo -e "${RED}✗${NC} PostgreSQL connection failed"
  echo "$POSTGRES_TEST"
fi
echo ""

# Test 4: Redis Connectivity
echo "========================================="
echo "Test 4: Redis Connectivity"
echo "========================================="

REDIS_TEST=$(kubectl run -it --rm redis-test \
  --image=redis:7-alpine \
  -n "$NAMESPACE" \
  --restart=Never \
  -- redis-cli -h redis PING 2>&1 || true)

if echo "$REDIS_TEST" | grep -q "PONG"; then
  echo -e "${GREEN}✓${NC} Redis connection successful"
else
  echo -e "${RED}✗${NC} Redis connection failed"
  echo "$REDIS_TEST"
fi
echo ""

# Test 5: Check Redis Streams
echo "========================================="
echo "Test 5: Redis Streams"
echo "========================================="

# Port forward Redis for direct access
kubectl port-forward -n "$NAMESPACE" svc/redis 6379:6379 > /dev/null 2>&1 &
PF_PID=$!
sleep 2

# Check stream exists and has consumer group
if command -v redis-cli &> /dev/null; then
  echo "Checking chat:raw stream..."
  STREAM_LEN=$(redis-cli -h localhost XLEN chat:raw 2>/dev/null || echo "0")
  echo "Stream length: $STREAM_LEN messages"

  echo ""
  echo "Checking consumer group..."
  redis-cli -h localhost XINFO GROUPS chat:raw 2>/dev/null || echo "Consumer group not created yet"
else
  echo -e "${YELLOW}⚠${NC} redis-cli not installed, skipping stream checks"
fi

# Kill port-forward
kill $PF_PID 2>/dev/null || true
echo ""

# Test 6: Leader Election
echo "========================================="
echo "Test 6: Leader Election"
echo "========================================="

echo "Scaling Source Controller to 3 replicas..."
kubectl scale deployment source-controller -n "$NAMESPACE" --replicas=3

echo "Waiting for pods to be ready..."
kubectl wait --for=condition=Ready pod -l app=source-controller -n "$NAMESPACE" --timeout=120s || true

echo ""
echo "Checking leadership distribution..."
LEADERSHIP=$(curl -s "http://localhost:8088/leadership")
echo "$LEADERSHIP" | jq '.'

INSTANCE_COUNT=$(kubectl get pods -n "$NAMESPACE" -l app=source-controller --no-headers | wc -l)
echo ""
echo -e "Source Controller instances: $INSTANCE_COUNT"

# Scale back to 1
echo "Scaling back to 1 replica..."
kubectl scale deployment source-controller -n "$NAMESPACE" --replicas=1
echo ""

# Summary
echo "========================================="
echo "Verification Complete"
echo "========================================="

if [ $FAILED -eq 0 ]; then
  echo -e "${GREEN}✓ All tests passed!${NC}"
  echo ""
  echo "Your All-Chat cluster is ready for development."
  echo ""
  echo "Useful commands:"
  echo "  kubectl get pods -n $NAMESPACE -o wide"
  echo "  kubectl logs -n $NAMESPACE -l app=youtube-listener -f"
  echo "  curl http://localhost:8086/status | jq"
  echo "  curl http://localhost:8088/status | jq"
else
  echo -e "${YELLOW}⚠ Some checks failed, but cluster may still be functional${NC}"
  echo ""
  echo "Check logs for errors:"
  echo "  kubectl logs -n $NAMESPACE -l app=youtube-listener --tail=100"
  echo "  kubectl logs -n $NAMESPACE -l app=source-controller --tail=100"
fi
