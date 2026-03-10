#!/usr/bin/env node
/**
 * Test WebSocket connection to overlay
 * Usage: node scripts/test-websocket.js [overlay_id]
 *
 * This script connects to the overlay WebSocket and logs all received messages.
 * Useful for testing message flow without starting the full frontend.
 */

const WebSocket = require('ws');

// Configuration
const WS_URL = process.env.WS_URL || 'ws://localhost:8080';
const DEFAULT_OVERLAY_ID = '00000000-0000-0000-0000-000000000002';
const overlayId = process.argv[2] || DEFAULT_OVERLAY_ID;

// ANSI colors
const colors = {
  reset: '\x1b[0m',
  green: '\x1b[32m',
  blue: '\x1b[34m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  gray: '\x1b[90m',
};

console.log(`${colors.blue}=== WebSocket Test Client ===${colors.reset}`);
console.log(`Overlay ID: ${colors.green}${overlayId}${colors.reset}`);
console.log(`URL: ${colors.green}${WS_URL}/ws/overlay/${overlayId}${colors.reset}`);
console.log('');

// Create WebSocket connection
const ws = new WebSocket(`${WS_URL}/ws/overlay/${overlayId}`);

let messageCount = 0;

ws.on('open', () => {
  console.log(`${colors.green}✓ Connected to WebSocket${colors.reset}`);
  console.log(`${colors.gray}Waiting for messages... (Press Ctrl+C to exit)${colors.reset}`);
  console.log('');
});

ws.on('message', (data) => {
  messageCount++;

  try {
    const message = JSON.parse(data.toString());

    // Format timestamp
    const timestamp = new Date(message.timestamp).toLocaleTimeString();

    // Color code by platform
    const platformColors = {
      twitch: colors.blue,
      youtube: colors.red,
      kick: colors.green,
      tiktok: colors.yellow,
    };
    const platformColor = platformColors[message.platform] || colors.reset;

    // Display message
    console.log(`[${colors.gray}${timestamp}${colors.reset}] ${platformColor}[${message.platform.toUpperCase()}]${colors.reset} ${colors.green}${message.username}${colors.reset}: ${message.message}`);

    // Display badges if present
    if (message.badges && message.badges.length > 0) {
      const badgeStr = message.badges.map(b => `[${b}]`).join(' ');
      console.log(`  ${colors.yellow}${badgeStr}${colors.reset}`);
    }

    // Display emotes if present
    if (message.emotes && message.emotes.length > 0) {
      const emoteStr = message.emotes.map(e => e.name).join(', ');
      console.log(`  ${colors.blue}Emotes: ${emoteStr}${colors.reset}`);
    }

  } catch (error) {
    console.error(`${colors.red}Error parsing message:${colors.reset}`, error.message);
    console.log('Raw data:', data.toString());
  }
});

ws.on('error', (error) => {
  console.error(`${colors.red}✗ WebSocket error:${colors.reset}`, error.message);
});

ws.on('close', (code, reason) => {
  console.log('');
  console.log(`${colors.yellow}✗ Connection closed${colors.reset}`);
  console.log(`Code: ${code}`);
  if (reason) {
    console.log(`Reason: ${reason}`);
  }
  console.log(`Total messages received: ${colors.green}${messageCount}${colors.reset}`);
  process.exit(0);
});

// Handle Ctrl+C
process.on('SIGINT', () => {
  console.log('');
  console.log(`${colors.yellow}Closing connection...${colors.reset}`);
  ws.close();
});

// Heartbeat to keep connection alive
const heartbeat = setInterval(() => {
  if (ws.readyState === WebSocket.OPEN) {
    ws.ping();
  }
}, 30000); // Ping every 30 seconds

ws.on('close', () => {
  clearInterval(heartbeat);
});
