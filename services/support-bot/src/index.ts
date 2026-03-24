import type { BotConfig } from './types.js';
import { startBot } from './bot.js';

export function validateEnv(): BotConfig {
  const required: Record<string, string | undefined> = {
    DISCORD_BOT_TOKEN: process.env['DISCORD_BOT_TOKEN'],
    CLAUDE_CODE_OAUTH_TOKEN: process.env['CLAUDE_CODE_OAUTH_TOKEN'],
    GITHUB_TOKEN: process.env['GITHUB_TOKEN'],
    ALL_CHAT_REPO_PATH: process.env['ALL_CHAT_REPO_PATH'],
    ALL_CHAT_EXTENSION_REPO_PATH: process.env['ALL_CHAT_EXTENSION_REPO_PATH'],
  };

  for (const [name, value] of Object.entries(required)) {
    if (!value) {
      console.error(`Missing required environment variable: ${name}`);
      process.exit(1);
    }
  }

  const githubOwner = process.env['GITHUB_OWNER'] ?? 'moersener';

  return {
    discordToken: required['DISCORD_BOT_TOKEN'] as string,
    claudeOAuthToken: required['CLAUDE_CODE_OAUTH_TOKEN'] as string,
    githubToken: required['GITHUB_TOKEN'] as string,
    githubOwner,
    allChatRepoPath: required['ALL_CHAT_REPO_PATH'] as string,
    allChatExtensionRepoPath: required['ALL_CHAT_EXTENSION_REPO_PATH'] as string,
  };
}

export async function shutdown(client?: { destroy: () => void }): Promise<void> {
  console.log('Shutting down support-bot...');
  client?.destroy();
  process.exit(0);
}

let discordClient: { destroy: () => void } | undefined;

process.on('SIGINT', () => void shutdown(discordClient));
process.on('SIGTERM', () => void shutdown(discordClient));

async function main(): Promise<void> {
  const config = validateEnv();
  try {
    await startBot(config);
  } catch (error) {
    console.error('Failed to start bot:', error);
    process.exit(1);
  }
}

// Only run main when this file is the entry point (not during tests)
const isMain = process.argv[1]?.endsWith('index.ts') || process.argv[1]?.endsWith('index.js');
if (isMain) {
  void main();
}
