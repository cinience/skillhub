import { useState } from 'react';
import { useTranslation } from 'react-i18next';

interface Props {
  command: string;
  prefix?: string;
  style?: React.CSSProperties;
}

export default function CodeBlock({ command, prefix = '$ ', style }: Props) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for insecure contexts
    }
  };

  return (
    <div className="code-block" style={style}>
      <button className="copy-btn" onClick={copy}>
        {copied ? t('common.copied') : t('common.copy')}
      </button>
      <span className="prefix">{prefix}</span>
      <span className="cmd">{command}</span>
    </div>
  );
}
