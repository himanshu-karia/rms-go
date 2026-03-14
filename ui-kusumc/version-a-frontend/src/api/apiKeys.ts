import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type ApiKeyRecord = {
  id: string;
  name: string;
  prefix: string;
  scopes?: string[];
  last_used_at?: string | null;
  is_active: boolean;
  project_id?: string | null;
  org_id?: string | null;
  created_at?: string;
  [key: string]: unknown;
};

export type ApiKeyCreateResponse = {
  secret: string;
};

function normalizeApiKeys(payload: unknown): ApiKeyRecord[] {
  if (Array.isArray(payload)) {
    return payload as ApiKeyRecord[];
  }
  if (payload && typeof payload === 'object') {
    const maybe = (payload as { keys?: unknown; apiKeys?: unknown }).keys ??
      (payload as { apiKeys?: unknown }).apiKeys;
    if (Array.isArray(maybe)) {
      return maybe as ApiKeyRecord[];
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

export async function fetchApiKeys(): Promise<ApiKeyRecord[]> {
  const response = await apiFetch(`${API_BASE_URL}/admin/apikeys`);
  const body = await parseJsonOrThrow<unknown>(response, 'Unable to load API keys');
  return normalizeApiKeys(body);
}

export async function createApiKey(payload: {
  name: string;
  scopes?: string[];
  project_id?: string | null;
  org_id?: string | null;
}): Promise<ApiKeyCreateResponse> {
  const response = await apiFetch(`${API_BASE_URL}/admin/apikeys`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  return parseJsonOrThrow<ApiKeyCreateResponse>(response, 'Unable to create API key');
}

export async function revokeApiKey(id: string): Promise<void> {
  const trimmed = id.trim();
  if (!trimmed) {
    throw new Error('id required');
  }

  const response = await apiFetch(`${API_BASE_URL}/admin/apikeys/${encodeURIComponent(trimmed)}`, {
    method: 'DELETE',
  });

  if (response.ok) {
    return;
  }

  const body = await readJsonBody<any>(response);
  const message =
    (body as { error?: string; message?: string } | null)?.error ??
    (body as { message?: string } | null)?.message ??
    'Unable to revoke API key';
  throw new Error(message);
}
