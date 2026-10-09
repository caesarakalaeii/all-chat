import { useSyncExternalStore } from 'react'

/**
 * Platforms whose listener runs but is only offered on the beta frontend
 * (beta.allch.at). Everywhere else the UI hides every place a streamer would
 * pick or configure one (add-source buttons, the dashboard showcase,
 * per-platform customizer rows). Rendering a message that does carry one of
 * these platforms is untouched, and overlay-manager's ADR-0008 `platform_*`
 * gate still decides who may add a source.
 *
 * Release a platform to production by removing it here.
 */
const BETA_ONLY_PLATFORMS: Partial<Record<string, true>> = {
  owncast: true,
  goodgame: true,
  picarto: true,
  facebook: true,
  instagram: true,
}

const BETA_HOST = 'beta.allch.at'

const noopSubscribe = () => () => {}

/**
 * Returns a predicate telling whether a platform may be offered on this host.
 * The server snapshot is "not beta", so prerendered HTML matches production and
 * the beta host reveals the extra platforms after hydration.
 */
export function usePlatformAvailable(): (platform: string) => boolean {
  const onBeta = useSyncExternalStore(
    noopSubscribe,
    () => window.location.hostname === BETA_HOST,
    () => false
  )
  return (platform) => onBeta || BETA_ONLY_PLATFORMS[platform] !== true
}
