import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type DnaRecord = {
  projectId: string;
  schemaVersion?: string;
  payload: Record<string, unknown>;
} & Record<string, unknown>;

async function parseJson<T>(response: Response, fallback: string): Promise<T> {
  const body = await readJsonBody<any>(response);
  if (!response.ok) {
    const message = (body as any)?.message ?? (typeof body === 'string' ? body : null) ?? fallback;
    throw new Error(message);
  }
  return body as T;
}

export async function fetchDna(projectId: string): Promise<DnaRecord | null> {
  const trimmed = projectId.trim();
  if (!trimmed) {
    throw new Error('projectId is required');
  }

  const response = await apiFetch(`${API_BASE_URL}/dna/${encodeURIComponent(trimmed)}`);
  if (response.status === 404) {
    return null;
  }
  return parseJson<DnaRecord>(response, 'Unable to load DNA');
}

export async function upsertDna(projectId: string, payload: Record<string, unknown>): Promise<void> {
  const trimmed = projectId.trim();
  if (!trimmed) {
    throw new Error('projectId is required');
  }

  const response = await apiFetch(`${API_BASE_URL}/dna/${encodeURIComponent(trimmed)}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (response.status === 204) {
    return;
  }

  // some errors are plain text; try JSON then fallback to text
  const contentType = response.headers.get('content-type') ?? '';
  if (!response.ok) {
    if (contentType.includes('application/json')) {
      const body = await readJsonBody<any>(response);
      const message = (body as any)?.message ?? 'Unable to save DNA';
      throw new Error(message);
    }
    const text = await response.text().catch(() => null);
    throw new Error(text?.trim() || 'Unable to save DNA');
  }
}
