// Chrome-shaped TLS for the listener's outbound WebSocket.
//
// The push-server handshake is the last leg of the connection that still
// presents Node's own TLS ClientHello: the signature and cookies come from a
// real Chromium session (tiktok-signer viewer), but the WebSocket that
// carries the live data rides plain Node TLS. Anti-bot systems increasingly
// reject that mismatch on sight, so the handshake here mirrors Chrome's
// ClientHello (JA3/JA4, see chrome-client-hello.ts) while everything else
// about the connection — CONNECT tunnel through the capture lane's
// residential proxy — stays exactly as it was.
import net from 'node:net';
import type { AgentConnectOpts } from 'agent-base';
import { HttpsProxyAgent } from 'https-proxy-agent';
import { impersonate, isSupported } from 'tls-impersonate';
import { CHROME_CLIENT_HELLO } from './chrome-client-hello.js';

/**
 * Chrome's TLS options, minus the parts ws must not inherit. ALPN is pinned
 * to http/1.1 because the ws upgrade speaks HTTP/1.1: advertising h2 as
 * real Chrome does would negotiate HTTP/2 on a connection the ws client then
 * cannot drive. Memoized on first secure connect.
 */
let chromeTlsOptions: Record<string, unknown> | undefined;

/**
 * HttpsProxyAgent whose endpoint TLS handshake presents Chrome's ClientHello.
 * The base class handles the CONNECT tunnel and proxy auth; the impersonated
 * TLS options only flow into the endpoint handshake (https-proxy-agent
 * merges `opts` into its `tls.connect` call), so the tunnel leg keeps its
 * plain fingerprint — which is correct: the proxy is not the party
 * fingerprinting us.
 */
class ChromeTlsProxyAgent extends HttpsProxyAgent<string> {
  connect(req: unknown, opts: AgentConnectOpts): Promise<net.Socket> {
    if ('secureEndpoint' in opts && opts.secureEndpoint) {
      chromeTlsOptions ??= {
        ...impersonate(CHROME_CLIENT_HELLO).tlsOptions,
        ALPNProtocols: ['http/1.1']
      };
      Object.assign(opts, chromeTlsOptions);
    }
    // agent-base types connect() loosely; the runtime result is the tunnel
    // socket, whatever the declared return type says.
    return super.connect(req as never, opts) as unknown as Promise<net.Socket>;
  }
}
/** The listener's WebSocket egress agent type, whatever shaped its TLS. */
export type WsEgressAgent = HttpsProxyAgent<string>;
/**
 * WebSocket egress agent for a capture lane's proxy. Chrome ClientHello
 * when the runtime supports it (Node >= 24.15 with the tls-impersonate
 * native addon loaded), the plain proxy agent otherwise — a Node-default
 * handshake is what runs today, so the fallback is a no-op change, never
 * a regression.
 */
export function createChromeTlsProxyAgent(proxyUrl: string): HttpsProxyAgent<string> {
  return isSupported() ? new ChromeTlsProxyAgent(proxyUrl) : new HttpsProxyAgent(proxyUrl);
}
