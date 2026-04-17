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
    listSkills(12, '', 'downloads').then(r => setSkills(r.data || [])).catch(e => console.error('Failed to load skills:', e));
  }, []);

  return (
    <>
      <section className="hero-section">
        <div className="hero-glow" />
        <div className="container">
          <h1 className="hero-title">
            <span className="hero-line-1">{t('index.hero_title_1')}</span>
            <span className="hero-line-2">{t('index.hero_title_2')}</span>
          </h1>
          <p className="hero-subtitle">
            {t('index.hero_subtitle')}
          </p>
          <div className="hero-actions">
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
