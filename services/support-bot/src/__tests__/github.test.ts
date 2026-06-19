import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock @octokit/rest
vi.mock('@octokit/rest', () => ({
  Octokit: vi.fn().mockImplementation(() => ({
    rest: {
      issues: {
        create: vi.fn().mockResolvedValue({
          data: {
            html_url: 'https://github.com/moersener/all-chat/issues/42',
          },
        }),
        createComment: vi.fn().mockResolvedValue({
          data: {
            html_url: 'https://github.com/moersener/all-chat/issues/42#issuecomment-1',
          },
        }),
      },
    },
  })),
}));

import { Octokit } from '@octokit/rest';
import { createOctokitClient, createIssue, createComment } from '../github/issues.js';

describe('createOctokitClient', () => {
  it('returns an Octokit instance', () => {
    const client = createOctokitClient('test-token');
    expect(client).toBeDefined();
    expect(Octokit).toHaveBeenCalledWith({ auth: 'test-token' });
  });
});

describe('createIssue', () => {
  let mockOctokit: InstanceType<typeof Octokit>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockOctokit = {
      rest: {
        issues: {
          create: vi.fn().mockResolvedValue({
            data: {
              html_url: 'https://github.com/moersener/all-chat/issues/42',
            },
          }),
        },
      },
    } as unknown as InstanceType<typeof Octokit>;
  });

  it('calls octokit.rest.issues.create with correct args including labels', async () => {
    await createIssue(
      mockOctokit,
      'moersener',
      'all-chat',
      'Fix bug',
      '## Details\nThis needs fixing.',
    );

    expect(mockOctokit.rest.issues.create).toHaveBeenCalledWith({
      owner: 'moersener',
      repo: 'all-chat',
      title: 'Fix bug',
      body: '## Details\nThis needs fixing.',
      labels: ['bot-proposed', 'needs-review'],
    });
  });

  it('returns the html_url from the response', async () => {
    const url = await createIssue(
      mockOctokit,
      'moersener',
      'all-chat',
      'Fix bug',
      '## Details\n...',
    );

    expect(url).toBe('https://github.com/moersener/all-chat/issues/42');
  });
});

describe('createComment', () => {
  let mockOctokit: InstanceType<typeof Octokit>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockOctokit = {
      rest: {
        issues: {
          createComment: vi.fn().mockResolvedValue({
            data: {
              html_url: 'https://github.com/moersener/all-chat/issues/447#issuecomment-99',
            },
          }),
        },
      },
    } as unknown as InstanceType<typeof Octokit>;
  });

  it('calls octokit.rest.issues.createComment with owner, repo, issue_number, and body', async () => {
    await createComment(
      mockOctokit,
      'moersener',
      'all-chat',
      447,
      'Here is the follow-up.',
    );

    expect(mockOctokit.rest.issues.createComment).toHaveBeenCalledWith({
      owner: 'moersener',
      repo: 'all-chat',
      issue_number: 447,
      body: 'Here is the follow-up.',
    });
  });

  it('returns the html_url of the created comment', async () => {
    const url = await createComment(
      mockOctokit,
      'moersener',
      'all-chat',
      447,
      'A comment.',
    );

    expect(url).toBe('https://github.com/moersener/all-chat/issues/447#issuecomment-99');
  });
});
