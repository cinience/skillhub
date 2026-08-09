import { apiFetch, apiFetchRaw, apiURL } from './client';

export interface Plugin {
  id: string;
  slug: string;
  displayName?: string;
  summary?: string;
  visibility: string;
  category: string;
  tags: string[];
  downloads: number;
  starsCount: number;
  versionsCount?: number;
  ownerHandle: string;
  latestVersionId?: string;
  version?: string;
  fingerprint?: string;
  sha256Hash?: string;
  files?: PluginFile[];
  createdAt: string;
  updatedAt: string;
}

export interface PluginFile {
  path: string;
  size: number;
  sha256: string;
}

export interface PluginVersion {
  id: string;
  pluginId: string;
  version: string;
  fingerprint: string;
  sha256Hash: string;
  files: PluginFile[];
  changelog?: string;
  createdAt: string;
  yankedAt?: string;
  yankReason?: string;
}

export interface PluginListResponse {
  data: Plugin[];
  nextCursor: string;
}

export function listPlugins(limit = 20, cursor = '', category = '', sort = ''): Promise<PluginListResponse> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set('cursor', cursor);
  if (category) params.set('category', category);
  if (sort) params.set('sort', sort);
  return apiFetch(`/plugins?${params}`);
}

export function getPlugin(slug: string): Promise<Plugin> {
  return apiFetch(`/plugins/${slug}`);
}

export function getPluginById(id: string): Promise<Plugin> {
  return apiFetch(`/plugins/by-id/${encodeURIComponent(id)}`);
}

export function getPluginVersions(slug: string): Promise<{ versions: PluginVersion[] }> {
  return apiFetch(`/plugins/${slug}/versions`);
}

export function getPluginFile(slug: string, version: string, path: string): Promise<Response> {
  const params = new URLSearchParams({ slug, version, path });
  return apiFetchRaw(`/plugins/file?${params}`);
}

export function pluginDownloadURL(slug: string, version: string): string {
  const params = new URLSearchParams({ slug, version });
  return apiURL(`/plugins/download?${params}`);
}
