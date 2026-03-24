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
      },
    },
  })),
}));

import { Octokit } from '@octokit/rest';
import { createOctokitClient, createIssue } from '../github/issues.js';

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
