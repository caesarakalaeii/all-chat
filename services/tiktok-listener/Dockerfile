# TikTok Listener Service Dockerfile
# Multi-stage build for Node.js/TypeScript service
# NOTE: This Dockerfile is designed to be run from the repository root
# Context: .
# File: ./services/tiktok-listener/Dockerfile

# Build stage
# bookworm (not alpine): tls-impersonate's native addon ships glibc prebuilds
# only — musl falls back to node-gyp and there is no toolchain here. Node 26:
# tls-impersonate's full Chrome ClientHello (incl. compress_certificate) is
# verified complete only on >= 26.4.
FROM node:26-bookworm-slim AS builder

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

# Production stage — same glibc and Node-floor reasoning as the builder.
FROM node:26-bookworm-slim

WORKDIR /app

# Install production dependencies only
COPY services/tiktok-listener/package*.json ./
RUN npm ci --omit=dev && npm cache clean --force

# Copy built files from builder
COPY --from=builder /app/dist ./dist
COPY LICENSE ./LICENSE

# Create non-root user
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 --ingroup nodejs nodejs

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
