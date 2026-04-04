import { apiFetch, apiFetchRaw } from './client';

export interface Skill {
  id: string;
  slug: string;
  displayName?: string;
  summary?: string;
  tags: string[];
  downloads: number;
  installs: number;
  starsCount: number;
  versionsCount: number;
  ownerHandle: string;
  createdAt: string;
  updatedAt: string;
}

export interface SkillVersion {
  id: string;
  skillId: string;
  version: string;
  files: string[];
  createdAt: string;
}

export interface SkillListResponse {
  skills: Skill[];
  nextCursor: string;
}

export function listSkills(limit = 20, cursor = '', sort = 'created'): Promise<SkillListResponse> {
  const params = new URLSearchParams({ limit: String(limit), sort });
  if (cursor) params.set('cursor', cursor);
  return apiFetch(`/skills?${params}`);
}

export function getSkill(slug: string): Promise<{ skill: Skill }> {
  return apiFetch(`/skills/${slug}`);
}

export function getVersions(slug: string): Promise<{ versions: SkillVersion[] }> {
  return apiFetch(`/skills/${slug}/versions`);
}

export function getFile(slug: string, version: string, path: string): Promise<Response> {
  const params = new URLSearchParams({ version, path });
  return apiFetchRaw(`/skills/${slug}/file?${params}`);
}

export function publishSkill(formData: FormData): Promise<{ skill: Skill; version: SkillVersion }> {
  return apiFetch('/skills', { method: 'POST', body: formData });
}

export function deleteSkill(slug: string): Promise<void> {
  return apiFetch(`/skills/${slug}`, { method: 'DELETE' });
}
