FROM node:20-alpine

WORKDIR /app

# Copy package files
COPY services/discord-bot/package*.json ./

# Install dependencies
RUN npm ci --production

# Copy source code
COPY services/discord-bot/src/ ./src/
COPY LICENSE ./LICENSE

# Run as non-root user
USER node

# Start the bot
CMD ["npm", "start"]
