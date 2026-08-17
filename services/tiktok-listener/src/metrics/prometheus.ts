/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Prometheus Metrics for TikTok Listener
 *
 * Provides metrics for monitoring connection health, message processing,
 * circuit breaker state, and error classification.
 */

import { Counter, Gauge, Histogram, Registry } from 'prom-client';
import { Logger } from '../types/logger.js';

export class PrometheusMetrics {
  private registry: Registry;
  private logger: Logger;

  // Connection health metrics
  private heartbeatTimeouts: Counter<string>;
  private heartbeatLastMessage: Gauge<string>;

  // Message processing metrics
  private messagesQueued: Gauge<string>;
  private messagesDropped: Counter<string>;
  private messageQueueSize: Gauge<string>;

  // Circuit breaker metrics
  private circuitBreakerState: Gauge<string>;
  private circuitBreakerTrips: Counter<string>;

  // Connection pooling metrics
  private pooledConnections: Gauge;
  private connectionSubscribers: Gauge<string>;

  // Error classification metrics
  private errorsByType: Counter<string>;

  // Backoff and detection metrics (NEW)
  private backoffCurrentInterval: Gauge<string>;
  private backoffUsernamesStuck: Gauge;
  private detectionSkippedTotal: Counter<string>;
  private usernamesAtRisk: Gauge<string>;
  private autoRecoveryTotal: Counter<string>;

  // Wire-format visibility
  private wireMessages: Counter<string>;
  private envelopeFrames: Counter<string>;

  // WebSocket signature provenance (issue #698: retiring Euler Stream)
  private signAttempts: Counter<string>;
  private signDuration: Histogram<string>;

  constructor(logger: Logger) {
    this.logger = logger;
    this.registry = new Registry();

    // Initialize connection health metrics
    this.heartbeatTimeouts = new Counter({
      name: 'tiktok_heartbeat_timeouts_total',
      help: 'Total number of heartbeat timeouts detected',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.heartbeatLastMessage = new Gauge({
      name: 'tiktok_heartbeat_last_message_timestamp',
      help: 'Unix timestamp of last message received',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize message processing metrics
    this.messagesQueued = new Gauge({
      name: 'tiktok_messages_queued',
      help: 'Current number of messages queued for processing',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.messagesDropped = new Counter({
      name: 'tiktok_messages_dropped_total',
      help: 'Total number of messages dropped',
      labelNames: ['username', 'reason'],
      registers: [this.registry]
    });

    this.messageQueueSize = new Gauge({
      name: 'tiktok_message_queue_size',
      help: 'Current size of message queue per username',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize circuit breaker metrics
    this.circuitBreakerState = new Gauge({
      name: 'tiktok_circuit_breaker_state',
      help: 'Circuit breaker state (0=closed, 1=open)',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.circuitBreakerTrips = new Counter({
      name: 'tiktok_circuit_breaker_trips_total',
      help: 'Total number of circuit breaker trips',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize connection pooling metrics
    this.pooledConnections = new Gauge({
      name: 'tiktok_pooled_connections',
      help: 'Current number of pooled connections',
      registers: [this.registry]
    });

    this.connectionSubscribers = new Gauge({
      name: 'tiktok_connection_subscribers',
      help: 'Number of subscribers per pooled connection',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize error classification metrics
    this.errorsByType = new Counter({
      name: 'tiktok_errors_by_type_total',
      help: 'Total number of errors by type',
      labelNames: ['username', 'type'],
      registers: [this.registry]
    });

    // Initialize backoff and detection metrics (NEW)
    this.backoffCurrentInterval = new Gauge({
      name: 'tiktok_backoff_current_interval_ms',
      help: 'Current backoff interval per username in milliseconds',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.backoffUsernamesStuck = new Gauge({
      name: 'tiktok_backoff_usernames_stuck',
      help: 'Number of usernames stuck in backoff >5 minutes',
      registers: [this.registry]
    });

    this.detectionSkippedTotal = new Counter({
      name: 'tiktok_detection_skipped_total',
      help: 'Detections skipped by reason',
      labelNames: ['reason'], // backoff, error, offline
      registers: [this.registry]
    });

    this.usernamesAtRisk = new Gauge({
      name: 'tiktok_usernames_at_risk',
      help: 'Usernames with long backoff (risk level)',
      labelNames: ['risk_level'], // high, medium, low
      registers: [this.registry]
    });

    this.autoRecoveryTotal = new Counter({
      name: 'tiktok_auto_recovery_total',
      help: 'Automatic stuck state recoveries',
      labelNames: ['username', 'reason'], // max_backoff_stuck
      registers: [this.registry]
    });

    // Wire-format visibility. TikTok's unofficial protocol drifts without notice and the connector
    // drops any frame whose method is missing from its schema *silently* (see `hasProtoName` in
    // tiktok-live-connector), so a renamed message looks exactly like a message that was never
    // sent. PR #539's empty-comment breakage and the coin chest never surfacing both had that
    // shape, and neither was visible from our side at all.
    //
    // This counts only what *decoded*, so read it as a baseline of what TikTok sends rather than as
    // a diagnosis: a method missing from it may have been dropped by TikTok, renamed, or broken in
    // decode, and those three are not separable here (the connector's DEBUG_DESERIALIZE_XD env var
    // is the only hook that names an unknown method). Its value is the negative result — proving a
    // message is not arriving in decodable form, which previously took a prod investigation.
    //
    // Cardinality is bounded: `method` is a proto type name from a fixed schema, never user input.
    this.wireMessages = new Counter({
      name: 'tiktok_wire_messages_total',
      help: 'Decoded TikTok wire messages by protobuf method name',
      labelNames: ['method'],
      registers: [this.registry]
    });

    // Envelope frames get their own outcome counter because the ENVELOPE message multiplexes
    // several products and is filtered down to coin chests, so "no chest appeared" has several
    // distinct causes that must be told apart without redeploying at debug level.
    this.envelopeFrames = new Counter({
      name: 'tiktok_envelope_frames_total',
      help: 'ENVELOPE frames by outcome (published, or the reason it was not)',
      labelNames: ['outcome'], // published, super_fan_box, not_a_drop, no_chest_payload, duplicate, error
      registers: [this.registry]
    });

    // Signature attempts, by who signed and how it went.
    //
    // This is the metric that decides whether #698 can be finished. Retiring Euler Stream trades
    // a cost ceiling for an on-call burden: after cutover, a TikTok change to the signing
    // algorithm takes TikTok ingest fully down, where today it is Euler's problem. That trade is
    // only defensible if we can see our own success rate before committing, which is why shadow
    // mode records the `self` signer's outcome for connections it is not actually serving.
    //
    // `load_bearing` is what separates the two readings. Filtered to `true`, this is availability
    // — did the connection get signed at all. Filtered to `false`, it is the shadow experiment:
    // how our signer would have done, on live rooms, with nothing riding on it.
    //
    // Cardinality is bounded on every label: `signer` is one of a handful of composed names,
    // `outcome` is success/failure, and `reason` comes from classifySignatureFailure's fixed set
    // rather than from raw error text.
    this.signAttempts = new Counter({
      name: 'tiktok_sign_attempts_total',
      help: 'WebSocket signature attempts by signer, outcome and failure reason',
      labelNames: ['signer', 'outcome', 'reason', 'load_bearing'],
      registers: [this.registry]
    });

    // Signing latency matters independently of success rate: Euler is a network round trip to a
    // third party, and an in-process signer should be markedly faster. If it is not, that is a
    // sign we have reimplemented the round trip rather than removed it.
    this.signDuration = new Histogram({
      name: 'tiktok_sign_duration_seconds',
      help: 'Time taken to obtain a signed WebSocket handshake, by signer and outcome',
      labelNames: ['signer', 'outcome'],
      buckets: [0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10],
      registers: [this.registry]
    });

    this.logger.info('Prometheus metrics initialized');
  }

  /**
   * Record one WebSocket signature attempt.
   *
   * Implements the `SignObserver` interface from `../sign/shadow.js` structurally, so the sign
   * layer can report into Prometheus without importing it.
   *
   * @param attempt Which signer ran, how it went, how long it took, and whether the result was
   *                actually used to connect.
   */
  recordSignAttempt(attempt: {
    signer: string;
    outcome: 'success' | 'failure';
    reason?: string;
    durationMs: number;
    loadBearing: boolean;
  }): void {
    this.signAttempts.inc({
      signer: attempt.signer,
      outcome: attempt.outcome,
      // Always present, so the series does not change shape between success and failure — an
      // absent label and an empty one are different series to Prometheus.
      reason: attempt.reason ?? 'none',
      load_bearing: String(attempt.loadBearing)
    });

    this.signDuration.observe(
      { signer: attempt.signer, outcome: attempt.outcome },
      attempt.durationMs / 1000
    );
  }

  // Wire-format visibility methods
  recordWireMessage(method: string): void {
    this.wireMessages.inc({ method });
  }

  recordEnvelopeFrame(outcome: string): void {
    this.envelopeFrames.inc({ outcome });
  }

  // Heartbeat monitoring methods
  recordHeartbeatTimeout(username: string): void {
    this.heartbeatTimeouts.inc({ username });
  }

  recordHeartbeatMessage(username: string, timestamp: number): void {
    this.heartbeatLastMessage.set({ username }, timestamp);
  }

  // Message processing methods
  recordMessageQueued(username: string, queueSize: number): void {
    this.messagesQueued.set({ username }, queueSize);
    this.messageQueueSize.set({ username }, queueSize);
  }

  recordMessageDropped(username: string, reason: string): void {
    this.messagesDropped.inc({ username, reason });
  }

  // Circuit breaker methods
  recordCircuitBreakerState(username: string, state: number): void {
    this.circuitBreakerState.set({ username }, state);
  }

  recordCircuitBreakerTrip(username: string): void {
    this.circuitBreakerTrips.inc({ username });
  }

  // Connection pooling methods
  recordPooledConnections(count: number): void {
    this.pooledConnections.set(count);
  }

  recordConnectionSubscribers(username: string, count: number): void {
    this.connectionSubscribers.set({ username }, count);
  }

  // Error classification methods
  recordError(username: string, errorType: string): void {
    this.errorsByType.inc({ username, type: errorType });
  }

  // Backoff and detection methods (NEW)
  recordBackoffInterval(username: string, intervalMs: number): void {
    this.backoffCurrentInterval.set({ username }, intervalMs);
  }

  setBackoffUsernamesStuck(count: number): void {
    this.backoffUsernamesStuck.set(count);
  }

  recordDetectionSkipped(reason: string): void {
    this.detectionSkippedTotal.inc({ reason });
  }

  setUsernamesAtRisk(riskLevel: string, count: number): void {
    this.usernamesAtRisk.set({ risk_level: riskLevel }, count);
  }

  recordAutoRecovery(username: string, reason: string): void {
    this.autoRecoveryTotal.inc({ username, reason });
  }

  // Cleanup methods
  clearMetricsForUsername(username: string): void {
    // Clear all labeled metrics for a username
    this.heartbeatLastMessage.remove({ username });
    this.messagesQueued.remove({ username });
    this.messageQueueSize.remove({ username });
    this.circuitBreakerState.remove({ username });
    this.connectionSubscribers.remove({ username });
    this.backoffCurrentInterval.remove({ username }); // NEW

    this.logger.debug('Cleared metrics for username', { username });
  }

  // Get metrics for Prometheus scraping
  async getMetrics(): Promise<string> {
    return await this.registry.metrics();
  }

  // Get content type for HTTP response
  getContentType(): string {
    return this.registry.contentType;
  }
}
