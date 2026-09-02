import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock discord.js
vi.mock('discord.js', () => {
  const mockMessages = new Map([
    ['msg1', { id: 'msg1', content: 'prior question', author: { bot: false, username: 'Alice', id: 'user1' } }],
    ['msg2', { id: 'msg2', content: 'another question', author: { bot: false, username: 'Bob', id: 'user2' } }],
    ['botmsg', { id: 'botmsg', content: 'bot reply', author: { bot: true, username: 'Bot', id: 'bot1' } }],
  ]);

  const mockThread = {
    id: 'thread1',
    isThread: () => true,
    ownerId: '123456789',
    messages: {
      fetch: vi.fn().mockResolvedValue(mockMessages),
    },
    send: vi.fn().mockResolvedValue({}),
    sendTyping: vi.fn().mockResolvedValue(undefined),
  };

  const mockChannel = {
    id: 'channel1',
    isThread: () => false,
    send: vi.fn().mockResolvedValue({}),
    sendTyping: vi.fn().mockResolvedValue(undefined),
  };

  const mockReply = {
    id: 'reply1',
    startThread: vi.fn().mockResolvedValue(mockThread),
  };

  const mockMessage = {
    id: 'msg-main',
    content: '<@123456789> how does twitch work?',
    author: { bot: false, username: 'Alice', id: 'user-alice' },
    channel: mockChannel,
    mentions: {
      has: vi.fn().mockReturnValue(true),
    },
    reply: vi.fn().mockResolvedValue(mockReply),
    startThread: vi.fn().mockResolvedValue(mockThread),
  };

  const mockInteraction = {
    isChatInputCommand: vi.fn().mockReturnValue(true),
    commandName: 'support',
    options: {
      getString: vi.fn().mockReturnValue('how does twitch work?'),
    },
    deferReply: vi.fn().mockResolvedValue(undefined),
    editReply: vi.fn().mockResolvedValue({ id: 'reply1', startThread: vi.fn().mockResolvedValue({}) }),
    fetchReply: vi.fn().mockResolvedValue({ id: 'reply1', startThread: vi.fn().mockResolvedValue({}) }),
  };

  const mockClientUser = { id: '123456789', tag: 'SupportBot#0000' };

  const mockClientInstance = {
    user: mockClientUser,
    on: vi.fn(),
    once: vi.fn(),
    login: vi.fn().mockResolvedValue('TOKEN'),
    destroy: vi.fn(),
  };

  const Client = vi.fn().mockImplementation(() => mockClientInstance);

  const EmbedBuilder = vi.fn().mockImplementation(() => ({
    setDescription: vi.fn().mockReturnThis(),
  }));

  return {
    Client,
    GatewayIntentBits: {
      Guilds: 1,
      GuildMessages: 2,
      MessageContent: 4,
    },
    Events: {
      MessageCreate: 'messageCreate',
      InteractionCreate: 'interactionCreate',
    },
    PermissionFlagsBits: {
      BanMembers: 1n << 2n,
      ManageMessages: 1n << 13n,
      Administrator: 1n << 3n,
    },
    EmbedBuilder,
    SlashCommandBuilder: vi.fn().mockImplementation(() => ({
      setName: vi.fn().mockReturnThis(),
      setDescription: vi.fn().mockReturnThis(),
      addStringOption: vi.fn().mockReturnThis(),
      toJSON: vi.fn().mockReturnValue({ name: 'support' }),
    })),
  };
});

// Mock the agent module
vi.mock('../claude/agent.js', () => ({
  queryCodebase: vi.fn(),
  // bot.ts reads this to tell the user how long it waited. The mock must export it, or
  // the timeout path throws on a missing mock export instead of replying.
  CLAUDE_TIMEOUT_MS: 600_000,
}));

// Mock the github issues module
vi.mock('../github/issues.js', () => ({
  createOctokitClient: vi.fn().mockReturnValue({ mock: 'octokit' }),
  createIssue: vi.fn().mockResolvedValue('https://github.com/owner/repo/issues/123'),
  createComment: vi.fn().mockResolvedValue('https://github.com/owner/repo/issues/447#issuecomment-1'),
}));

// Mock the memory repository module
vi.mock('../memory/repository.js', () => ({
  MemoryRepository: vi.fn().mockImplementation(() => ({
    retrieveMemories: vi.fn().mockResolvedValue([]),
    storeMemory: vi.fn().mockResolvedValue(undefined),
    updateMemory: vi.fn().mockResolvedValue(undefined),
  })),
  extractTagsFromQuestion: vi.fn().mockReturnValue([]),
}));

import { Client, GatewayIntentBits, Events, EmbedBuilder } from 'discord.js';
import { queryCodebase } from '../claude/agent.js';
import { createComment, createIssue, createOctokitClient } from '../github/issues.js';
import { MemoryRepository } from '../memory/repository.js';
import { startBot } from '../bot.js';

const mockQueryCodebase = vi.mocked(queryCodebase);
const mockCreateIssue = vi.mocked(createIssue);
const mockCreateComment = vi.mocked(createComment);
const mockCreateOctokitClient = vi.mocked(createOctokitClient);

const testConfig = {
  discordToken: 'test-discord-token',
  claudeOAuthToken: 'test-oauth-token',
  githubToken: 'test-github-token',
  githubOwner: 'testowner',
  allChatRepoPath: '/repos/all-chat',
  allChatExtensionRepoPath: '/repos/all-chat-extension',
  leadDeveloperDiscordId: '198569499228766208',
  grafanaUrl: 'https://grafana.caes.ar',
  grafanaServiceAccountToken: 'test-grafana-token',
  databaseUrl: 'postgresql://test:test@localhost:5432/testdb',
};

function createMockMemoryRepo() {
  return new MemoryRepository({} as import('pg').Pool);
}

function getClientInstance() {
  return vi.mocked(Client).mock.results[0]?.value as ReturnType<typeof Client>;
}

function getEventHandler(eventName: string) {
  const instance = getClientInstance();
  const calls = vi.mocked(instance.on).mock.calls;
  const call = calls.find(([event]) => event === eventName);
  return call?.[1] as ((...args: unknown[]) => Promise<void>) | undefined;
}

describe('startBot', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQueryCodebase.mockResolvedValue({
      answer: 'Twitch uses IRC protocol for chat.',
      issueProposals: [],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });
  });

  it('creates a Discord client with Guilds, GuildMessages, and MessageContent intents', async () => {
    await startBot(testConfig, createMockMemoryRepo());

    expect(Client).toHaveBeenCalledWith({
      intents: expect.arrayContaining([
        GatewayIntentBits.Guilds,
        GatewayIntentBits.GuildMessages,
        GatewayIntentBits.MessageContent,
      ]),
    });
  });

  it('logs in with the discord token', async () => {
    await startBot(testConfig, createMockMemoryRepo());

    const instance = getClientInstance();
    expect(instance.login).toHaveBeenCalledWith(testConfig.discordToken);
  });

  it('registers MessageCreate and InteractionCreate handlers', async () => {
    await startBot(testConfig, createMockMemoryRepo());

    const instance = getClientInstance();
    const registeredEvents = vi.mocked(instance.on).mock.calls.map(([event]) => event);
    expect(registeredEvents).toContain(Events.MessageCreate);
    expect(registeredEvents).toContain(Events.InteractionCreate);
  });

  it('returns the Discord client', async () => {
    const client = await startBot(testConfig, createMockMemoryRepo());

    expect(client).toBeDefined();
    expect(client).toBe(getClientInstance());
  });
});

function buildMessage(overrides: Partial<{
  content: string;
  authorBot: boolean;
  isThread: boolean;
  isBotThread: boolean;
  mentionsBot: boolean;
  mentionsEveryone: boolean;
  mentionsViaRole: boolean;
  channelSend: ReturnType<typeof vi.fn>;
  startThread: ReturnType<typeof vi.fn>;
}> = {}) {
  const defaults = {
    content: '<@123456789> how does twitch work?',
    authorBot: false,
    isThread: false,
    isBotThread: false,
    mentionsBot: true,
    mentionsEveryone: false,
    mentionsViaRole: false,
    channelSend: vi.fn().mockResolvedValue({}),
    startThread: vi.fn().mockResolvedValue({ id: 'new-thread', send: vi.fn().mockResolvedValue({}) }),
  };
  const opts = { ...defaults, ...overrides };

  const channel: {
    id: string;
    isThread: () => boolean;
    ownerId?: string;
    send: ReturnType<typeof vi.fn>;
    sendTyping: ReturnType<typeof vi.fn>;
    messages?: { fetch: ReturnType<typeof vi.fn> };
  } = {
    id: 'channel1',
    isThread: () => opts.isThread,
    send: opts.channelSend,
    sendTyping: vi.fn().mockResolvedValue(undefined),
  };

  if (opts.isThread) {
    channel.ownerId = opts.isBotThread ? '123456789' : 'other-user-id';
    channel.messages = {
      fetch: vi.fn().mockResolvedValue(new Map([
        ['m1', { id: 'm1', content: 'prior question', author: { bot: false, username: 'Alice' } }],
        ['m2', { id: 'm2', content: 'bot reply', author: { bot: true, username: 'Bot' } }],
      ])),
    };
  }

  return {
    id: 'msg-main',
    content: opts.content,
    author: { bot: opts.authorBot, username: 'Alice', id: 'user-alice' },
    channel,
    channelId: 'channel1',
    attachments: new Map(),
    mentions: {
      // Faithful model of discord.js MessageMentions.has(userId, options):
      // a direct user mention lives in `users`, @here/@everyone is `everyone`,
      // and a role ping is `roles`. The options let callers exclude categories.
      has: vi.fn().mockImplementation((id: string, options: {
        ignoreDirect?: boolean;
        ignoreRoles?: boolean;
        ignoreEveryone?: boolean;
      } = {}) => {
        if (!options.ignoreDirect && opts.mentionsBot && id === '123456789') return true;
        if (!options.ignoreEveryone && opts.mentionsEveryone) return true;
        if (!options.ignoreRoles && opts.mentionsViaRole) return true;
        return false;
      }),
    },
    reply: vi.fn().mockResolvedValue({ id: 'reply1', startThread: vi.fn().mockResolvedValue({ id: 'r-thread', send: vi.fn().mockResolvedValue({}) }) }),
    startThread: opts.startThread,
  };
}

describe('MessageCreate handler', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQueryCodebase.mockResolvedValue({
      answer: 'Twitch uses IRC.',
      issueProposals: [],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });
  });

  it('ignores messages from bots', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);
    expect(handler).toBeDefined();

    const msg = buildMessage({ authorBot: true });
    await handler!(msg);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('ignores messages that are not mentions and not in bot threads', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ mentionsBot: false, isThread: false });
    await handler!(msg);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('ignores @everyone/@here even though the bot is technically mentioned', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({
      content: '@everyone please read this',
      mentionsBot: false,
      mentionsEveryone: true,
    });
    await handler!(msg);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('ignores role pings that include the bot', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({
      content: '<@&999> heads up team',
      mentionsBot: false,
      mentionsViaRole: true,
    });
    await handler!(msg);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('still responds to a direct @mention sent alongside @everyone', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({
      content: '<@123456789> @everyone how does twitch work?',
      mentionsBot: true,
      mentionsEveryone: true,
    });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalled();
  });

  it('strips mention tags from content before calling queryCodebase', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@123456789> how does twitch work?' });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'how does twitch work?',
      [testConfig.allChatRepoPath, testConfig.allChatExtensionRepoPath],
      [],
      [],
    );
  });

  it('calls queryCodebase with empty history for @mention in non-thread channel', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@123456789> explain the architecture', isThread: false });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'explain the architecture',
      expect.any(Array),
      [],
      expect.any(Array),
    );
  });

  it('creates a thread after replying to @mention in non-thread channel', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const startThread = vi.fn().mockResolvedValue({ id: 'new-thread' });
    const msg = buildMessage({ content: '<@123456789> how does twitch work?', isThread: false, startThread });
    await handler!(msg);

    expect(startThread).toHaveBeenCalledWith(expect.objectContaining({
      name: expect.any(String),
      autoArchiveDuration: 1440,
    }));
  });

  it('collects thread history when @mentioned in a bot-owned thread', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({
      content: '<@123456789> follow-up question',
      isThread: true,
      isBotThread: true,
    });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'follow-up question',
      expect.any(Array),
      expect.arrayContaining(['[Alice]: prior question']),
      expect.any(Array),
    );
  });

  it('handles message in bot-owned thread without @mention as follow-up', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({
      content: 'follow-up without mention',
      mentionsBot: false,
      isThread: true,
      isBotThread: true,
    });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'follow-up without mention',
      expect.any(Array),
      expect.any(Array),
      expect.any(Array),
    );
  });

  it('does NOT create a new thread when already in a thread', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const startThread = vi.fn().mockResolvedValue({ id: 'new-thread' });
    const msg = buildMessage({
      content: '<@123456789> question in thread',
      isThread: true,
      isBotThread: true,
      startThread,
    });
    await handler!(msg);

    expect(startThread).not.toHaveBeenCalled();
  });

  it('calls createIssue when queryCodebase returns an issueProposal and includes URL in reply', async () => {
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'We need to fix this.\n\nPROPOSE_ISSUE:all-chat|||Fix bug|||Bug body',
      issueProposals: [{
        repo: 'all-chat',
        title: 'Fix bug',
        body: 'Bug body',
      }],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });
    mockCreateIssue.mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/42');

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> there is a bug',
      startThread: vi.fn().mockResolvedValue({ id: 'issue-thread', send: threadSend }),
    });
    await handler!(msg);

    expect(mockCreateIssue).toHaveBeenCalledWith(
      expect.anything(),
      testConfig.githubOwner,
      'all-chat',
      'Fix bug',
      'Bug body',
    );
    // The sent message should contain the issue URL
    const sendCalls = threadSend.mock.calls;
    const allSentContent = sendCalls.map(([arg]) => typeof arg === 'string' ? arg : JSON.stringify(arg)).join(' ');
    expect(allSentContent).toContain('https://github.com/testowner/all-chat/issues/42');
  });

  it('files EVERY proposed issue, not just the first', async () => {
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'Filing these.',
      issueProposals: [
        { repo: 'all-chat', title: 'One', body: 'Body one' },
        { repo: 'all-chat', title: 'Two', body: 'Body two' },
        { repo: 'all-chat-extension', title: 'Three', body: 'Body three' },
      ],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });
    mockCreateIssue
      .mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/1')
      .mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/2')
      .mockResolvedValueOnce('https://github.com/testowner/all-chat-extension/issues/3');

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> file these four things',
      startThread: vi.fn().mockResolvedValue({ id: 'multi-thread', send: threadSend }),
    });
    await handler!(msg);

    expect(mockCreateIssue).toHaveBeenCalledTimes(3);
    const sent = threadSend.mock.calls
      .map(([arg]) => (typeof arg === 'string' ? arg : JSON.stringify(arg)))
      .join(' ');
    expect(sent).toContain('/issues/1');
    expect(sent).toContain('/issues/2');
    expect(sent).toContain('/issues/3');
  });

  it('reports the issues it could not file without losing the ones it could', async () => {
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'Filing these.',
      issueProposals: [
        { repo: 'all-chat', title: 'Works', body: 'Body' },
        { repo: 'all-chat', title: 'Breaks', body: 'Body' },
      ],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });
    mockCreateIssue
      .mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/9')
      .mockRejectedValueOnce(new Error('GitHub said no'));

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> file these',
      startThread: vi.fn().mockResolvedValue({ id: 'partial-thread', send: threadSend }),
    });
    await handler!(msg);

    const sent = threadSend.mock.calls
      .map(([arg]) => (typeof arg === 'string' ? arg : JSON.stringify(arg)))
      .join(' ');
    // The one that worked is still reported...
    expect(sent).toContain('/issues/9');
    // ...and the one that did not is named, rather than silently dropped.
    expect(sent).toContain('Breaks');
    expect(sent).toContain('Could NOT create');
  });

  it('answers a subprocess TIMEOUT with how long it waited and how to retry', async () => {
    mockQueryCodebase.mockRejectedValueOnce(
      Object.assign(new Error('Command timed out after 600000 milliseconds'), { timedOut: true }),
    );

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@123456789> do something enormous' });
    await handler!(msg);

    expect(msg.reply).toHaveBeenCalledOnce();
    const reply = msg.reply.mock.calls[0][0] as string;
    expect(reply).toContain('10 minutes');
    expect(reply).toContain('one issue or one area at a time');
    // "check the bot logs" is useless to the streamer who asked, and this failure is not
    // a fault to be investigated — the work simply did not fit in the budget.
    expect(reply).not.toContain('Check the bot logs');
  });

  it('answers a non-timeout failure with the generic message', async () => {
    mockQueryCodebase.mockRejectedValueOnce(new Error('something else broke'));

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@123456789> hello' });
    await handler!(msg);

    const reply = msg.reply.mock.calls[0][0] as string;
    expect(reply).toContain('Check the bot logs');
  });

  it('redacts credentials out of a logged failure', async () => {
    // Assembled at runtime so this file holds no literal that LOOKS like a Grafana token.
    // A hard-coded `glsa_…` fixture trips secret scanners (GitGuardian flagged exactly
    // that on the first push of this change), and a test fixture that cries wolf teaches
    // people to wave the scanner through.
    const fakeToken = ['glsa', 'fake', 'value', 'for', 'tests'].join('_');
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    mockQueryCodebase.mockRejectedValueOnce(
      new Error(`boom: "GRAFANA_SERVICE_ACCOUNT_TOKEN":"${fakeToken}"`),
    );

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);
    await handler!(buildMessage({ content: '<@123456789> hello' }));

    const logged = consoleError.mock.calls.map(args => args.join(' ')).join('\n');
    expect(logged).not.toContain(fakeToken);
    expect(logged).toContain('[REDACTED]');
    consoleError.mockRestore();
  });

  it('calls createComment when queryCodebase returns a commentProposal and includes URL in reply', async () => {
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'Following up on the issue.',
      issueProposals: [],
      commentProposals: [{
        repo: 'all-chat',
        issueNumber: 447,
        body: 'This is fixed in main.',
      }],
      infraVerdict: null,
      memoryMarker: null,
      updateMemoryMarker: null,
    });
    mockCreateComment.mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/447#issuecomment-7');

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> any update on issue 447?',
      startThread: vi.fn().mockResolvedValue({ id: 'comment-thread', send: threadSend }),
    });
    await handler!(msg);

    expect(mockCreateComment).toHaveBeenCalledWith(
      expect.anything(),
      testConfig.githubOwner,
      'all-chat',
      447,
      'This is fixed in main.',
    );
    const sendCalls = threadSend.mock.calls;
    const allSentContent = sendCalls.map(([arg]) => typeof arg === 'string' ? arg : JSON.stringify(arg)).join(' ');
    expect(allSentContent).toContain('https://github.com/testowner/all-chat/issues/447#issuecomment-7');
  });

  it('does NOT call createComment when commentProposal is null', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@123456789> how does twitch work?' });
    await handler!(msg);

    expect(mockCreateComment).not.toHaveBeenCalled();
  });
});

describe('Lead developer @mention', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  async function getSentContent(): Promise<string> {
    const handler = getEventHandler(Events.MessageCreate);
    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> question',
      startThread: vi.fn().mockResolvedValue({ id: 'mention-thread', send: threadSend }),
    });
    await handler!(msg);
    const sendCalls = threadSend.mock.calls;
    return sendCalls.map(([arg]) => typeof arg === 'string' ? arg : JSON.stringify(arg)).join(' ');
  }

  it('prepends lead dev @mention when infraVerdict.type is infrastructure', async () => {
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'There is a memory leak in the pod.',
      issueProposals: [],
      infraVerdict: { type: 'infrastructure', summary: 'Memory leak' },
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });

    await startBot(testConfig, createMockMemoryRepo());
    const content = await getSentContent();

    expect(content).toMatch(/^<@198569499228766208>/);
  });

  it('prepends lead dev @mention when issueProposal is not null and infraVerdict is null', async () => {
    mockCreateIssue.mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/99');
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'Here is a proposal.',
      issueProposals: [{ repo: 'all-chat', title: 'New issue', body: 'Body text' }],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });

    await startBot(testConfig, createMockMemoryRepo());
    const content = await getSentContent();

    expect(content).toMatch(/^<@198569499228766208>/);
  });

  it('prepends lead dev @mention when commentProposal is not null', async () => {
    mockCreateComment.mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/447#issuecomment-9');
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'Posted a follow-up.',
      issueProposals: [],
      commentProposals: [{ repo: 'all-chat', issueNumber: 447, body: 'Follow-up body' }],
      infraVerdict: null,
      memoryMarker: null,
      updateMemoryMarker: null,
    });

    await startBot(testConfig, createMockMemoryRepo());
    const content = await getSentContent();

    expect(content).toMatch(/^<@198569499228766208>/);
  });

  it('does NOT prepend lead dev @mention when infraVerdict is null and issueProposal is null', async () => {
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'Here is a code-level answer.',
      issueProposals: [],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });

    await startBot(testConfig, createMockMemoryRepo());
    const content = await getSentContent();

    expect(content).not.toContain('<@198569499228766208>');
  });

  it('does NOT prepend lead dev @mention when infraVerdict.type is code and issueProposal is null', async () => {
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'The issue is in the frontend code.',
      issueProposals: [],
      infraVerdict: { type: 'code', summary: 'Frontend code issue' },
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });

    await startBot(testConfig, createMockMemoryRepo());
    const content = await getSentContent();

    expect(content).not.toContain('<@198569499228766208>');
  });

  it('prepends @mention exactly once when both infraVerdict.type is infrastructure AND issueProposal is not null', async () => {
    mockCreateIssue.mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/77');
    mockQueryCodebase.mockResolvedValueOnce({
      answer: 'Infrastructure problem with a proposal.',
      issueProposals: [{ repo: 'all-chat', title: 'Fix infra', body: 'Body' }],
      infraVerdict: { type: 'infrastructure', summary: 'Critical infra issue' },
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });

    await startBot(testConfig, createMockMemoryRepo());
    const content = await getSentContent();

    const mentionCount = (content.match(/<@198569499228766208>/g) ?? []).length;
    expect(mentionCount).toBe(1);
  });
});

describe('Response formatting', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sends response <= 2000 chars as plain text', async () => {
    const shortAnswer = 'Short answer.';
    mockQueryCodebase.mockResolvedValueOnce({ answer: shortAnswer, issueProposals: [], commentProposals: [], infraVerdict: null, memoryMarker: null, updateMemoryMarker: null });

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> short question',
      startThread: vi.fn().mockResolvedValue({ id: 't1', send: threadSend }),
    });

    await handler!(msg);

    expect(threadSend).toHaveBeenCalledWith(shortAnswer);
  });

  it('sends response 2001-4096 chars as an embed', async () => {
    const mediumAnswer = 'A'.repeat(2001);
    mockQueryCodebase.mockResolvedValueOnce({ answer: mediumAnswer, issueProposals: [], commentProposals: [], infraVerdict: null, memoryMarker: null, updateMemoryMarker: null });

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> medium question',
      startThread: vi.fn().mockResolvedValue({ id: 't1', send: threadSend }),
    });

    await handler!(msg);

    expect(threadSend).toHaveBeenCalledWith(
      expect.objectContaining({ embeds: expect.any(Array) }),
    );
  });

  it('splits response > 4096 chars into 2000-char chunks', async () => {
    const longAnswer = 'B'.repeat(5000);
    mockQueryCodebase.mockResolvedValueOnce({ answer: longAnswer, issueProposals: [], commentProposals: [], infraVerdict: null, memoryMarker: null, updateMemoryMarker: null });

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const threadSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({
      content: '<@123456789> long question',
      startThread: vi.fn().mockResolvedValue({ id: 't1', send: threadSend }),
    });

    await handler!(msg);

    // 5000 chars -> 3 chunks of 2000/2000/1000
    expect(threadSend).toHaveBeenCalledTimes(3);
    // Each call should be a string (plain text chunk)
    for (const call of threadSend.mock.calls) {
      expect(typeof call[0]).toBe('string');
      expect((call[0] as string).length).toBeLessThanOrEqual(2000);
    }
  });
});

describe('InteractionCreate handler (/support slash command)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQueryCodebase.mockResolvedValue({
      answer: 'Slash command answer.',
      issueProposals: [],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });
  });

  function buildInteraction(overrides: Partial<{
    commandName: string;
    isChatInputCommand: boolean;
    question: string;
    fetchReplyStartThread: ReturnType<typeof vi.fn>;
  }> = {}) {
    const mockThreadWithSend = { id: 'slash-thread', send: vi.fn().mockResolvedValue({}) };
    const startThread = overrides.fetchReplyStartThread ?? vi.fn().mockResolvedValue(mockThreadWithSend);
    return {
      isChatInputCommand: vi.fn().mockReturnValue(overrides.isChatInputCommand ?? true),
      commandName: overrides.commandName ?? 'support',
      options: {
        getString: vi.fn().mockReturnValue(overrides.question ?? 'slash question'),
      },
      deferReply: vi.fn().mockResolvedValue(undefined),
      editReply: vi.fn().mockResolvedValue({ id: 'reply1', startThread }),
      fetchReply: vi.fn().mockResolvedValue({ id: 'reply1', startThread }),
    };
  }

  it('ignores interactions that are not chat input commands', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction({ isChatInputCommand: false });
    await handler!(interaction);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('ignores interactions for other commands', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction({ commandName: 'other' });
    await handler!(interaction);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('calls deferReply before queryCodebase', async () => {
    let deferCalled = false;
    let queryCalled = false;

    mockQueryCodebase.mockImplementationOnce(async () => {
      queryCalled = true;
      expect(deferCalled).toBe(true);
      return { answer: 'answer', issueProposals: [], commentProposals: [], infraVerdict: null, memoryMarker: null, updateMemoryMarker: null };
    });

    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction();
    interaction.deferReply = vi.fn().mockImplementationOnce(async () => {
      deferCalled = true;
    });

    await handler!(interaction);

    expect(deferCalled).toBe(true);
    expect(queryCalled).toBe(true);
  });

  it('calls queryCodebase with the slash command question and empty history', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction({ question: 'how does it work?' });
    await handler!(interaction);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'how does it work?',
      [testConfig.allChatRepoPath, testConfig.allChatExtensionRepoPath],
      [],
      expect.any(Array),
    );
  });

  it('calls editReply after queryCodebase completes', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction();
    await handler!(interaction);

    expect(interaction.editReply).toHaveBeenCalled();
  });

  it('creates a thread on the reply after slash command', async () => {
    const startThread = vi.fn().mockResolvedValue({ id: 'slash-reply-thread', send: vi.fn().mockResolvedValue({}) });
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction({ fetchReplyStartThread: startThread });
    await handler!(interaction);

    expect(startThread).toHaveBeenCalledWith(expect.objectContaining({
      name: expect.any(String),
      autoArchiveDuration: 1440,
    }));
  });
});

const MODERATION_GUILD_ID = 'guild-allchat-official';

function buildModeratedMessage(overrides: {
  authorId: string;
  channelId: string;
  content: string;
  guildId?: string;
  bansCreate?: ReturnType<typeof vi.fn>;
  isPrivileged?: boolean;
  authorBot?: boolean;
  attachmentSizes?: number[];
}) {
  const bansCreate = overrides.bansCreate ?? vi.fn().mockResolvedValue(undefined);
  const guildId = overrides.guildId ?? MODERATION_GUILD_ID;
  const isPrivileged = overrides.isPrivileged ?? false;

  // Privileged member: BanMembers permission set so the detector skips them.
  const memberPerms = {
    has: vi.fn().mockImplementation((flag: bigint) => {
      if (!isPrivileged) return false;
      return flag === (1n << 2n) || flag === (1n << 13n) || flag === (1n << 3n);
    }),
  };

  return {
    id: `msg-${overrides.authorId}-${overrides.channelId}`,
    content: overrides.content,
    author: {
      bot: overrides.authorBot ?? false,
      username: `user-${overrides.authorId}`,
      id: overrides.authorId,
      tag: `user-${overrides.authorId}#0000`,
    },
    channel: {
      id: overrides.channelId,
      isThread: () => false,
      send: vi.fn().mockResolvedValue({}),
      sendTyping: vi.fn().mockResolvedValue(undefined),
    },
    channelId: overrides.channelId,
    guildId,
    guild: { id: guildId, bans: { create: bansCreate } },
    member: { permissions: memberPerms },
    attachments: new Map(
      (overrides.attachmentSizes ?? []).map((size, i) => [
        `att-${i}`,
        { id: `att-${i}`, size, url: `https://cdn.example/att-${i}` },
      ]),
    ),
    mentions: { has: vi.fn().mockReturnValue(false) },
    reply: vi.fn().mockResolvedValue({ id: 'r' }),
    startThread: vi.fn().mockResolvedValue({ id: 't', send: vi.fn().mockResolvedValue({}) }),
  };
}

describe('Cross-channel spam moderation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQueryCodebase.mockResolvedValue({
      answer: 'irrelevant',
      issueProposals: [],
      infraVerdict: null,
      commentProposals: [],
      memoryMarker: null,
      updateMemoryMarker: null,
    });
  });

  it('does NOT moderate when moderationGuildId is not configured', async () => {
    await startBot(testConfig, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    const spam = 'free nitro check it out at evil.example';
    for (const ch of ['c1', 'c2', 'c3']) {
      await handler!(buildModeratedMessage({ authorId: 'spammer', channelId: ch, content: spam, bansCreate }));
    }

    expect(bansCreate).not.toHaveBeenCalled();
  });

  it('does NOT moderate messages from other guilds', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    const spam = 'free nitro check it out at evil.example';
    for (const ch of ['c1', 'c2', 'c3']) {
      await handler!(buildModeratedMessage({
        authorId: 'spammer', channelId: ch, content: spam, bansCreate,
        guildId: 'some-other-guild',
      }));
    }

    expect(bansCreate).not.toHaveBeenCalled();
  });

  it('bans a user who posts the same message in 3 channels of the moderated guild', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    const spam = 'free nitro check it out at evil.example';
    for (const ch of ['c1', 'c2', 'c3']) {
      await handler!(buildModeratedMessage({ authorId: 'spammer', channelId: ch, content: spam, bansCreate }));
    }
    // Allow the fire-and-forget ban promise to resolve.
    await new Promise(resolve => setImmediate(resolve));

    expect(bansCreate).toHaveBeenCalledTimes(1);
    expect(bansCreate).toHaveBeenCalledWith(
      'spammer',
      expect.objectContaining({
        deleteMessageSeconds: 6 * 60 * 60,
        reason: expect.stringMatching(/compromised|cross-channel|3\+/i),
      }),
    );
  });

  it('does NOT ban when same message is posted in only 2 channels', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    const spam = 'free nitro check it out at evil.example';
    for (const ch of ['c1', 'c2']) {
      await handler!(buildModeratedMessage({ authorId: 'spammer', channelId: ch, content: spam, bansCreate }));
    }

    expect(bansCreate).not.toHaveBeenCalled();
  });

  it('does NOT ban privileged members (mods/admins/lead-dev)', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    const announcement = 'reminder: server maintenance window tonight';
    for (const ch of ['c1', 'c2', 'c3', 'c4']) {
      await handler!(buildModeratedMessage({
        authorId: 'mod-user', channelId: ch, content: announcement,
        bansCreate, isPrivileged: true,
      }));
    }

    expect(bansCreate).not.toHaveBeenCalled();
  });

  it('does NOT ban the lead developer', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    const cross = 'announcement to multiple channels';
    for (const ch of ['c1', 'c2', 'c3']) {
      await handler!(buildModeratedMessage({
        authorId: testConfig.leadDeveloperDiscordId, channelId: ch, content: cross, bansCreate,
      }));
    }

    expect(bansCreate).not.toHaveBeenCalled();
  });

  it('short-circuits the support flow for the spam-trigger message that gets banned', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    // Spam @mentions the bot — without the short-circuit it would call queryCodebase 3x.
    const spam = '<@123456789> free nitro check it out at evil.example';
    for (const ch of ['c1', 'c2', 'c3']) {
      const msg = buildModeratedMessage({ authorId: 'spammer', channelId: ch, content: spam, bansCreate });
      msg.mentions.has = vi.fn().mockReturnValue(true);
      await handler!(msg);
    }
    await new Promise(resolve => setImmediate(resolve));

    expect(bansCreate).toHaveBeenCalledTimes(1);
    // c1+c2 still legitimately hit the support flow; c3 is the trigger and must be intercepted.
    expect(mockQueryCodebase).toHaveBeenCalledTimes(2);
  });

  it('bans a user posting short text "bro" + same image (size) across 3 channels', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    for (const ch of ['c1', 'c2', 'c3']) {
      await handler!(buildModeratedMessage({
        authorId: 'spammer', channelId: ch, content: 'bro',
        attachmentSizes: [314_159], bansCreate,
      }));
    }
    await new Promise(resolve => setImmediate(resolve));

    expect(bansCreate).toHaveBeenCalledTimes(1);
    expect(bansCreate).toHaveBeenCalledWith('spammer', expect.objectContaining({
      deleteMessageSeconds: 6 * 60 * 60,
    }));
  });

  it('does NOT ban when "bro" is posted with different attachment sizes', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const bansCreate = vi.fn().mockResolvedValue(undefined);
    for (const [ch, size] of [['c1', 1000], ['c2', 2000], ['c3', 3000]] as const) {
      await handler!(buildModeratedMessage({
        authorId: 'maybe-genuine', channelId: ch, content: 'bro',
        attachmentSizes: [size], bansCreate,
      }));
    }

    expect(bansCreate).not.toHaveBeenCalled();
  });

  it('still answers normal @mention questions when moderation is enabled', async () => {
    const config = { ...testConfig, moderationGuildId: MODERATION_GUILD_ID };
    await startBot(config, createMockMemoryRepo());
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@123456789> how does twitch work?' });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalled();
  });
});

describe('supportCommand', () => {
  it('has name "support" and a required "question" string option', async () => {
    const { supportCommand } = await import('../commands/support.js');

    expect(supportCommand).toBeDefined();
    // The SlashCommandBuilder mock tracks calls
    const builderMock = vi.mocked(
      (await import('discord.js')).SlashCommandBuilder,
    ).mock.results[0]?.value;
    expect(builderMock).toBeDefined();
    expect(builderMock.setName).toHaveBeenCalledWith('support');
    expect(builderMock.addStringOption).toHaveBeenCalled();
  });
});
