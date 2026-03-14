import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type AuditEvent = {
  id: string;
  actor?: { type?: string; id?: string | null } | null;
  action?: string | null;
  metadata?: unknown;
  createdAt?: string;
  [key: string]: unknown;
};

export type AuditListResponse = {
  events: AuditEvent[];
  nextCursor: string | null;
};

async function parseJsonOrThrow<T>(response: Response, fallback: string): Promise<T> {
  const body = await readJsonBody<any>(response);
  if (!response.ok) {
    const message =
      (body as { message?: string; error?: string } | null)?.message ??
      (body as { error?: string } | null)?.error ??
      fallback;
    throw new Error(message);
  }
  return body as T;
}

export async function fetchAuditEvents(params: {
  limit?: number;
  afterId?: string;
  actorId?: string;
  action?: string;
  stateId?: string;
  authorityId?: string;
  projectId?: string;
}): Promise<AuditListResponse> {
  const search = new URLSearchParams();
  if (params.limit) search.set('limit', String(params.limit));
  if (params.afterId) search.set('afterId', params.afterId);
  if (params.actorId) search.set('actorId', params.actorId);
  if (params.action) search.set('action', params.action);
  if (params.stateId) search.set('stateId', params.stateId);
  if (params.authorityId) search.set('authorityId', params.authorityId);
  if (params.projectId) search.set('projectId', params.projectId);

  const url = search.toString() ? `${API_BASE_URL}/audit?${search.toString()}` : `${API_BASE_URL}/audit`;
  const response = await apiFetch(url);
  const body = await parseJsonOrThrow<AuditListResponse>(response, 'Unable to load audit logs');

  return {
    events: Array.isArray(body.events) ? body.events : [],
    nextCursor: typeof body.nextCursor === 'string' && body.nextCursor.trim() ? body.nextCursor : null,
  };
}
