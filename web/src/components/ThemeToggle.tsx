import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';

export default function ThemeToggle() {
  const { t } = useTranslation();
  const [theme, setTheme] = useState(() => {
    const saved = localStorage.getItem('theme');
    return saved || (window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark');
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
  }, [theme]);

  return (
    <button
      className="icon-btn"
      onClick={() => setTheme(t => t === 'dark' ? 'light' : 'dark')}
      title={t('common.toggle_theme')}
    >
      {theme === 'dark' ? '\u263D' : '\u2600'}
    </button>
  );
}
