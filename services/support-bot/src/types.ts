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
