import { apiFetch } from './client';

export interface SearchHit {
  slug: string;
  displayName: string;
  summary: string;
  tags: string[];
  ownerHandle?: string;
  stars?: number;
  downloads?: number;
}

export interface SearchResult {
  hits: SearchHit[];
  estimatedTotal: number;
  processingTimeMs: number;
}

export function searchSkills(query: string, limit = 20, offset = 0): Promise<SearchResult> {
  const params = new URLSearchParams({ q: query, limit: String(limit), offset: String(offset) });
  return apiFetch(`/search?${params}`);
}
