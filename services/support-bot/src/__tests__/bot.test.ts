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
    ownerId: 'bot-user-id',
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
    content: '@bot how does twitch work?',
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

  const mockClientUser = { id: 'bot-user-id', tag: 'SupportBot#0000' };

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
}));

// Mock the github issues module
vi.mock('../github/issues.js', () => ({
  createOctokitClient: vi.fn().mockReturnValue({ mock: 'octokit' }),
  createIssue: vi.fn().mockResolvedValue('https://github.com/owner/repo/issues/123'),
}));

import { Client, GatewayIntentBits, Events, EmbedBuilder } from 'discord.js';
import { queryCodebase } from '../claude/agent.js';
import { createIssue, createOctokitClient } from '../github/issues.js';
import { startBot } from '../bot.js';

const mockQueryCodebase = vi.mocked(queryCodebase);
const mockCreateIssue = vi.mocked(createIssue);
const mockCreateOctokitClient = vi.mocked(createOctokitClient);

const testConfig = {
  discordToken: 'test-discord-token',
  claudeOAuthToken: 'test-oauth-token',
  githubToken: 'test-github-token',
  githubOwner: 'testowner',
  allChatRepoPath: '/repos/all-chat',
  allChatExtensionRepoPath: '/repos/all-chat-extension',
};

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
      issueProposal: null,
    });
  });

  it('creates a Discord client with Guilds, GuildMessages, and MessageContent intents', async () => {
    await startBot(testConfig);

    expect(Client).toHaveBeenCalledWith({
      intents: expect.arrayContaining([
        GatewayIntentBits.Guilds,
        GatewayIntentBits.GuildMessages,
        GatewayIntentBits.MessageContent,
      ]),
    });
  });

  it('logs in with the discord token', async () => {
    await startBot(testConfig);

    const instance = getClientInstance();
    expect(instance.login).toHaveBeenCalledWith(testConfig.discordToken);
  });

  it('registers MessageCreate and InteractionCreate handlers', async () => {
    await startBot(testConfig);

    const instance = getClientInstance();
    const registeredEvents = vi.mocked(instance.on).mock.calls.map(([event]) => event);
    expect(registeredEvents).toContain(Events.MessageCreate);
    expect(registeredEvents).toContain(Events.InteractionCreate);
  });

  it('returns the Discord client', async () => {
    const client = await startBot(testConfig);

    expect(client).toBeDefined();
    expect(client).toBe(getClientInstance());
  });
});

describe('MessageCreate handler', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQueryCodebase.mockResolvedValue({
      answer: 'Twitch uses IRC.',
      issueProposal: null,
    });
  });

  function buildMessage(overrides: Partial<{
    content: string;
    authorBot: boolean;
    isThread: boolean;
    isBotThread: boolean;
    mentionsBot: boolean;
    channelSend: ReturnType<typeof vi.fn>;
    startThread: ReturnType<typeof vi.fn>;
  }> = {}) {
    const defaults = {
      content: '<@bot-user-id> how does twitch work?',
      authorBot: false,
      isThread: false,
      isBotThread: false,
      mentionsBot: true,
      channelSend: vi.fn().mockResolvedValue({}),
      startThread: vi.fn().mockResolvedValue({ id: 'new-thread' }),
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
      channel.ownerId = opts.isBotThread ? 'bot-user-id' : 'other-user-id';
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
      mentions: {
        has: vi.fn().mockReturnValue(opts.mentionsBot),
      },
      reply: vi.fn().mockResolvedValue({ id: 'reply1', startThread: vi.fn().mockResolvedValue({}) }),
      startThread: opts.startThread,
    };
  }

  it('ignores messages from bots', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);
    expect(handler).toBeDefined();

    const msg = buildMessage({ authorBot: true });
    await handler!(msg);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('ignores messages that are not mentions and not in bot threads', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ mentionsBot: false, isThread: false });
    await handler!(msg);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('strips mention tags from content before calling queryCodebase', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@bot-user-id> how does twitch work?' });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'how does twitch work?',
      [testConfig.allChatRepoPath, testConfig.allChatExtensionRepoPath],
      [],
    );
  });

  it('calls queryCodebase with empty history for @mention in non-thread channel', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({ content: '<@bot-user-id> explain the architecture', isThread: false });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'explain the architecture',
      expect.any(Array),
      [],
    );
  });

  it('creates a thread after replying to @mention in non-thread channel', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const startThread = vi.fn().mockResolvedValue({ id: 'new-thread' });
    const msg = buildMessage({ content: '<@bot-user-id> how does twitch work?', isThread: false, startThread });
    await handler!(msg);

    expect(startThread).toHaveBeenCalledWith(expect.objectContaining({
      name: expect.any(String),
      autoArchiveDuration: 1440,
    }));
  });

  it('collects thread history when @mentioned in a bot-owned thread', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const msg = buildMessage({
      content: '<@bot-user-id> follow-up question',
      isThread: true,
      isBotThread: true,
    });
    await handler!(msg);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'follow-up question',
      expect.any(Array),
      expect.arrayContaining(['[Alice]: prior question']),
    );
  });

  it('handles message in bot-owned thread without @mention as follow-up', async () => {
    await startBot(testConfig);
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
    );
  });

  it('does NOT create a new thread when already in a thread', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const startThread = vi.fn().mockResolvedValue({ id: 'new-thread' });
    const msg = buildMessage({
      content: '<@bot-user-id> question in thread',
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
      issueProposal: {
        repo: 'all-chat',
        title: 'Fix bug',
        body: 'Bug body',
      },
    });
    mockCreateIssue.mockResolvedValueOnce('https://github.com/testowner/all-chat/issues/42');

    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const channelSend = vi.fn().mockResolvedValue({});
    const msg = buildMessage({ content: '<@bot-user-id> there is a bug', channelSend });
    await handler!(msg);

    expect(mockCreateIssue).toHaveBeenCalledWith(
      expect.anything(),
      testConfig.githubOwner,
      'all-chat',
      'Fix bug',
      'Bug body',
    );
    // The sent message should contain the issue URL
    const sendCalls = channelSend.mock.calls;
    const allSentContent = sendCalls.map(([arg]) => typeof arg === 'string' ? arg : JSON.stringify(arg)).join(' ');
    expect(allSentContent).toContain('https://github.com/testowner/all-chat/issues/42');
  });
});

describe('Response formatting', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sends response <= 2000 chars as plain text', async () => {
    const shortAnswer = 'Short answer.';
    mockQueryCodebase.mockResolvedValueOnce({ answer: shortAnswer, issueProposal: null });

    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const channelSend = vi.fn().mockResolvedValue({});
    const msg = {
      id: 'msg-main',
      content: '<@bot-user-id> short question',
      author: { bot: false, username: 'Alice', id: 'user-alice' },
      channel: {
        id: 'channel1',
        isThread: () => false,
        send: channelSend,
        sendTyping: vi.fn().mockResolvedValue(undefined),
      },
      mentions: { has: vi.fn().mockReturnValue(true) },
      reply: vi.fn().mockResolvedValue({ id: 'r1', startThread: vi.fn().mockResolvedValue({}) }),
      startThread: vi.fn().mockResolvedValue({}),
    };

    await handler!(msg);

    expect(channelSend).toHaveBeenCalledWith(shortAnswer);
  });

  it('sends response 2001-4096 chars as an embed', async () => {
    const mediumAnswer = 'A'.repeat(2001);
    mockQueryCodebase.mockResolvedValueOnce({ answer: mediumAnswer, issueProposal: null });

    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const channelSend = vi.fn().mockResolvedValue({});
    const msg = {
      id: 'msg-main',
      content: '<@bot-user-id> medium question',
      author: { bot: false, username: 'Alice', id: 'user-alice' },
      channel: {
        id: 'channel1',
        isThread: () => false,
        send: channelSend,
        sendTyping: vi.fn().mockResolvedValue(undefined),
      },
      mentions: { has: vi.fn().mockReturnValue(true) },
      reply: vi.fn().mockResolvedValue({ id: 'r1', startThread: vi.fn().mockResolvedValue({}) }),
      startThread: vi.fn().mockResolvedValue({}),
    };

    await handler!(msg);

    expect(channelSend).toHaveBeenCalledWith(
      expect.objectContaining({ embeds: expect.any(Array) }),
    );
  });

  it('splits response > 4096 chars into 2000-char chunks', async () => {
    const longAnswer = 'B'.repeat(5000);
    mockQueryCodebase.mockResolvedValueOnce({ answer: longAnswer, issueProposal: null });

    await startBot(testConfig);
    const handler = getEventHandler(Events.MessageCreate);

    const channelSend = vi.fn().mockResolvedValue({});
    const msg = {
      id: 'msg-main',
      content: '<@bot-user-id> long question',
      author: { bot: false, username: 'Alice', id: 'user-alice' },
      channel: {
        id: 'channel1',
        isThread: () => false,
        send: channelSend,
        sendTyping: vi.fn().mockResolvedValue(undefined),
      },
      mentions: { has: vi.fn().mockReturnValue(true) },
      reply: vi.fn().mockResolvedValue({ id: 'r1', startThread: vi.fn().mockResolvedValue({}) }),
      startThread: vi.fn().mockResolvedValue({}),
    };

    await handler!(msg);

    // 5000 chars -> 3 chunks of 2000/2000/1000
    expect(channelSend).toHaveBeenCalledTimes(3);
    // Each call should be a string (plain text chunk)
    for (const call of channelSend.mock.calls) {
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
      issueProposal: null,
    });
  });

  function buildInteraction(overrides: Partial<{
    commandName: string;
    isChatInputCommand: boolean;
    question: string;
    fetchReplyStartThread: ReturnType<typeof vi.fn>;
  }> = {}) {
    const startThread = overrides.fetchReplyStartThread ?? vi.fn().mockResolvedValue({});
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
    await startBot(testConfig);
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction({ isChatInputCommand: false });
    await handler!(interaction);

    expect(mockQueryCodebase).not.toHaveBeenCalled();
  });

  it('ignores interactions for other commands', async () => {
    await startBot(testConfig);
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
      return { answer: 'answer', issueProposal: null };
    });

    await startBot(testConfig);
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
    await startBot(testConfig);
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction({ question: 'how does it work?' });
    await handler!(interaction);

    expect(mockQueryCodebase).toHaveBeenCalledWith(
      'how does it work?',
      [testConfig.allChatRepoPath, testConfig.allChatExtensionRepoPath],
      [],
    );
  });

  it('calls editReply after queryCodebase completes', async () => {
    await startBot(testConfig);
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction();
    await handler!(interaction);

    expect(interaction.editReply).toHaveBeenCalled();
  });

  it('creates a thread on the reply after slash command', async () => {
    const startThread = vi.fn().mockResolvedValue({});
    await startBot(testConfig);
    const handler = getEventHandler(Events.InteractionCreate);

    const interaction = buildInteraction({ fetchReplyStartThread: startThread });
    await handler!(interaction);

    expect(startThread).toHaveBeenCalledWith(expect.objectContaining({
      name: expect.any(String),
      autoArchiveDuration: 1440,
    }));
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
