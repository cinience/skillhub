import { useState, useRef, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../hooks/useAuth';
import { publishSkill } from '../api/skills';
import CodeBlock from '../components/CodeBlock';

interface SelectedFile {
  file: File;
  path: string;
}

function isHidden(path: string) {
  return path.split('/').some(p => p.startsWith('.'));
}

export default function Publish() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [tab, setTab] = useState<'upload' | 'cli'>('upload');
  const [slug, setSlug] = useState('');
  const [version, setVersion] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [summary, setSummary] = useState('');
  const [tags, setTags] = useState('');
  const [files, setFiles] = useState<SelectedFile[]>([]);
  const [status, setStatus] = useState('');
  const [statusType, setStatusType] = useState<'error' | 'info' | 'success'>('info');
  const [publishing, setPublishing] = useState(false);
  const [result, setResult] = useState<{ slug: string; displayName?: string; version: string; fileCount: number } | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const folderRef = useRef<HTMLInputElement>(null);
  const [dragover, setDragover] = useState(false);

  const addFiles = useCallback((fileList: FileList) => {
    const newFiles: SelectedFile[] = [];
    for (let i = 0; i < fileList.length; i++) {
      const f = fileList[i];
      let path = f.webkitRelativePath || f.name;
      if (f.webkitRelativePath) {
        const parts = path.split('/');
        if (parts.length > 1) path = parts.slice(1).join('/');
      }
      if (isHidden(path)) continue;
      newFiles.push({ file: f, path });
    }
    setFiles(prev => {
      const existing = new Set(prev.map(f => f.path));
      return [...prev, ...newFiles.filter(f => !existing.has(f.path))];
    });
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragover(false);
    addFiles(e.dataTransfer.files);
  }, [addFiles]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!slug.trim() || !version.trim()) {
      setStatus(t('publish.fill_required'));
      setStatusType('error');
      return;
    }
    if (files.length === 0) {
      setStatus(t('publish.add_file'));
      setStatusType('error');
      return;
    }

    setPublishing(true);
    setStatus(t('publish.uploading'));
    setStatusType('info');

    const formData = new FormData();
    formData.append('slug', slug.trim());
    formData.append('version', version.trim());
    if (displayName.trim()) formData.append('displayName', displayName.trim());
    if (summary.trim()) formData.append('summary', summary.trim());
    if (tags.trim()) formData.append('tags', tags.trim());
    files.forEach(f => formData.append('files', f.file, f.path));

    try {
      const data = await publishSkill(formData);
      setStatus('');
      setResult({
        slug: data.skill?.slug || slug,
        displayName: data.skill?.displayName,
        version: data.version?.version || version,
        fileCount: data.version?.files?.length || files.length,
      });
    } catch (err) {
      setStatus(t('publish.error_prefix') + (err instanceof Error ? err.message : t('publish.unknown_error')));
      setStatusType('error');
    } finally {
      setPublishing(false);
    }
  };

  if (!user) {
    return (
      <div className="container" style={{ padding: '48px 24px' }}>
        <div style={{ maxWidth: 800, margin: '0 auto' }}>
          <h1 style={{ fontSize: '2rem', fontWeight: 700, marginBottom: 8 }}>{t('publish.title')}</h1>
          <p style={{ color: 'var(--text-secondary)', marginBottom: 24 }}>{t('publish.subtitle')}</p>
          <div className="card" style={{ padding: 32, textAlign: 'center' }}>
            <div style={{ fontSize: '1.5rem', marginBottom: 12 }}>&#128274;</div>
            <p style={{ color: 'var(--text-secondary)', marginBottom: 16 }}>{t('publish.login_required')}</p>
            <Link to="/login?redirect=/publish" className="btn btn-primary" style={{ padding: '12px 32px' }}>
              {t('publish.login_to_publish')}
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const baseURL = window.location.origin;

  return (
    <div className="container" style={{ padding: '48px 24px' }}>
      <div style={{ maxWidth: 800, margin: '0 auto' }}>
        <h1 style={{ fontSize: '2rem', fontWeight: 700, marginBottom: 8 }}>{t('publish.title')}</h1>
        <p style={{ color: 'var(--text-secondary)', marginBottom: 24 }}>{t('publish.subtitle')}</p>

        <div className="tab-bar">
          <button className={`tab-btn ${tab === 'upload' ? 'active' : ''}`} onClick={() => setTab('upload')}>{t('publish.tab_upload')}</button>
          <button className={`tab-btn ${tab === 'cli' ? 'active' : ''}`} onClick={() => setTab('cli')}>{t('publish.tab_cli')}</button>
        </div>

        {tab === 'upload' && (
          <div>
            <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
              <div className="card" style={{ padding: 24 }}>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                  <div>
                    <label className="form-label">{t('publish.slug')} *</label>
                    <input className="form-input" required placeholder="my-awesome-skill" pattern="[a-z0-9][a-z0-9\-]*[a-z0-9]" value={slug} onChange={e => setSlug(e.target.value)} />
                    <div className="form-hint">{t('publish.slug_hint')}</div>
                  </div>
                  <div>
                    <label className="form-label">{t('publish.version')} *</label>
                    <input className="form-input" required placeholder="1.0.0" pattern="\d+\.\d+\.\d+" value={version} onChange={e => setVersion(e.target.value)} />
                    <div className="form-hint">{t('publish.version_hint')}</div>
                  </div>
                </div>
                <div style={{ marginTop: 16 }}>
                  <label className="form-label">{t('publish.display_name')}</label>
                  <input className="form-input" placeholder="My Awesome Skill" value={displayName} onChange={e => setDisplayName(e.target.value)} />
                </div>
                <div style={{ marginTop: 16 }}>
                  <label className="form-label">{t('publish.summary')}</label>
                  <textarea className="form-input" rows={2} placeholder="A brief description..." value={summary} onChange={e => setSummary(e.target.value)} style={{ resize: 'vertical', fontFamily: 'inherit' }} />
                </div>
                <div style={{ marginTop: 16 }}>
                  <label className="form-label">{t('publish.tags')}</label>
                  <input className="form-input" placeholder="go, automation, tools" value={tags} onChange={e => setTags(e.target.value)} />
                  <div className="form-hint">{t('publish.tags_hint')}</div>
                </div>
              </div>

              <div className="card" style={{ padding: 24 }}>
                <label className="form-label" style={{ marginBottom: 12 }}>{t('publish.skill_files')} *</label>
                <div
                  className={`drop-zone ${dragover ? 'dragover' : ''}`}
                  onDragOver={e => { e.preventDefault(); setDragover(true); }}
                  onDragLeave={() => setDragover(false)}
                  onDrop={handleDrop}
                >
                  <div style={{ fontSize: '2rem', marginBottom: 8 }}>&#128193;</div>
                  <div style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>{t('publish.drop_files')}</div>
                  <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', marginTop: 6 }}>{t('publish.drop_hint')}</div>
                  <div style={{ marginTop: 14, display: 'flex', gap: 10, justifyContent: 'center' }}>
                    <button type="button" className="btn btn-primary btn-small" onClick={() => folderRef.current?.click()}>{t('publish.select_folder')}</button>
                    <button type="button" className="btn btn-secondary btn-small" onClick={() => fileRef.current?.click()}>{t('publish.select_files')}</button>
                  </div>
                </div>
                <input ref={fileRef} type="file" multiple style={{ display: 'none' }} onChange={e => e.target.files && addFiles(e.target.files)} />
                <input ref={folderRef} type="file" {...{ webkitdirectory: '' } as React.InputHTMLAttributes<HTMLInputElement>} style={{ display: 'none' }} onChange={e => e.target.files && addFiles(e.target.files)} />

                {files.length > 0 && (
                  <div style={{ marginTop: 12 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                      <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>{t('publish.files_selected', { count: files.length })}</span>
                      <button type="button" onClick={() => setFiles([])} style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '0.8rem', textDecoration: 'underline' }}>{t('publish.clear_all')}</button>
                    </div>
                    {files.map((f, i) => (
                      <div key={f.path} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 10px', background: 'var(--bg-secondary)', borderRadius: 4, marginBottom: 4, fontSize: '0.85rem' }}>
                        <span style={{ color: 'var(--text)' }}>{f.path} <span style={{ color: 'var(--text-muted)' }}>({f.file.size < 1024 ? `${f.file.size} B` : `${(f.file.size / 1024).toFixed(1)} KB`})</span></span>
                        <button type="button" onClick={() => setFiles(prev => prev.filter((_, j) => j !== i))} style={{ background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer', fontSize: '1rem', padding: '0 4px' }}>&times;</button>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                <button type="submit" className="btn btn-primary" disabled={publishing} style={{ padding: '12px 32px', fontSize: '1rem' }}>
                  {publishing ? t('publish.publishing') : t('publish.publish_btn')}
                </button>
                {status && <span style={{ fontSize: '0.9rem', color: statusType === 'error' ? 'var(--danger)' : 'var(--text-muted)' }}>{status}</span>}
              </div>
            </form>

            {result && (
              <div style={{ marginTop: 20 }}>
                <div className="card" style={{ borderColor: 'var(--accent)', padding: 20 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
                    <span style={{ color: 'var(--accent)', fontSize: '1.2rem' }}>&#10003;</span>
                    <strong>{t('publish.published_ok')}</strong>
                  </div>
                  <div style={{ fontSize: '0.9rem', color: 'var(--text-secondary)' }}>
                    <strong>{result.displayName || result.slug}</strong> v{result.version} &mdash; {result.fileCount} file(s)<br />
                    <Link to={`/skills/${result.slug}`} style={{ marginTop: 8, display: 'inline-block' }}>{t('publish.view_skill')} &rarr;</Link>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {tab === 'cli' && (
          <div>
            <div className="card" style={{ marginBottom: 24 }}>
              <h2 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: 12 }}>{t('publish.cli_title')}</h2>
              <CodeBlock command={`skillhub login\nskillhub publish ./my-skill --slug my-skill --version 1.0.0`} />
            </div>

            <div className="card" style={{ marginBottom: 24 }}>
              <h2 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: 12 }}>{t('publish.curl_title')}</h2>
              <CodeBlock command={`curl -X POST ${baseURL}/api/v1/skills \\\n  -H "Authorization: Bearer YOUR_TOKEN" \\\n  -F "slug=my-skill" \\\n  -F "version=1.0.0" \\\n  -F "displayName=My Skill" \\\n  -F "summary=A brief description" \\\n  -F "tags=python,automation" \\\n  -F "files=@SKILL.md" \\\n  -F "files=@prompt.md"`} />
            </div>

            <div className="card">
              <h2 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: 12 }}>{t('publish.api_title')}</h2>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--card-border)' }}>
                      <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text-muted)' }}>{t('publish.endpoint')}</th>
                      <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text-muted)' }}>{t('publish.method')}</th>
                      <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text-muted)' }}>{t('publish.description')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {[
                      ['POST /api/v1/skills', 'Multipart', t('publish.api_publish')],
                      ['GET /api/v1/skills', 'JSON', t('publish.api_list')],
                      ['GET /api/v1/skills/:slug', 'JSON', t('publish.api_detail')],
                      ['GET /api/v1/download', 'ZIP', t('publish.api_download')],
                      ['GET /api/v1/search', 'JSON', t('publish.api_search')],
                      ['GET /.well-known/clawhub.json', 'JSON', t('publish.api_discovery')],
                    ].map(([endpoint, method, desc]) => (
                      <tr key={endpoint} style={{ borderBottom: '1px solid var(--card-border)' }}>
                        <td style={{ padding: '8px 12px' }}><code style={{ color: 'var(--accent)' }}>{endpoint}</code></td>
                        <td style={{ padding: '8px 12px' }}>{method}</td>
                        <td style={{ padding: '8px 12px', color: 'var(--text-secondary)' }}>{desc}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
