import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SessionTimersProvider } from './SessionTimersProvider';
import { usePollingGate } from './usePollingGate';

const { refreshMock, logoutMock, introspectSessionMock, getSessionSnapshotMock } = vi.hoisted(
  () => ({
    refreshMock: vi.fn(async () => ({
      expiresAt: new Date(Date.now() + 90 * 60 * 1_000).toISOString(),
    })),
    logoutMock: vi.fn(),
    introspectSessionMock: vi.fn(async () => ({
      session: { expiresAt: new Date(Date.now() + 90 * 60 * 1_000).toISOString() },
    })),
    getSessionSnapshotMock: vi.fn(() => ({
      expiresAt: new Date(Date.now() + 90 * 60 * 1_000).toISOString(),
    })),
  }),
);

vi.mock('../auth', () => ({
  useAuth: () => ({
    isAuthenticated: true,
    session: { sessionId: 'session-1' },
    refresh: refreshMock,
    logout: logoutMock,
  }),
}));

vi.mock('../api/auth', () => ({
  introspectSession: introspectSessionMock,
}));

vi.mock('../api/session', () => ({
  getSessionSnapshot: getSessionSnapshotMock,
}));

function PollingProbe({ fetchMock }: { fetchMock: () => Promise<string> }) {
  const gate = usePollingGate('integration-test', { isActive: true });

  useEffect(() => {
    if (!gate.enabled) {
      return undefined;
    }

    void fetchMock();

    const interval = setInterval(() => {
      void fetchMock();
    }, 1_000);

    return () => {
      clearInterval(interval);
    };
  }, [fetchMock, gate.enabled]);

  return (
    <div
      data-testid="polling-probe"
      data-enabled={String(gate.enabled)}
      data-idle={String(gate.isIdle)}
      data-remaining={gate.remainingMs?.toString() ?? 'null'}
    >
      <button type="button" onClick={gate.resume}>
        Resume Polling
      </button>
    </div>
  );
}

describe('usePollingGate integration', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true, advanceTimeDelta: 20 });
    vi.setSystemTime(new Date('2025-11-01T00:00:00Z'));
    Object.defineProperty(document, 'hidden', { configurable: true, value: false });
    refreshMock.mockClear();
    logoutMock.mockClear();
    introspectSessionMock.mockClear();
    getSessionSnapshotMock.mockClear();
  });

  afterEach(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: false });
    vi.useRealTimers();
  });

  it('pauses polling after idle threshold and resumes when requested', async () => {
    const fetchMock = vi.fn(async () => 'ok');

    render(
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <SessionTimersProvider>
          <PollingProbe fetchMock={fetchMock} />
        </SessionTimersProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalled();
    });

    act(() => {
      vi.advanceTimersByTime(31 * 60 * 1_000);
    });

    await waitFor(() => {
      const probe = screen.getByTestId('polling-probe');
      expect(probe.dataset.enabled).toBe('false');
      expect(probe.dataset.idle).toBe('true');
    });

    const pausedCallCount = fetchMock.mock.calls.length;

    act(() => {
      vi.advanceTimersByTime(5_000);
    });

    expect(fetchMock.mock.calls.length).toBe(pausedCallCount);

    fireEvent.click(screen.getByRole('button', { name: /resume polling/i }));

    act(() => {
      vi.advanceTimersByTime(1_500);
    });

    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBeGreaterThan(pausedCallCount);
      const probe = screen.getByTestId('polling-probe');
      expect(probe.dataset.enabled).toBe('true');
      expect(probe.dataset.idle).toBe('false');
    });
  });
});
