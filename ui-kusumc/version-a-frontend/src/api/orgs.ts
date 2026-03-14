import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type OrgRecord = {
  id: string;
  name: string;
  type: string;
  path?: string | null;
  parent_id?: string | null;
  parentId?: string | null;
  metadata?: Record<string, unknown> | null;
  [key: string]: unknown;
};

function normalizeOrgs(payload: unknown): OrgRecord[] {
  if (Array.isArray(payload)) {
    return payload as OrgRecord[];
  }
  if (payload && typeof payload === 'object') {
    const maybe = (payload as { orgs?: unknown }).orgs;
    if (Array.isArray(maybe)) {
      return maybe as OrgRecord[];
    }
  }
  return [];
}

async function parseJsonOrThrow<T>(response: Response, fallback: string): Promise<T> {
  const body = await readJsonBody<any>(response);
  if (!response.ok) {
    const message =
      (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      fallback;
    throw new Error(message);
  }
  return body as T;
}

export async function fetchOrgs(): Promise<OrgRecord[]> {
  const response = await apiFetch(`${API_BASE_URL}/orgs`);
  const body = await parseJsonOrThrow<unknown>(response, 'Unable to load orgs');
  return normalizeOrgs(body);
}

export async function createOrg(payload: {
  name: string;
  type: string;
  path?: string;
  parent_id?: string | null;
  metadata?: Record<string, unknown> | null;
}): Promise<OrgRecord> {
  const response = await apiFetch(`${API_BASE_URL}/orgs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  return parseJsonOrThrow<OrgRecord>(response, 'Unable to create org');
}

export async function updateOrg(params: {
  id: string;
  payload: {
    name: string;
    type: string;
    path?: string;
    parent_id?: string | null;
    metadata?: Record<string, unknown> | null;
  };
}): Promise<OrgRecord> {
  const id = params.id.trim();
  if (!id) {
    throw new Error('id required');
  }

  const response = await apiFetch(`${API_BASE_URL}/orgs/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params.payload),
  });

  return parseJsonOrThrow<OrgRecord>(response, 'Unable to update org');
}
