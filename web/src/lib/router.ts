// Simple client-side router for Svelte 5
export type Route = {
  path: string;
  params?: Record<string, string>;
};

export function parseRoute(): Route {
  const path = window.location.pathname;
  const params: Record<string, string> = {};

  // Match /overlay/:id pattern
  const overlayMatch = path.match(/^\/overlay\/([^/]+)$/);
  if (overlayMatch) {
    return {
      path: '/overlay/:id',
      params: { id: overlayMatch[1] },
    };
  }

  // Default route
  return { path };
}

export function navigateTo(path: string) {
  window.history.pushState({}, '', path);
  window.dispatchEvent(new PopStateEvent('popstate'));
}

export function onRouteChange(callback: () => void) {
  window.addEventListener('popstate', callback);
  return () => window.removeEventListener('popstate', callback);
}
