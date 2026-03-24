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
