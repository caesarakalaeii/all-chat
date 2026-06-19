import { execa } from 'execa';
import type { QueryResult, IssueProposal, CommentProposal, InfraVerdict, StoredMemory, ParsedMemoryMarker, ParsedUpdateMemoryMarker, MemoryType } from '../types.js';

export async function queryCodebase(
  question: string,
  repoPaths: string[],
  conversationHistory?: string[],
  memories?: StoredMemory[],
): Promise<QueryResult> {
  const systemPrompt = [
    'You are a friendly support bot for All-Chat, a platform that lets streamers combine chat messages from Twitch, YouTube, Kick, and TikTok into a single overlay.',
    'Your primary audience is streamers and end users. When a question is vague or ambiguous, assume it comes from an end user and answer in simple, non-technical language. If someone explicitly asks about code, architecture, or deployment, answer those questions fully.',
    'You help with: getting started, setting up overlays and chat sources, connecting streaming platforms, troubleshooting common issues, understanding features, setup & configuration, architecture questions, bug triage, and UI/UX review.',
    `You can read the project source at: ${repoPaths.join(', ')}`,
    'When asked about UI/UX, read the relevant frontend source files, identify concrete usability or visual issues, and propose specific improvements.',
    'Keep answers concise and actionable. Use step-by-step instructions when guiding users through setup or troubleshooting.',
    '',
    'GitHub actions -- you can file a new issue or comment on an existing issue/PR. These are posted automatically when you emit the marker; do NOT ask the user for approval and do NOT claim you need permission. You cannot browse GitHub, run git, or call the gh CLI -- the only way you affect GitHub is by emitting one of the markers below.',
    "- To open a NEW issue for a code change or improvement, end your response with exactly: PROPOSE_ISSUE:repo_name|||title|||body",
    "- To COMMENT on an EXISTING issue or pull request (for example, the user references one by number or asks you to follow up on it), end your response with exactly: PROPOSE_COMMENT:repo_name|||issue_number|||body",
    "repo_name must be 'all-chat' or 'all-chat-extension'.",
    'issue_number is the numeric issue or PR number (GitHub treats PR comments as issue comments, so the same number works for either). body is the full Markdown comment.',
    'Only emit PROPOSE_COMMENT when you have an explicit issue or PR number from the user or the conversation -- never guess or invent a number, and never claim you commented if you did not emit the marker.',
    'Emit at most one of PROPOSE_ISSUE or PROPOSE_COMMENT per response.',
    '',
    'IMPORTANT -- Infrastructure data handling rules:',
    '1. NEVER include raw log lines, stack traces, or raw error messages in your response.',
    '2. NEVER include environment variable values, secret names with their values, or internal hostnames (*.svc.cluster.local, pod IPs) in your response.',
    '3. DO summarize counts and patterns: "auth-service logged 47 connection timeout errors in the last 15 minutes"',
    '4. DO name the service and error type: "kick-listener had 2 OOMKill restarts in the last hour"',
    '5. DO state your verdict: begin infrastructure analysis summary with "Infrastructure status:" and end with INFRA_VERDICT:',
    '6. When kubectl returns secret information, report only whether a secret EXISTS or is MISSING -- never its value or key count.',
    '',
    'PRIORITY: Always answer the user\'s actual question FIRST. Your primary job is to help with their specific question. Infrastructure checks are secondary context, not the main response.',
    '',
    'Infrastructure health checks (secondary -- only after answering the question):',
    '- Only run infra checks when the question is about troubleshooting, errors, or deployment, OR as a brief addendum to your main answer',
    '- Use kubectl to check pod status, restarts, and recent warning events in the allchat namespace',
    '- Use Grafana Loki to check for recent error logs (last 15 min) across allchat services',
    '- Use Grafana Prometheus to check pod restart counts and resource pressure',
    '- Keep infra findings brief when the question is not about infrastructure',
    '',
    'When you determine the issue is infrastructure-related (pod crashes, OOMKills, missing secrets, high error rates, connectivity issues), append to your response:',
    'INFRA_VERDICT:infrastructure|||<one-sentence summary of the infrastructure issue>',
    '',
    'When the issue is code-related (bug in logic, missing feature, configuration error in code), append:',
    'INFRA_VERDICT:code|||<one-sentence summary>',
    '',
    'Always include INFRA_VERDICT: at the end of responses where you checked infrastructure.',
    '',
    'You have access to a memory bank of past observations about this codebase. When the section "Relevant memories" appears below, treat these as verified past knowledge -- weave relevant memories naturally into your answer.',
    'When you observe something memory-worthy (a correction from a user, a recurring error pattern, or a non-obvious codebase insight), append to your response:',
    'STORE_MEMORY:type|||tag1,tag2,tag3|||one or two sentence description',
    'where type is one of: error_pattern, correction, codebase_insight',
    'Tags should be service names (e.g. kick-listener), error types (e.g. OOMKill), or concepts (e.g. quota).',
    'Memory content must be concise -- one to two sentences maximum.',
    'If you need to update an existing memory you can see above, append: UPDATE_MEMORY:id|||updated content',
    'Emit STORE_MEMORY or UPDATE_MEMORY at most once per response, after INFRA_VERDICT, PROPOSE_ISSUE, and PROPOSE_COMMENT.',
  ].join('\n');

  let memoriesBlock = '';
  if (memories && memories.length > 0) {
    const lines = memories.map(m => `- [${m.type}] (id:${m.id}) ${m.content}`).join('\n');
    memoriesBlock = `\n\n## Relevant memories:\n${lines}`;
  }

  let fullPrompt: string;
  if (conversationHistory && conversationHistory.length > 0) {
    fullPrompt = `${systemPrompt}${memoriesBlock}\n\n## Conversation so far:\n${conversationHistory.join('\n')}\n\n## New question:\n${question}`;
  } else {
    fullPrompt = `${systemPrompt}${memoriesBlock}\n\n${question}`;
  }

  const grafanaUrl = process.env['GRAFANA_URL'];
  const grafanaToken = process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'];
  const hasGrafana = Boolean(grafanaUrl) && Boolean(grafanaToken);

  let mcpConfigArg: string[] = [];
  if (hasGrafana) {
    const mcpConfig = JSON.stringify({
      mcpServers: {
        'grafana-caesar': {
          command: '/usr/local/bin/mcp-grafana',
          args: [],
          env: {
            GRAFANA_URL: grafanaUrl,
            GRAFANA_SERVICE_ACCOUNT_TOKEN: grafanaToken,
          },
        },
      },
    });
    mcpConfigArg = ['--mcp-config', mcpConfig];
  }

  const baseTools = ['Read', 'Glob', 'Grep', 'Bash(kubectl:*)'];
  const grafanaTools = hasGrafana
    ? [
        'mcp__grafana-caesar__query_loki_logs',
        'mcp__grafana-caesar__query_loki_stats',
        'mcp__grafana-caesar__query_prometheus',
        'mcp__grafana-caesar__list_loki_label_names',
        'mcp__grafana-caesar__list_loki_label_values',
        'mcp__grafana-caesar__list_prometheus_metric_names',
        'mcp__grafana-caesar__list_datasources',
      ]
    : [];
  const allowedTools = [...baseTools, ...grafanaTools].join(',');

  console.log('[claude] Starting subprocess (timeout: 600s)');
  const { stdout } = await execa(
    'claude',
    [
      '-p', fullPrompt,
      '--model', 'claude-sonnet-4-6',
      '--allowedTools', allowedTools,
      ...mcpConfigArg,
      '--output-format', 'json',
    ],
    {
      stdin: 'ignore',
      env: { ...process.env },
      timeout: 600_000,
    },
  );
  console.log('[claude] Subprocess completed, parsing response');

  const parsed = JSON.parse(stdout) as { result: string };
  const resultText = parsed.result;

  // Parse and strip INFRA_VERDICT marker
  let infraVerdict: InfraVerdict | null = null;
  const verdictMarker = 'INFRA_VERDICT:';
  const verdictIndex = resultText.indexOf(verdictMarker);
  if (verdictIndex !== -1) {
    const verdictString = resultText.slice(verdictIndex + verdictMarker.length).split('\n')[0];
    const parts = verdictString.split('|||');
    if (parts.length >= 2) {
      const type = parts[0].trim() as 'infrastructure' | 'code';
      const summary = parts[1].trim();
      infraVerdict = { type, summary };
    }
  }

  let cleanAnswer = resultText;
  if (verdictIndex !== -1) {
    cleanAnswer = resultText.slice(0, verdictIndex).trimEnd();
  }

  // Parse and strip PROPOSE_ISSUE marker
  let issueProposal: IssueProposal | null = null;
  const proposeMarker = 'PROPOSE_ISSUE:';
  const proposeIndex = cleanAnswer.indexOf(proposeMarker);
  if (proposeIndex !== -1) {
    const proposeString = cleanAnswer.slice(proposeIndex + proposeMarker.length);
    const parts = proposeString.split('|||');
    if (parts.length >= 3) {
      const repoName = parts[0].trim() as 'all-chat' | 'all-chat-extension';
      const title = parts[1].trim();
      const body = parts.slice(2).join('|||').trim();
      issueProposal = { repo: repoName, title, body };
    }
    cleanAnswer = cleanAnswer.slice(0, proposeIndex).trimEnd();
  }

  // Parse and strip PROPOSE_COMMENT marker
  let commentProposal: CommentProposal | null = null;
  const commentMarker = 'PROPOSE_COMMENT:';
  const commentIndex = cleanAnswer.indexOf(commentMarker);
  if (commentIndex !== -1) {
    const commentString = cleanAnswer.slice(commentIndex + commentMarker.length);
    const parts = commentString.split('|||');
    if (parts.length >= 3) {
      const repoName = parts[0].trim();
      const issueNumber = parseInt(parts[1].trim(), 10);
      const body = parts.slice(2).join('|||').trim();
      if ((repoName === 'all-chat' || repoName === 'all-chat-extension') && !isNaN(issueNumber)) {
        commentProposal = { repo: repoName, issueNumber, body };
      }
    }
    cleanAnswer = cleanAnswer.slice(0, commentIndex).trimEnd();
  }

  // Parse and strip STORE_MEMORY marker
  const storeMarker = 'STORE_MEMORY:';
  const storeIndex = cleanAnswer.indexOf(storeMarker);
  let memoryMarker: ParsedMemoryMarker | null = null;
  if (storeIndex !== -1) {
    const markerString = cleanAnswer.slice(storeIndex + storeMarker.length).split('\n')[0];
    const parts = markerString.split('|||');
    if (parts.length >= 3) {
      const type = parts[0].trim() as MemoryType;
      const tags = parts[1].trim().split(',').map(t => t.trim()).filter(Boolean);
      const content = parts.slice(2).join('|||').trim();
      if (['error_pattern', 'correction', 'codebase_insight'].includes(type)) {
        memoryMarker = { type, tags, content };
      }
    }
    cleanAnswer = cleanAnswer.slice(0, storeIndex).trimEnd();
  }

  // Parse and strip UPDATE_MEMORY marker
  const updateMarker = 'UPDATE_MEMORY:';
  const updateIndex = cleanAnswer.indexOf(updateMarker);
  let updateMemoryMarker: ParsedUpdateMemoryMarker | null = null;
  if (updateIndex !== -1) {
    const markerString = cleanAnswer.slice(updateIndex + updateMarker.length).split('\n')[0];
    const parts = markerString.split('|||');
    if (parts.length >= 2) {
      const id = parseInt(parts[0].trim(), 10);
      const content = parts.slice(1).join('|||').trim();
      if (!isNaN(id)) {
        updateMemoryMarker = { id, content };
      }
    }
    cleanAnswer = cleanAnswer.slice(0, updateIndex).trimEnd();
  }

  return { answer: cleanAnswer, issueProposal, commentProposal, infraVerdict, memoryMarker, updateMemoryMarker };
}
