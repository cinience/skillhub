const BASE = '/api/v1';

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {};
  // Don't set Content-Type for FormData — browser sets it with boundary
  if (!(init?.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }
  const res = await fetch(BASE + path, {
    credentials: 'same-origin',
    ...init,
    headers: { ...headers, ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function apiFetchRaw(path: string, init?: RequestInit): Promise<Response> {
  return fetch(BASE + path, { credentials: 'same-origin', ...init });
}
