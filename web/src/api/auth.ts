export interface User {
  id: string;
  handle: string;
  role: string;
}

export async function whoami(): Promise<User | null> {
  try {
    const res = await fetch('/api/v1/whoami', { credentials: 'same-origin' });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function login(handle: string, password: string): Promise<void> {
  const body = new URLSearchParams({ handle, password });
  const res = await fetch('/login', {
    method: 'POST',
    body,
    credentials: 'same-origin',
  });
  if (res.ok) return;
  const data = await res.json().catch(() => ({}));
  throw new Error(data.error || 'Invalid username or password');
}

export async function logout(): Promise<void> {
  await fetch('/logout', { method: 'POST', credentials: 'same-origin', redirect: 'manual' });
}
