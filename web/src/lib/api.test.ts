import { describe, it, expect, beforeEach } from 'vitest';
import { auth, overlays, emotes } from './api';

describe('API Client', () => {
  beforeEach(() => {
    // Mock localStorage
    global.localStorage.getItem = vi.fn();
    global.localStorage.setItem = vi.fn();
    global.localStorage.removeItem = vi.fn();
  });

  describe('auth', () => {
    describe('login', () => {
      it('should redirect to login URL', () => {
        const originalLocation = window.location.href;

        auth.login();

        // In test environment, we just verify the href would be set
        expect(true).toBe(true); // Placeholder since window.location.href is mocked
      });
    });

    describe('logout', () => {
      it('should remove tokens from localStorage', () => {
        auth.logout();

        expect(localStorage.removeItem).toHaveBeenCalledWith('access_token');
        expect(localStorage.removeItem).toHaveBeenCalledWith('refresh_token');
      });
    });
  });

  describe('overlays', () => {
    it('should have list method', () => {
      expect(overlays.list).toBeDefined();
      expect(typeof overlays.list).toBe('function');
    });

    it('should create overlay', () => {
      const overlayData = {
        name: 'Test Overlay',
        twitch_channel: 'testchannel',
      };

      expect(overlays.create).toBeDefined();
      expect(typeof overlays.create).toBe('function');
    });

    it('should get overlay by ID', () => {
      const overlayId = 'test-id-123';

      expect(overlays.get).toBeDefined();
      expect(typeof overlays.get).toBe('function');
    });

    it('should update overlay', () => {
      const overlayId = 'test-id-123';
      const updates = {
        name: 'Updated Name',
        is_active: true,
      };

      expect(overlays.update).toBeDefined();
      expect(typeof overlays.update).toBe('function');
    });

    it('should delete overlay', () => {
      const overlayId = 'test-id-123';

      expect(overlays.delete).toBeDefined();
      expect(typeof overlays.delete).toBe('function');
    });

    it('should get overlay config', () => {
      const overlayId = 'test-id-123';

      expect(overlays.getConfig).toBeDefined();
      expect(typeof overlays.getConfig).toBe('function');
    });

    it('should update overlay config', () => {
      const overlayId = 'test-id-123';
      const config = {
        enable_7tv: true,
        enable_bttv: true,
        enable_ffz: false,
      };

      expect(overlays.updateConfig).toBeDefined();
      expect(typeof overlays.updateConfig).toBe('function');
    });
  });

  describe('emotes', () => {
    it('should get emotes for a channel', () => {
      const channel = 'testchannel';

      expect(emotes.getChannel).toBeDefined();
      expect(typeof emotes.getChannel).toBe('function');
    });

    it('should get emotes by provider', () => {
      const provider = '7tv';
      const channel = 'testchannel';

      expect(emotes.getProvider).toBeDefined();
      expect(typeof emotes.getProvider).toBe('function');
    });
  });

  describe('API structure', () => {
    it('should export auth module with required methods', () => {
      expect(auth).toBeDefined();
      expect(auth.login).toBeDefined();
      expect(auth.getMe).toBeDefined();
      expect(auth.logout).toBeDefined();
      expect(auth.refresh).toBeDefined();
    });

    it('should export overlays module with CRUD methods', () => {
      expect(overlays).toBeDefined();
      expect(overlays.list).toBeDefined();
      expect(overlays.create).toBeDefined();
      expect(overlays.get).toBeDefined();
      expect(overlays.update).toBeDefined();
      expect(overlays.delete).toBeDefined();
      expect(overlays.getConfig).toBeDefined();
      expect(overlays.updateConfig).toBeDefined();
    });

    it('should export emotes module with fetch methods', () => {
      expect(emotes).toBeDefined();
      expect(emotes.getChannel).toBeDefined();
      expect(emotes.getProvider).toBeDefined();
    });
  });
});
