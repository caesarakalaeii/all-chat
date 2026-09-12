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
 * MERCHANTABILITY or FITNESS FOR A particular purpose. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Silent-failure contract of the heartbeat monitor.
 *
 * The monitor is the ONLY component that recovers a zombie WebSocket: a connection the
 * library still reports as CONNECTED but that has stopped delivering frames. When its
 * state probe throws — as it did for two months after the 2.4.0 connector upgrade, where
 * the old `getState()` call was gone — the catch branch returns and the zombie is never
 * reconnected. That regression is the reason this suite exists.
 *
 * The connection is replaced with a fake shaped like the connector's real 2.4.0 API
 * (`state` getter, `disconnect()`), so the suite runs offline and deterministically.
 * The `state` getter is HARDCODED in the fakes rather than discovered from the connector:
 * a future connector API change must fail these tests loudly, not silently change what
 * "connected" means.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { HeartbeatMonitor } from './heartbeat-monitor.js';
import type { Logger } from '../types/logger.js';
import type { PrometheusMetrics } from '../metrics/prometheus.js';

/** Timer values chosen so one interval tick lands past the timeout. */
const INTERVAL_MS = 30_000;
const TIMEOUT_MS = 90_000;

interface FakeConnectionOptions {
  isConnected: boolean;
}

/**
 * Fake connector connection exposing the 2.4.0 surface the monitor reads.
 * `state` throws if the getter itself is misconfigured, mirroring the real class.
 */
function fakeConnection({ isConnected }: FakeConnectionOptions) {
  return {
    get state() {
      return {
        isConnected,
        isConnecting: false,
        roomId: '123',
        roomInfo: null,
        availableGifts: null
      };
    },
    disconnect: vi.fn()
  };
}

function fakeLogger() {
  const logger = {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn()
  } as unknown as Logger;
  return logger;
}

function fakeMetrics() {
  return {
    recordHeartbeatMessage: vi.fn(),
    recordHeartbeatTimeout: vi.fn(),
    clearMetricsForUsername: vi.fn()
  } as unknown as PrometheusMetrics;
}

describe('HeartbeatMonitor silent-failure recovery', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('forces a disconnect when the library reports connected but no message arrived within the timeout', () => {
    const logger = fakeLogger();
    const metrics = fakeMetrics();
    const monitor = new HeartbeatMonitor(logger, metrics, INTERVAL_MS, TIMEOUT_MS);
    const connection = fakeConnection({ isConnected: true });

    monitor.start('lejoe_tiktok', connection as never);
    // No recordMessage: the stream is silent.
    vi.advanceTimersByTime(TIMEOUT_MS + INTERVAL_MS);

    expect(connection.disconnect).toHaveBeenCalledTimes(1);
    expect(metrics.recordHeartbeatTimeout).toHaveBeenCalledWith('lejoe_tiktok');
    expect(monitor.getStats().total_monitored).toBe(0);
  });

  it('does not disconnect while messages keep arriving', () => {
    const logger = fakeLogger();
    const metrics = fakeMetrics();
    const monitor = new HeartbeatMonitor(logger, metrics, INTERVAL_MS, TIMEOUT_MS);
    const connection = fakeConnection({ isConnected: true });

    monitor.start('lejoe_tiktok', connection as never);
    // Simulate a healthy stream: a message just before every check.
    vi.advanceTimersByTime(INTERVAL_MS);
    monitor.recordMessage('lejoe_tiktok');
    vi.advanceTimersByTime(INTERVAL_MS);
    monitor.recordMessage('lejoe_tiktok');
    vi.advanceTimersByTime(INTERVAL_MS);

    expect(connection.disconnect).not.toHaveBeenCalled();
    expect(monitor.getStats().total_monitored).toBe(1);
  });

  it('skips forced reconnection when the library already reports disconnected', () => {
    const logger = fakeLogger();
    const metrics = fakeMetrics();
    const monitor = new HeartbeatMonitor(logger, metrics, INTERVAL_MS, TIMEOUT_MS);
    const connection = fakeConnection({ isConnected: false });

    monitor.start('lejoe_tiktok', connection as never);
    vi.advanceTimersByTime(TIMEOUT_MS + INTERVAL_MS);

    expect(connection.disconnect).not.toHaveBeenCalled();
    expect(metrics.recordHeartbeatTimeout).not.toHaveBeenCalled();
  });

  it('treats a throwing state getter as healthy and does not disconnect', () => {
    const logger = fakeLogger();
    const metrics = fakeMetrics();
    const monitor = new HeartbeatMonitor(logger, metrics, INTERVAL_MS, TIMEOUT_MS);
    const disconnect = vi.fn();
    const connection = {
      get state() {
        throw new Error('state getter unavailable');
      },
      disconnect
    };

    monitor.start('lejoe_tiktok', connection as never);
    vi.advanceTimersByTime(TIMEOUT_MS + INTERVAL_MS);

    expect(disconnect).not.toHaveBeenCalled();
    expect(logger.warn).toHaveBeenCalledWith(
      'Failed to get connection state from library',
      expect.objectContaining({ username: 'lejoe_tiktok' })
    );
  });
});
