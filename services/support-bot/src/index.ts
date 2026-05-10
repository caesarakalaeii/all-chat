import type { Client } from 'discord.js';
import pg from 'pg';
import type { BotConfig } from './types.js';
import { startBot } from './bot.js';
import { MemoryRepository } from './memory/repository.js';

export function validateEnv(): BotConfig {
  const oauthToken = process.env['CLAUDE_CODE_OAUTH_TOKEN'];

  if (!oauthToken) {
    console.error('Missing required environment variable: CLAUDE_CODE_OAUTH_TOKEN');
    process.exit(1);
  }

  const required: Record<string, string | undefined> = {
    DISCORD_BOT_TOKEN: process.env['DISCORD_BOT_TOKEN'],
    GITHUB_TOKEN: process.env['GITHUB_TOKEN'],
    ALL_CHAT_REPO_PATH: process.env['ALL_CHAT_REPO_PATH'],
    ALL_CHAT_EXTENSION_REPO_PATH: process.env['ALL_CHAT_EXTENSION_REPO_PATH'],
    LEAD_DEVELOPER_DISCORD_ID: process.env['LEAD_DEVELOPER_DISCORD_ID'],
    GRAFANA_URL: process.env['GRAFANA_URL'],
    GRAFANA_SERVICE_ACCOUNT_TOKEN: process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'],
    DATABASE_URL: process.env['DATABASE_URL'],
  };

  for (const [name, value] of Object.entries(required)) {
    if (!value) {
      console.error(`Missing required environment variable: ${name}`);
      process.exit(1);
    }
  }

  return {
    discordToken: required['DISCORD_BOT_TOKEN'] as string,
    claudeOAuthToken: oauthToken,
    githubToken: required['GITHUB_TOKEN'] as string,
    githubOwner: process.env['GITHUB_OWNER'] ?? 'moersener',
    allChatRepoPath: required['ALL_CHAT_REPO_PATH'] as string,
    allChatExtensionRepoPath: required['ALL_CHAT_EXTENSION_REPO_PATH'] as string,
    leadDeveloperDiscordId: required['LEAD_DEVELOPER_DISCORD_ID'] as string,
    grafanaUrl: required['GRAFANA_URL'] as string,
    grafanaServiceAccountToken: required['GRAFANA_SERVICE_ACCOUNT_TOKEN'] as string,
    databaseUrl: required['DATABASE_URL'] as string,
    moderationGuildId: process.env['MODERATION_GUILD_ID'],
  };
}

export async function shutdown(client?: { destroy: () => void }, pool?: pg.Pool): Promise<void> {
  console.log('Shutting down support-bot...');
  client?.destroy();
  await pool?.end();
  process.exit(0);
}

let discordClient: Client | undefined;
let pgPool: pg.Pool | undefined;

process.on('SIGINT', () => void shutdown(discordClient, pgPool));
process.on('SIGTERM', () => void shutdown(discordClient, pgPool));

async function main(): Promise<void> {
  const config = validateEnv();

  const pool = new pg.Pool({
    connectionString: config.databaseUrl,
    max: 5,
    idleTimeoutMillis: 30_000,
    connectionTimeoutMillis: 2_000,
  });
  await pool.query('SELECT 1');
  console.log('[db] PostgreSQL connection established');
  pgPool = pool;
  const memoryRepo = new MemoryRepository(pool);

  try {
    discordClient = await startBot(config, memoryRepo);
  } catch (error) {
    console.error('Failed to start bot:', error);
    process.exit(1);
  }
}

const isMain = process.argv[1]?.endsWith('index.ts') || process.argv[1]?.endsWith('index.js');
if (isMain) {
  void main();
}
