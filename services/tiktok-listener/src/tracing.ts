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
 * OpenTelemetry Tracing Configuration for TikTok Listener
 *
 * Initializes distributed tracing using OpenTelemetry with OTLP gRPC exporter.
 * Compatible with Jaeger, Tempo, and other OTLP-compatible backends.
 *
 * Environment Variables:
 * - OTEL_ENABLED: Enable/disable tracing (default: false)
 * - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP collector endpoint (default: localhost:4317)
 * - ENVIRONMENT: Deployment environment (default: development)
 * - APP_VERSION: Service version (default: dev)
 */

import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { SEMRESATTRS_SERVICE_NAME, SEMRESATTRS_SERVICE_VERSION, SEMRESATTRS_DEPLOYMENT_ENVIRONMENT } from '@opentelemetry/semantic-conventions';

// Global SDK instance
let sdk: NodeSDK | null = null;

/**
 * Initialize OpenTelemetry tracing
 * Must be called before any other imports for auto-instrumentation to work properly
 */
export function initTracing(): void {
  // Check if tracing is enabled
  const enabled = process.env.OTEL_ENABLED === 'true';

  if (!enabled) {
    console.log('[Tracing] OpenTelemetry tracing is disabled');
    return;
  }

  const serviceName = 'tiktok-listener';
  const serviceVersion = process.env.APP_VERSION || 'dev';
  const environment = process.env.ENVIRONMENT || 'development';
  const otlpEndpoint = process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'localhost:4317';

  console.log('[Tracing] Initializing OpenTelemetry tracer', {
    service: serviceName,
    version: serviceVersion,
    environment,
    endpoint: otlpEndpoint,
  });

  try {
    // Create resource with service information
    const resource = resourceFromAttributes({
      [SEMRESATTRS_SERVICE_NAME]: serviceName,
      [SEMRESATTRS_SERVICE_VERSION]: serviceVersion,
      [SEMRESATTRS_DEPLOYMENT_ENVIRONMENT]: environment,
    });

    // Create OTLP exporter
    const traceExporter = new OTLPTraceExporter({
      url: `grpc://${otlpEndpoint}`,
    });

    // Initialize Node SDK with auto-instrumentations
    sdk = new NodeSDK({
      resource,
      traceExporter,
      instrumentations: [
        getNodeAutoInstrumentations({
          '@opentelemetry/instrumentation-http': { enabled: true },
          '@opentelemetry/instrumentation-pg': { enabled: true },
          '@opentelemetry/instrumentation-ioredis': { enabled: true },
        }),
      ],
    });

    // Start the SDK
    sdk.start();

    console.log('[Tracing] OpenTelemetry tracer initialized successfully');

    // Register shutdown handler
    process.on('SIGTERM', async () => {
      await shutdownTracing();
    });

    process.on('SIGINT', async () => {
      await shutdownTracing();
    });

  } catch (error) {
    console.error('[Tracing] Failed to initialize tracer (continuing without tracing):', error);
  }
}

/**
 * Shutdown OpenTelemetry SDK gracefully
 * Flushes remaining spans before shutdown
 */
export async function shutdownTracing(): Promise<void> {
  if (sdk) {
    console.log('[Tracing] Shutting down OpenTelemetry tracer');
    try {
      await sdk.shutdown();
      console.log('[Tracing] OpenTelemetry tracer shutdown complete');
    } catch (error) {
      console.error('[Tracing] Error shutting down tracer:', error);
    }
  }
}
