export interface DisplayNamePresentation {
  text: string;
  tooltip?: string;
  truncated: boolean;
}

const DEFAULT_MAX_LENGTH = 80;

function normalizeWhitespace(value: string): string {
  return value.replace(/\s+/g, ' ').trim();
}

export function formatDisplayName(
  displayName: string | undefined,
  fallback: string,
  maxLength = DEFAULT_MAX_LENGTH,
): DisplayNamePresentation {
  const normalizedFallback = normalizeWhitespace(fallback);
  const normalizedDisplayName = normalizeWhitespace(displayName ?? '');
  const source = normalizedDisplayName || normalizedFallback;

  if (source.length <= maxLength) {
    return {
      text: source,
      tooltip: normalizedDisplayName && normalizedDisplayName !== source ? normalizedDisplayName : undefined,
      truncated: false,
    };
  }

  const clipped = source.slice(0, Math.max(1, maxLength - 1)).trimEnd();
  return {
    text: `${clipped}\u2026`,
    tooltip: source,
    truncated: true,
  };
}
