import { useState, useEffect } from 'react'

/**
 * Hook to detect when React hydration is complete.
 *
 * Returns `false` during SSR and initial hydration, then `true` after hydration.
 * Use this to safely access browser APIs (localStorage, window, etc.) without
 * causing hydration mismatches.
 */
export function useHydrated(): boolean {
  const [isHydrated, setIsHydrated] = useState(false)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsHydrated(true)
  }, [])

  return isHydrated
}
