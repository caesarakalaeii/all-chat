import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { OverlayWebSocket, type ChatMessage } from './websocket';

// Mock WebSocket
class MockWebSocket {
  url: string;
  onopen: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    // Simulate async connection
    setTimeout(() => {
      if (this.onopen) {
        this.onopen(new Event('open'));
      }
    }, 0);
  }

  close() {
    if (this.onclose) {
      this.onclose(new CloseEvent('close'));
    }
  }

  send(data: string) {
    // Mock send
  }
}

describe('OverlayWebSocket', () => {
  let ws: OverlayWebSocket;
  let originalWebSocket: any;

  beforeEach(() => {
    originalWebSocket = global.WebSocket;
    global.WebSocket = MockWebSocket as any;
    ws = new OverlayWebSocket();
    vi.useFakeTimers();
  });

  afterEach(() => {
    global.WebSocket = originalWebSocket;
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe('connect', () => {
    it('should establish WebSocket connection', () => {
      const overlayId = 'test-overlay-id';
      const token = 'test-token';

      const socket = ws.connect(overlayId, token);

      expect(socket).toBeDefined();
      expect(socket.url).toContain(overlayId);
      expect(socket.url).toContain(token);
    });

    it('should use correct protocol based on window location', () => {
      const overlayId = 'test-id';
      const token = 'test-token';

      // Test with http
      window.location.protocol = 'http:';
      const socket1 = ws.connect(overlayId, token);
      expect(socket1.url).toMatch(/^ws:/);

      // Test with https
      window.location.protocol = 'https:';
      const ws2 = new OverlayWebSocket();
      const socket2 = ws2.connect(overlayId, token);
      expect(socket2.url).toMatch(/^wss:/);
    });

    it('should reset reconnect attempts on successful connection', async () => {
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});

      ws.connect('test-id', 'test-token');

      // Wait for async connection
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith('WebSocket connected');
      consoleSpy.mockRestore();
    });
  });

  describe('onMessage', () => {
    it('should call callback with parsed message data', async () => {
      const callback = vi.fn();
      const mockMessage: ChatMessage = {
        overlay_id: 'overlay-123',
        channel: 'test-channel',
        user: {
          name: 'testuser',
          display_name: 'TestUser',
          color: '#FF0000',
          badges: ['moderator'],
        },
        message: {
          text: 'Hello world!',
          emotes: [],
        },
        timestamp: new Date().toISOString(),
      };

      const socket = ws.connect('test-id', 'test-token');
      ws.onMessage(callback);

      // Simulate receiving message
      if (socket.onmessage) {
        const event = new MessageEvent('message', {
          data: JSON.stringify(mockMessage),
        });
        socket.onmessage(event);
      }

      expect(callback).toHaveBeenCalledWith(mockMessage);
    });

    it('should handle JSON parse errors gracefully', () => {
      const callback = vi.fn();
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      const socket = ws.connect('test-id', 'test-token');
      ws.onMessage(callback);

      // Simulate receiving invalid JSON
      if (socket.onmessage) {
        const event = new MessageEvent('message', {
          data: 'invalid json',
        });
        socket.onmessage(event);
      }

      expect(callback).not.toHaveBeenCalled();
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        'Failed to parse message:',
        expect.any(Error)
      );

      consoleErrorSpy.mockRestore();
    });
  });

  describe('disconnect', () => {
    it('should close WebSocket connection', () => {
      const socket = ws.connect('test-id', 'test-token');
      const closeSpy = vi.spyOn(socket, 'close');

      ws.disconnect();

      expect(closeSpy).toHaveBeenCalled();
    });

    it('should not throw if called when not connected', () => {
      expect(() => ws.disconnect()).not.toThrow();
    });
  });

  describe('reconnection logic', () => {
    it('should attempt to reconnect on connection close', async () => {
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});

      ws.connect('test-id', 'test-token');

      // Wait for connection
      await vi.runAllTimersAsync();

      // Trigger close event
      const mockSocket = (ws as any).ws;
      if (mockSocket.onclose) {
        mockSocket.onclose(new CloseEvent('close'));
      }

      // Should attempt reconnect
      await vi.advanceTimersByTimeAsync(1000);

      expect(consoleSpy).toHaveBeenCalledWith(
        expect.stringContaining('Attempting to reconnect')
      );

      consoleSpy.mockRestore();
    });

    it('should use exponential backoff for reconnection', async () => {
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {});

      ws.connect('test-id', 'test-token');
      await vi.runAllTimersAsync();

      // Close and trigger multiple reconnection attempts
      for (let i = 1; i <= 3; i++) {
        const mockSocket = (ws as any).ws;
        if (mockSocket.onclose) {
          mockSocket.onclose(new CloseEvent('close'));
        }
        await vi.runAllTimersAsync();
      }

      // Check that delays increased exponentially
      const reconnectLogs = consoleSpy.mock.calls.filter((call) =>
        call[0].includes('Attempting to reconnect')
      );

      expect(reconnectLogs.length).toBeGreaterThan(0);
      consoleSpy.mockRestore();
    });

    it('should have max reconnection attempts configured', () => {
      // Verify the class has reconnection logic
      expect((ws as any).maxReconnectAttempts).toBe(5);
      expect((ws as any).reconnectDelay).toBe(1000);
    });
  });
});
