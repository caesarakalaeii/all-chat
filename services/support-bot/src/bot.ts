import {
  Client,
  EmbedBuilder,
  Events,
  GatewayIntentBits,
  PermissionFlagsBits,
  type Message,
  type TextChannel,
  type ThreadChannel,
} from 'discord.js';
import type { Octokit } from '@octokit/rest';
import { CLAUDE_TIMEOUT_MS, queryCodebase } from './claude/agent.js';
import { createComment, createIssue, createOctokitClient } from './github/issues.js';
import { MemoryRepository, extractTagsFromQuestion } from './memory/repository.js';
import { CrossChannelSpamDetector } from './moderation/cross-channel-spam.js';
import type { BotConfig } from './types.js';

const SIX_HOURS_SECONDS = 6 * 60 * 60;

async function autoBanForCrossChannelSpam(message: Message): Promise<void> {
  if (!message.guild) return;
  try {
    await message.guild.bans.create(message.author.id, {
      deleteMessageSeconds: SIX_HOURS_SECONDS,
      reason: 'Auto-ban: same message posted in 3+ channels (suspected compromised account)',
    });
    console.log(
      `[moderation] Banned ${message.author.tag} (${message.author.id}) in guild ${message.guildId} for cross-channel spam; deleted last 6h of messages`,
    );
  } catch (err) {
    console.error('[moderation] Failed to auto-ban user:', err);
  }
}

function isPrivilegedMember(message: Message, leadDeveloperDiscordId: string): boolean {
  if (message.author.id === leadDeveloperDiscordId) return true;
  const perms = message.member?.permissions;
  if (!perms) return false;
  return (
    perms.has(PermissionFlagsBits.BanMembers) ||
    perms.has(PermissionFlagsBits.ManageMessages) ||
    perms.has(PermissionFlagsBits.Administrator)
  );
}

async function fetchThreadHistory(thread: ThreadChannel): Promise<string[]> {
  const messages = await thread.messages.fetch({ limit: 20 });
  return [...messages.values()]
    .reverse()
    .filter(m => m.content.length > 0)
    .map(m => `[${m.author.bot ? 'Bot' : m.author.username}]: ${m.content}`);
}

/** Cap on how much of one error reaches the log. An ExecaError carries the whole argv. */
const MAX_ERROR_LOG_CHARS = 4000;

/** Credential shapes that must never be logged, whatever dragged them in. */
const SECRET_PATTERNS: readonly RegExp[] = [
  /glsa_[A-Za-z0-9_]+/g, // Grafana service-account token
  /gh[pousr]_[A-Za-z0-9]{20,}/g, // GitHub token
  /sk-ant-[A-Za-z0-9_-]{20,}/g, // Anthropic API key
];

function redactSecrets(text: string): string {
  return SECRET_PATTERNS.reduce((acc, pattern) => acc.replace(pattern, '[REDACTED]'), text);
}

/**
 * Logs a failure without re-leaking what the error dragged along.
 *
 * execa embeds the full argv in every error it throws. For this bot that argv is the
 * entire system prompt plus every flag -- thousands of lines per failure -- and until the
 * MCP config was moved out of the command line it also carried the Grafana
 * service-account token in plaintext, which is how that token ended up in the pod log.
 * The file-based config fixes the source; this is the belt to that pair of braces, so a
 * secret reaching here by some future route still does not reach the log.
 */
function logHandlingError(context: string, err: unknown): void {
  if (typeof err === 'object' && err !== null && (err as { timedOut?: boolean }).timedOut === true) {
    console.error(`${context}: claude subprocess timed out after ${CLAUDE_TIMEOUT_MS}ms`);
    return;
  }
  const text = err instanceof Error ? (err.stack ?? err.message) : String(err);
  console.error(`${context}: ${redactSecrets(text).slice(0, MAX_ERROR_LOG_CHARS)}`);
}

/**
 * Turns a subprocess failure into a reply the asker can act on.
 *
 * A timeout is the common failure and it is not random: an expensive request (several
 * issues at once, a wide sweep of a 137MB checkout) simply does not finish inside the
 * budget. The old catch-all answered "something went wrong, check the bot logs" for every
 * case, which is useless to the streamer who asked and misleading to everyone else --
 * nothing "went wrong", the work was too big. Naming the limit and how to get under it is
 * the difference between a dead end and a retry that succeeds.
 */
function failureReply(err: unknown): string {
  if (typeof err === 'object' && err !== null && (err as { timedOut?: boolean }).timedOut === true) {
    const minutes = Math.round(CLAUDE_TIMEOUT_MS / 60_000);
    return (
      `I ran out of time on this one and stopped after ${minutes} minutes. ` +
      'That usually means the request needed too much digging at once. ' +
      'Try splitting it up: ask for one issue or one area at a time, and name the file or service if you know it.'
    );
  }
  return 'Sorry, something went wrong while processing your question. Check the bot logs for details.';
}

async function handleQuestion(
  question: string,
  repoPaths: string[],
  config: BotConfig,
  octokit: Octokit,
  history: string[],
  memoryRepo: MemoryRepository,
): Promise<string> {
  // Retrieve relevant memories before calling Claude
  const tags = extractTagsFromQuestion(question);
  const memories = await memoryRepo.retrieveMemories(tags);

  const result = await queryCodebase(question, repoPaths, history, memories);
  let answer = result.answer;

  // Store memory if Claude emitted STORE_MEMORY marker (errors do not block the Discord answer)
  if (result.memoryMarker) {
    await memoryRepo.storeMemory(result.memoryMarker);
  }

  // Update memory if Claude emitted UPDATE_MEMORY marker (errors do not block the Discord answer)
  if (result.updateMemoryMarker) {
    await memoryRepo.updateMemory(result.updateMemoryMarker.id, result.updateMemoryMarker.content);
  }

  // Each proposal is created independently and a failure is reported rather than thrown:
  // when the model files four issues, one rejected by the GitHub API must not discard the
  // three that succeeded, and the user needs to know which of the four is missing.
  const created: string[] = [];

  for (const proposal of result.issueProposals) {
    try {
      const issueUrl = await createIssue(
        octokit,
        config.githubOwner,
        proposal.repo,
        proposal.title,
        proposal.body,
      );
      created.push(`Created **${proposal.title}**: ${issueUrl}`);
    } catch (err) {
      logHandlingError(`Failed to create issue "${proposal.title}" in ${proposal.repo}`, err);
      created.push(`Could NOT create **${proposal.title}** in ${proposal.repo} (see bot logs).`);
    }
  }

  for (const proposal of result.commentProposals) {
    try {
      const commentUrl = await createComment(
        octokit,
        config.githubOwner,
        proposal.repo,
        proposal.issueNumber,
        proposal.body,
      );
      created.push(`Commented on ${proposal.repo} #${proposal.issueNumber}: ${commentUrl}`);
    } catch (err) {
      logHandlingError(`Failed to comment on ${proposal.repo} #${proposal.issueNumber}`, err);
      created.push(`Could NOT comment on ${proposal.repo} #${proposal.issueNumber} (see bot logs).`);
    }
  }

  if (created.length > 0) {
    answer = `${answer}\n\n${created.map(line => `- ${line}`).join('\n')}`;
  }

  const shouldPingLeadDev =
    result.infraVerdict?.type === 'infrastructure' ||
    result.issueProposals.length > 0 ||
    result.commentProposals.length > 0;

  if (shouldPingLeadDev && config.leadDeveloperDiscordId) {
    answer = `<@${config.leadDeveloperDiscordId}> ${answer}`;
  }

  return answer;
}

async function sendResponse(
  channel: TextChannel | ThreadChannel,
  text: string,
): Promise<void> {
  if (text.length <= 2000) {
    await channel.send(text);
  } else if (text.length <= 4096) {
    await channel.send({ embeds: [new EmbedBuilder().setDescription(text)] });
  } else {
    const chunks = text.match(/.{1,2000}/gs) ?? [text];
    for (const chunk of chunks) {
      await channel.send(chunk);
    }
  }
}

export async function startBot(config: BotConfig, memoryRepo: MemoryRepository): Promise<Client> {
  const client = new Client({
    intents: [
      GatewayIntentBits.Guilds,
      GatewayIntentBits.GuildMessages,
      GatewayIntentBits.MessageContent,
    ],
  });

  const octokit = createOctokitClient(config.githubToken);
  const repoPaths = [config.allChatRepoPath, config.allChatExtensionRepoPath];

  const spamDetector = new CrossChannelSpamDetector();
  if (config.moderationGuildId) {
    console.log(`[moderation] Cross-channel spam detection enabled for guild ${config.moderationGuildId}`);
  }

  // Track thread IDs the bot has created or posted in this session.
  // Used to recognise bot-managed threads even if ownerId doesn't match.
  const botThreadIds = new Set<string>();

  // Per-channel queue: each message waits for the previous one to finish so
  // history is complete and responses arrive in order.
  const queues = new Map<string, Promise<void>>();

  const enqueue = (channelId: string, task: () => Promise<void>): Promise<void> => {
    const prev = queues.get(channelId) ?? Promise.resolve();
    const next = prev.then(task, task); // run task regardless of previous outcome
    queues.set(channelId, next);
    // clean up once the chain is idle
    next.then(() => { if (queues.get(channelId) === next) queues.delete(channelId); });
    return next;
  };

  client.on(Events.MessageCreate, (message) => {
    if (message.author.bot) return;

    // Cross-channel spam moderation — only in the official all-chat guild.
    // We exempt mods/admins/lead-dev so privileged cross-posting doesn't trip it.
    if (
      config.moderationGuildId &&
      message.guildId === config.moderationGuildId &&
      !isPrivilegedMember(message, config.leadDeveloperDiscordId)
    ) {
      const attachmentSizes = [...message.attachments.values()].map(a => a.size);
      const triggered = spamDetector.record(
        message.author.id,
        message.channelId,
        message.content,
        attachmentSizes,
      );
      if (triggered) {
        void autoBanForCrossChannelSpam(message);
        return;
      }
    }

    const inBotThread =
      message.channel.isThread() && (
        (message.channel as ThreadChannel).ownerId === client.user!.id ||
        botThreadIds.has(message.channelId)
      );

    // Only treat a direct user ping as a summon. @here/@everyone and role pings
    // (which would otherwise satisfy mentions.has) must not trigger the bot.
    const isMentioned = message.mentions.has(client.user!.id, {
      ignoreEveryone: true,
      ignoreRoles: true,
    });

    if (!inBotThread && !isMentioned) return;

    const stripped = message.content.replace(/<@[!&]?\d+>/g, '').trim();

    return enqueue(message.channelId, async () => {
      await message.channel.sendTyping();
      const typingInterval = setInterval(() => { void message.channel.sendTyping(); }, 8000);

      try {
        const history = inBotThread
          ? await fetchThreadHistory(message.channel as ThreadChannel)
          : [];

        // Build question: if user replied to another message, use that as context.
        // If stripped is empty (e.g. bare @mention reply), use the referenced message as the question.
        let question = stripped;
        if (message.reference?.messageId) {
          try {
            const referenced = await message.channel.messages.fetch(message.reference.messageId);
            if (!referenced.author.bot) {
              question = stripped
                ? `Context (message being replied to by ${referenced.author.username}): ${referenced.content}\n\nQuestion: ${stripped}`
                : referenced.content;
            }
          } catch {
            // ignore if fetch fails
          }
        }

        if (!question) {
          clearInterval(typingInterval);
          // Bare role/user ping with no reference — silently ignore
          return;
        }

        const answer = await handleQuestion(question, repoPaths, config, octokit, history, memoryRepo);

        clearInterval(typingInterval);

        if (message.channel.isThread()) {
          botThreadIds.add(message.channelId);
          await sendResponse(message.channel as ThreadChannel, answer);
        } else {
          const thread = await message.startThread({
            name: (stripped || question).slice(0, 50),
            autoArchiveDuration: 1440,
          });
          botThreadIds.add(thread.id);
          await sendResponse(thread, answer);
        }
      } catch (err) {
        clearInterval(typingInterval);
        logHandlingError('Error handling message', err);
        await message.reply(failureReply(err));
      }
    });
  });

  client.on(Events.InteractionCreate, async (interaction) => {
    if (!interaction.isChatInputCommand()) return;
    if (interaction.commandName !== 'support') return;

    const question = interaction.options.getString('question', true);

    await interaction.deferReply();
    console.log(`[interaction] Received /support question: "${question.slice(0, 80)}"`);

    let answer: string;
    try {
      console.log('[interaction] Spawning claude subprocess...');
      answer = await handleQuestion(question, repoPaths, config, octokit, [], memoryRepo);
      console.log('[interaction] Claude responded successfully');
    } catch (err) {
      logHandlingError('[interaction] Error handling interaction', err);
      await interaction.editReply(failureReply(err));
      return;
    }
    // Note: deferReply keeps the "thinking..." indicator alive until editReply is called, no interval needed.

    // Send the answer (chunked if needed)
    if (answer.length <= 2000) {
      await interaction.editReply(answer);
    } else if (answer.length <= 4096) {
      await interaction.editReply({ embeds: [new EmbedBuilder().setDescription(answer)] });
    } else {
      const chunks = answer.match(/.{1,2000}/gs) ?? [answer];
      await interaction.editReply(chunks[0] ?? answer);
    }

    // Create a thread and post the original question as context so history works
    const reply = await interaction.fetchReply();
    const thread = await reply.startThread({
      name: question.slice(0, 50),
      autoArchiveDuration: 1440,
    });
    botThreadIds.add(thread.id);
    await thread.send(`**Original question:** ${question}`);

    // Post any remaining chunks into the thread
    if (answer.length > 2000) {
      const chunks = answer.match(/.{1,2000}/gs) ?? [];
      for (const chunk of chunks.slice(1)) {
        await thread.send(chunk);
      }
    }
  });

  client.once('ready', () => {
    console.log(`Support bot ready as ${client.user?.tag}`);
  });

  await client.login(config.discordToken);

  return client;
}
