# TikTok Signer Service Dockerfile
# Multi-stage build for Node.js/TypeScript service with bundled Chromium.
# NOTE: This Dockerfile is designed to be run from the repository root
# Context: .
# File: ./services/tiktok-signer/Dockerfile

# Build stage
FROM node:22-bookworm-slim AS builder

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

# Brotli stage: bookworm's libbrotli1 (1.0.9) carries CVE-2026-111497, an
# out-of-bounds write in the decoder, and Debian has no fixed package yet.
# Chromium decodes untrusted brotli (Content-Encoding: br, WOFF2 fonts) with
# the system libbrotlidec, so build upstream 1.0.10 (same ABI series) and let
# it shadow the distro copy. Drop this stage once bookworm ships a fixed
# libbrotli1.
FROM debian:bookworm-slim AS brotli
RUN apt-get update && \
    apt-get install -y --no-install-recommends cmake gcc libc6-dev make && \
    rm -rf /var/lib/apt/lists/*
ADD --checksum=sha256:b517ebdb5266b963d28d6e639b973479874ec72086deb6ee4a2d639e1b13606b \
    https://github.com/google/brotli/archive/refs/tags/v1.0.10.tar.gz /tmp/brotli.tar.gz
RUN tar -xzf /tmp/brotli.tar.gz -C /tmp && \
    cmake -S /tmp/brotli-1.0.10 -B /tmp/build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr/local && \
    cmake --build /tmp/build -j"$(nproc)" && \
    cmake --install /tmp/build

# Production stage: Chromium is required for the signing session. Bookworm
# (not alpine) because puppeteer's bundled chromium needs glibc.
FROM node:22-bookworm-slim

# Chromium, the shared libs it needs on bookworm, and an X server with Mesa
# GL. Page-viewer mode (ADR-0052 post-2026-09-09) runs Chromium non-headless:
# Xvfb + llvmpipe passes TikTok's session gate (measured 2026-09-15; the
# earlier failures were --disable-gpu/SwiftShader, not software GL). The
# entrypoint starts X on the node GPU when /dev/dri is passed through,
# falling back to Xvfb otherwise.
# No Vulkan/lavapipe stack: the ANGLE-on-Vulkan viewer args that needed it
# wedge the page (see viewer.ts); it was removed with them.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      chromium \
      xvfb \
      xauth \
      x11-utils \
      mesa-utils \
      libgl1-mesa-dri \
      libgl1 \
      libegl1 \
      libglx-mesa0 \
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

# Patched brotli (see the brotli stage). /usr/local/lib is searched before the
# multiarch dir, so after ldconfig Chromium and every other consumer resolve
# the fixed copy; the build fails if Chromium still links the distro one or
# cannot resolve every symbol against the replacement.
COPY --from=brotli /usr/local/lib/libbrotli*.so.1* /usr/local/lib/
RUN ldconfig && \
    ldd /usr/lib/chromium/chromium | grep -q '/usr/local/lib/libbrotlidec.so.1' && \
    LD_BIND_NOW=1 /usr/lib/chromium/chromium --version

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

# Starts the display (node GPU when /dev/dri exists, else Xvfb), then the
# service. Page mode needs the display before the viewer browser launches.
# --chmod keeps this a single layer after USER nodejs (RUN chmod would fail
# unprivileged).
COPY --chmod=0755 services/tiktok-signer/entrypoint.sh /app/entrypoint.sh

CMD ["/app/entrypoint.sh"]
