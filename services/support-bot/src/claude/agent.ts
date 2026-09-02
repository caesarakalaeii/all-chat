import { execa } from 'execa';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { QueryResult, IssueProposal, CommentProposal, InfraVerdict, StoredMemory, ParsedMemoryMarker, ParsedUpdateMemoryMarker, MemoryType } from '../types.js';

/**
 * How long the `claude` subprocess gets before it is killed.
 *
 * Exported because the Discord layer needs the same number to tell a user how long
 * it waited; two copies of "10 minutes" drift the moment one is tuned.
 */
export const CLAUDE_TIMEOUT_MS = 600_000;

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
    'You may emit SEVERAL markers in one response -- one per issue or comment. If the user lists four problems and asks for issues, emit four PROPOSE_ISSUE markers, not one covering all four. Put every marker at the very end, each starting on its own line, after all of your prose.',
    'Issue templates live in .github/ISSUE_TEMPLATE/ in each repo. The Caterpillar / agent-task template is .github/ISSUE_TEMPLATE/agent_task.md -- read it directly by path when a template is requested rather than searching the repo for it.',
    'Filing several issues is expensive. Budget your research: skim what you need to make each issue concrete and stop there, rather than fully investigating every one. An answer that arrives is worth more than one that times out.',
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

  // The MCP config carries the Grafana service-account token, so it goes to the
  // subprocess as a FILE rather than on the command line.
  //
  // execa embeds the full argv in every error it throws, and that error is what the
  // catch-all logs. With the config inline, one subprocess timeout printed
  // `"GRAFANA_SERVICE_ACCOUNT_TOKEN":"glsa_..."` in plaintext into the pod log — and
  // from there into any log aggregation scraping this pod. A 0600 file in a private
  // temp dir keeps the secret out of argv, out of the error, and out of `ps`.
  let mcpConfigArg: string[] = [];
  let mcpConfigDir: string | null = null;
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
    mcpConfigDir = await mkdtemp(join(tmpdir(), 'support-bot-mcp-'));
    const mcpConfigPath = join(mcpConfigDir, 'mcp.json');
    await writeFile(mcpConfigPath, mcpConfig, { mode: 0o600 });
    mcpConfigArg = ['--mcp-config', mcpConfigPath];
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

  console.log(`[claude] Starting subprocess (timeout: ${CLAUDE_TIMEOUT_MS / 1000}s)`);
  let stdout: string;
  try {
    ({ stdout } = await execa(
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
        timeout: CLAUDE_TIMEOUT_MS,
      },
    ));
  } finally {
    // Unconditional, and only AFTER the subprocess has exited: claude reads the config
    // at startup, so removing it earlier would race the read. Losing the cleanup would
    // leave a token-bearing file behind, which is the thing this whole detour avoids.
    if (mcpConfigDir !== null) {
      await rm(mcpConfigDir, { recursive: true, force: true }).catch((err: unknown) => {
        console.warn('[claude] Failed to remove temporary MCP config dir:', err);
      });
    }
  }
  console.log('[claude] Subprocess completed, parsing response');

  const parsed = JSON.parse(stdout) as { result: string };
  return parseMarkers(parsed.result);
}

/** The repos the bot is allowed to act on. Anything else is dropped, not guessed at. */
const KNOWN_REPOS = ['all-chat', 'all-chat-extension'] as const;
type KnownRepo = (typeof KNOWN_REPOS)[number];

function isKnownRepo(value: string): value is KnownRepo {
  return (KNOWN_REPOS as readonly string[]).includes(value);
}

/** Every trailing marker the model may append. Order here is irrelevant; position in the text decides. */
const MARKERS = [
  'INFRA_VERDICT:',
  'PROPOSE_ISSUE:',
  'PROPOSE_COMMENT:',
  'STORE_MEMORY:',
  'UPDATE_MEMORY:',
] as const;

type Marker = (typeof MARKERS)[number];

interface MarkerSegment {
  marker: Marker;
  payload: string;
}

/**
 * Splits a response into its prose and its trailing marker segments.
 *
 * A segment runs from the end of its marker to the start of the NEXT marker, which is
 * what makes multiple markers parseable at all: an issue body is Markdown and contains
 * newlines, so a marker's payload cannot be delimited by the end of a line. The previous
 * parser took `indexOf` of each marker once and read the payload to the end of the
 * string, so a second PROPOSE_ISSUE was swallowed into the first one's body and only one
 * issue was ever filed no matter how many the user asked for.
 *
 * Single-line markers (INFRA_VERDICT, STORE_MEMORY, UPDATE_MEMORY) take the first line of
 * their segment, which preserves the old behaviour for them.
 */
function splitMarkers(text: string): { prose: string; segments: MarkerSegment[] } {
  const hits: Array<{ index: number; marker: Marker }> = [];
  for (const marker of MARKERS) {
    let from = 0;
    for (;;) {
      const index = text.indexOf(marker, from);
      if (index === -1) break;
      hits.push({ index, marker });
      from = index + marker.length;
    }
  }
  hits.sort((a, b) => a.index - b.index);

  const segments = hits.map((hit, i) => ({
    marker: hit.marker,
    payload: text.slice(hit.index + hit.marker.length, i + 1 < hits.length ? hits[i + 1].index : text.length),
  }));

  return {
    prose: hits.length > 0 ? text.slice(0, hits[0].index).trimEnd() : text,
    segments,
  };
}

/** Parses a model response into prose plus whatever actions it asked for. */
export function parseMarkers(resultText: string): QueryResult {
  const { prose, segments } = splitMarkers(resultText);

  const issueProposals: IssueProposal[] = [];
  const commentProposals: CommentProposal[] = [];
  let infraVerdict: InfraVerdict | null = null;
  let memoryMarker: ParsedMemoryMarker | null = null;
  let updateMemoryMarker: ParsedUpdateMemoryMarker | null = null;

  for (const { marker, payload } of segments) {
    switch (marker) {
      case 'PROPOSE_ISSUE:': {
        const parts = payload.split('|||');
        if (parts.length < 3) break;
        const repo = parts[0].trim();
        // Validated rather than cast. The old code asserted the repo name was one of the
        // two without checking, so a typo went straight to the GitHub API as a 404.
        if (!isKnownRepo(repo)) break;
        const title = parts[1].trim();
        const body = parts.slice(2).join('|||').trim();
        if (!title) break;
        issueProposals.push({ repo, title, body });
        break;
      }
      case 'PROPOSE_COMMENT:': {
        const parts = payload.split('|||');
        if (parts.length < 3) break;
        const repo = parts[0].trim();
        const issueNumber = parseInt(parts[1].trim(), 10);
        const body = parts.slice(2).join('|||').trim();
        if (!isKnownRepo(repo) || isNaN(issueNumber)) break;
        commentProposals.push({ repo, issueNumber, body });
        break;
      }
      case 'INFRA_VERDICT:': {
        if (infraVerdict !== null) break;
        const parts = payload.split('\n')[0].split('|||');
        if (parts.length < 2) break;
        infraVerdict = { type: parts[0].trim() as 'infrastructure' | 'code', summary: parts[1].trim() };
        break;
      }
      case 'STORE_MEMORY:': {
        if (memoryMarker !== null) break;
        const parts = payload.split('\n')[0].split('|||');
        if (parts.length < 3) break;
        const type = parts[0].trim() as MemoryType;
        if (!['error_pattern', 'correction', 'codebase_insight'].includes(type)) break;
        memoryMarker = {
          type,
          tags: parts[1].trim().split(',').map(t => t.trim()).filter(Boolean),
          content: parts.slice(2).join('|||').trim(),
        };
        break;
      }
      case 'UPDATE_MEMORY:': {
        if (updateMemoryMarker !== null) break;
        const parts = payload.split('\n')[0].split('|||');
        if (parts.length < 2) break;
        const id = parseInt(parts[0].trim(), 10);
        if (isNaN(id)) break;
        updateMemoryMarker = { id, content: parts.slice(1).join('|||').trim() };
        break;
      }
    }
  }

  return { answer: prose, issueProposals, commentProposals, infraVerdict, memoryMarker, updateMemoryMarker };
}
