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
