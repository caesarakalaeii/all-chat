import { Octokit } from '@octokit/rest';

export function createOctokitClient(token: string): Octokit {
  return new Octokit({ auth: token });
}

export async function createIssue(
  octokit: Octokit,
  owner: string,
  repo: string,
  title: string,
  body: string,
): Promise<string> {
  const response = await octokit.rest.issues.create({
    owner,
    repo,
    title,
    body,
    labels: ['bot-proposed', 'needs-review'],
  });
  return response.data.html_url;
}

/**
 * Posts a comment on an existing issue or pull request. GitHub's REST API
 * treats PR comments as issue comments, so `issueNumber` may be either an
 * issue or a PR number. Returns the html_url of the created comment.
 */
export async function createComment(
  octokit: Octokit,
  owner: string,
  repo: string,
  issueNumber: number,
  body: string,
): Promise<string> {
  const response = await octokit.rest.issues.createComment({
    owner,
    repo,
    issue_number: issueNumber,
    body,
  });
  return response.data.html_url;
}
