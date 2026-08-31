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
 * Error Message Catalog
 *
 * Centralized user-friendly messages and actionable guidance for each error
 * type. The copy itself lives in the i18n catalog under `errors.*` (#799); this
 * module keeps its Record shape and its two accessors because errorParser.ts
 * calls them from ten places, and issue #799 requires those call sites not to
 * change. See src/lib/__tests__/errorMessages.test.ts, which is the guard on
 * that surface.
 */

import { getTranslations } from './i18n'
import { ChatErrorType } from './types/errors'

export interface ErrorMessageTemplate {
  title: string
  message: string
  actionableSteps: string[]
}

const t = getTranslations()

/**
 * Error type to its `errors.*` namespace, and how many numbered step keys that
 * namespace has. PLATFORM_API_ERROR is the only one with four.
 *
 * `as const satisfies` rather than a plain annotation: an annotation widens the
 * namespaces to string, and a typo would then resolve to a missing key at
 * runtime instead of failing tsc where the key is built.
 */
const ERROR_NAMESPACES = {
  [ChatErrorType.UNAUTHORIZED]: { namespace: 'unauthorized', steps: 3 },
  [ChatErrorType.TOKEN_EXPIRED]: { namespace: 'tokenExpired', steps: 3 },
  [ChatErrorType.RATE_LIMITED]: { namespace: 'rateLimited', steps: 3 },
  [ChatErrorType.BANNED]: { namespace: 'banned', steps: 3 },
  [ChatErrorType.STREAMER_OFFLINE]: { namespace: 'streamerOffline', steps: 3 },
  [ChatErrorType.PLATFORM_API_ERROR]: { namespace: 'platformApiError', steps: 4 },
  [ChatErrorType.NETWORK_ERROR]: { namespace: 'networkError', steps: 3 },
  [ChatErrorType.VALIDATION_ERROR]: { namespace: 'validationError', steps: 3 },
  [ChatErrorType.UNKNOWN_ERROR]: { namespace: 'unknownError', steps: 3 },
} as const satisfies Record<ChatErrorType, { namespace: string; steps: 3 | 4 }>

function resolveTemplate(errorType: ChatErrorType): ErrorMessageTemplate {
  const { namespace, steps } = ERROR_NAMESPACES[errorType]
  // Each step is resolved on its own so word order stays the catalog's
  // business; joining them here would be the fragment concatenation
  // docs/frontend/I18N.md forbids.
  const actionableSteps = [
    t(`errors.${namespace}.step1`),
    t(`errors.${namespace}.step2`),
    t(`errors.${namespace}.step3`),
  ]
  if (steps === 4) {
    actionableSteps.push(t(`errors.${namespace}.step4`))
  }
  return {
    title: t(`errors.${namespace}.title`),
    message: t(`errors.${namespace}.message`),
    actionableSteps,
  }
}

/**
 * Error message templates for each error type
 */
export const ERROR_MESSAGES: Record<ChatErrorType, ErrorMessageTemplate> = Object.fromEntries(
  Object.values(ChatErrorType).map((errorType) => [errorType, resolveTemplate(errorType)])
) as Record<ChatErrorType, ErrorMessageTemplate>

/**
 * Get error message template for a specific error type
 */
export function getErrorMessage(errorType: ChatErrorType): ErrorMessageTemplate {
  return ERROR_MESSAGES[errorType]
}

/**
 * Get error message with platform placeholder replaced
 */
export function formatErrorMessage(
  errorType: ChatErrorType,
  platform?: string
): ErrorMessageTemplate {
  const template = ERROR_MESSAGES[errorType]

  if (!platform) {
    return template
  }

  // Replace {platform} placeholder in actionable steps
  const formattedSteps = template.actionableSteps.map((step) =>
    step.replace('{platform}', capitalizeFirst(platform))
  )

  return {
    ...template,
    actionableSteps: formattedSteps,
  }
}

/**
 * Capitalize first letter of a string
 */
function capitalizeFirst(str: string): string {
  if (!str) return str
  return str.charAt(0).toUpperCase() + str.slice(1)
}
