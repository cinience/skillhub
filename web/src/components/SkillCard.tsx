import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

interface Props {
  slug: string;
  displayName?: string;
  summary?: string;
  tags?: string[];
  ownerHandle: string;
  starsCount: number;
  downloads: number;
  updatedAt?: string;
}

export default function SkillCard({ slug, displayName, summary, tags, ownerHandle, starsCount, downloads, updatedAt }: Props) {
  const { t } = useTranslation();
  const initial = ownerHandle ? ownerHandle[0].toUpperCase() : '?';

  return (
    <Link to={`/skills/${slug}`}>
      <div className="card">
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
          <div className="owner-avatar" style={{ width: 28, height: 28, fontSize: '0.7rem' }}>{initial}</div>
          <div style={{ minWidth: 0 }}>
            <div className="card-title" style={{ marginBottom: 0 }}>{displayName || slug}</div>
            <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{ownerHandle}/{slug}</div>
          </div>
        </div>
        <div className="card-summary">{summary || t('common.no_description')}</div>
        {tags && tags.length > 0 && (
          <div className="card-tags">
            {tags.slice(0, 3).map(tag => <span className="tag" key={tag}>{tag}</span>)}
            {tags.length > 3 && <span className="tag">+{tags.length - 3}</span>}
          </div>
        )}
        <div className="card-meta">
          <span>&#9733; {starsCount}</span>
          <span>&#8615; {downloads}</span>
          {updatedAt && <span>{new Date(updatedAt).toLocaleDateString()}</span>}
          {!updatedAt && <span>{ownerHandle}</span>}
        </div>
      </div>
    </Link>
  );
}
