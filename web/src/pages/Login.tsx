import { useState, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { login } from '../api/auth';
import { useAuth } from '../hooks/useAuth';

function safeRedirect(url: string | null): string {
  if (!url) return '/';
  try {
    const parsed = new URL(url, window.location.origin);
    if (parsed.origin !== window.location.origin) return '/';
    return parsed.pathname + parsed.search;
  } catch {
    return '/';
  }
}

export default function Login() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const { user, refresh } = useAuth();
  const [handle, setHandle] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (user) {
      navigate(safeRedirect(params.get('redirect')), { replace: true });
    }
  }, [user, navigate, params]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!handle.trim() || !password) {
      setError(t('login.required'));
      return;
    }
    setLoading(true);
    setError('');
    try {
      await login(handle.trim(), password);
      await refresh();
      navigate(safeRedirect(params.get('redirect')), { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container" style={{ padding: '48px 24px' }}>
      <div style={{ maxWidth: 420, margin: '0 auto' }}>
        <h1 style={{ fontSize: '2rem', fontWeight: 700, marginBottom: 8, textAlign: 'center' }}>{t('login.title')}</h1>
        <p style={{ color: 'var(--text-secondary)', marginBottom: 32, textAlign: 'center' }}>{t('login.subtitle')}</p>

        {error && <div className="error-box">{error}</div>}

        <form onSubmit={handleSubmit} className="card" style={{ padding: 24 }}>
          <div style={{ marginBottom: 16 }}>
            <label className="form-label">{t('login.username')}</label>
            <input
              type="text"
              className="form-input"
              required
              autoComplete="username"
              autoFocus
              value={handle}
              onChange={e => setHandle(e.target.value)}
            />
          </div>
          <div style={{ marginBottom: 24 }}>
            <label className="form-label">{t('login.password')}</label>
            <input
              type="password"
              className="form-input"
              required
              autoComplete="current-password"
              value={password}
              onChange={e => setPassword(e.target.value)}
            />
          </div>
          <button type="submit" className="btn btn-primary" disabled={loading} style={{ width: '100%', justifyContent: 'center', padding: 12, fontSize: '1rem' }}>
            {loading ? t('common.loading') : t('login.submit')}
          </button>
        </form>
      </div>
    </div>
  );
}
