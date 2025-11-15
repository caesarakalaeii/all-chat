# TikTok Listener Service Dockerfile
# Multi-stage build for Node.js/TypeScript service
# NOTE: This Dockerfile is designed to be run from the repository root
# Context: .
# File: ./services/tiktok-listener/Dockerfile

# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY services/tiktok-listener/package*.json ./
COPY services/tiktok-listener/tsconfig.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY services/tiktok-listener/src/ ./src/

# Build TypeScript
RUN npm run build

# Production stage
FROM node:20-alpine

WORKDIR /app

# Install production dependencies only
COPY services/tiktok-listener/package*.json ./
RUN npm ci --only=production && npm cache clean --force

# Copy built files from builder
COPY --from=builder /app/dist ./dist

# Create non-root user
RUN addgroup -g 1001 -S nodejs && \
    adduser -S nodejs -u 1001

# Change ownership
RUN chown -R nodejs:nodejs /app

USER nodejs

# Expose port
EXPOSE 8089

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:8089/health/live', (r) => { process.exit(r.statusCode === 200 ? 0 : 1); }).on('error', () => process.exit(1));"

# Start service
CMD ["node", "dist/index.js"]
