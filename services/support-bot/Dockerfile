FROM node:20-alpine

WORKDIR /app

# Install claude CLI globally (provides the `claude` binary for subprocess queries)
RUN npm install -g @anthropic-ai/claude-code

# Install kubectl (match cluster server version v1.33.5)
RUN apk add --no-cache curl && \
    curl -LO "https://dl.k8s.io/release/v1.33.5/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    rm kubectl

# Install mcp-grafana binary (Grafana MCP server for Claude subprocess)
RUN curl -L "https://github.com/grafana/mcp-grafana/releases/download/v0.11.3/mcp-grafana_Linux_x86_64.tar.gz" \
    -o /tmp/mcp-grafana.tar.gz && \
    tar -xzf /tmp/mcp-grafana.tar.gz -C /usr/local/bin/ mcp-grafana && \
    chmod 0755 /usr/local/bin/mcp-grafana && \
    rm /tmp/mcp-grafana.tar.gz

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
