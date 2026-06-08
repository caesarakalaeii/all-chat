# All-Chat Frontend Dockerfile
# Multi-stage build for Next.js application

# Stage 1: Dependencies
FROM node:22-alpine AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

# Copy package files
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci

# Stage 2: Builder
FROM node:22-alpine AS builder
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

# Umami analytics (cookieless). Baked in at build time; when unset the tracker
# is not rendered (see src/components/Analytics.tsx). The Website ID is created
# in the shared Umami dashboard and passed via the build-and-push workflow.
ARG NEXT_PUBLIC_UMAMI_WEBSITE_ID
ARG NEXT_PUBLIC_UMAMI_SRC
ENV NEXT_PUBLIC_UMAMI_WEBSITE_ID=$NEXT_PUBLIC_UMAMI_WEBSITE_ID
ENV NEXT_PUBLIC_UMAMI_SRC=$NEXT_PUBLIC_UMAMI_SRC

# Clean Next.js cache before build to prevent stale Server Actions
RUN rm -rf .next

# Build Next.js application
RUN npm run build

# Stage 3: Runner
FROM node:22-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Create non-root user
RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# Copy built application
# Note: public directory may not exist if there are no static files
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
# Copy public directory for static assets (fonts, images, etc.)
COPY --from=builder --chown=nextjs:nodejs /app/public ./public
COPY LICENSE ./LICENSE

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Start Next.js server
CMD ["node", "server.js"]
