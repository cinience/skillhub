import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { getSkill, getVersions, getFile, type Skill, type SkillVersion } from '../api/skills';
import CodeBlock from '../components/CodeBlock';
import { formatDisplayName } from '../utils/displayName';

marked.setOptions({ breaks: true, gfm: true });

function stripFrontmatter(text: string): string {
  const match = text.match(/^---\s*\n[\s\S]*?\n---\s*\n/);
  return match ? text.slice(match[0].length) : text;
}

export default function SkillDetail() {
  const { t } = useTranslation();
  const { slug } = useParams<{ slug: string }>();
  const [skill, setSkill] = useState<Skill | null>(null);
  const [versions, setVersions] = useState<SkillVersion[]>([]);
  const [mdHtml, setMdHtml] = useState('');
  const [notFound, setNotFound] = useState(false);

  useEffect(() => {
    if (!slug) return;
    getSkill(slug)
      .then(r => setSkill(r))
      .catch(() => setNotFound(true));
    getVersions(slug)
      .then(r => setVersions(r.versions || []))
      .catch(e => console.error('Failed to load skill data:', e));
    // Load SKILL.md content
    getFile(slug, 'latest', 'SKILL.md')
      .then(async res => {
        if (!res.ok) return;
        const text = await res.text();
        setMdHtml(stripFrontmatter(text));
      })
      .catch(e => console.error('Failed to load skill data:', e));
  }, [slug]);

  if (notFound) {
    return (
      <div className="container" style={{ padding: '64px 0', textAlign: 'center', color: 'var(--text-muted)' }}>
        {t('detail.not_found')}
      </div>
    );
  }

  if (!skill) {
    return (
      <div className="container" style={{ padding: '64px 0', textAlign: 'center', color: 'var(--text-muted)' }}>
        {t('common.loading')}
      </div>
    );
  }

  const latestVersion = versions.length > 0 ? versions[0] : null;
  const initial = skill.ownerHandle?.[0]?.toUpperCase() || '?';
  const fmt = (d: string) => new Date(d).toLocaleDateString();
  const title = formatDisplayName(skill.displayName, skill.slug);

  return (
    <section style={{ padding: '40px 0' }}>
      <div className="container">
        <div className="detail-header">
          <div className="owner-info" style={{ marginBottom: 12 }}>
            <div className="owner-avatar">{initial}</div>
            <span style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>{skill.ownerHandle}</span>
          </div>
          <h1 className="detail-title" title={title.tooltip}>
            {title.text}
          </h1>
          <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: 12, fontFamily: 'monospace' }}>
            {skill.ownerHandle}/{skill.slug}
          </div>
          {skill.summary && <p className="detail-summary">{skill.summary}</p>}
          <div className="detail-stats">
            <div className="stat"><span className="stat-value">{skill.starsCount}</span><span className="stat-label">{t('detail.stars')}</span></div>
            <div className="stat"><span className="stat-value">{skill.downloads}</span><span className="stat-label">{t('detail.downloads')}</span></div>
            <div className="stat"><span className="stat-value">{skill.installs}</span><span className="stat-label">{t('detail.installs')}</span></div>
            <div className="stat"><span className="stat-value">{skill.versionsCount}</span><span className="stat-label">{t('detail.versions')}</span></div>
          </div>
          {skill.tags?.length > 0 && (
            <div className="card-tags">
              {skill.tags.map(tag => <span className="tag" key={tag}>{tag}</span>)}
            </div>
          )}
        </div>

        <div className="detail-layout">
          <div>
            {mdHtml ? (
              <div className="markdown-body" dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(marked.parse(mdHtml) as string) }} />
            ) : (
              <div style={{ color: 'var(--text-muted)', padding: '40px 0', textAlign: 'center' }}>
                {t('detail.no_content')}
              </div>
            )}
          </div>

          <div>
            <div className="detail-sidebar">
              {latestVersion && (
                <>
                  <div className="sidebar-section">
                    <div className="sidebar-label">{t('detail.install')}</div>
                    <CodeBlock command={`skillhub install ${skill.slug}`} style={{ padding: '10px 14px', fontSize: '0.8rem' }} />
                  </div>
                  <div className="sidebar-section">
                    <div className="sidebar-label">{t('detail.latest_version')}</div>
                    <div className="sidebar-value" style={{ fontFamily: 'monospace', color: 'var(--accent)' }}>v{latestVersion.version}</div>
                  </div>
                </>
              )}
              <div className="sidebar-section">
                <div className="sidebar-label">{t('detail.published_by')}</div>
                <div className="owner-info">
                  <div className="owner-avatar">{initial}</div>
                  <span className="sidebar-value">{skill.ownerHandle}</span>
                </div>
              </div>
              <div className="sidebar-section">
                <div className="sidebar-label">{t('detail.created')}</div>
                <div className="sidebar-value">{fmt(skill.createdAt)}</div>
              </div>
              <div className="sidebar-section">
                <div className="sidebar-label">{t('detail.updated')}</div>
                <div className="sidebar-value">{fmt(skill.updatedAt)}</div>
              </div>
              {latestVersion && (
                <div className="sidebar-section">
                  <a href={`/api/v1/download?slug=${skill.slug}&version=${latestVersion.version}`} className="btn btn-secondary" style={{ width: '100%', justifyContent: 'center' }}>
                    &#8615; {t('detail.download_zip')}
                  </a>
                </div>
              )}
            </div>

            {versions.length > 0 && (
              <div className="detail-sidebar" style={{ marginTop: 16 }}>
                <div className="sidebar-label" style={{ marginBottom: 12 }}>{t('detail.version_history')}</div>
                <ul className="version-list">
                  {versions.map(v => (
                    <li className="version-item" key={v.version}>
                      <span className="version-tag">v{v.version}</span>
                      <span className="version-date">{fmt(v.createdAt)}</span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
