import { describe, expect, it, vi, afterEach } from 'vitest';

import { apiFetch } from './http';

const { clearSessionSnapshotMock } = vi.hoisted(() => ({
  clearSessionSnapshotMock: vi.fn(),
}));

vi.mock('./session', () => ({
  getSessionToken: () => null,
  clearSessionSnapshot: clearSessionSnapshotMock,
}));

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  clearSessionSnapshotMock.mockReset();
});

describe('apiFetch', () => {
  function createFetchMock() {
    return vi.fn().mockResolvedValue(
      new Response(null, {
        status: 204,
      }),
    );
  }

  it('forces credentials to include by default', async () => {
    const fetchMock = createFetchMock();
    vi.stubGlobal('fetch', fetchMock);

    await apiFetch('/test');

    expect(fetchMock).toHaveBeenCalledWith(
      '/test',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('ignores attempts to override credentials', async () => {
    const fetchMock = createFetchMock();
    vi.stubGlobal('fetch', fetchMock);

    await apiFetch('/test', {
      credentials: 'omit',
    });

    expect(fetchMock).toHaveBeenCalledWith(
      '/test',
      expect.objectContaining({ credentials: 'include' }),
    );
  });

  it('normalizes query params to snake_case', async () => {
    const fetchMock = createFetchMock();
    vi.stubGlobal('fetch', fetchMock);

    await apiFetch('/api/rules?projectId=proj1&deviceUuid=dev1');

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining('/api/rules?project_id=proj1&device_uuid=dev1'),
      expect.any(Object),
    );
  });

  it('normalizes JSON body keys to snake_case', async () => {
    const fetchMock = createFetchMock();
    vi.stubGlobal('fetch', fetchMock);

    await apiFetch('/api/simulator/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ deviceUuid: 'dev1', expiresInMinutes: 10 }),
    });

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/simulator/sessions',
      expect.objectContaining({
        body: JSON.stringify({ device_uuid: 'dev1', expires_in_minutes: 10 }),
      }),
    );
  });

  it('clears session and emits forced logout event on authenticated 401', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));
    const dispatchEventSpy = vi.spyOn(window, 'dispatchEvent');
    vi.stubGlobal('fetch', fetchMock);

    await apiFetch('/api/protected', {
      headers: {
        Authorization: 'Bearer test-token',
      },
    });

    expect(clearSessionSnapshotMock).toHaveBeenCalledTimes(1);
    expect(dispatchEventSpy).toHaveBeenCalledWith(expect.any(CustomEvent));
  });
});
