import { NodeSDK } from '@opentelemetry/sdk-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { resourceFromAttributes } from '@opentelemetry/resources';

let sdk;

export function initTracing() {
  // Check if tracing is enabled
  const tracingEnabled = process.env.OTEL_ENABLED === 'true';

  if (!tracingEnabled) {
    console.log('OpenTelemetry tracing is disabled (OTEL_ENABLED != "true")');
    return;
  }

  try {
    // Configure the OTLP exporter
    const traceExporter = new OTLPTraceExporter({
      url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'http://localhost:4317',
    });

    // Create a resource with service information
    const resource = resourceFromAttributes({
      'service.name': 'discord-bot',
      'service.version': process.env.APP_VERSION || 'dev',
      'deployment.environment': process.env.ENVIRONMENT || 'development',
    });

    // Initialize the SDK
    sdk = new NodeSDK({
      resource,
      traceExporter,
      instrumentations: [
        getNodeAutoInstrumentations({
          // Disable specific instrumentations if needed
          '@opentelemetry/instrumentation-fs': {
            enabled: false,
          },
        }),
      ],
    });

    // Start the SDK
    sdk.start();
    console.log('OpenTelemetry tracing initialized successfully');

    // Handle graceful shutdown
    process.on('SIGTERM', () => {
      sdk
        .shutdown()
        .then(() => console.log('OpenTelemetry SDK shut down successfully'))
        .catch((error) => console.error('Error shutting down OpenTelemetry SDK', error))
        .finally(() => process.exit(0));
    });
  } catch (error) {
    console.error('Failed to initialize OpenTelemetry tracing:', error);
    // Don't exit the process if tracing fails - allow the bot to run without it
  }
}

export function shutdownTracing() {
  if (sdk) {
    return sdk.shutdown();
  }
  return Promise.resolve();
}
