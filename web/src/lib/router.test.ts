import { describe, it, expect, beforeEach, vi } from 'vitest';
import { parseRoute, navigateTo, onRouteChange } from './router';

describe('Router', () => {
  beforeEach(() => {
    // Reset window location before each test
    window.history.pushState({}, '', '/');
  });

  describe('parseRoute', () => {
    it('should parse root route', () => {
      Object.defineProperty(window, 'location', {
        writable: true,
        value: { pathname: '/' }
      });
      const route = parseRoute();
      expect(route.path).toBe('/');
      expect(route.params).toBeUndefined();
    });

    it('should parse overlay route with ID', () => {
      Object.defineProperty(window, 'location', {
        writable: true,
        value: { pathname: '/overlay/123abc' }
      });
      const route = parseRoute();
      expect(route.path).toBe('/overlay/:id');
      expect(route.params).toEqual({ id: '123abc' });
    });

    it('should parse overlay route with UUID', () => {
      const uuid = '550e8400-e29b-41d4-a716-446655440000';
      Object.defineProperty(window, 'location', {
        writable: true,
        value: { pathname: `/overlay/${uuid}` }
      });
      const route = parseRoute();
      expect(route.path).toBe('/overlay/:id');
      expect(route.params).toEqual({ id: uuid });
    });

    it('should return path as-is for unknown routes', () => {
      Object.defineProperty(window, 'location', {
        writable: true,
        value: { pathname: '/unknown/path' }
      });
      const route = parseRoute();
      expect(route.path).toBe('/unknown/path');
      expect(route.params).toBeUndefined();
    });
  });

  describe('navigateTo', () => {
    it('should update browser history', () => {
      // Mock pushState to actually update our location mock
      const originalPushState = window.history.pushState;
      window.history.pushState = vi.fn((data, title, url) => {
        Object.defineProperty(window, 'location', {
          writable: true,
          value: { ...window.location, pathname: url as string }
        });
      });

      navigateTo('/test-path');
      expect(window.location.pathname).toBe('/test-path');

      // Restore
      window.history.pushState = originalPushState;
    });

    it('should trigger popstate event', () => {
      const handler = vi.fn();
      window.addEventListener('popstate', handler);

      navigateTo('/test-path');

      expect(handler).toHaveBeenCalled();
      window.removeEventListener('popstate', handler);
    });
  });

  describe('onRouteChange', () => {
    it('should call callback on popstate event', () => {
      const callback = vi.fn();
      const cleanup = onRouteChange(callback);

      window.dispatchEvent(new PopStateEvent('popstate'));

      expect(callback).toHaveBeenCalled();
      cleanup();
    });

    it('should remove listener when cleanup is called', () => {
      const callback = vi.fn();
      const cleanup = onRouteChange(callback);

      cleanup();
      window.dispatchEvent(new PopStateEvent('popstate'));

      expect(callback).not.toHaveBeenCalled();
    });

    it('should handle multiple listeners', () => {
      const callback1 = vi.fn();
      const callback2 = vi.fn();
      const cleanup1 = onRouteChange(callback1);
      const cleanup2 = onRouteChange(callback2);

      window.dispatchEvent(new PopStateEvent('popstate'));

      expect(callback1).toHaveBeenCalled();
      expect(callback2).toHaveBeenCalled();

      cleanup1();
      cleanup2();
    });
  });
});
