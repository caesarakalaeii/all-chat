// Type the vendored MIT encoder (vendor/xgnarly.mjs) without touching it.
// Signature copied from its own `encode` export; see vendor/PROVENANCE.md.
declare module '*/vendor/xgnarly.mjs' {
  export interface XGnarlyOptions {
    timestampMs?: number;
    ubcode?: number;
    sdkVersion?: string;
    randomLow16?: Buffer;
    random32?: Buffer;
    randomKey?: Buffer;
  }

  export function encode(
    queryString: string,
    body: string,
    userAgent: string,
    counters: {
      totalXHRRequests?: number;
      totalFetchRequests?: number;
      interceptedXHRRequests?: number;
      interceptedFetchRequests?: number;
    },
    options?: XGnarlyOptions
  ): string;
}
