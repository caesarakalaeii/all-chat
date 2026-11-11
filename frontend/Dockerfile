# All-Chat Frontend Dockerfile
# Multi-stage build for Next.js application

# Stage 1: Dependencies
# node:26 ships npm 11.x. The lockfile (lockfileVersion 3) is also installable by the
# npm 10.x in node:22 — verified with `npm@10.9.4 ci` — so this pin is a "stay
# current" choice, not a hard requirement. The floor is Node 22 (see
# frontend/README.md): @testing-library/jest-dom 7 declares engines node >= 22,
# and Node 20 went EOL 2026-04-30.
# The build stages are Debian trixie (glibc) to match the distroless runtime: native
# packages in the standalone output (sharp) are installed for the build's libc, so
# a musl (alpine) build would ship binaries the runtime cannot load (ADR-0092).
FROM node:26-trixie-slim AS deps
WORKDIR /app

# Copy package files
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci

# Stage 2: Builder
FROM node:26-trixie-slim AS builder
WORKDIR /app

# Copy dependencies from deps stage
COPY --from=deps /app/node_modules ./node_modules

# Copy source code
COPY frontend/ .

# Set environment variable for standalone build
ENV NEXT_TELEMETRY_DISABLED=1

# Build argument for cache busting
ARG BUILD_DATE
ARG GIT_COMMIT
ENV NEXT_PUBLIC_BUILD_DATE=$BUILD_DATE
ENV NEXT_PUBLIC_GIT_COMMIT=$GIT_COMMIT

# Umami analytics (cookieless) is configured directly in src/components/Analytics.tsx
# (Website ID + script host are pinned there, not secrets), so no build args needed.

# Clean Next.js cache before build to prevent stale Server Actions
RUN rm -rf .next

# Build Next.js application
RUN npm run build

# Stage 3: Runner — distroless Node (ADR-0092): no shell or package manager, and
# the image's entrypoint is node itself. UID/GID 1001 match runAsUser in the k8s
# manifests; distroless has no passwd entry for them, so they stay numeric.
FROM gcr.io/distroless/nodejs26-debian13 AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Copy built application
# Note: public directory may not exist if there are no static files
COPY --from=builder --chown=1001:1001 /app/.next/standalone ./
COPY --from=builder --chown=1001:1001 /app/.next/static ./.next/static
# Copy public directory for static assets (fonts, images, etc.)
COPY --from=builder --chown=1001:1001 /app/public ./public
COPY LICENSE ./LICENSE

USER 1001:1001

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Start Next.js server
CMD ["server.js"]
