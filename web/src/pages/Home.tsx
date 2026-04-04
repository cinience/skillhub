import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { listSkills, type Skill } from '../api/skills';
import SkillCard from '../components/SkillCard';
import CodeBlock from '../components/CodeBlock';

export default function Home() {
  const { t } = useTranslation();
  const [skills, setSkills] = useState<Skill[]>([]);

  useEffect(() => {
    listSkills(6, '', 'downloads').then(r => setSkills(r.skills || [])).catch(e => console.error('Failed to load skills:', e));
  }, []);

  return (
    <>
      <section style={{ textAlign: 'center', padding: '80px 0 48px' }}>
        <div className="container">
          <h1 style={{ fontSize: '3rem', fontWeight: 800, marginBottom: 16 }} className="hero-title">
            {t('index.hero_title_1')}<br />
            <span style={{ color: 'var(--accent)' }}>{t('index.hero_title_2')}</span>
          </h1>
          <p style={{ fontSize: '1.15rem', color: 'var(--text-secondary)', maxWidth: 560, margin: '0 auto 32px' }}>
            {t('index.hero_subtitle')}
          </p>
          <div style={{ display: 'flex', gap: 12, justifyContent: 'center', flexWrap: 'wrap' }}>
            <Link to="/skills" className="btn btn-primary">{t('index.browse')}</Link>
            <Link to="/publish" className="btn btn-secondary">{t('index.publish')}</Link>
          </div>
        </div>
      </section>

      <section style={{ padding: '0 0 48px' }}>
        <div className="container" style={{ maxWidth: 640 }}>
          <CodeBlock command="skillhub install @owner/skill-name" />
        </div>
      </section>

      {skills.length > 0 && (
        <section style={{ padding: '0 0 64px' }}>
          <div className="container">
            <h2 style={{ fontSize: '1.5rem', fontWeight: 700, marginBottom: 24 }}>{t('index.popular')}</h2>
            <div className="skill-grid">
              {skills.map(s => (
                <SkillCard key={s.slug} {...s} />
              ))}
            </div>
            <div style={{ textAlign: 'center', marginTop: 32 }}>
              <Link to="/skills" className="btn btn-secondary">{t('index.view_all')} &rarr;</Link>
            </div>
          </div>
        </section>
      )}
    </>
  );
}
