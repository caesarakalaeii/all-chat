export interface IssueProposal {
  repo: 'all-chat' | 'all-chat-extension';
  title: string;
  body: string;
}

export interface InfraVerdict {
  type: 'infrastructure' | 'code';
  summary: string;
}

export interface QueryResult {
  answer: string;
  issueProposal: IssueProposal | null;
  infraVerdict: InfraVerdict | null;
  memoryMarker: ParsedMemoryMarker | null;
  updateMemoryMarker: ParsedUpdateMemoryMarker | null;
}

export interface BotConfig {
  discordToken: string;
  claudeOAuthToken: string;
  githubToken: string;
  githubOwner: string;
  allChatRepoPath: string;
  allChatExtensionRepoPath: string;
  leadDeveloperDiscordId: string;
  grafanaUrl: string;
  grafanaServiceAccountToken: string;
  databaseUrl: string;
  /** Discord guild ID of the official all-chat server. When set, the bot will
   *  auto-ban users whose identical messages appear in 3+ channels (suspected
   *  compromised account) and delete their last 6h of messages. */
  moderationGuildId?: string;
}

export type MemoryType = 'error_pattern' | 'correction' | 'codebase_insight';

export interface StoredMemory {
  id: number;
  type: MemoryType;
  tags: string[];
  content: string;
  accessCount: number;
  updatedAt: Date;
}

export interface ParsedMemoryMarker {
  type: MemoryType;
  tags: string[];
  content: string;
}

export interface ParsedUpdateMemoryMarker {
  id: number;
  content: string;
}
