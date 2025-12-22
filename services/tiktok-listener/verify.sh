#!/bin/bash
# TikTok Listener Verification Script
# This script helps verify that the deduplication and timestamp fixes are working

set -e

echo "=== TikTok Listener Verification Script ==="
echo ""

# Check if service is running
echo "1. Checking if TikTok listener is accessible..."
if curl -s -f http://localhost:8089/health/live > /dev/null 2>&1; then
    echo "✅ Service is running"
else
    echo "❌ Service is not accessible at http://localhost:8089"
    echo "   Make sure the service is running with: docker-compose up tiktok-listener"
    exit 1
fi

echo ""
echo "2. Checking readiness..."
STATUS=$(curl -s http://localhost:8089/health/ready | jq -r '.status')
if [ "$STATUS" = "ready" ]; then
    echo "✅ Service is ready"
else
    echo "⚠️  Service status: $STATUS"
fi

echo ""
echo "3. Fetching service status..."
curl -s http://localhost:8089/status | jq '.'

echo ""
echo "4. Checking deduplication stats..."
DEDUP_STATS=$(curl -s http://localhost:8089/status | jq '.deduplication')
echo "$DEDUP_STATS"

PROCESSED=$(echo "$DEDUP_STATS" | jq -r '.processedCount')
DUPLICATES=$(echo "$DEDUP_STATS" | jq -r '.duplicateCount')
DUP_RATE=$(echo "$DEDUP_STATS" | jq -r '.duplicateRatePercent')

echo ""
echo "=== Deduplication Summary ==="
echo "Processed messages: $PROCESSED"
echo "Duplicate messages: $DUPLICATES"
echo "Duplicate rate: $DUP_RATE%"

if [ "$DUPLICATES" -gt 0 ]; then
    echo "✅ Deduplication is working! ($DUPLICATES duplicates prevented)"
else
    echo "ℹ️  No duplicates detected yet (normal if service just started)"
fi

echo ""
echo "5. Checking Redis for recent messages..."
if command -v redis-cli &> /dev/null; then
    echo "Latest TikTok message from Redis:"
    redis-cli XREVRANGE chat:raw + - COUNT 1 | grep -A 20 "platform.*tiktok" || echo "No TikTok messages found yet"
else
    echo "⚠️  redis-cli not found, skipping Redis check"
fi

echo ""
echo "=== Verification Complete ==="
echo ""
echo "To monitor duplicates in real-time, watch the logs:"
echo "  docker-compose logs -f tiktok-listener | grep 'Duplicate message'"
echo ""
echo "To check message timestamps in Redis:"
echo "  redis-cli XRANGE chat:raw - + COUNT 10"
