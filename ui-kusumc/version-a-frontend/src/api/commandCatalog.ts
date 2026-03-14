import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type CommandCatalogItem = {
  id: string;
  name: string;
  scope: string;
  transport: string;
  protocolId?: string | null;
  modelId?: string | null;
  projectId?: string | null;
  payloadSchema?: Record<string, unknown> | null;
  deviceIds?: string[];
} & Record<string, unknown>;

export async function listCommandCatalogAdmin(params: {
  projectId: string;
  deviceId: string;
}): Promise<CommandCatalogItem[]> {
  const projectId = params.projectId.trim();
  const deviceId = params.deviceId.trim();
  if (!projectId) throw new Error('projectId is required');
  if (!deviceId) throw new Error('deviceId is required');

  const query = new URLSearchParams({ projectId, deviceId });
  const response = await apiFetch(`${API_BASE_URL}/commands/catalog-admin?${query.toString()}`);
  const body = await readJsonBody<unknown>(response);

  if (!response.ok || !body) {
    const message = body?.error ?? body?.message ?? 'Unable to load command catalog';
    throw new Error(message);
  }

  return (Array.isArray(body) ? body : []) as CommandCatalogItem[];
}

export type UpsertCommandCatalogPayload = {
  id?: string;
  name: string;
  scope: string;
  transport: string;
  protocolId?: string | null;
  modelId?: string | null;
  projectId?: string | null;
  payloadSchema: Record<string, unknown>;
  deviceIds?: string[];
};

export async function upsertCommandCatalog(payload: UpsertCommandCatalogPayload): Promise<{ id: string }> {
  const response = await apiFetch(`${API_BASE_URL}/commands/catalog`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<unknown>(response);
  if (!response.ok || !body) {
    const message = body?.error ?? body?.message ?? 'Unable to upsert command catalog item';
    throw new Error(message);
  }

  return body as { id: string };
}

export async function deleteCommandCatalogItem(id: string): Promise<void> {
  const trimmed = id.trim();
  if (!trimmed) throw new Error('id is required');

  const response = await apiFetch(`${API_BASE_URL}/commands/catalog/${encodeURIComponent(trimmed)}`, {
    method: 'DELETE',
  });

  if (response.status === 204) {
    return;
  }

  if (!response.ok) {
    const body = await readJsonBody<any>(response);
    const message = body?.error ?? body?.message ?? 'Unable to delete catalog item';
    throw new Error(message);
  }
}
