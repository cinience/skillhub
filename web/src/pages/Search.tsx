import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { searchSkills, type SearchHit } from '../api/search';
import SkillCard from '../components/SkillCard';

export default function Search() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const query = params.get('q') || '';
  const [input, setInput] = useState(query);
  const [results, setResults] = useState<SearchHit[]>([]);
  const [total, setTotal] = useState(0);
  const [timeMs, setTimeMs] = useState(0);
  const [searched, setSearched] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!query) { setSearched(false); return; }
    setSearched(true);
    setError('');
    searchSkills(query)
      .then(r => { setResults(r.hits || []); setTotal(r.estimatedTotal); setTimeMs(r.processingTimeMs); })
      .catch(err => setError(err instanceof Error ? err.message : 'Search failed'));
  }, [query]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (input.trim()) setParams({ q: input.trim() });
  };

  return (
    <section style={{ padding: '40px 0' }}>
      <div className="container">
        <div style={{ maxWidth: 640, margin: '0 auto 40px' }}>
          <form onSubmit={handleSubmit} className="search-box">
            <input
              type="text"
              className="search-input"
              placeholder={t('search.placeholder')}
              value={input}
              onChange={e => setInput(e.target.value)}
              autoFocus
            />
            <button type="submit" className="btn btn-primary">{t('search.button')}</button>
          </form>
        </div>

        {error && <div className="error-box">{error}</div>}

        {searched && query && !error && (
          <div style={{ marginBottom: 24, color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
            {results.length > 0 ? (
              <span>{t('search.found_text', { count: total })} "<strong>{query}</strong>"</span>
            ) : (
              <span>{t('search.no_results_text')} "<strong>{query}</strong>"</span>
            )}
            {results.length > 0 && (
              <span style={{ color: 'var(--text-muted)' }}> ({timeMs}ms)</span>
            )}
          </div>
        )}

        {searched && results.length > 0 && (
          <div className="skill-grid">
            {results.map((hit) => (
              <SkillCard
                key={hit.slug}
                slug={hit.slug}
                displayName={hit.displayName}
                summary={hit.summary}
                tags={hit.tags}
                ownerHandle={hit.ownerHandle || ''}
                starsCount={hit.stars || 0}
                downloads={hit.downloads || 0}
              />
            ))}
          </div>
        )}

        {!searched && (
          <div style={{ textAlign: 'center', padding: '64px 0', color: 'var(--text-muted)' }}>
            <p style={{ fontSize: '1.1rem' }}>{t('search.enter_query')}</p>
          </div>
        )}
      </div>
    </section>
  );
}
