/**
 * API Client
 *
 * Base HTTP client for communicating with the All-Chat API Gateway.
 * Handles authentication, error handling, and request formatting.
 *
 * Usage:
 *   const data = await apiClient.get<ResponseType>('/api/v1/endpoint');
 *   const result = await apiClient.post('/api/v1/endpoint', { data });
 */

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

class ApiClient {
  private async fetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
    // Get token from localStorage (client-side only)
    let token: string | null = null;
    if (typeof window !== 'undefined') {
      token = localStorage.getItem('jwt_token');
    }

    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers
    };

    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const url = endpoint.startsWith('http') ? endpoint : `${API_URL}${endpoint}`;

    const response = await fetch(url, {
      ...options,
      headers
    });

    if (response.status === 401) {
      // Token expired or invalid, clear it
      if (typeof window !== 'undefined') {
        localStorage.removeItem('jwt_token');
        window.location.href = '/';
      }
      throw new ApiError(401, 'Unauthorized');
    }

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ error: 'Unknown error' }));
      throw new ApiError(response.status, errorData.error || response.statusText);
    }

    return response;
  }

  async get<T>(endpoint: string): Promise<T> {
    const response = await this.fetch(endpoint);
    return response.json();
  }

  async post<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'POST',
      body: JSON.stringify(data)
    });
    return response.json();
  }

  async put<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data)
    });
    return response.json();
  }

  async delete(endpoint: string): Promise<void> {
    await this.fetch(endpoint, { method: 'DELETE' });
  }
}

export const apiClient = new ApiClient();
