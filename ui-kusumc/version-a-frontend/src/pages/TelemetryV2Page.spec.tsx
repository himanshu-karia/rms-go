import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider, type UseMutationResult } from '@tanstack/react-query';

import {
  TelemetryV2Page,
  CommandsTab,
  buildThresholdPayloadFromDraft,
  formatThresholdSummary,
  type DashboardGaugeConfig,
} from './TelemetryV2Page';
import { SessionTimersProvider } from '../session/SessionTimersProvider';
import type { SessionSnapshot } from '../api/session';
import type { IssueDeviceCommandPayload, IssueDeviceCommandResponse } from '../api/deviceCommands';
import { fetchTelemetryThresholds, type TelemetryThresholdEntry } from '../api/telemetry';
import type { DeviceLookupResponse } from '../api/devices';

vi.mock('../auth', () => {
  const login = vi.fn(async () => {
    throw new Error('login not implemented in tests');
  });
  const logout = vi.fn(async () => {});

  return {
    useAuth: () => {
      const now = Date.now();
      const session: SessionSnapshot = {
        token: 'test-token',
        username: 'analyst',
        displayName: 'Telemetry Analyst',
        expiresAt: new Date(now + 60 * 60 * 1000).toISOString(),
        sessionId: 'session-telemetry',
        capabilities: ['telemetry:read'],
      };

      return {
        session,
        user: { username: session.username, displayName: session.displayName },
        isAuthenticated: true,
        login,
        logout,
        refresh: vi.fn(async () => session),
        capabilities: session.capabilities,
        hasCapability: () => true,
      };
    },
  };
});

vi.mock('../api/telemetry', () => ({
  fetchTelemetryHistory: vi.fn(async () => ({ deviceUuid: '', count: 0, records: [] })),
  subscribeToTelemetryStream: vi.fn(() => vi.fn()),
  fetchTelemetryThresholds: vi.fn(),
  upsertTelemetryThresholds: vi.fn(async () => ({ success: true })),
  deleteTelemetryThresholds: vi.fn(async () => ({ success: true })),
}));

vi.mock('../api/devices', () => ({
  fetchDeviceStatus: vi.fn(async () => ({
    device: {
      uuid: 'device-123',
      imei: '123456789012345',
      status: null,
      configurationStatus: null,
      lastTelemetryAt: null,
      lastHeartbeatAt: null,
      connectivityStatus: 'unknown',
      connectivityUpdatedAt: null,
      offlineThresholdMs: 0,
      offlineNotificationChannelCount: 0,
      originImportJobId: null,
      protocolVersion: null,
    },
    telemetry: [],
    recentEvents: [],
    credentialsHistory: [],
    activeCredentials: { local: null, government: null },
    mqttProvisioning: null,
    thresholds: null,
  })),
  lookupDevice: vi.fn(async () => ({
    device: {
      uuid: 'device-123',
      imei: '123456789012345',
      status: null,
      configurationStatus: null,
      connectivityStatus: 'online',
      connectivityUpdatedAt: null,
      lastTelemetryAt: null,
      lastHeartbeatAt: null,
      offlineThresholdMs: 0,
      offlineNotificationChannelCount: 0,
      protocolVersion: null,
    },
  })),
}));

vi.mock('../api/deviceCommands', () => ({
  fetchDeviceCommandHistory: vi.fn(async () => ({
    deviceUuid: 'device-123',
    pageInfo: { hasNextPage: false, cursor: null },
    records: [],
  })),
  issueDeviceCommand: vi.fn(async () => ({
    msgid: 'cmd-1',
    topic: 'device/ondemand',
    simulatorSessionId: null,
  })),
}));

function renderPage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <SessionTimersProvider>
        <QueryClientProvider client={client}>
          <TelemetryV2Page />
        </QueryClientProvider>
      </SessionTimersProvider>
    </MemoryRouter>,
  );
}

function createMutationMock(): UseMutationResult<
  IssueDeviceCommandResponse,
  Error,
  IssueDeviceCommandPayload
> {
  return {
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    reset: vi.fn(),
    status: 'idle',
    data: undefined,
    error: null,
    variables: undefined,
    isError: false,
    isIdle: true,
    isPending: false,
    isPaused: false,
    isSuccess: false,
    failureCount: 0,
    failureReason: null,
    context: undefined,
  } as unknown as UseMutationResult<IssueDeviceCommandResponse, Error, IssueDeviceCommandPayload>;
}

describe('TelemetryV2Page', () => {
  const defaultThresholdResponse = {
    deviceUuid: 'device-123',
    thresholds: {
      effective: [],
      installation: null,
      override: null,
    },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_ENABLE_TELEMETRY_THRESHOLD_OVERRIDES', 'true');
    vi.stubEnv('VITE_ENABLE_TELEMETRY_ANALYTICS', 'true');
    vi.stubEnv('VITE_ENABLE_TELEMETRY_RAW_DIFF', 'true');
    vi.mocked(fetchTelemetryThresholds).mockResolvedValue(defaultThresholdResponse);
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('renders lookup form and tabs in idle state', () => {
    renderPage();

    expect(screen.getByRole('heading', { name: /Telemetry v2/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/Device IMEI or UUID/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Find Device/i })).toBeEnabled();
    expect(screen.getByText(/Select a device to load telemetry insights/i)).toBeInTheDocument();

    expect(screen.queryByRole('button', { name: 'Dashboard' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Graphs' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Data Table' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Commands & Raw' })).not.toBeInTheDocument();
  });

  it('locks threshold overrides when backend data is unavailable', async () => {
    const thresholdMock = vi.mocked(fetchTelemetryThresholds);
    thresholdMock.mockImplementation(() => Promise.reject(new Error('backend offline')));

    renderPage();

    fireEvent.change(screen.getByLabelText(/Device IMEI or UUID/i), {
      target: { value: '123456789012345' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Find Device/i }));

    await screen.findByRole('button', { name: 'Dashboard' });
    await waitFor(() => expect(thresholdMock).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('button', { name: /View thresholds/i }));
    const drawer = await screen.findByRole('dialog', { name: /Device thresholds/i });
    const drawerScope = within(drawer);
    await waitFor(() => {
      expect(drawerScope.queryByText(/Loading thresholds/i)).not.toBeInTheDocument();
    });
    expect(drawerScope.getByText(/Threshold overrides are currently locked/i)).toBeInTheDocument();
    expect(
      drawerScope.getByText(
        /Threshold overrides unavailable. Gauges are using built-in defaults until backend support ships./i,
      ),
    ).toBeInTheDocument();
    const readOnlyMessages = drawerScope.getAllByText(
      /editing disabled because threshold data failed to load from the backend/i,
    );
    expect(readOnlyMessages.length).toBeGreaterThan(0);
    expect(drawerScope.queryAllByLabelText(/^Min$/i, { selector: 'input' })).toHaveLength(0);
    expect(drawerScope.queryByRole('button', { name: /Save changes/i })).not.toBeInTheDocument();
    const closeButtons = drawerScope.getAllByRole('button', { name: /^Close$/i });
    expect(closeButtons.length).toBeGreaterThan(0);
    for (const button of closeButtons) {
      expect(button).toBeEnabled();
    }
  });
});

describe('buildThresholdPayloadFromDraft', () => {
  const baseGaugeConfigs: DashboardGaugeConfig[] = [
    {
      id: 'freq',
      label: 'Pump Frequency',
      payloadKey: 'POPFREQ1',
      unit: 'Hz',
      decimalPlaces: 1,
      suffix: 'data',
    },
  ];

  it('keeps reason even when no thresholds supplied', () => {
    const payload = buildThresholdPayloadFromDraft({}, baseGaugeConfigs, 'Audit only');
    expect(payload).toEqual({ scope: 'override', thresholds: [], reason: 'Audit only' });
  });

  it('parses numeric inputs and attaches metadata', () => {
    const payload = buildThresholdPayloadFromDraft(
      {
        POPFREQ1: {
          min: '35.5',
          max: '55',
          warnLow: '',
          warnHigh: '57.5',
          alertLow: '30',
          alertHigh: '60',
          target: '48',
        },
      },
      baseGaugeConfigs,
      '',
    );

    expect(payload.thresholds).toHaveLength(1);
    expect(payload.thresholds[0]).toMatchObject({
      parameter: 'POPFREQ1',
      min: 35.5,
      max: 55,
      warnHigh: 57.5,
      alertLow: 30,
      alertHigh: 60,
      target: 48,
      unit: 'Hz',
      decimalPlaces: 1,
    });
    expect(payload.scope).toBe('override');
  });
});

describe('formatThresholdSummary', () => {
  it('returns fallback when entry is missing', () => {
    expect(formatThresholdSummary(null)).toBe('—');
  });

  it('prints min/max/warn values with units', () => {
    const entry: TelemetryThresholdEntry = {
      parameter: 'POPFREQ1',
      source: 'override',
      min: 10,
      max: 20,
      warnLow: 12,
      warnHigh: 18,
      alertHigh: 25,
      target: 15,
      unit: 'Hz',
      decimalPlaces: 1,
    };

    const summary = formatThresholdSummary(entry);

    expect(summary).toContain('min 10.0 Hz');
    expect(summary).toContain('max 20.0 Hz');
    expect(summary).toContain('warn high 18.0 Hz');
    expect(summary).toContain('alert high 25.0 Hz');
    expect(summary).toContain('target 15.0 Hz');
  });
});

describe('CommandsTab', () => {
  const device: DeviceLookupResponse['device'] = {
    uuid: 'device-123',
    imei: '123456789012345',
    status: null,
    configurationStatus: null,
    connectivityStatus: 'online',
    connectivityUpdatedAt: null,
    lastTelemetryAt: null,
    lastHeartbeatAt: null,
    offlineThresholdMs: 300_000,
    offlineNotificationChannelCount: 0,
    protocolVersion: null,
  };

  const baseProps = {
    device,
    commandStatusFilter: 'all' as const,
    onCommandStatusFilterChange: vi.fn(),
    commandHistoryRecords: [],
    isHistoryLoading: false,
    isHistoryRefreshing: false,
    historyError: null,
    hasNextPage: false,
    onLoadMore: vi.fn(),
    isFetchingNextPage: false,
    onRefreshHistory: vi.fn(),
    telemetryRecords: [],
    enableRawDiff: false,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('validates JSON payload input before sending', async () => {
    const issueCommandMutation = createMutationMock();
    render(<CommandsTab {...baseProps} issueCommandMutation={issueCommandMutation} />);

    fireEvent.change(screen.getByLabelText(/Command Name/i), { target: { value: 'START' } });
    fireEvent.change(screen.getByLabelText(/Payload \(JSON\)/i), {
      target: { value: '{invalid JSON' },
    });

    fireEvent.click(screen.getByRole('button', { name: /Send Command/i }));

    expect(await screen.findByText(/Command payload must be valid JSON/i)).toBeInTheDocument();
    expect(issueCommandMutation.mutateAsync).not.toHaveBeenCalled();
  });

  it('submits valid command payloads', async () => {
    const issueCommandMutation = createMutationMock();
    (issueCommandMutation.mutateAsync as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      msgid: 'msg-123',
      topic: 'device/ondemand',
      simulatorSessionId: null,
    });

    render(<CommandsTab {...baseProps} issueCommandMutation={issueCommandMutation} />);

    fireEvent.change(screen.getByLabelText(/Command Name/i), { target: { value: 'START' } });
    fireEvent.change(screen.getByLabelText(/Payload \(JSON\)/i), {
      target: { value: '{"flag":true}' },
    });
    fireEvent.change(screen.getByLabelText(/QoS/i), { target: { value: '1' } });

    fireEvent.click(screen.getByRole('button', { name: /Send Command/i }));

    await waitFor(() => {
      expect(issueCommandMutation.mutateAsync).toHaveBeenCalledWith({
        command: { name: 'START', payload: { flag: true } },
        qos: 1,
        timeoutSeconds: 30,
      });
    });

    expect(await screen.findByText(/Command queued/i)).toBeInTheDocument();
  });
});
