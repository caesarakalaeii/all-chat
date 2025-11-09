import axios, { type AxiosInstance } from 'axios';

const api: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080',
});

// Add JWT token to requests
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Auth API
export const auth = {
  login: () => {
    window.location.href = `${api.defaults.baseURL}/api/v1/auth/login`;
  },

  getMe: () => api.get('/api/v1/auth/me'),

  logout: () => {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    return api.post('/api/v1/auth/logout');
  },

  refresh: (refreshToken: string) =>
    api.post('/api/v1/auth/refresh', { refresh_token: refreshToken }),
};

// Overlay API
export const overlays = {
  list: () => api.get('/api/v1/overlays'),

  create: (data: { name: string; twitch_channel: string }) =>
    api.post('/api/v1/overlays', data),

  get: (id: string) => api.get(`/api/v1/overlays/${id}`),

  update: (id: string, data: { name?: string; twitch_channel?: string; is_active?: boolean }) =>
    api.put(`/api/v1/overlays/${id}`, data),

  delete: (id: string) => api.delete(`/api/v1/overlays/${id}`),

  getConfig: (id: string) => api.get(`/api/v1/overlays/${id}/config`),

  updateConfig: (id: string, data: any) =>
    api.put(`/api/v1/overlays/${id}/config`, data),
};

// Emote API
export const emotes = {
  getChannel: (channel: string) =>
    api.get(`/api/v1/emotes/channel/${channel}`),

  getProvider: (provider: string, channel: string) =>
    api.get(`/api/v1/emotes/${provider}/${channel}`),
};
