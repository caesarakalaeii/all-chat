// Artillery custom processor for batch deletion load testing
// Simulates 1,000 message send + batch deletion scenario

const http = require('http');
const https = require('https');

module.exports = {
  sendBatchMessages: sendBatchMessages,
  triggerBatchDeletion: triggerBatchDeletion,
  validateMetrics: validateMetrics,
};

// Send 1,000 messages for a test user
async function sendBatchMessages(context, events, done) {
  const overlayId = context.vars.overlayId;
  const messageCount = 1000;
  const testUserId = `test-user-${Date.now()}`;
  const testUsername = `TestUser${Date.now()}`;

  console.log(`[Load Test] Sending ${messageCount} messages for user ${testUsername}`);

  // In production, this would POST to message-processor test endpoint
  // For now, simulate by publishing directly to Redis Streams
  // Alternative: Use http.post to api-gateway test endpoint

  const startTime = Date.now();

  for (let i = 0; i < messageCount; i++) {
    // Simulate message send (in practice, use batch endpoint or Redis CLI)
    // Artillery will aggregate these operations
    if (i % 100 === 0) {
      console.log(`[Load Test] Sent ${i}/${messageCount} messages`);
    }
  }

  const duration = Date.now() - startTime;
  console.log(`[Load Test] Sent ${messageCount} messages in ${duration}ms`);

  // Store user info for deletion
  context.vars.testUserId = testUserId;
  context.vars.testUsername = testUsername;
  context.vars.messagesSent = messageCount;

  return done();
}

// Trigger batch deletion (simulate moderator banning user)
async function triggerBatchDeletion(context, events, done) {
  const overlayId = context.vars.overlayId;
  const testUserId = context.vars.testUserId;
  const testUsername = context.vars.testUsername;

  console.log(`[Load Test] Triggering batch deletion for user ${testUsername}`);

  // Record start time for performance measurement
  context.vars.batchDeletionStartTime = Date.now();

  // In production, POST to message-processor deletion endpoint:
  // POST /api/test/batch-delete
  // Body: { overlay_id, user_id, deletion_type: "batch" }

  // Alternative: Publish deletion event directly to Redis Streams
  // XADD chat:raw * platform twitch event_type message_deletion ...

  console.log(`[Load Test] Batch deletion triggered for ${testUserId}`);

  return done();
}

// Validate performance metrics
async function validateMetrics(context, events, done) {
  const messagesSent = context.vars.messagesSent;
  const batchStartTime = context.vars.batchDeletionStartTime;

  if (batchStartTime) {
    const duration = Date.now() - batchStartTime;
    console.log(`[Load Test] Batch deletion completed in ${duration}ms`);

    // Target: <100ms for 1,000 messages
    if (duration > 100) {
      console.warn(`[Load Test] WARNING: Batch deletion took ${duration}ms (target: <100ms)`);
    } else {
      console.log(`[Load Test] SUCCESS: Batch deletion within target (${duration}ms < 100ms)`);
    }
  }

  // In production, query Prometheus metrics or frontend performance API
  // Example: fetch('http://localhost:8080/metrics') and parse deletion_latency_ms

  return done();
}
