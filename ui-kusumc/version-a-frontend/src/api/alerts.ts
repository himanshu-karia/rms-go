import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type AlertRecord = {
  id?: string;
  status?: string;
  message?: string;
  createdAt?: string;
  ackedAt?: string | null;
  acknowledgedAt?: string | null;
  projectId?: string;
  deviceId?: string;
  [key: string]: unknown;
};

function normalizeAlerts(payload: unknown): AlertRecord[] {
  if (Array.isArray(payload)) {
    return payload as AlertRecord[];
  }

  if (payload && typeof payload === 'object') {
    const maybeAlerts = (payload as { alerts?: unknown }).alerts;
    if (Array.isArray(maybeAlerts)) {
      return maybeAlerts as AlertRecord[];
    }
  }

  return [];
}

export async function fetchAlerts(params: {
  projectId?: string;
  status?: string;
}): Promise<AlertRecord[]> {
  const query = new URLSearchParams();
  if (params.projectId) {
    query.set('projectId', params.projectId);
  }
  if (params.status) {
    query.set('status', params.status);
  }

  const url = query.toString() ? `${API_BASE_URL}/alerts?${query.toString()}` : `${API_BASE_URL}/alerts`;
  const response = await apiFetch(url);
  const body = await readJsonBody<any>(response);

  if (!response.ok) {
    const message = (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      'Unable to load alerts';
    throw new Error(message);
  }

  return normalizeAlerts(body);
}

export async function ackAlert(alertId: string): Promise<void> {
  const trimmed = alertId.trim();
  if (!trimmed) {
    throw new Error('alertId required');
  }

  const response = await apiFetch(`${API_BASE_URL}/alerts/${encodeURIComponent(trimmed)}/ack`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  });

  if (!response.ok) {
    const body = await readJsonBody<any>(response);
    const message = (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      'Unable to acknowledge alert';
    throw new Error(message);
  }
}
