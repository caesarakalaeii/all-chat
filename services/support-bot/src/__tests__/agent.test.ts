import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock execa module
vi.mock('execa', () => ({
  execa: vi.fn(),
}));

import { execa } from 'execa';
import { queryCodebase } from '../claude/agent.js';

const mockExeca = vi.mocked(execa);

describe('queryCodebase', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    process.env['CLAUDE_CODE_OAUTH_TOKEN'] = 'test-oauth-token';
    process.env['GRAFANA_URL'] = 'https://grafana.caes.ar';
    process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'] = 'test-grafana-token';
  });

  afterEach(() => {
    delete process.env['GRAFANA_URL'];
    delete process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'];
  });

  it('returns an answer string from the parsed JSON output', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Twitch IRC is used for chat messages.' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('how does twitch work?', ['/repos/all-chat']);

    expect(result.answer).toBe('Twitch IRC is used for chat messages.');
    expect(result.issueProposal).toBeNull();
    expect(result.infraVerdict).toBeNull();
  });

  it('passes --allowedTools with Read,Glob,Grep,Bash(kubectl:*) and Grafana MCP tools when Grafana env vars are set', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Some answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat']);

    expect(mockExeca).toHaveBeenCalledOnce();
    const [, args] = mockExeca.mock.calls[0];
    const allowedToolsIndex = (args as string[]).indexOf('--allowedTools');
    expect(allowedToolsIndex).toBeGreaterThan(-1);
    const allowedToolsValue = (args as string[])[allowedToolsIndex + 1];
    expect(allowedToolsValue).toContain('Read');
    expect(allowedToolsValue).toContain('Glob');
    expect(allowedToolsValue).toContain('Grep');
    expect(allowedToolsValue).toContain('Bash(kubectl:*)');
    expect(allowedToolsValue).toContain('mcp__grafana-caesar__query_loki_logs');
    // Ensure Write and Edit are not in allowedTools
    expect(allowedToolsValue).not.toContain('Write');
    expect(allowedToolsValue).not.toContain('Edit');
  });

  it('passes --allowedTools as Read,Glob,Grep,Bash(kubectl:*) only when Grafana env vars are absent', async () => {
    delete process.env['GRAFANA_URL'];
    delete process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'];

    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Some answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat']);

    expect(mockExeca).toHaveBeenCalledOnce();
    const [, args] = mockExeca.mock.calls[0];
    const allowedToolsIndex = (args as string[]).indexOf('--allowedTools');
    expect(allowedToolsIndex).toBeGreaterThan(-1);
    const allowedToolsValue = (args as string[])[allowedToolsIndex + 1];
    expect(allowedToolsValue).toBe('Read,Glob,Grep,Bash(kubectl:*)');
    expect(allowedToolsValue).not.toContain('mcp__grafana-caesar');
    expect(allowedToolsValue).not.toContain('Write');
    expect(allowedToolsValue).not.toContain('Edit');
  });

  it('includes --mcp-config with grafana-caesar server when Grafana env vars are set', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Some answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const mcpConfigIndex = (args as string[]).indexOf('--mcp-config');
    expect(mcpConfigIndex).toBeGreaterThan(-1);
    const mcpConfigValue = (args as string[])[mcpConfigIndex + 1];
    const mcpConfig = JSON.parse(mcpConfigValue) as {
      mcpServers: {
        'grafana-caesar': {
          command: string;
          args: string[];
          env: { GRAFANA_URL: string; GRAFANA_SERVICE_ACCOUNT_TOKEN: string };
        };
      };
    };
    expect(mcpConfig.mcpServers['grafana-caesar']).toBeDefined();
    expect(mcpConfig.mcpServers['grafana-caesar'].env['GRAFANA_URL']).toBe('https://grafana.caes.ar');
    expect(mcpConfig.mcpServers['grafana-caesar'].env['GRAFANA_SERVICE_ACCOUNT_TOKEN']).toBe('test-grafana-token');
  });

  it('omits --mcp-config entirely when GRAFANA_URL is missing', async () => {
    delete process.env['GRAFANA_URL'];

    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Some answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const mcpConfigIndex = (args as string[]).indexOf('--mcp-config');
    expect(mcpConfigIndex).toBe(-1);
  });

  it('omits --mcp-config entirely when GRAFANA_SERVICE_ACCOUNT_TOKEN is missing', async () => {
    delete process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'];

    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Some answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const mcpConfigIndex = (args as string[]).indexOf('--mcp-config');
    expect(mcpConfigIndex).toBe(-1);
  });

  it('parses INFRA_VERDICT:infrastructure into infraVerdict field', async () => {
    const responseText =
      'The auth service is having issues.\n\nINFRA_VERDICT:infrastructure|||auth-service crashed 5 times';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('why is auth broken?', ['/repos/all-chat']);

    expect(result.infraVerdict).not.toBeNull();
    expect(result.infraVerdict?.type).toBe('infrastructure');
    expect(result.infraVerdict?.summary).toBe('auth-service crashed 5 times');
  });

  it('parses INFRA_VERDICT:code into infraVerdict field', async () => {
    const responseText =
      'There is a code issue.\n\nINFRA_VERDICT:code|||missing null check in handler';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('what is wrong with the handler?', ['/repos/all-chat']);

    expect(result.infraVerdict).not.toBeNull();
    expect(result.infraVerdict?.type).toBe('code');
    expect(result.infraVerdict?.summary).toBe('missing null check in handler');
  });

  it('strips INFRA_VERDICT marker from result.answer', async () => {
    const responseText =
      'The auth service is having issues.\n\nINFRA_VERDICT:infrastructure|||auth-service crashed 5 times';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('why is auth broken?', ['/repos/all-chat']);

    expect(result.answer).not.toContain('INFRA_VERDICT:');
    expect(result.answer).toBe('The auth service is having issues.');
  });

  it('returns infraVerdict: null when response has no INFRA_VERDICT marker', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Just a normal answer without any verdict.' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('some question', ['/repos/all-chat']);

    expect(result.infraVerdict).toBeNull();
  });

  it('uses timeout of 600_000ms (not 180_000ms)', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('question', ['/repos/all-chat']);

    const [, , options] = mockExeca.mock.calls[0] as [string, string[], { timeout: number }];
    expect(options.timeout).toBe(600_000);
  });

  it('system prompt contains "NEVER include raw log lines" leak prevention guardrail', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1]; // '-p' is index 0, prompt is index 1
    expect(prompt).toContain('NEVER include raw log lines');
  });

  it('system prompt contains INFRA_VERDICT: instruction text', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).toContain('INFRA_VERDICT:');
  });

  it('parses PROPOSE_ISSUE in the response into an IssueProposal', async () => {
    const responseText =
      'The fix is straightforward.\n\nPROPOSE_ISSUE:all-chat|||Fix the bug|||## Details\nThis bug needs fixing.';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('there is a bug', ['/repos/all-chat']);

    expect(result.issueProposal).not.toBeNull();
    expect(result.issueProposal?.repo).toBe('all-chat');
    expect(result.issueProposal?.title).toBe('Fix the bug');
    expect(result.issueProposal?.body).toBe('## Details\nThis bug needs fixing.');
    expect(result.answer).toContain('The fix is straightforward.');
  });

  it('passes CLAUDE_CODE_OAUTH_TOKEN in the env option', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('question', ['/repos/all-chat']);

    const [, , options] = mockExeca.mock.calls[0] as [string, string[], { env: NodeJS.ProcessEnv }];
    expect(options.env).toBeDefined();
    expect(options.env['CLAUDE_CODE_OAUTH_TOKEN']).toBe('test-oauth-token');
  });

  it('includes conversation history in prompt when conversationHistory is provided', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'followup answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const history = ['User: first question', 'Bot: first answer'];
    await queryCodebase('followup question', ['/repos/all-chat'], history);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1]; // '-p' is index 0, prompt is index 1
    expect(prompt).toContain('Conversation so far:');
    expect(prompt).toContain('User: first question');
    expect(prompt).toContain('Bot: first answer');
  });

  it('does not include "Conversation so far:" when conversationHistory is empty', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'fresh answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('fresh question', ['/repos/all-chat'], []);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).not.toContain('Conversation so far:');
  });

  it('does not include "Conversation so far:" when conversationHistory is undefined', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'fresh answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('fresh question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).not.toContain('Conversation so far:');
  });

  // Memory injection tests

  it('injects "## Relevant memories:" block into prompt when memories are non-empty', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer about kick' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const memories = [
      { id: 42, type: 'error_pattern' as const, tags: ['kick-listener', 'oomkill'], content: 'kick-listener OOMs under heavy load', accessCount: 3, updatedAt: new Date() },
    ];

    await queryCodebase('why does kick crash?', ['/repos/all-chat'], undefined, memories);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).toContain('## Relevant memories:');
    expect(prompt).toContain('[error_pattern] (id:42) kick-listener OOMs under heavy load');
  });

  it('does NOT inject "## Relevant memories:" when memories array is empty', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'fresh answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat'], undefined, []);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).not.toContain('## Relevant memories:');
  });

  it('does NOT inject "## Relevant memories:" when memories is undefined', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'fresh answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat'], undefined, undefined);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).not.toContain('## Relevant memories:');
  });

  it('memory block format includes id for UPDATE_MEMORY', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const memories = [
      { id: 42, type: 'error_pattern' as const, tags: ['kick-listener'], content: 'some content', accessCount: 1, updatedAt: new Date() },
    ];

    await queryCodebase('question', ['/repos/all-chat'], undefined, memories);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).toContain('(id:42)');
  });

  it('injects memories block after system prompt and before "## Conversation so far:" when both are present', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const memories = [
      { id: 1, type: 'codebase_insight' as const, tags: ['api-gateway'], content: 'gateway uses WebSocket', accessCount: 0, updatedAt: new Date() },
    ];
    const history = ['User: what is the gateway?', 'Bot: it handles WebSocket'];

    await queryCodebase('follow up?', ['/repos/all-chat'], history, memories);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    const memoriesPos = prompt.indexOf('## Relevant memories:');
    const historyPos = prompt.indexOf('## Conversation so far:');
    expect(memoriesPos).toBeGreaterThan(-1);
    expect(historyPos).toBeGreaterThan(-1);
    expect(memoriesPos).toBeLessThan(historyPos);
  });

  // STORE_MEMORY parsing tests

  it('parses STORE_MEMORY marker into memoryMarker field on QueryResult', async () => {
    const responseText =
      'kick-listener has memory issues.\n\nSTORE_MEMORY:error_pattern|||kick-listener,oomkill|||kick-listener OOMs under heavy load';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('why does kick crash?', ['/repos/all-chat']);

    expect(result.memoryMarker).not.toBeNull();
    expect(result.memoryMarker?.type).toBe('error_pattern');
    expect(result.memoryMarker?.tags).toEqual(['kick-listener', 'oomkill']);
    expect(result.memoryMarker?.content).toBe('kick-listener OOMs under heavy load');
  });

  it('strips STORE_MEMORY marker from answer text', async () => {
    const responseText =
      'kick-listener has memory issues.\n\nSTORE_MEMORY:error_pattern|||kick-listener,oomkill|||kick-listener OOMs under heavy load';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('why does kick crash?', ['/repos/all-chat']);

    expect(result.answer).not.toContain('STORE_MEMORY:');
    expect(result.answer).toBe('kick-listener has memory issues.');
  });

  it('parses UPDATE_MEMORY marker into updateMemoryMarker field on QueryResult', async () => {
    const responseText =
      'I have updated my knowledge.\n\nUPDATE_MEMORY:42|||updated insight about kick-listener';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('any updates?', ['/repos/all-chat']);

    expect(result.updateMemoryMarker).not.toBeNull();
    expect(result.updateMemoryMarker?.id).toBe(42);
    expect(result.updateMemoryMarker?.content).toBe('updated insight about kick-listener');
  });

  it('strips UPDATE_MEMORY marker from answer text', async () => {
    const responseText =
      'I have updated my knowledge.\n\nUPDATE_MEMORY:42|||updated insight about kick-listener';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('any updates?', ['/repos/all-chat']);

    expect(result.answer).not.toContain('UPDATE_MEMORY:');
    expect(result.answer).toBe('I have updated my knowledge.');
  });

  it('returns memoryMarker: null when no STORE_MEMORY in response', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Just a normal answer.' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('some question', ['/repos/all-chat']);

    expect(result.memoryMarker).toBeNull();
  });

  it('returns updateMemoryMarker: null when no UPDATE_MEMORY in response', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Just a normal answer.' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('some question', ['/repos/all-chat']);

    expect(result.updateMemoryMarker).toBeNull();
  });

  it('parses both INFRA_VERDICT and STORE_MEMORY: INFRA_VERDICT stripped first, then STORE_MEMORY from remaining', async () => {
    const responseText =
      'kick-listener is crashing due to OOM.\n\nSTORE_MEMORY:error_pattern|||kick-listener,oomkill|||kick-listener OOMs under heavy load\n\nINFRA_VERDICT:infrastructure|||kick-listener had 3 OOMKill restarts';
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: responseText }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('why does kick crash?', ['/repos/all-chat']);

    expect(result.infraVerdict).not.toBeNull();
    expect(result.infraVerdict?.type).toBe('infrastructure');
    expect(result.memoryMarker).not.toBeNull();
    expect(result.memoryMarker?.type).toBe('error_pattern');
    expect(result.answer).not.toContain('INFRA_VERDICT:');
    expect(result.answer).not.toContain('STORE_MEMORY:');
  });

  it('system prompt contains STORE_MEMORY instruction text', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).toContain('STORE_MEMORY:');
  });

  it('system prompt contains "Memory content must be concise"', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('question', ['/repos/all-chat']);

    const [, args] = mockExeca.mock.calls[0];
    const prompt = (args as string[])[1];
    expect(prompt).toContain('Memory content must be concise');
  });
});
