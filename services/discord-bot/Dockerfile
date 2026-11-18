# Install production dependencies with npm; the runtime image has no npm.
FROM node:22-trixie-slim AS deps

WORKDIR /app

# Copy package files
COPY services/discord-bot/package*.json ./

# Install dependencies
RUN npm ci --omit=dev

# Runtime stage: distroless Node (ADR-0092). No shell, package manager or npm, and
# the image's entrypoint is node itself. UID 1000 matches runAsUser in the k8s manifest.
FROM gcr.io/distroless/nodejs22-debian13

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY services/discord-bot/package.json ./

# Copy source code
COPY services/discord-bot/src/ ./src/
COPY LICENSE ./LICENSE

USER 1000:1000

# Start the bot (package.json "start" is `node src/index.js`)
CMD ["src/index.js"]
