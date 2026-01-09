FROM node:20-alpine

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci --production

# Copy source code
COPY src/ ./src/

# Run as non-root user
USER node

# Start the bot
CMD ["npm", "start"]
