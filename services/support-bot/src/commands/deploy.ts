import { REST, Routes } from 'discord.js';
import { supportCommand } from './support.js';

const TOKEN = process.env['DISCORD_BOT_TOKEN'];
const CLIENT_ID = process.env['DISCORD_CLIENT_ID'];
const GUILD_ID = process.env['DISCORD_GUILD_ID']; // Optional — for guild-scoped dev registration

if (!TOKEN || !CLIENT_ID) {
  console.error('Missing DISCORD_BOT_TOKEN or DISCORD_CLIENT_ID');
  process.exit(1);
}

const commands = [supportCommand.toJSON()];
const rest = new REST().setToken(TOKEN);

async function deploy(): Promise<void> {
  try {
    console.log(`Registering ${commands.length} slash command(s)...`);

    if (GUILD_ID) {
      // Guild-scoped — instant, for development
      await rest.put(Routes.applicationGuildCommands(CLIENT_ID, GUILD_ID), {
        body: commands,
      });
      console.log(`Registered guild commands for guild ${GUILD_ID}`);
    } else {
      // Global — takes up to 1 hour to propagate
      await rest.put(Routes.applicationCommands(CLIENT_ID), { body: commands });
      console.log('Registered global commands (may take up to 1 hour)');
    }
  } catch (error) {
    console.error('Failed to register commands:', error);
    process.exit(1);
  }
}

void deploy();
