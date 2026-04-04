import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { listSkills, type Skill } from '../api/skills';
import SkillCard from '../components/SkillCard';

const SORT_OPTIONS = ['created', 'updated', 'downloads', 'stars', 'name'] as const;
const SORT_KEYS: Record<string, string> = {
  created: 'skills.newest',
  updated: 'skills.recently_updated',
  downloads: 'skills.downloads',
  stars: 'skills.stars',
  name: 'skills.name',
};

export default function Skills() {
  const { t } = useTranslation();
  const [params] = useSearchParams();
  const sort = params.get('sort') || 'created';
  const [skills, setSkills] = useState<Skill[]>([]);
  const [nextCursor, setNextCursor] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    listSkills(20, '', sort)
      .then(r => { setSkills(r.skills || []); setNextCursor(r.nextCursor || ''); })
      .catch(e => console.error('Failed to load skills:', e))
      .finally(() => setLoading(false));
  }, [sort]);

  const loadMore = () => {
    if (!nextCursor) return;
    setLoading(true);
    listSkills(20, nextCursor, sort)
      .then(r => { setSkills(prev => [...prev, ...(r.skills || [])]); setNextCursor(r.nextCursor || ''); })
      .catch(e => console.error('Failed to load more skills:', e))
      .finally(() => setLoading(false));
  };

  return (
    <section style={{ padding: '40px 0' }}>
      <div className="container">
        <h1 style={{ fontSize: '1.75rem', fontWeight: 700, marginBottom: 24 }}>{t('skills.title')}</h1>

        <div className="filter-bar">
          {SORT_OPTIONS.map(s => (
            <Link key={s} to={`/skills?sort=${s}`} className={`filter-btn ${sort === s ? 'active' : ''}`}>
              {t(SORT_KEYS[s])}
            </Link>
          ))}
        </div>

        {skills.length > 0 ? (
          <>
            <div className="skill-grid">
              {skills.map(s => <SkillCard key={s.slug} {...s} />)}
            </div>
            {nextCursor && (
              <div className="load-more">
                <button className="btn btn-secondary" onClick={loadMore} disabled={loading}>
                  {t('skills.load_more')} &darr;
                </button>
              </div>
            )}
          </>
        ) : !loading ? (
          <div style={{ textAlign: 'center', padding: '64px 0', color: 'var(--text-muted)' }}>
            <p style={{ fontSize: '1.1rem' }}>{t('skills.no_skills')}</p>
            <p style={{ marginTop: 8 }}><Link to="/publish">{t('skills.be_first')}</Link></p>
          </div>
        ) : null}
      </div>
    </section>
  );
}
