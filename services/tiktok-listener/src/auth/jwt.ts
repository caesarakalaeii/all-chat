/**
 * JWT Utilities
 *
 * Service JWT generation matching Go shared/auth/jwt.go implementation.
 */

import jwt from 'jsonwebtoken';

/**
 * ServiceClaims represents the payload of a service JWT.
 * Matches Go shared/auth/jwt.go ServiceClaims structure.
 */
interface ServiceClaims {
  service_name: string;
  sub: string;
  iss: string;
  aud: string[];
  iat: number;
  exp: number;
}

/**
 * Generate a service JWT token for service-to-service authentication.
 *
 * Matches Go shared/auth/jwt.go GenerateServiceJWT implementation:
 * - Uses HS256 signing method
 * - Includes service_name claim
 * - Sets standard JWT claims (sub, iss, aud, iat, exp)
 *
 * @param serviceName - Name of the service (e.g., "tiktok-listener")
 * @param secret - Shared secret for signing the JWT
 * @param expiryMs - Token expiry time in milliseconds (default: 24 hours)
 * @returns Signed JWT token string
 */
export function generateServiceJWT(
  serviceName: string,
  secret: string,
  expiryMs: number = 24 * 60 * 60 * 1000, // 24 hours default
): string {
  const now = Math.floor(Date.now() / 1000); // Current time in seconds

  const claims: ServiceClaims = {
    service_name: serviceName,
    sub: serviceName,
    iss: 'all-chat-services',
    aud: ['internal'],
    iat: now,
    exp: now + Math.floor(expiryMs / 1000),
  };

  return jwt.sign(claims, secret, {
    algorithm: 'HS256',
  });
}
