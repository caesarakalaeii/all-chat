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
