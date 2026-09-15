# TikTok Signer Service Dockerfile
# Multi-stage build for Node.js/TypeScript service with bundled Chromium.
# NOTE: This Dockerfile is designed to be run from the repository root
# Context: .
# File: ./services/tiktok-signer/Dockerfile

# Build stage
FROM node:20-bookworm-slim AS builder

WORKDIR /app

# Copy package files
COPY services/tiktok-signer/package*.json ./
COPY services/tiktok-signer/tsconfig.json ./

# Install dependencies (tsc lives in devDependencies)
RUN npm ci

# Copy source and vendored signing assets
COPY services/tiktok-signer/src/ ./src/
COPY services/tiktok-signer/vendor/ ./vendor/

# Build TypeScript
RUN npm run build

# Production stage: Chromium is required for the signing session. Bookworm
# (not alpine) because puppeteer's bundled chromium needs glibc.
FROM node:20-bookworm-slim

# Chromium plus the shared libs headless Chromium needs on bookworm.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      chromium \
      fonts-liberation \
      libasound2 \
      libatk-bridge2.0-0 \
      libatk1.0-0 \
      libcups2 \
      libdbus-1-3 \
      libdrm2 \
      libgbm1 \
      libnspr4 \
      libnss3 \
      libx11-6 \
      libxcomposite1 \
      libxdamage1 \
      libxext6 \
      libxfixes3 \
      libxkbcommon0 \
      libxrandr2 \
    && rm -rf /var/lib/apt/lists/*

ENV PUPPETEER_SKIP_DOWNLOAD=true \
    PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium \
    SIGNER_USER_DATA_DIR=/home/signer/.chrome-profile

WORKDIR /app

# Install production dependencies only
COPY services/tiktok-signer/package*.json ./
RUN npm ci --omit=dev && npm cache clean --force

# Copy built files and vendored signing assets
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/vendor ./vendor
COPY --from=builder /app/node_modules ./node_modules
COPY LICENSE ./LICENSE

# Create non-root user with a writable home for the browser profile
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 --ingroup nodejs --home /home/signer nodejs && \
    mkdir -p /home/signer/.chrome-profile && \
    chown -R nodejs:nodejs /app /home/signer

USER nodejs

EXPOSE 8092

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:8092/health/live', (r) => { process.exit(r.statusCode === 200 ? 0 : 1); }).on('error', () => process.exit(1));"

CMD ["node", "dist/index.js"]
