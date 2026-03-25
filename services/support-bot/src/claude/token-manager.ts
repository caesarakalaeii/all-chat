import { readFileSync } from 'fs';

const OAUTH_TOKEN_URL = 'https://platform.claude.com/v1/oauth/token';
const OAUTH_CLIENT_ID = '22422756-60c9-4084-8eb7-27705fd5cf9a';
const REFRESH_BUFFER_MS = 15 * 60 * 1000; // 15 minutes

export interface ClaudeCredentials {
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
  scopes: string[];
  subscriptionType: string;
  rateLimitTier: string;
}

interface CredentialsEnvelope {
  claudeAiOauth: ClaudeCredentials;
}

interface OAuthTokenResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

export class ClaudeTokenManager {
  private creds: ClaudeCredentials;
  private readonly k8sApiBase: string;
  private readonly namespace: string;
  private readonly secretName = 'allchat-secrets';
  private readonly secretKey = 'claude-code-oauth-credentials';

  constructor(credentialsJson: string) {
    const envelope = JSON.parse(credentialsJson) as CredentialsEnvelope;
    this.creds = envelope.claudeAiOauth;

    // In-cluster K8s API
    const host = process.env['KUBERNETES_SERVICE_HOST'] ?? 'kubernetes.default.svc.cluster.local';
    const port = process.env['KUBERNETES_SERVICE_PORT'] ?? '443';
    this.k8sApiBase = `https://${host}:${port}`;

    // Namespace from mounted service account
    try {
      this.namespace = readFileSync(
        '/var/run/secrets/kubernetes.io/serviceaccount/namespace',
        'utf-8',
      ).trim();
    } catch {
      this.namespace = process.env['POD_NAMESPACE'] ?? 'allchat';
    }
  }

  async ensureFreshToken(): Promise<string> {
    const now = Date.now();
    if (this.creds.expiresAt - now > REFRESH_BUFFER_MS) {
      return this.creds.accessToken;
    }

    console.log('[token-manager] Token expires soon, refreshing...');
    await this.refresh();
    return this.creds.accessToken;
  }

  private async refresh(): Promise<void> {
    const body = new URLSearchParams({
      grant_type: 'refresh_token',
      refresh_token: this.creds.refreshToken,
      client_id: OAUTH_CLIENT_ID,
    });

    const response = await fetch(OAUTH_TOKEN_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: body.toString(),
    });

    if (!response.ok) {
      const text = await response.text();
      const fallback = process.env['CLAUDE_CODE_OAUTH_TOKEN'];
      if (fallback) {
        console.warn(`[token-manager] OAuth refresh failed (${response.status}), falling back to CLAUDE_CODE_OAUTH_TOKEN`);
        this.creds = { ...this.creds, accessToken: fallback, expiresAt: Date.now() + 365 * 24 * 60 * 60 * 1000 };
        return;
      }
      throw new Error(`OAuth refresh failed (${response.status}): ${text}`);
    }

    const data = (await response.json()) as OAuthTokenResponse;
    const expiresAt = Date.now() + data.expires_in * 1000;

    this.creds = {
      ...this.creds,
      accessToken: data.access_token,
      refreshToken: data.refresh_token,
      expiresAt,
    };

    console.log(`[token-manager] Token refreshed, expires at ${new Date(expiresAt).toISOString()}`);

    // Persist new credentials back to K8s secret (best-effort)
    try {
      await this.patchK8sSecret();
    } catch (err) {
      console.warn('[token-manager] Failed to persist refreshed credentials to K8s secret:', err);
      console.warn('[token-manager] Continuing with in-memory refreshed token');
    }
  }

  private async patchK8sSecret(): Promise<void> {
    const saTokenPath = '/var/run/secrets/kubernetes.io/serviceaccount/token';
    let saToken: string;
    try {
      saToken = readFileSync(saTokenPath, 'utf-8').trim();
    } catch {
      throw new Error('Could not read service account token — RBAC patch skipped');
    }

    const envelope: CredentialsEnvelope = { claudeAiOauth: this.creds };
    const newValue = Buffer.from(JSON.stringify(envelope)).toString('base64');

    const patchUrl = `${this.k8sApiBase}/api/v1/namespaces/${this.namespace}/secrets/${this.secretName}`;
    const patch = {
      data: { [this.secretKey]: newValue },
    };

    const response = await fetch(patchUrl, {
      method: 'PATCH',
      headers: {
        Authorization: `Bearer ${saToken}`,
        'Content-Type': 'application/strategic-merge-patch+json',
      },
      body: JSON.stringify(patch),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`K8s secret patch failed (${response.status}): ${text}`);
    }

    console.log('[token-manager] K8s secret updated with refreshed credentials');
  }
}
