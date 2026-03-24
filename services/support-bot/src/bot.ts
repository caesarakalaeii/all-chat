import {
  Client,
  EmbedBuilder,
  Events,
  GatewayIntentBits,
  type TextChannel,
  type ThreadChannel,
} from 'discord.js';
import type { Octokit } from '@octokit/rest';
import { queryCodebase } from './claude/agent.js';
import { createIssue, createOctokitClient } from './github/issues.js';
import type { BotConfig } from './types.js';

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
): Promise<string> {
  const result = await queryCodebase(question, repoPaths, history);
  let answer = result.answer;

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

export async function startBot(config: BotConfig): Promise<Client> {
  const client = new Client({
    intents: [
      GatewayIntentBits.Guilds,
      GatewayIntentBits.GuildMessages,
      GatewayIntentBits.MessageContent,
    ],
  });

  const octokit = createOctokitClient(config.githubToken);
  const repoPaths = [config.allChatRepoPath, config.allChatExtensionRepoPath];

  client.on(Events.MessageCreate, async (message) => {
    if (message.author.bot) return;

    const inBotThread =
      message.channel.isThread() &&
      (message.channel as ThreadChannel).ownerId === client.user!.id;

    if (!inBotThread && !message.mentions.has(client.user!.id)) return;

    const stripped = message.content.replace(/<@!?\d+>/g, '').trim();

    if (!stripped) {
      await message.reply('Please include a question after the mention.');
      return;
    }

    await message.channel.sendTyping();
    const typingInterval = setInterval(() => { void message.channel.sendTyping(); }, 8000);

    try {
      const history = inBotThread
        ? await fetchThreadHistory(message.channel as ThreadChannel)
        : [];

      // If this is a reply to another message, prepend that message as context
      let question = stripped;
      if (message.reference?.messageId) {
        try {
          const referenced = await message.channel.messages.fetch(message.reference.messageId);
          if (!referenced.author.bot) {
            question = `Context (message being replied to by ${referenced.author.username}): ${referenced.content}\n\nQuestion: ${stripped}`;
          }
        } catch {
          // ignore if fetch fails
        }
      }

      const answer = await handleQuestion(question, repoPaths, config, octokit, history);

      clearInterval(typingInterval);

      if (message.channel.isThread()) {
        await sendResponse(message.channel as ThreadChannel, answer);
      } else {
        const thread = await message.startThread({
          name: stripped.slice(0, 50),
          autoArchiveDuration: 1440,
        });
        await sendResponse(thread, answer);
      }
    } catch (err) {
      clearInterval(typingInterval);
      console.error('Error handling message:', err);
      await message.reply('Sorry, something went wrong while processing your question. Check the bot logs for details.');
    }
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
      answer = await handleQuestion(question, repoPaths, config, octokit, []);
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
