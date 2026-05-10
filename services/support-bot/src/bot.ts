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
import { queryCodebase } from './claude/agent.js';
import { createIssue, createOctokitClient } from './github/issues.js';
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

  if (result.issueProposal !== null) {
    const issueUrl = await createIssue(
      octokit,
      config.githubOwner,
      result.issueProposal.repo,
      result.issueProposal.title,
      result.issueProposal.body,
    );
    answer = `${answer}\n\nI've created a GitHub issue for this proposed change: ${issueUrl}`;
  }

  const shouldPingLeadDev =
    result.infraVerdict?.type === 'infrastructure' ||
    result.issueProposal !== null;

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

    const isMentioned = message.mentions.has(client.user!.id);

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
        console.error('Error handling message:', err);
        await message.reply('Sorry, something went wrong while processing your question. Check the bot logs for details.');
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
      console.error('[interaction] Error handling interaction:', err);
      await interaction.editReply('Sorry, something went wrong while processing your question. Check the bot logs for details.');
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
