import { SlashCommandBuilder } from 'discord.js';

export const supportCommand = new SlashCommandBuilder()
  .setName('support')
  .setDescription('Ask a question about All-Chat')
  .addStringOption(option =>
    option
      .setName('question')
      .setDescription('What would you like to know?')
      .setRequired(true),
  );
