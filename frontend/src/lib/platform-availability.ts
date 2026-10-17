import { useSyncExternalStore } from 'react'
import type { OverlayType } from '@/lib/types/overlay'

/**
 * Platforms and overlay kinds that are only offered on the beta frontend
 * (beta.allch.at). Everywhere else the UI hides every place a streamer would
 * pick or configure one: add-source buttons, the dashboard showcase and
 * per-platform customizer rows for a platform; the create-overlay kind picker,
 * the onboarding extras tour and the /upgrade feature list for an overlay kind.
 *
 * Rendering is untouched: a message carrying a beta-only platform still
 * renders, and an existing overlay of a beta-only kind still renders in OBS
 * (/overlay/[id]) and opens in its editor on any host. The ADR-0008 gates
 * (`platform_*`, `alerts_overlay`, `goal_overlay`, `list_overlay`) still decide
 * server-side who may add a source or create an overlay.
 *
 * Release a platform or an overlay kind to production by removing it here.
 */
const BETA_ONLY_PLATFORMS: Partial<Record<string, true>> = {
  owncast: true,
  goodgame: true,
  picarto: true,
  facebook: true,
  instagram: true,
}

const BETA_ONLY_OVERLAY_KINDS: Partial<Record<OverlayType, true>> = {
  alerts: true,
  goal: true,
  list: true,
}

const BETA_HOST = 'beta.allch.at'

const noopSubscribe = () => () => {}

/**
 * Whether this page is served from the beta host. The server snapshot is "not
 * beta", so prerendered HTML matches production and the beta host reveals the
 * extra platforms and kinds after hydration.
 */
function useOnBetaHost(): boolean {
  return useSyncExternalStore(
    noopSubscribe,
    () => window.location.hostname === BETA_HOST,
    () => false
  )
}

/** Returns a predicate telling whether a platform may be offered on this host. */
export function usePlatformAvailable(): (platform: string) => boolean {
  const onBeta = useOnBetaHost()
  return (platform) => onBeta || BETA_ONLY_PLATFORMS[platform] !== true
}

/** Returns a predicate telling whether an overlay kind may be offered on this host. */
export function useOverlayKindAvailable(): (kind: OverlayType) => boolean {
  const onBeta = useOnBetaHost()
  return (kind) => onBeta || BETA_ONLY_OVERLAY_KINDS[kind] !== true
}
