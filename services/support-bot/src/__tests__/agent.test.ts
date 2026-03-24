import { describe, it, expect, vi, beforeEach } from 'vitest';

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
    process.env.CLAUDE_CODE_OAUTH_TOKEN = 'test-oauth-token';
  });

  it('returns an answer string from the parsed JSON output', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Twitch IRC is used for chat messages.' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    const result = await queryCodebase('how does twitch work?', ['/repos/all-chat']);

    expect(result.answer).toBe('Twitch IRC is used for chat messages.');
    expect(result.issueProposal).toBeNull();
  });

  it('passes --allowedTools as Read,Glob,Grep (read-only tools only)', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: JSON.stringify({ result: 'Some answer' }),
    } as ReturnType<typeof execa> extends Promise<infer T> ? T : never);

    await queryCodebase('some question', ['/repos/all-chat']);

    expect(mockExeca).toHaveBeenCalledOnce();
    const [, args] = mockExeca.mock.calls[0];
    const allowedToolsIndex = (args as string[]).indexOf('--allowedTools');
    expect(allowedToolsIndex).toBeGreaterThan(-1);
    const allowedToolsValue = (args as string[])[allowedToolsIndex + 1];
    expect(allowedToolsValue).toBe('Read,Glob,Grep');
    // Ensure Write, Edit, Bash are not in allowedTools
    expect(allowedToolsValue).not.toContain('Write');
    expect(allowedToolsValue).not.toContain('Edit');
    expect(allowedToolsValue).not.toContain('Bash');
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
});
