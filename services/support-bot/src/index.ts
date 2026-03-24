import type { Client } from 'discord.js';
import type { BotConfig } from './types.js';
import { startBot } from './bot.js';
import { ClaudeTokenManager } from './claude/token-manager.js';

export function validateEnv(): BotConfig {
  const credentialsJson = process.env['CLAUDE_CREDENTIALS_JSON'];
  const oauthToken = process.env['CLAUDE_CODE_OAUTH_TOKEN'];

  if (!credentialsJson && !oauthToken) {
    console.error('Missing required environment variable: CLAUDE_CREDENTIALS_JSON or CLAUDE_CODE_OAUTH_TOKEN');
    process.exit(1);
  }

  const required: Record<string, string | undefined> = {
    DISCORD_BOT_TOKEN: process.env['DISCORD_BOT_TOKEN'],
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

  return {
    discordToken: required['DISCORD_BOT_TOKEN'] as string,
    claudeOAuthToken: oauthToken ?? '',
    githubToken: required['GITHUB_TOKEN'] as string,
    githubOwner: process.env['GITHUB_OWNER'] ?? 'moersener',
    allChatRepoPath: required['ALL_CHAT_REPO_PATH'] as string,
    allChatExtensionRepoPath: required['ALL_CHAT_EXTENSION_REPO_PATH'] as string,
    claudeCredentialsJson: credentialsJson,
  };
}

export async function shutdown(client?: { destroy: () => void }): Promise<void> {
  console.log('Shutting down support-bot...');
  client?.destroy();
  process.exit(0);
}

let discordClient: Client | undefined;

process.on('SIGINT', () => void shutdown(discordClient));
process.on('SIGTERM', () => void shutdown(discordClient));

async function main(): Promise<void> {
  const config = validateEnv();

  if (config.claudeCredentialsJson) {
    try {
      const startupManager = new ClaudeTokenManager(config.claudeCredentialsJson);
      await startupManager.ensureFreshToken();
      console.log('[startup] Claude token validated successfully');
    } catch (error) {
      console.error('[startup] Failed to obtain Claude token:', error);
      process.exit(1);
    }
  }

  try {
    discordClient = await startBot(config);
  } catch (error) {
    console.error('Failed to start bot:', error);
    process.exit(1);
  }
}

const isMain = process.argv[1]?.endsWith('index.ts') || process.argv[1]?.endsWith('index.js');
if (isMain) {
  void main();
}
