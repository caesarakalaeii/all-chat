export interface IssueProposal {
  repo: 'all-chat' | 'all-chat-extension';
  title: string;
  body: string;
}

export interface QueryResult {
  answer: string;
  issueProposal: IssueProposal | null;
}

export interface BotConfig {
  discordToken: string;
  claudeOAuthToken: string;
  githubToken: string;
  githubOwner: string;
  allChatRepoPath: string;
  allChatExtensionRepoPath: string;
}
