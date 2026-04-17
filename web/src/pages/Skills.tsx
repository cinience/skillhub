import { useEffect, useState, useCallback } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { listSkills, type Skill } from '../api/skills';
import { searchSkills, type SearchHit } from '../api/search';
import SkillCard from '../components/SkillCard';

const PAGE_SIZE = 50;
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
  const [params, setParams] = useSearchParams();
  const sort = params.get('sort') || 'created';
  const queryParam = params.get('q') || '';

  // Browse mode state
  const [skills, setSkills] = useState<Skill[]>([]);
  const [cursors, setCursors] = useState<string[]>(['']); // cursor history for page navigation
  const [currentPage, setCurrentPage] = useState(0);
  const [hasNext, setHasNext] = useState(false);
  const [loading, setLoading] = useState(false);

  // Search mode state
  const [searchInput, setSearchInput] = useState(queryParam);
  const [searchQuery, setSearchQuery] = useState(queryParam);
  const [searchResults, setSearchResults] = useState<SearchHit[]>([]);
  const [searchTotal, setSearchTotal] = useState(0);
  const [searchPage, setSearchPage] = useState(0);
  const [searchLoading, setSearchLoading] = useState(false);

  const isSearchMode = searchQuery.length > 0;

  // Load skills (browse mode)
  const loadPage = useCallback((page: number, cursor: string, sortBy: string) => {
    setLoading(true);
    listSkills(PAGE_SIZE, cursor, sortBy)
      .then(r => {
        setSkills(r.data || []);
        setCurrentPage(page);
        const next = r.nextCursor || '';
        setHasNext(!!next);
        setCursors(prev => {
          const updated = [...prev];
          updated[page + 1] = next;
          return updated;
        });
      })
      .catch(e => console.error('Failed to load skills:', e))
      .finally(() => setLoading(false));
  }, []);

  // Initial load / sort change
  useEffect(() => {
    if (!isSearchMode) {
      setCursors(['']);
      loadPage(0, '', sort);
    }
  }, [sort, isSearchMode, loadPage]);

  // Search mode
  useEffect(() => {
    if (!searchQuery) return;
    setSearchLoading(true);
    searchSkills(searchQuery, PAGE_SIZE, searchPage * PAGE_SIZE)
      .then(r => {
        setSearchResults(r.hits || []);
        setSearchTotal(r.estimatedTotal || 0);
      })
      .catch(e => console.error('Search failed:', e))
      .finally(() => setSearchLoading(false));
  }, [searchQuery, searchPage]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const q = searchInput.trim();
    if (q) {
      setSearchQuery(q);
      setSearchPage(0);
      setParams({ q, sort });
    } else {
      clearSearch();
    }
  };

  const clearSearch = () => {
    setSearchInput('');
    setSearchQuery('');
    setSearchResults([]);
    setSearchTotal(0);
    setSearchPage(0);
    setParams({ sort });
  };

  const goToPage = (page: number) => {
    if (isSearchMode) {
      setSearchPage(page);
    } else {
      loadPage(page, cursors[page] || '', sort);
    }
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const displayItems = isSearchMode ? searchResults : skills;
  const isLoading = isSearchMode ? searchLoading : loading;
  const totalPages = isSearchMode ? Math.ceil(searchTotal / PAGE_SIZE) : undefined;
  const page = isSearchMode ? searchPage : currentPage;
  const canNext = isSearchMode ? (page + 1) < (totalPages || 0) : hasNext;
  const canPrev = page > 0;

  return (
    <section style={{ padding: '40px 0' }}>
      <div className="container">
        <h1 style={{ fontSize: '1.75rem', fontWeight: 700, marginBottom: 20 }}>{t('skills.title')}</h1>

        {/* Search bar */}
        <form onSubmit={handleSearch} className="skills-search">
          <input
            type="text"
            placeholder={t('search.placeholder')}
            value={searchInput}
            onChange={e => setSearchInput(e.target.value)}
          />
          <button type="submit" className="btn btn-primary" style={{ padding: '10px 20px' }}>
            {t('search.button')}
          </button>
          {isSearchMode && (
            <button type="button" className="btn btn-secondary" style={{ padding: '10px 16px' }} onClick={clearSearch}>
              &times;
            </button>
          )}
        </form>

        {/* Search result info */}
        {isSearchMode && !searchLoading && (
          <div style={{ marginBottom: 16, color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
            {searchResults.length > 0
              ? <span>{t('search.found_text', { count: searchTotal })} "<strong>{searchQuery}</strong>"</span>
              : <span>{t('search.no_results_text')} "<strong>{searchQuery}</strong>"</span>
            }
          </div>
        )}

        {/* Sort bar (browse mode only) */}
        {!isSearchMode && (
          <div className="filter-bar">
            {SORT_OPTIONS.map(s => (
              <Link key={s} to={`/skills?sort=${s}`} className={`filter-btn ${sort === s ? 'active' : ''}`}>
                {t(SORT_KEYS[s])}
              </Link>
            ))}
          </div>
        )}

        {/* Loading spinner */}
        {isLoading && (
          <div style={{ textAlign: 'center', padding: displayItems.length > 0 ? '16px 0' : '64px 0' }}>
            <div className="spinner" />
          </div>
        )}

        {/* Skills grid */}
        {displayItems.length > 0 && (
          <>
            <div className="skill-grid">
              {isSearchMode
                ? searchResults.map(hit => (
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
                  ))
                : skills.map(s => <SkillCard key={s.slug} {...s} />)
              }
            </div>

            {/* Pagination */}
            <div className="pagination">
              <button
                className="pagination-btn"
                disabled={!canPrev || isLoading}
                onClick={() => goToPage(page - 1)}
              >
                &larr; {t('skills.prev') || 'Prev'}
              </button>

              <span className="pagination-info">
                {t('skills.page') || 'Page'} {page + 1}
                {totalPages != null && ` / ${totalPages}`}
              </span>

              <button
                className="pagination-btn"
                disabled={!canNext || isLoading}
                onClick={() => goToPage(page + 1)}
              >
                {t('skills.next') || 'Next'} &rarr;
              </button>
            </div>
          </>
        )}

        {/* Empty state */}
        {!isLoading && displayItems.length === 0 && !isSearchMode && (
          <div style={{ textAlign: 'center', padding: '64px 0', color: 'var(--text-muted)' }}>
            <p style={{ fontSize: '1.1rem' }}>{t('skills.no_skills')}</p>
            <p style={{ marginTop: 8 }}><Link to="/publish">{t('skills.be_first')}</Link></p>
          </div>
        )}
      </div>
    </section>
  );
}
