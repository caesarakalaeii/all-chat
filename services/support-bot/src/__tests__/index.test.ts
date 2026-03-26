import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// We need to test validateEnv and shutdown which are exported from index.ts
// We mock process.exit and process.env to test behavior

describe('validateEnv', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    vi.resetModules();
    process.env = {
      ...originalEnv,
      DISCORD_BOT_TOKEN: 'discord-token',
      CLAUDE_CODE_OAUTH_TOKEN: 'claude-token',
      GITHUB_TOKEN: 'github-token',
      ALL_CHAT_REPO_PATH: '/repos/all-chat',
      ALL_CHAT_EXTENSION_REPO_PATH: '/repos/all-chat-extension',
      LEAD_DEVELOPER_DISCORD_ID: '198569499228766208',
      GRAFANA_URL: 'https://grafana.caes.ar',
      GRAFANA_SERVICE_ACCOUNT_TOKEN: 'test-grafana-token',
      DATABASE_URL: 'postgresql://test:test@localhost:5432/testdb',
    };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it('calls process.exit(1) when DISCORD_BOT_TOKEN is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['DISCORD_BOT_TOKEN'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('DISCORD_BOT_TOKEN'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when CLAUDE_CODE_OAUTH_TOKEN is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['CLAUDE_CODE_OAUTH_TOKEN'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('CLAUDE_CODE_OAUTH_TOKEN'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when GITHUB_TOKEN is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['GITHUB_TOKEN'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('GITHUB_TOKEN'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when ALL_CHAT_REPO_PATH is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['ALL_CHAT_REPO_PATH'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('ALL_CHAT_REPO_PATH'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when ALL_CHAT_EXTENSION_REPO_PATH is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['ALL_CHAT_EXTENSION_REPO_PATH'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('ALL_CHAT_EXTENSION_REPO_PATH'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when LEAD_DEVELOPER_DISCORD_ID is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['LEAD_DEVELOPER_DISCORD_ID'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('LEAD_DEVELOPER_DISCORD_ID'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when GRAFANA_URL is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['GRAFANA_URL'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('GRAFANA_URL'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when GRAFANA_SERVICE_ACCOUNT_TOKEN is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('GRAFANA_SERVICE_ACCOUNT_TOKEN'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls process.exit(1) when DATABASE_URL is missing', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    delete process.env['DATABASE_URL'];

    const { validateEnv } = await import('../index.js');
    validateEnv();

    expect(exitSpy).toHaveBeenCalledWith(1);
    const errorMessages = consoleSpy.mock.calls.map((call) => String(call[0]));
    expect(errorMessages.some((msg) => msg.includes('DATABASE_URL'))).toBe(true);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('returns a BotConfig when all required vars are set', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);

    const { validateEnv } = await import('../index.js');
    const config = validateEnv();

    expect(exitSpy).not.toHaveBeenCalled();
    expect(config).toBeDefined();
    expect(config.discordToken).toBe('discord-token');
    expect(config.claudeOAuthToken).toBe('claude-token');
    expect(config.githubToken).toBe('github-token');
    expect(config.allChatRepoPath).toBe('/repos/all-chat');
    expect(config.allChatExtensionRepoPath).toBe('/repos/all-chat-extension');
    expect(config.leadDeveloperDiscordId).toBe('198569499228766208');
    expect(config.grafanaUrl).toBe('https://grafana.caes.ar');
    expect(config.grafanaServiceAccountToken).toBe('test-grafana-token');
    expect(config.databaseUrl).toBe('postgresql://test:test@localhost:5432/testdb');

    exitSpy.mockRestore();
  });
});

describe('shutdown', () => {
  it('calls client.destroy() when client is provided', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => undefined);

    const { shutdown } = await import('../index.js');
    const mockClient = { destroy: vi.fn() };
    await shutdown(mockClient);

    expect(mockClient.destroy).toHaveBeenCalled();
    expect(exitSpy).toHaveBeenCalledWith(0);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('does not throw when called without a client', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => undefined);

    const { shutdown } = await import('../index.js');
    await expect(shutdown()).resolves.not.toThrow();

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });

  it('calls pool.end() when pool is provided', async () => {
    const exitSpy = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
    const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => undefined);

    const { shutdown } = await import('../index.js');
    const mockPool = { end: vi.fn().mockResolvedValue(undefined) };
    await shutdown(undefined, mockPool as unknown as import('pg').Pool);

    expect(mockPool.end).toHaveBeenCalled();
    expect(exitSpy).toHaveBeenCalledWith(0);

    exitSpy.mockRestore();
    consoleSpy.mockRestore();
  });
});
