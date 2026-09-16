// Chrome ClientHello spec for tls-impersonate. Vendored from
// httptoolkit/node-tls-impersonate test/chrome.spec.ts (Apache-2.0), which
// pins the expected JA3 f418dd9b4f923541607d5763fa771b1f and
// JA4 t13d1516h2_8daaf6152771_d8a2da3f94cd for Chrome 133+. Kept verbatim
// (modulo formatting) so future Chrome majors can be diffed against upstream.
import type { ClientHelloSpec } from 'tls-impersonate';

export const CHROME_CLIENT_HELLO: ClientHelloSpec = {
  cipherSuites: [
    0x3a3a, // GREASE cipher (skipped by impersonate, not in fingerprint)
    // TLS 1.3 (Chrome order: AES-128-GCM, AES-256-GCM, ChaCha20)
    0x1301, 0x1302, 0x1303,
    // TLS 1.2 (Chrome order)
    0xc02b, 0xc02f, 0xc02c, 0xc030,
    0xcca9, 0xcca8,
    0xc013, 0xc014,
    0x009c, 0x009d, 0x002f, 0x0035
  ],
  extensions: [
    { type: 0x2a2a }, // GREASE extension 1
    { type: 0 }, // server_name
    { type: 23 }, // extended_master_secret
    { type: 65281 }, // renegotiation_info
    { type: 10 }, // supported_groups
    { type: 11 }, // ec_point_formats
    { type: 35 }, // session_ticket
    { type: 16 }, // ALPN
    { type: 5 }, // status_request (OCSP)
    { type: 18 }, // signed_certificate_timestamp (default: empty)
    { type: 27, data: Buffer.from([0x02, 0x00, 0x02]) }, // compress_certificate: brotli only
    { type: 13 }, // signature_algorithms
    { type: 43 }, // supported_versions
    { type: 45 }, // psk_key_exchange_modes
    { type: 51 }, // key_share
    { type: 17613 }, // application_settings (ALPS, default data: "h2")
    { type: 65037 }, // encrypted_client_hello (GREASE ECH, default data)
    { type: 0x4a4a } // GREASE extension 2
  ],
  supportedGroups: [
    0x6a6a, // GREASE group (skipped)
    0x11ec, 0x001d, 0x0017, 0x0018
  ],
  signatureAlgorithms: [
    0x0403, 0x0804, 0x0401, 0x0503, 0x0805, 0x0501, 0x0806, 0x0601
  ],
  alpnProtocols: ['h2', 'http/1.1']
};
