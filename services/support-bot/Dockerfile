FROM node:20-alpine

WORKDIR /app

# Install claude CLI globally (provides the `claude` binary for subprocess queries)
RUN npm install -g @anthropic-ai/claude-code

# Copy package files
COPY services/support-bot/package*.json ./

# Install all dependencies (tsx needed at runtime for TypeScript execution)
RUN npm ci

# Copy source code
COPY services/support-bot/src/ ./src/
COPY services/support-bot/tsconfig.json ./

# Run as non-root user
USER node

# Start the bot
CMD ["npm", "start"]
