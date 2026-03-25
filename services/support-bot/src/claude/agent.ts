import { execa } from 'execa';
import type { QueryResult, IssueProposal } from '../types.js';

export async function queryCodebase(
  question: string,
  repoPaths: string[],
  conversationHistory?: string[],
): Promise<QueryResult> {
  const systemPrompt = [
    'You are a friendly support bot for All-Chat, a platform that lets streamers combine chat messages from Twitch, YouTube, Kick, and TikTok into a single overlay.',
    'Your primary audience is streamers and end users. When a question is vague or ambiguous, assume it comes from an end user and answer in simple, non-technical language. If someone explicitly asks about code, architecture, or deployment, answer those questions fully.',
    'You help with: getting started, setting up overlays and chat sources, connecting streaming platforms, troubleshooting common issues, understanding features, setup & configuration, architecture questions, bug triage, and UI/UX review.',
    `You can read the project source at: ${repoPaths.join(', ')}`,
    'When asked about UI/UX, read the relevant frontend source files, identify concrete usability or visual issues, and propose specific improvements.',
    'Keep answers concise and actionable. Use step-by-step instructions when guiding users through setup or troubleshooting.',
    "If a code change or improvement is needed, end your response with exactly: PROPOSE_ISSUE:repo_name|||title|||body",
    "repo_name must be 'all-chat' or 'all-chat-extension'",
  ].join('\n');

  let fullPrompt: string;
  if (conversationHistory && conversationHistory.length > 0) {
    fullPrompt = `${systemPrompt}\n\n## Conversation so far:\n${conversationHistory.join('\n')}\n\n## New question:\n${question}`;
  } else {
    fullPrompt = `${systemPrompt}\n\n${question}`;
  }

  console.log('[claude] Starting subprocess (timeout: 120s)');
  const { stdout } = await execa(
    'claude',
    [
      '-p', fullPrompt,
      '--model', 'claude-sonnet-4-6',
      '--allowedTools', 'Read,Glob,Grep',
      '--output-format', 'json',
    ],
    {
      stdin: 'ignore',
      env: { ...process.env },
      timeout: 120_000,
    },
  );
  console.log('[claude] Subprocess completed, parsing response');

  const parsed = JSON.parse(stdout) as { result: string };
  const resultText = parsed.result;

  let issueProposal: IssueProposal | null = null;
  const proposeMarker = 'PROPOSE_ISSUE:';
  const proposeIndex = resultText.indexOf(proposeMarker);
  if (proposeIndex !== -1) {
    const proposeString = resultText.slice(proposeIndex + proposeMarker.length);
    const parts = proposeString.split('|||');
    if (parts.length >= 3) {
      const repoName = parts[0].trim() as 'all-chat' | 'all-chat-extension';
      const title = parts[1].trim();
      const body = parts.slice(2).join('|||').trim();
      issueProposal = { repo: repoName, title, body };
    }
  }

  return { answer: resultText, issueProposal };
}
