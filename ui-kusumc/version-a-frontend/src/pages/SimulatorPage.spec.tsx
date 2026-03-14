import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const bootstrapMocks = vi.hoisted(() => ({
  fetchStatesMock: vi.fn(),
  fetchAuthoritiesMock: vi.fn(),
  fetchProjectsMock: vi.fn(),
  fetchAdminStatesMock: vi.fn(),
  fetchAdminStateAuthoritiesMock: vi.fn(),
  fetchAdminProjectsMock: vi.fn(),
  fetchAdminProtocolVersionsMock: vi.fn(),
  fetchAdminVendorsMock: vi.fn(),
  createAdminStateMock: vi.fn(),
  createAdminStateAuthorityMock: vi.fn(),
  createAdminProjectMock: vi.fn(),
  createAdminProtocolVersionMock: vi.fn(),
  createAdminVendorMock: vi.fn(),
}));

vi.mock('../auth', () => ({
  useAuth: () => ({
    session: {
      token: 'token-1',
      username: 'admin',
      displayName: 'Admin User',
      expiresAt: '2099-01-01T00:00:00.000Z',
      sessionId: 'session-1',
      capabilities: ['admin:all'],
    },
    login: vi.fn(async () => {
      throw new Error('not implemented');
    }),
    logout: vi.fn(async () => {}),
    hasCapability: () => true,
  }),
}));

vi.mock('../api/lookups', async () => {
  const actual = await vi.importActual<typeof import('../api/lookups')>('../api/lookups');

  return {
    ...actual,
    fetchStates: bootstrapMocks.fetchStatesMock,
    fetchAuthorities: bootstrapMocks.fetchAuthoritiesMock,
    fetchProjects: bootstrapMocks.fetchProjectsMock,
  };
});

vi.mock('../api/admin', async () => {
  const actual = await vi.importActual<typeof import('../api/admin')>('../api/admin');

  return {
    ...actual,
    fetchAdminStates: bootstrapMocks.fetchAdminStatesMock,
    fetchAdminStateAuthorities: bootstrapMocks.fetchAdminStateAuthoritiesMock,
    fetchAdminProjects: bootstrapMocks.fetchAdminProjectsMock,
    fetchAdminProtocolVersions: bootstrapMocks.fetchAdminProtocolVersionsMock,
    fetchAdminVendors: bootstrapMocks.fetchAdminVendorsMock,
    createAdminState: bootstrapMocks.createAdminStateMock,
    createAdminStateAuthority: bootstrapMocks.createAdminStateAuthorityMock,
    createAdminProject: bootstrapMocks.createAdminProjectMock,
    createAdminProtocolVersion: bootstrapMocks.createAdminProtocolVersionMock,
    createAdminVendor: bootstrapMocks.createAdminVendorMock,
  };
});

import {
  buildDaqPayload,
  buildDataPayload,
  buildHeartbeatPayload,
  buildPumpPayload,
  createSimulatorTelemetryRuntime,
  SimulatorPage,
} from './SimulatorPage';

function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

function renderSimulatorPage(queryClient: QueryClient) {
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <QueryClientProvider client={queryClient}>
        <SimulatorPage />
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

describe('Simulator payload builders', () => {
  const fixedDate = new Date('2025-01-02T06:30:45.000Z');

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(fixedDate);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('builds heartbeat payload with device identifiers and timestamp', () => {
    const payload = buildHeartbeatPayload('123456789012345', 'MH-2025-00001');

    expect(payload.IMEI).toBe('123456789012345');
    expect(payload.ASN).toBe('MH-2025-00001');
    expect(payload.TIMESTAMP).toBe('2025-01-02 06:30:45');
    expect(payload.DATE).toBe('2501');
    expect(payload.VD).toBe('0');
  });

  it('builds pump payload with dynamic counters', () => {
    const runtime = createSimulatorTelemetryRuntime();
    const payload = buildPumpPayload('123456789012345', 'MH-2025-00001', runtime);

    expect(payload.IMEI).toBe('123456789012345');
    expect(payload.ASN).toBe('MH-2025-00001');
    expect(payload.VD).toBe('1');
    expect(payload).toHaveProperty('PTDAYE1');
    expect(payload).toHaveProperty('PTDAYW1');
    expect(payload).toHaveProperty('PFLWRT1');
  });

  it('builds data payload with water and irradiance metrics', () => {
    const runtime = createSimulatorTelemetryRuntime();
    const payload = buildDataPayload('123456789012345', 'MH-2025-00001', runtime);

    expect(payload.PFLWRT1).toBeDefined();
    expect(payload.PTDAYW1).toBeDefined();
    expect(payload.PSOLIR1).toBeDefined();
  });

  it('builds daq payload with analog channels and fault state', () => {
    const runtime = createSimulatorTelemetryRuntime();
    const payload = buildDaqPayload('123456789012345', 'MH-2025-00001', runtime);

    expect(payload.IMEI).toBe('123456789012345');
    expect(payload.ASN).toBe('MH-2025-00001');
    expect(payload.VD).toBe('12');
    expect(payload).toHaveProperty('AI11');
    expect(payload).toHaveProperty('DI11');
    expect(payload).toHaveProperty('PFAULT1');
    expect(payload).toHaveProperty('PRESET1');
  });
});

describe('SimulatorPage bootstrap UI', () => {
  beforeEach(() => {
    const now = '2025-01-02T06:30:45.000Z';
    const state = {
      id: 'state-1',
      name: 'Maharashtra',
      isoCode: 'MH',
      metadata: null,
      createdAt: now,
      updatedAt: now,
    };
    const authority = {
      id: 'authority-1',
      stateId: 'state-1',
      name: 'MSEDCL',
      metadata: null,
      createdAt: now,
      updatedAt: now,
    };
    const project = {
      id: 'project-1',
      authorityId: 'authority-1',
      name: 'PM_KUSUM_SolarPump_RMS',
      metadata: null,
      createdAt: now,
      updatedAt: now,
    };
    const serverVendor = {
      id: 'server-1',
      name: 'Local RMS Server',
      metadata: null,
      createdAt: now,
      updatedAt: now,
    };
    const solarVendor = {
      id: 'solar-1',
      name: 'Generic Solar Pump Vendor',
      metadata: null,
      createdAt: now,
      updatedAt: now,
    };
    const protocol = {
      id: 'protocol-1',
      stateId: 'state-1',
      authorityId: 'authority-1',
      projectId: 'project-1',
      serverVendorId: 'server-1',
      serverVendorName: 'Local RMS Server',
      version: 'MSEDCL-v1',
      name: 'MSEDCL Phase 1',
      metadata: null,
      createdAt: now,
      updatedAt: now,
    };

    bootstrapMocks.fetchAdminStatesMock.mockResolvedValue([state]);
    bootstrapMocks.fetchAdminStateAuthoritiesMock.mockResolvedValue([authority]);
    bootstrapMocks.fetchAdminProjectsMock.mockResolvedValue([project]);
    bootstrapMocks.fetchAdminProtocolVersionsMock.mockResolvedValue([protocol]);
    bootstrapMocks.fetchAdminVendorsMock.mockImplementation(async (collection: string) => {
      if (collection === 'server') {
        return [serverVendor];
      }
      if (collection === 'solarPump') {
        return [solarVendor];
      }
      return [];
    });

    bootstrapMocks.fetchStatesMock.mockResolvedValue([
      {
        id: 'state-1',
        name: 'Maharashtra',
        isoCode: 'MH',
        authorityCount: 1,
      },
    ]);
    bootstrapMocks.fetchAuthoritiesMock.mockResolvedValue([
      {
        id: 'authority-1',
        name: 'MSEDCL',
        stateId: 'state-1',
        projectCount: 1,
      },
    ]);
    bootstrapMocks.fetchProjectsMock.mockResolvedValue([
      {
        id: 'project-1',
        name: 'PM_KUSUM_SolarPump_RMS',
        authorityId: 'authority-1',
        protocolVersions: [
          {
            id: 'protocol-1',
            version: 'MSEDCL-v1',
            serverVendorId: 'server-1',
            serverVendorName: 'Local RMS Server',
          },
        ],
      },
    ]);

    bootstrapMocks.createAdminStateMock.mockReset();
    bootstrapMocks.createAdminStateAuthorityMock.mockReset();
    bootstrapMocks.createAdminProjectMock.mockReset();
    bootstrapMocks.createAdminProtocolVersionMock.mockReset();
    bootstrapMocks.createAdminVendorMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders bootstrap trace steps after default setup runs', async () => {
    const queryClient = createTestQueryClient();
    renderSimulatorPage(queryClient);

    await waitFor(() => {
      expect(screen.getByText('Default Simulator Setup')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Bootstrap Steps')).toBeInTheDocument();
      expect(screen.getByText('states:list')).toBeInTheDocument();
      expect(screen.getByText('lookups:refresh')).toBeInTheDocument();
    });

    expect(bootstrapMocks.createAdminStateMock).not.toHaveBeenCalled();
    expect(bootstrapMocks.createAdminStateAuthorityMock).not.toHaveBeenCalled();
    expect(bootstrapMocks.createAdminProjectMock).not.toHaveBeenCalled();
    expect(bootstrapMocks.createAdminProtocolVersionMock).not.toHaveBeenCalled();
    expect(bootstrapMocks.createAdminVendorMock).not.toHaveBeenCalled();
  });

  it('shows first failing bootstrap step with actionable message', async () => {
    bootstrapMocks.fetchAdminStateAuthoritiesMock.mockRejectedValueOnce(
      new Error('simulated authority lookup failure'),
    );

    const queryClient = createTestQueryClient();
    renderSimulatorPage(queryClient);

    await waitFor(() => {
      expect(screen.getByText('Bootstrap Steps')).toBeInTheDocument();
      expect(screen.getByText('authorities:list')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText(/Step authorities:list failed:/i)).toBeInTheDocument();
      expect(screen.getByText(/simulated authority lookup failure/i)).toBeInTheDocument();
    });
  });
});
