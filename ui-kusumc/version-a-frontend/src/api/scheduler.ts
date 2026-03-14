import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type SchedulerRecord = {
  id?: string;
  project_id?: string;
  name?: string | null;
  cron_expression?: string | null;
  time?: string | null;
  command?: unknown;
  is_active?: boolean;
  last_run?: string | null;
  created_at?: string;
  [key: string]: unknown;
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

export async function fetchSchedules(): Promise<SchedulerRecord[]> {
  const response = await apiFetch(`${API_BASE_URL}/scheduler/schedules`);
  const body = await parseJsonOrThrow<unknown>(response, 'Unable to load schedules');
  return Array.isArray(body) ? (body as SchedulerRecord[]) : [];
}

export async function createSchedule(payload: {
  project_id: string;
  time?: string;
  cron_expression?: string;
  command: unknown;
}): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/scheduler/schedules`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (response.ok) {
    return;
  }

  const body = await readJsonBody<any>(response);
  const message =
    (body as { message?: string; error?: string } | null)?.message ??
    (body as { error?: string } | null)?.error ??
    'Unable to create schedule';
  throw new Error(message);
}

export async function toggleSchedule(id: string): Promise<void> {
  const trimmed = id.trim();
  if (!trimmed) {
    throw new Error('id required');
  }

  const response = await apiFetch(`${API_BASE_URL}/scheduler/schedules/${encodeURIComponent(trimmed)}/toggle`, {
    method: 'PUT',
  });

  if (response.ok) {
    return;
  }

  const body = await readJsonBody<any>(response);
  const message =
    (body as { message?: string; error?: string } | null)?.message ??
    (body as { error?: string } | null)?.error ??
    'Unable to toggle schedule';
  throw new Error(message);
}
