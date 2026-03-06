/**
 * Coordinator Client
 *
 * HTTP client for coordinator integration (Phase 6).
 * Mirrors Go shared/coordination/client.go patterns in TypeScript.
 */

import axios, { AxiosInstance, AxiosError } from 'axios';
import { Logger } from '../types/logger.js';
import { Assignment, AssignmentResponse, HeartbeatRequest } from './models.js';
import { generateServiceJWT } from '../auth/jwt.js';

/**
 * CoordinatorClient is an HTTP client for coordinator integration.
 * Matches Go shared/coordination/client.go CoordinatorClient behavior.
 */
export class CoordinatorClient {
  private baseURL: string;
  private serviceSecret: string;
  private serviceJWT: string;
  private serviceName: string;
  private httpClient: AxiosInstance;
  private logger: Logger;

  /**
   * Creates a new coordinator client.
   *
   * @param baseURL - Coordinator base URL (e.g., "http://source-manager:8088")
   * @param serviceSecret - Shared secret for generating service JWT tokens
   * @param logger - Logger instance
   */
  constructor(baseURL: string, serviceSecret: string, logger: Logger) {
    this.baseURL = baseURL;
    this.serviceSecret = serviceSecret;
    this.logger = logger;

    // Determine service name from hostname (pod name)
    const hostname = process.env.HOSTNAME || 'tiktok-listener';
    this.serviceName = 'tiktok-listener'; // Default
    if (hostname.startsWith('twitch-listener')) {
      this.serviceName = 'twitch-listener';
    } else if (hostname.startsWith('twitch-eventsub-listener')) {
      this.serviceName = 'twitch-eventsub-listener';
    } else if (hostname.startsWith('kick-listener')) {
      this.serviceName = 'kick-listener';
    } else if (hostname.startsWith('tiktok-listener')) {
      this.serviceName = 'tiktok-listener';
    }

    // Generate service JWT (24 hour expiry, matching Go implementation)
    this.serviceJWT = generateServiceJWT(this.serviceName, this.serviceSecret, 24 * 60 * 60 * 1000);

    this.logger.info('Generated service JWT for coordinator authentication', {
      service_name: this.serviceName,
      hostname: hostname,
    });

    this.httpClient = axios.create({
      baseURL: this.baseURL,
      timeout: 10000, // 10 seconds
      headers: {
        Authorization: `Bearer ${this.serviceJWT}`,
      },
    });
  }

  /**
   * QueryAssignments queries the coordinator for channel assignments for a specific pod.
   *
   * Implements TIKTOK-01: "TikTok listener queries coordinator on startup and connects ONLY to assigned channels"
   *
   * Blocks indefinitely with exponential backoff until coordinator responds
   * (per CONTEXT.md user decision).
   *
   * Exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (max)
   *
   * @param podID - Kubernetes pod name (from HOSTNAME environment variable)
   * @returns Promise resolving to array of assignments
   * @throws Error only if context is canceled (should never happen in normal startup)
   */
  async queryAssignments(podID: string): Promise<Assignment[]> {
    const url = `/assignments?pod_id=${encodeURIComponent(podID)}`;

    // Exponential backoff configuration: 1s, 2s, 4s, 8s, 16s, 30s (max)
    let backoff = 1000; // Start at 1 second
    const maxBackoff = 30000; // Max 30 seconds

    this.logger.info('Querying coordinator for assignments', {
      pod_id: podID,
      url: this.baseURL + url,
    });

    // Infinite retry loop (per CONTEXT.md user decision: blocks indefinitely)
    while (true) {
      try {
        const response = await this.httpClient.get<AssignmentResponse>(url);

        if (response.status === 200) {
          this.logger.info('Successfully retrieved assignments from coordinator', {
            pod_id: podID,
            assignment_count: response.data.count,
          });

          return response.data.assignments;
        }

        // Unexpected 2xx response
        this.logger.error('Coordinator returned unexpected success status code', {
          pod_id: podID,
          status_code: response.status,
        });

        throw new Error(`Unexpected status code ${response.status}`);
      } catch (error) {
        if (axios.isAxiosError(error)) {
          const axiosError = error as AxiosError;

          // Client error (4xx) - configuration issue, don't retry
          if (axiosError.response && axiosError.response.status >= 400 && axiosError.response.status < 500) {
            this.logger.error('Coordinator returned client error', {
              pod_id: podID,
              status_code: axiosError.response.status,
              body: axiosError.response.data,
            });

            throw new Error(
              `Coordinator returned ${axiosError.response.status}: ${JSON.stringify(axiosError.response.data)}`
            );
          }

          // Server error (5xx) or network error - coordinator might be starting, retry with backoff
          if (!axiosError.response || (axiosError.response && axiosError.response.status >= 500)) {
            this.logger.warn('Failed to connect to coordinator, retrying with backoff', {
              pod_id: podID,
              backoff_ms: backoff,
              error: axiosError.message,
              status_code: axiosError.response?.status,
            });

            // Sleep with backoff
            await this.sleep(backoff);

            // Increase backoff exponentially
            backoff *= 2;
            if (backoff > maxBackoff) {
              backoff = maxBackoff;
            }

            continue;
          }
        }

        // Unknown error - log and retry
        this.logger.warn('Unknown error querying coordinator, retrying with backoff', {
          pod_id: podID,
          backoff_ms: backoff,
          error: String(error),
        });

        await this.sleep(backoff);

        // Increase backoff exponentially
        backoff *= 2;
        if (backoff > maxBackoff) {
          backoff = maxBackoff;
        }
      }
    }
  }

  /**
   * PublishHeartbeat publishes a heartbeat to the coordinator.
   *
   * Implements TIKTOK-01: "TikTok listener publishes heartbeat every 10 seconds to coordinator"
   *
   * @param podID - Kubernetes pod name
   * @returns Promise resolving to void on success (200 status)
   * @throws Error if heartbeat fails
   */
  async publishHeartbeat(podID: string): Promise<void> {
    const url = '/heartbeat';

    const reqBody: HeartbeatRequest = {
      pod_id: podID,
    };

    try {
      const response = await this.httpClient.post(url, reqBody, {
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (response.status !== 200) {
        this.logger.error('Heartbeat request failed', {
          pod_id: podID,
          status_code: response.status,
          body: response.data,
        });

        throw new Error(`Heartbeat failed with status ${response.status}: ${JSON.stringify(response.data)}`);
      }

      this.logger.debug('Successfully published heartbeat', {
        pod_id: podID,
      });
    } catch (error) {
      if (axios.isAxiosError(error)) {
        const axiosError = error as AxiosError;

        this.logger.error('Failed to publish heartbeat', {
          pod_id: podID,
          error: axiosError.message,
          status_code: axiosError.response?.status,
          body: axiosError.response?.data,
        });

        throw new Error(`Failed to publish heartbeat: ${axiosError.message}`);
      }

      // Re-throw unknown errors
      throw error;
    }
  }

  /**
   * Sleep helper for backoff logic.
   *
   * @param ms - Milliseconds to sleep
   */
  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}
