import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type SimulatorSessionRecord = {
  id: string;
  deviceUuid: string;
  token: string;
  status: 'active' | 'revoked' | 'expired' | string;
  expiresAt: string;
  createdAt: string;
  requestedBy?: string | null;
  revokedAt?: string | null;
  revokedBy?: string | null;
} & Record<string, unknown>;

export async function listSimulatorSessions(params?: {
  status?: string;
  limit?: number;
  cursor?: string | null;
}): Promise<{ sessions: SimulatorSessionRecord[]; nextCursor: string | null; count: number }> {
  const query = new URLSearchParams();
  if (params?.status) query.set('status', params.status);
  if (typeof params?.limit === 'number') query.set('limit', String(params.limit));
  if (params?.cursor) query.set('cursor', params.cursor);

  const url = `${API_BASE_URL}/simulator/sessions${query.toString() ? `?${query.toString()}` : ''}`;
  const response = await apiFetch(url);
  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to load simulator sessions';
    throw new Error(message);
  }

  return body as { sessions: SimulatorSessionRecord[]; nextCursor: string | null; count: number };
}

export async function createSimulatorSession(payload: {
  deviceUuid: string;
  expiresInMinutes?: number;
}): Promise<SimulatorSessionRecord> {
  const response = await apiFetch(`${API_BASE_URL}/simulator/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      deviceUuid: payload.deviceUuid,
      expiresInMinutes: payload.expiresInMinutes ?? 60,
    }),
  });

  const body = await readJsonBody<any>(response);
  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to create simulator session';
    throw new Error(message);
  }

  return body as SimulatorSessionRecord;
}

export async function revokeSimulatorSession(sessionId: string): Promise<SimulatorSessionRecord> {
  const trimmed = sessionId.trim();
  if (!trimmed) throw new Error('sessionId is required');

  const response = await apiFetch(`${API_BASE_URL}/simulator/sessions/${encodeURIComponent(trimmed)}`, {
    method: 'DELETE',
  });

  const body = await readJsonBody<any>(response);
  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to revoke simulator session';
    throw new Error(message);
  }

  return body as SimulatorSessionRecord;
}
