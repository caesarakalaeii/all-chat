/**
 * ErrorDisplay Component
 *
 * Displays chat errors with type-specific styling, icons, and actionable guidance.
 * Includes special handling for rate limits (countdown timer), bans (reason display),
 * and collapsible technical details.
 */

'use client'

import { useState, useEffect } from 'react'
import clsx from 'clsx'
import {
  ChatError,
  ChatErrorType,
  isRateLimitedError,
  isBannedError,
  isAuthError,
  isPlatformApiError,
} from '@/lib/types/errors'

interface ErrorDisplayProps {
  error: ChatError
  onRetry?: () => void
  onDismiss?: () => void
  className?: string
}

export default function ErrorDisplay({
  error,
  onRetry,
  onDismiss,
  className = '',
}: ErrorDisplayProps) {
  const [showDetails, setShowDetails] = useState(false)
  const [countdown, setCountdown] = useState<number | null>(null)

  // Handle rate limit countdown
  useEffect(() => {
    if (!isRateLimitedError(error)) return

    if (error.retryAfter) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setCountdown(error.retryAfter)
      const interval = setInterval(() => {
        setCountdown((prev) => {
          if (prev === null || prev <= 1) {
            clearInterval(interval)
            return null
          }
          return prev - 1
        })
      }, 1000)

      return () => clearInterval(interval)
    } else if (error.resetTime) {
      const resetDate = new Date(error.resetTime)
      const updateCountdown = () => {
        const now = new Date()
        const diff = Math.floor((resetDate.getTime() - now.getTime()) / 1000)
        if (diff <= 0) {
          setCountdown(null)
        } else {
          setCountdown(diff)
        }
      }

      updateCountdown()
      const interval = setInterval(updateCountdown, 1000)
      return () => clearInterval(interval)
    }
  }, [error])

  // Get styling based on error type
  const getErrorStyle = () => {
    switch (error.type) {
      case ChatErrorType.BANNED:
      case ChatErrorType.PLATFORM_API_ERROR:
      case ChatErrorType.UNKNOWN_ERROR:
        return {
          bg: 'bg-red-50 dark:bg-red-900/20',
          border: 'border-red-200 dark:border-red-800',
          text: 'text-red-800 dark:text-red-200',
          icon: '🚫',
        }

      case ChatErrorType.TOKEN_EXPIRED:
      case ChatErrorType.UNAUTHORIZED:
      case ChatErrorType.STREAMER_OFFLINE:
        return {
          bg: 'bg-orange-50 dark:bg-orange-900/20',
          border: 'border-orange-200 dark:border-orange-800',
          text: 'text-orange-800 dark:text-orange-200',
          icon: '⚠️',
        }

      case ChatErrorType.RATE_LIMITED:
        return {
          bg: 'bg-yellow-50 dark:bg-yellow-900/20',
          border: 'border-yellow-200 dark:border-yellow-800',
          text: 'text-yellow-800 dark:text-yellow-200',
          icon: '⏱️',
        }

      case ChatErrorType.NETWORK_ERROR:
      case ChatErrorType.VALIDATION_ERROR:
        return {
          bg: 'bg-slate-50 dark:bg-slate-800',
          border: 'border-slate-200 dark:border-slate-700',
          text: 'text-slate-800 dark:text-slate-200',
          icon: '⚠️',
        }

      default:
        return {
          bg: 'bg-slate-50 dark:bg-slate-800',
          border: 'border-slate-200 dark:border-slate-700',
          text: 'text-slate-800 dark:text-slate-200',
          icon: '❌',
        }
    }
  }

  const style = getErrorStyle()

  // Determine if retry button should be shown
  const canRetry =
    error.type === ChatErrorType.NETWORK_ERROR ||
    error.type === ChatErrorType.PLATFORM_API_ERROR ||
    error.type === ChatErrorType.UNKNOWN_ERROR ||
    (isRateLimitedError(error) && countdown === null)

  return (
    <div className={clsx('rounded-lg border p-4', style.bg, style.border, className)}>
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-1 items-start gap-3">
          <span className="text-2xl" role="img" aria-label="Error icon">
            {style.icon}
          </span>
          <div className="min-w-0 flex-1">
            <h3 className={clsx('mb-1 font-semibold', style.text)}>{error.userMessage}</h3>

            {/* Rate limit countdown */}
            {isRateLimitedError(error) && countdown !== null && (
              <p className={clsx('mb-2 text-sm', style.text)}>
                You can send another message in <strong>{formatCountdown(countdown)}</strong>
              </p>
            )}

            {/* Ban reason */}
            {isBannedError(error) && error.reason && (
              <p className={clsx('mb-2 text-sm', style.text)}>
                <strong>Reason:</strong> {error.reason}
              </p>
            )}

            {/* Ban expiration */}
            {isBannedError(error) && error.expiresAt && (
              <p className={clsx('mb-2 text-sm', style.text)}>
                <strong>Expires:</strong> {new Date(error.expiresAt).toLocaleString()}
              </p>
            )}

            {/* Platform message */}
            {isPlatformApiError(error) && error.platformMessage && (
              <p className={clsx('mb-2 text-sm italic', style.text)}>
                Platform message: {error.platformMessage}
              </p>
            )}

            {/* Actionable steps */}
            {error.actionableSteps.length > 0 && (
              <div className="mt-3">
                <p className={clsx('mb-1 text-sm font-medium', style.text)}>What you can do:</p>
                <ul className={clsx('list-inside list-disc space-y-1 text-sm', style.text)}>
                  {error.actionableSteps.map((step, index) => (
                    <li key={index}>{step}</li>
                  ))}
                </ul>
              </div>
            )}

            {/* Action buttons */}
            <div className="mt-3 flex gap-2">
              {canRetry && onRetry && (
                <button
                  onClick={onRetry}
                  className={clsx(
                    'rounded border px-3 py-1.5 text-sm font-medium hover:opacity-80',
                    style.text,
                    style.border
                  )}
                  disabled={countdown !== null}
                >
                  Try Again
                </button>
              )}

              {/* Re-auth button for auth errors */}
              {isAuthError(error) && error.platform && (
                <a
                  href={`/api/v1/auth/viewer/${error.platform}/login`}
                  className={clsx(
                    'rounded border px-3 py-1.5 text-sm font-medium hover:opacity-80',
                    style.text,
                    style.border
                  )}
                >
                  Sign in with {capitalizeFirst(error.platform)}
                </a>
              )}

              {/* Technical details toggle */}
              {error.technicalDetails && (
                <button
                  onClick={() => setShowDetails(!showDetails)}
                  className={clsx(
                    'rounded border px-3 py-1.5 text-sm font-medium hover:opacity-80',
                    style.text,
                    style.border
                  )}
                >
                  {showDetails ? 'Hide' : 'Show'} Details
                </button>
              )}
            </div>

            {/* Technical details (collapsible) */}
            {showDetails && error.technicalDetails && (
              <div className="mt-3 overflow-x-auto rounded bg-black/10 p-3 font-mono text-xs dark:bg-black/30">
                <pre className={style.text}>{error.technicalDetails}</pre>
              </div>
            )}
          </div>
        </div>

        {/* Dismiss button */}
        {onDismiss && (
          <button
            onClick={onDismiss}
            className={clsx('flex-shrink-0 text-xl hover:opacity-70', style.text)}
            aria-label="Dismiss error"
          >
            ×
          </button>
        )}
      </div>
    </div>
  )
}

/**
 * Format countdown in MM:SS format
 */
function formatCountdown(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

/**
 * Capitalize first letter
 */
function capitalizeFirst(str: string): string {
  if (!str) return str
  return str.charAt(0).toUpperCase() + str.slice(1)
}
