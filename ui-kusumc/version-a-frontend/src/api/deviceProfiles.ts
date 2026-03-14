import { API_BASE_URL } from './config';
import { apiFetch } from './http';

export type DeviceProfileRecord = Record<string, unknown>;

export async function fetchDeviceProfiles(): Promise<DeviceProfileRecord[]> {
  const response = await apiFetch(`${API_BASE_URL}/config/profiles`);
  const body = await response.json().catch(() => null);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to load device profiles';
    throw new Error(message);
  }

  // backend returns arbitrary JSON (config store)
  if (Array.isArray(body)) {
    return body as DeviceProfileRecord[];
  }
  if (Array.isArray((body as any).profiles)) {
    return (body as any).profiles as DeviceProfileRecord[];
  }

  return [body as DeviceProfileRecord];
}

export async function createDeviceProfile(payload: Record<string, unknown>): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/config/profiles`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (response.status === 201) {
    return;
  }

  if (!response.ok) {
    const contentType = response.headers.get('content-type') ?? '';
    if (contentType.includes('application/json')) {
      const body = await response.json().catch(() => null);
      const message = body?.message ?? 'Unable to create device profile';
      throw new Error(message);
    }
    const text = await response.text().catch(() => null);
    throw new Error(text?.trim() || 'Unable to create device profile');
  }
}
