import { useCallback, useEffect, useState } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../hooks/useAuth';
import {
  getNamespace,
  listMembers,
  type Namespace,
  type NamespaceMember,
} from '@saker/skillhub-client/namespaces';
import { listSkills, type Skill } from '@saker/skillhub-client/skills';
import SkillCard from '../components/SkillCard';
import {
  listTeamTokens,
  createTeamToken,
  revokeTeamToken,
  type TeamToken,
} from '@saker/skillhub-client/team-tokens';

// Pre-defined expiresIn options — matches CLI defaults plus a 365d cap that
// the server enforces (maxTeamTokenLifetime). Custom strings (e.g. "168h")
// are still allowed via free-text input.
const EXPIRES_OPTIONS = [
  { value: '24h', labelKey: 'tokens.expires_1d' },
  { value: '168h', labelKey: 'tokens.expires_7d' },
  { value: '720h', labelKey: 'tokens.expires_30d' },
  { value: '2160h', labelKey: 'tokens.expires_90d' },
  { value: '8760h', labelKey: 'tokens.expires_365d' },
];

export default function NamespaceDetail() {
  const { t } = useTranslation();
  const { slug = '' } = useParams<{ slug: string }>();
  const { user, loading: authLoading } = useAuth();
  const navigate = useNavigate();

  const [ns, setNs] = useState<Namespace | null>(null);
  const [members, setMembers] = useState<NamespaceMember[]>([]);
  const [tokens, setTokens] = useState<TeamToken[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Create-token form state
  const [showCreate, setShowCreate] = useState(false);
  const [tLabel, setTLabel] = useState('');
  const [tScope, setTScope] = useState<'read' | 'publish'>('publish');
  const [tExpires, setTExpires] = useState('720h');
  const [creating, setCreating] = useState(false);

  // Just-minted raw token — shown ONCE, dismissed on user action.
  const [rawToken, setRawToken] = useState('');
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!authLoading && !user) {
      navigate(`/login?redirect=/namespaces/${slug}`, { replace: true });
    }
  }, [authLoading, user, navigate, slug]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [n, m, tk, sk] = await Promise.all([
        getNamespace(slug),
        listMembers(slug),
        listTeamTokens(slug).catch(() => ({ data: [] as TeamToken[] })),
        listSkills(100, '', 'updated', slug).catch(() => ({ data: [] as Skill[], nextCursor: '' })),
      ]);
      setNs(n);
      setMembers(m.data || []);
      setTokens(tk.data || []);
      setSkills(sk.data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load namespace');
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    if (user && slug) refresh();
  }, [user, slug, refresh]);

  // Whether the current user can manage tokens (owner or admin in this ns).
  const myMembership = members.find(m => m.handle === user?.handle);
  const canManageTokens =
    user?.role === 'admin' || myMembership?.role === 'owner' || myMembership?.role === 'admin';

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    setError('');
    try {
      const res = await createTeamToken(slug, {
        label: tLabel.trim() || undefined,
        scope: tScope,
        expiresIn: tExpires.trim(),
      });
      setRawToken(res.token);
      setShowCreate(false);
      setTLabel('');
      setTScope('publish');
      setTExpires('720h');
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create token');
    } finally {
      setCreating(false);
    }
  };

  const handleRevoke = async (id: string) => {
    if (!confirm(t('tokens.confirm_revoke') || 'Revoke this token?')) return;
    setError('');
    try {
      await revokeTeamToken(slug, id);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke token');
    }
  };

  const handleCopy = async () => {
    if (!rawToken) return;
    try {
      await navigator.clipboard.writeText(rawToken);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Some browsers / non-https contexts block clipboard API; fall through.
    }
  };

  if (authLoading || !user) return <div className="container">{t('common.loading')}</div>;
  if (loading) return <div className="container">{t('common.loading')}</div>;
  if (!ns) return <div className="container">{error || t('namespaces.not_found')}</div>;

  return (
    <div className="container" style={{ paddingTop: '2rem', paddingBottom: '4rem' }}>
      <Link to="/namespaces" style={{ fontSize: '0.9rem', color: 'var(--muted, #6b7280)' }}>
        ← {t('namespaces.back')}
      </Link>

      <div style={{ marginTop: '0.5rem', marginBottom: '2rem' }}>
        <h1 style={{ margin: 0 }}>{ns.displayName || ns.slug}</h1>
        <div style={{ color: 'var(--muted, #6b7280)' }}>@{ns.slug} · {ns.type} · {ns.status}</div>
        {ns.description && <p style={{ marginTop: '0.5rem' }}>{ns.description}</p>}
      </div>

      {error && <div className="error-box" style={{ marginBottom: '1rem' }}>{error}</div>}

      {/* Just-minted raw token banner — appears once, persists until dismissed
          so the user has time to copy. The metadata.id row is what shows up in
          the table below; this banner is the ONLY place where the secret is
          ever visible. */}
      {rawToken && (
        <div style={{
          background: 'var(--warning-bg, #fef3c7)',
          border: '2px solid var(--warning-border, #f59e0b)',
          padding: '1rem 1.25rem',
          borderRadius: '8px',
          marginBottom: '1.5rem',
        }}>
          <div style={{ fontWeight: 700, color: 'var(--warning-text, #92400e)', marginBottom: '0.5rem' }}>
            ⚠ {t('tokens.show_once_warning')}
          </div>
          <div style={{
            display: 'flex',
            gap: '0.5rem',
            alignItems: 'center',
            fontFamily: 'monospace',
            background: 'var(--code-bg, #fff)',
            color: 'var(--text, #000)',
            padding: '0.5rem 0.75rem',
            borderRadius: '4px',
            wordBreak: 'break-all',
          }}>
            <code style={{ flex: 1, color: 'inherit' }}>{rawToken}</code>
            <button className="btn btn-secondary btn-small" onClick={handleCopy}>
              {copied ? t('common.copied') : t('common.copy')}
            </button>
            <button className="btn btn-secondary btn-small" onClick={() => setRawToken('')}>
              {t('common.dismiss')}
            </button>
          </div>
        </div>
      )}

      {/* Members section */}
      <section style={{ marginBottom: '2.5rem' }}>
        <h2>{t('namespaces.members')}</h2>
        <div style={{ display: 'grid', gap: '0.5rem' }}>
          {members.map(m => (
            <div key={m.id} style={{
              display: 'flex',
              justifyContent: 'space-between',
              padding: '0.5rem 0.75rem',
              background: 'var(--surface, #fff)',
              border: '1px solid var(--border, #e5e7eb)',
              borderRadius: '4px',
            }}>
              <span>@{m.handle}{m.displayName ? ` (${m.displayName})` : ''}</span>
              <span style={{ color: 'var(--muted, #6b7280)' }}>{m.role}</span>
            </div>
          ))}
        </div>
      </section>

      {/* Skills in this namespace */}
      <section style={{ marginBottom: '2.5rem' }}>
        <h2>{t('namespaces.skills', 'Skills')}</h2>
        {skills.length === 0 ? (
          <div style={{ color: 'var(--muted, #6b7280)' }}>{t('namespaces.no_skills', 'No skills in this namespace yet.')}</div>
        ) : (
          <div className="skill-grid">
            {skills.map(s => <SkillCard key={s.slug} {...s} />)}
          </div>
        )}
      </section>

      {/* Tokens section — only visible if user can manage them. The list comes
          back empty for non-managers anyway (handler returns 403, swallowed
          above), but hide the whole section to avoid confusing UI. */}
      {canManageTokens && (
        <section>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <h2 style={{ margin: 0 }}>{t('tokens.title')}</h2>
            <button className="btn btn-primary" onClick={() => setShowCreate(v => !v)}>
              {showCreate ? t('common.cancel') : t('tokens.create_btn')}
            </button>
          </div>

          {showCreate && (
            <form onSubmit={handleCreate} style={{
              background: 'var(--surface, #fff)',
              padding: '1.25rem',
              borderRadius: '8px',
              marginBottom: '1.5rem',
              border: '1px solid var(--border, #e5e7eb)',
              display: 'grid',
              gap: '0.75rem',
            }}>
              <div>
                <label className="form-label">{t('tokens.label')}</label>
                <input type="text" className="form-input" value={tLabel}
                  onChange={e => setTLabel(e.target.value)} placeholder="ci-runner" />
              </div>
              <div>
                <label className="form-label">{t('tokens.scope')}</label>
                <select className="form-input" value={tScope}
                  onChange={e => setTScope(e.target.value as 'read' | 'publish')}>
                  <option value="publish">publish</option>
                  <option value="read">read</option>
                </select>
                <small style={{ color: 'var(--muted, #6b7280)' }}>{t('tokens.scope_hint')}</small>
              </div>
              <div>
                <label className="form-label">{t('tokens.expires_in')} *</label>
                <select className="form-input" value={tExpires}
                  onChange={e => setTExpires(e.target.value)}>
                  {EXPIRES_OPTIONS.map(opt => (
                    <option key={opt.value} value={opt.value}>{t(opt.labelKey)}</option>
                  ))}
                </select>
                <small style={{ color: 'var(--muted, #6b7280)' }}>{t('tokens.expires_hint')}</small>
              </div>
              <div>
                <button type="submit" className="btn btn-primary" disabled={creating}>
                  {creating ? t('common.loading') : t('tokens.create_submit')}
                </button>
              </div>
            </form>
          )}

          {tokens.length === 0 ? (
            <div style={{ color: 'var(--muted, #6b7280)' }}>{t('tokens.empty')}</div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ borderBottom: '2px solid var(--border, #e5e7eb)', textAlign: 'left' }}>
                    <th style={{ padding: '0.5rem' }}>{t('tokens.col_label')}</th>
                    <th style={{ padding: '0.5rem' }}>{t('tokens.col_prefix')}</th>
                    <th style={{ padding: '0.5rem' }}>{t('tokens.col_scope')}</th>
                    <th style={{ padding: '0.5rem' }}>{t('tokens.col_created')}</th>
                    <th style={{ padding: '0.5rem' }}>{t('tokens.col_expires')}</th>
                    <th style={{ padding: '0.5rem' }}></th>
                  </tr>
                </thead>
                <tbody>
                  {tokens.map(tok => (
                    <tr key={tok.id} style={{ borderBottom: '1px solid var(--border, #e5e7eb)' }}>
                      <td style={{ padding: '0.5rem' }}>{tok.label || '—'}</td>
                      <td style={{ padding: '0.5rem', fontFamily: 'monospace' }}>{tok.prefix}…</td>
                      <td style={{ padding: '0.5rem' }}>{tok.scope}</td>
                      <td style={{ padding: '0.5rem', fontSize: '0.85rem', color: 'var(--muted, #6b7280)' }}>
                        {new Date(tok.createdAt).toLocaleDateString()}
                      </td>
                      <td style={{ padding: '0.5rem', fontSize: '0.85rem', color: 'var(--muted, #6b7280)' }}>
                        {tok.expiresAt ? new Date(tok.expiresAt).toLocaleDateString() : t('tokens.never')}
                      </td>
                      <td style={{ padding: '0.5rem', textAlign: 'right' }}>
                        <button
                          className="btn btn-secondary btn-small"
                          onClick={() => handleRevoke(tok.id)}
                          style={{ color: 'var(--danger, #dc2626)' }}
                        >
                          {t('tokens.revoke')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
