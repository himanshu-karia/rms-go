import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SessionTimersProvider, useSessionTimers } from './SessionTimersProvider';
import { useMqttBudget } from './useMqttBudget';
import { usePollingGate } from './usePollingGate';

const { refreshMock, logoutMock, introspectSessionMock, getSessionSnapshotMock } = vi.hoisted(
  () => ({
    refreshMock: vi.fn(),
    logoutMock: vi.fn(),
    introspectSessionMock: vi.fn(),
    getSessionSnapshotMock: vi.fn(),
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

function SessionGenerationProbe() {
  const { sessionGeneration } = useSessionTimers();
  const { summary, pauseForHidden } = useMqttBudget('probe');

  useEffect(() => {
    pauseForHidden();
  }, [pauseForHidden]);

  return (
    <div
      data-testid="session-generation-probe"
      data-generation={String(sessionGeneration)}
      data-paused={String(summary.isPaused)}
      data-reason={summary.reason ?? 'none'}
    />
  );
}

describe('SessionTimersProvider', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true, advanceTimeDelta: 20 });
    const now = new Date('2025-11-01T00:00:00Z');
    vi.setSystemTime(now);

    refreshMock.mockReset();
    logoutMock.mockReset();
    introspectSessionMock.mockReset();
    getSessionSnapshotMock.mockReset();

    Object.defineProperty(document, 'hidden', {
      configurable: true,
      value: false,
    });
  });

  afterEach(() => {
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      value: false,
    });
    vi.useRealTimers();
  });

  it('resumes MQTT budgets and increments session generation after renewal', async () => {
    const soonExpiry = new Date(Date.now() + 2 * 60 * 1000).toISOString();
    getSessionSnapshotMock.mockReturnValue({ expiresAt: soonExpiry });
    introspectSessionMock.mockResolvedValue({ session: { expiresAt: soonExpiry } });
    refreshMock.mockResolvedValue({
      expiresAt: new Date(Date.now() + 90 * 60 * 1000).toISOString(),
    });

    render(
      <MemoryRouter>
        <SessionTimersProvider>
          <>
            <div>child</div>
            <SessionGenerationProbe />
          </>
        </SessionTimersProvider>
      </MemoryRouter>,
    );

    await screen.findByText(/Session will expire in under 3 minutes/i);

    await waitFor(() => {
      const sessionProbe = screen.getByTestId('session-generation-probe');
      expect(sessionProbe.dataset.paused).toBe('true');
      expect(sessionProbe.dataset.reason).toBe('hidden');
      expect(sessionProbe.dataset.generation).toBe('0');
    });

    fireEvent.click(screen.getByRole('button', { name: /Extend session/i }));

    await waitFor(() => expect(refreshMock).toHaveBeenCalledTimes(1));

    await waitFor(() => {
      expect(screen.queryByText(/Session will expire in under 3 minutes/i)).not.toBeInTheDocument();
      const sessionProbe = screen.getByTestId('session-generation-probe');
      expect(sessionProbe.dataset.generation).toBe('1');
      expect(sessionProbe.dataset.paused).toBe('false');
      expect(sessionProbe.dataset.reason).toBe('none');
    });
  });

  it('disables polling and shows a hidden-tab toast when the tab is hidden', async () => {
    const futureExpiry = new Date(Date.now() + 90 * 60 * 1000).toISOString();
    getSessionSnapshotMock.mockReturnValue({ expiresAt: futureExpiry });
    introspectSessionMock.mockResolvedValue({ session: { expiresAt: futureExpiry } });
    refreshMock.mockResolvedValue({ expiresAt: futureExpiry });

    function Probe() {
      const gate = usePollingGate('test');
      const { recordPacket } = useMqttBudget('probe');

      useEffect(() => {
        recordPacket();
      }, [recordPacket]);

      return (
        <div
          data-testid="polling-probe"
          data-enabled={String(gate.enabled)}
          data-idle={String(gate.isIdle)}
        />
      );
    }

    render(
      <MemoryRouter>
        <SessionTimersProvider>
          <Probe />
        </SessionTimersProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('polling-probe').dataset.enabled).toBe('true');
    });

    act(() => {
      Object.defineProperty(document, 'hidden', { configurable: true, value: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });

    await waitFor(() => {
      const pollingProbe = screen.getByTestId('polling-probe');
      expect(pollingProbe.dataset.enabled).toBe('false');
      expect(pollingProbe.dataset.idle).toBe('true');
    });

    await screen.findByText(/Live streams paused/i);
  });

  it('shows a resume toast when live data pauses from inactivity', async () => {
    const futureExpiry = new Date(Date.now() + 90 * 60 * 1000).toISOString();
    getSessionSnapshotMock.mockReturnValue({ expiresAt: futureExpiry });
    introspectSessionMock.mockResolvedValue({ session: { expiresAt: futureExpiry } });
    refreshMock.mockResolvedValue({ expiresAt: futureExpiry });

    function Probe() {
      const gate = usePollingGate('idle-test');
      return (
        <div
          data-testid="polling-idle-probe"
          data-enabled={String(gate.enabled)}
          data-idle={String(gate.isIdle)}
        />
      );
    }

    render(
      <MemoryRouter>
        <SessionTimersProvider>
          <Probe />
        </SessionTimersProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('polling-idle-probe').dataset.enabled).toBe('true');
    });

    act(() => {
      vi.advanceTimersByTime(30 * 60 * 1000 + 1_000);
    });

    await waitFor(() => {
      const probe = screen.getByTestId('polling-idle-probe');
      expect(probe.dataset.enabled).toBe('false');
      expect(probe.dataset.idle).toBe('true');
    });

    await screen.findByText(/Session expired. Resume to continue./i);
    await screen.findByText(/HTTP polling was paused after 30 minutes of inactivity./i);

    fireEvent.click(screen.getByRole('button', { name: /Resume live data/i }));

    await waitFor(() => {
      const probe = screen.getByTestId('polling-idle-probe');
      expect(probe.dataset.enabled).toBe('true');
      expect(probe.dataset.idle).toBe('false');
    });

    await waitFor(() => {
      expect(screen.queryByText(/Session expired. Resume to continue./i)).not.toBeInTheDocument();
    });
  });
});
