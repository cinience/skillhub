import { useTranslation } from 'react-i18next';

export default function LangSwitch() {
  const { t, i18n } = useTranslation();
  const isZh = i18n.language?.startsWith('zh');

  return (
    <button
      className="icon-btn"
      onClick={() => i18n.changeLanguage(isZh ? 'en' : 'zh')}
      title={t('common.switch_lang')}
    >
      {isZh ? 'EN' : '中'}
    </button>
  );
}
