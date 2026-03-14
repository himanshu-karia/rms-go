import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type MockInstance,
} from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import type { DeviceStatusResponse } from '../api/devices';
import { DeviceConfigurationPage } from './DeviceConfigurationPage';
import type { VfdModel } from '../api/vfd';
import type { CapabilityKey } from '../api/capabilities';
import { SessionTimersProvider } from '../session/SessionTimersProvider';
import type { SessionSnapshot } from '../api/session';

const mocks = vi.hoisted(() => ({
  acknowledgeDeviceConfiguration: vi.fn(),
  fetchPendingDeviceConfiguration: vi.fn(),
  importDeviceConfigurationsCsv: vi.fn(),
  queueDeviceConfiguration: vi.fn(),
  fetchVfdModels: vi.fn(),
  fetchDeviceStatus: vi.fn(),
  rotateDeviceCredentials: vi.fn(),
  retryDeviceMqttProvisioning: vi.fn(),
  revokeDeviceCredentials: vi.fn(),
  resyncDeviceMqttProvisioning: vi.fn(),
  upsertDeviceGovernmentCredentials: vi.fn(),
}));

const authEnv = vi.hoisted(() => {
  const defaults: CapabilityKey[] = [
    'admin:all',
    'devices:read',
    'devices:write',
    'devices:credentials',
    'devices:commands',
    'catalog:protocols',
    'catalog:drives',
  ];

  const controller = {
    defaultCapabilities: [...defaults],
    capabilities: new Set<CapabilityKey>(defaults),
    setCapabilities(next: CapabilityKey[]) {
      controller.capabilities = new Set(next);
    },
    reset() {
      controller.capabilities = new Set(controller.defaultCapabilities);
    },
  };

  return { defaults, controller } as const;
});

const DEFAULT_CAPABILITIES = authEnv.defaults;
const authController = authEnv.controller;

vi.mock('../api/deviceConfigurations', async () => {
  const actual = await vi.importActual<typeof import('../api/deviceConfigurations')>(
    '../api/deviceConfigurations',
  );
  return {
    ...actual,
    acknowledgeDeviceConfiguration: mocks.acknowledgeDeviceConfiguration,
    fetchPendingDeviceConfiguration: mocks.fetchPendingDeviceConfiguration,
    importDeviceConfigurationsCsv: mocks.importDeviceConfigurationsCsv,
    queueDeviceConfiguration: mocks.queueDeviceConfiguration,
  };
});

vi.mock('../api/vfd', async () => {
  const actual = await vi.importActual<typeof import('../api/vfd')>('../api/vfd');
  return {
    ...actual,
    fetchVfdModels: mocks.fetchVfdModels,
  };
});

vi.mock('../api/devices', async () => {
  const actual = await vi.importActual<typeof import('../api/devices')>('../api/devices');
  return {
    ...actual,
    fetchDeviceStatus: mocks.fetchDeviceStatus,
    rotateDeviceCredentials: mocks.rotateDeviceCredentials,
    retryDeviceMqttProvisioning: mocks.retryDeviceMqttProvisioning,
    revokeDeviceCredentials: mocks.revokeDeviceCredentials,
    resyncDeviceMqttProvisioning: mocks.resyncDeviceMqttProvisioning,
    upsertDeviceGovernmentCredentials: mocks.upsertDeviceGovernmentCredentials,
  };
});

vi.mock('../auth', () => {
  const login = vi.fn(async () => {
    throw new Error('login not implemented in tests');
  });
  const logout = vi.fn(async () => {});

  return {
    useAuth: () => {
      const capabilities = [...authController.capabilities];
      const now = Date.now();
      const session: SessionSnapshot = {
        token: 'test-token',
        username: 'admin',
        displayName: 'Admin',
        expiresAt: new Date(now + 60 * 60 * 1000).toISOString(),
        sessionId: 'session-test',
        capabilities,
      };
      return {
        session,
        user: { username: 'admin', displayName: 'Admin' },
        isAuthenticated: true,
        login,
        logout,
        refresh: vi.fn(async () => session),
        capabilities,
        hasCapability: (required: CapabilityKey | CapabilityKey[]) => {
          const list = Array.isArray(required) ? required : [required];
          return list.every((capability) => authController.capabilities.has(capability));
        },
      };
    },
  };
});

const {
  acknowledgeDeviceConfiguration,
  fetchPendingDeviceConfiguration,
  importDeviceConfigurationsCsv,
  queueDeviceConfiguration,
  fetchVfdModels,
  fetchDeviceStatus,
  rotateDeviceCredentials,
  retryDeviceMqttProvisioning,
  revokeDeviceCredentials,
  resyncDeviceMqttProvisioning,
  upsertDeviceGovernmentCredentials,
} = mocks;

let queryClient: QueryClient;
let consoleErrorSpy: MockInstance | undefined;

function renderPage() {
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <SessionTimersProvider>
        <QueryClientProvider client={queryClient}>
          <DeviceConfigurationPage />
        </QueryClientProvider>
      </SessionTimersProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });

  authController.reset();

  acknowledgeDeviceConfiguration.mockReset();
  fetchPendingDeviceConfiguration.mockReset();
  importDeviceConfigurationsCsv.mockReset();
  queueDeviceConfiguration.mockReset();
  fetchVfdModels.mockReset();
  fetchDeviceStatus.mockReset();
  rotateDeviceCredentials.mockReset();
  retryDeviceMqttProvisioning.mockReset();
  revokeDeviceCredentials.mockReset();
  resyncDeviceMqttProvisioning.mockReset();
  upsertDeviceGovernmentCredentials.mockReset();

  fetchVfdModels.mockResolvedValue([]);
  fetchDeviceStatus.mockResolvedValue({
    device: {
      uuid: 'uuid-1',
      imei: '123456789012345',
      status: 'active',
      configurationStatus: 'pending',
      lastTelemetryAt: null,
      lastHeartbeatAt: null,
      connectivityStatus: 'unknown',
      connectivityUpdatedAt: null,
      offlineThresholdMs: 86_400_000,
      offlineNotificationChannelCount: 0,
      originImportJobId: null,
      protocolVersion: null,
    },
    telemetry: [],
    recentEvents: [],
    credentialsHistory: [],
    activeCredentials: {
      local: null,
      government: null,
    },
    mqttProvisioning: null,
  });
  retryDeviceMqttProvisioning.mockResolvedValue({
    device: { id: 'uuid-1', imei: '123456789012345' },
    mqttProvisioning: {
      status: 'pending',
      jobId: 'job-1',
      attemptCount: 0,
      maxAttempts: 5,
      baseRetryDelayMs: 30000,
      lastAttemptAt: null,
      nextAttemptAt: new Date().toISOString(),
      lastError: null,
    },
    attemptsReset: false,
  });
  revokeDeviceCredentials.mockResolvedValue({
    device: { id: 'uuid-1', imei: '123456789012345' },
    revokedCount: 1,
    lifecycleTransitions: [{ type: 'local', from: 'active', to: 'revoked', count: 1 }],
  });
  resyncDeviceMqttProvisioning.mockResolvedValue({
    device: { id: 'uuid-1', imei: '123456789012345' },
    credentialHistoryId: 'cred-1',
    previousJobId: 'job-1',
    resyncCount: 1,
    mqttProvisioning: {
      status: 'pending',
      jobId: 'job-2',
      attemptCount: 0,
      maxAttempts: 5,
      baseRetryDelayMs: 30000,
      lastAttemptAt: null,
      nextAttemptAt: new Date().toISOString(),
      lastError: null,
    },
    scope: {
      stateId: null,
      authorityId: null,
      projectId: null,
    },
  });
});

afterEach(() => {
  queryClient.clear();
});

beforeAll(() => {
  const originalError = console.error;
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((...args) => {
    const message = typeof args[0] === 'string' ? args[0] : '';
    if (message.includes('not wrapped in act(')) {
      return;
    }
    originalError(...args);
  });
});

afterAll(() => {
  consoleErrorSpy?.mockRestore();
});

describe('DeviceConfigurationPage credential rotation', () => {
  it('shows validation error when device UUID is missing', async () => {
    await renderPage();

    const section = screen.getByRole('region', { name: 'Regenerate Device Credentials' });
    expect(section).toBeInTheDocument();

    const rotateButton = within(section).getByRole('button', {
      name: 'Rotate Credentials',
    });

    const deviceUuidInput = within(section).getByLabelText('Device UUID');
    const user = userEvent.setup();
    await user.type(deviceUuidInput, ' ');

    await user.click(rotateButton);

    const errorBanner = await within(section).findByText(
      /Provide a device UUID before rotating credentials/i,
    );
    expect(errorBanner).toBeTruthy();
    expect(rotateDeviceCredentials).not.toHaveBeenCalled();
  });

  it('rotates credentials, resets the form, and refreshes device status', async () => {
    const rotationResponse = {
      device: { id: 'device-uuid-1', imei: '123456789012345' },
      credentials: {
        clientId: 'client-123',
        username: 'user-123',
        password: 'secret',
        endpoints: [
          {
            protocol: 'mqtt' as const,
            host: 'localhost',
            port: 1886,
            url: 'mqtt://localhost:1886',
          },
          {
            protocol: 'mqtts' as const,
            host: 'localhost',
            port: 8883,
            url: 'mqtts://localhost:8883',
          },
        ],
        topics: {
          publish: ['123456789012345/heartbeat'],
          subscribe: ['123456789012345/ondemand'],
        },
        dynsec: { applied: true },
      },
    };

    rotateDeviceCredentials.mockResolvedValue(rotationResponse);

    await renderPage();

    const section = screen.getByRole('region', { name: 'Regenerate Device Credentials' });

    const user = userEvent.setup();

    const deviceUuidInput = within(section).getByLabelText('Device UUID');
    const reasonInput = within(section).getByLabelText('Rotation Reason (optional)');
    const issuedByInput = within(section).getByLabelText('Issued By (optional)');
    const rotateButton = within(section).getByRole('button', { name: 'Rotate Credentials' });

    await user.type(deviceUuidInput, 'device-uuid-1');
    await user.type(reasonInput, 'scheduled-rotation');
    await user.type(issuedByInput, 'operator-1');

    fetchDeviceStatus.mockClear();

    await user.click(rotateButton);

    await waitFor(() => {
      expect(rotateDeviceCredentials).toHaveBeenCalledWith('device-uuid-1', {
        reason: 'scheduled-rotation',
        issuedBy: 'operator-1',
      });
    });

    const successHeading = await within(section).findByText('Credentials regenerated');
    expect(successHeading).toBeTruthy();

    expect((deviceUuidInput as HTMLInputElement).value).toBe('');
    expect((reasonInput as HTMLInputElement).value).toBe('');
    expect((issuedByInput as HTMLInputElement).value).toBe('');

    await waitFor(() => {
      expect(fetchDeviceStatus).toHaveBeenCalled();
    });
  });

  it('renders credential history records returned from the device status endpoint', async () => {
    const historyResponse: DeviceStatusResponse = {
      device: {
        uuid: 'device-uuid-1',
        imei: '123456789012345',
        status: 'active',
        configurationStatus: 'configured',
        lastTelemetryAt: '2025-05-01T10:00:00.000Z',
        lastHeartbeatAt: '2025-05-01T09:55:00.000Z',
        connectivityStatus: 'online',
        connectivityUpdatedAt: '2025-05-01T09:55:30.000Z',
        offlineThresholdMs: 86_400_000,
        offlineNotificationChannelCount: 1,
        originImportJobId: 'device-import-1',
        protocolVersion: null,
      },
      telemetry: [],
      recentEvents: [],
      credentialsHistory: [
        {
          type: 'local',
          clientId: 'client-local',
          username: 'user-local',
          password: 'local-pass',
          endpoints: [],
          topics: { publish: [], subscribe: [] },
          validFrom: '2025-05-01T10:00:00.000Z',
          validTo: null,
          rotationReason: 'scheduled rotation',
          mqttAccessApplied: true,
          issuedBy: '507f1f77bcf86cd799439011',
          mqttAccess: {
            applied: true,
            jobId: 'job-1',
            lastAttemptAt: '2025-05-01T10:00:00.000Z',
            lastSuccessAt: '2025-05-01T10:00:00.000Z',
            lastFailureAt: null,
            operations: [],
            error: null,
          },
          lifecycle: 'active',
          lifecycleHistory: [
            {
              state: 'pending',
              occurredAt: '2025-04-30T09:00:00.000Z',
              reason: null,
              actorId: null,
              source: 'system',
            },
            {
              state: 'active',
              occurredAt: '2025-05-01T10:00:00.000Z',
              reason: 'scheduled rotation',
              actorId: '507f1f77bcf86cd799439011',
              source: 'user',
            },
          ],
          originImportJobId: 'import-job-1',
          protocolSelector: {
            stateId: 'state-1',
            stateAuthorityId: 'authority-1',
            projectId: 'project-1',
            serverVendorId: 'server-1',
            protocolVersionId: 'proto-1',
            version: '1.0.0',
          },
        },
        {
          type: 'government',
          clientId: 'client-gov',
          username: 'user-gov',
          password: 'gov-pass',
          endpoints: [],
          topics: { publish: [], subscribe: [] },
          validFrom: '2025-04-01T10:00:00.000Z',
          validTo: '2025-05-01T09:59:00.000Z',
          rotationReason: null,
          mqttAccessApplied: null,
          issuedBy: null,
          mqttAccess: null,
          lifecycle: 'revoked',
          lifecycleHistory: [
            {
              state: 'active',
              occurredAt: '2025-04-01T10:00:00.000Z',
              reason: null,
              actorId: null,
              source: 'system',
            },
            {
              state: 'revoked',
              occurredAt: '2025-05-01T09:59:00.000Z',
              reason: 'Rotated to local bundle',
              actorId: null,
              source: 'system',
            },
          ],
          originImportJobId: null,
          protocolSelector: null,
        },
      ],
      activeCredentials: {
        local: null,
        government: null,
      },
      mqttProvisioning: null,
      thresholds: {
        effective: {},
        installation: null,
        override: null,
      },
    };

    fetchDeviceStatus.mockResolvedValue(historyResponse);

    await renderPage();

    const section = screen.getByRole('region', { name: 'Regenerate Device Credentials' });

    const user = userEvent.setup();
    const deviceUuidInput = within(section).getByLabelText('Device UUID');

    await user.type(deviceUuidInput, 'device-uuid-1');

    await waitFor(() => {
      expect(fetchDeviceStatus).toHaveBeenCalledWith('device-uuid-1');
    });

    const table = await screen.findByRole('table');
    const rows = within(table).getAllByRole('row');
    expect(rows).toHaveLength(3);

    const firstDataRow = rows[1];
    const secondDataRow = rows[2];

    expect(firstDataRow.textContent ?? '').toContain('local');
    expect(firstDataRow.textContent ?? '').toContain('client-local');
    expect(firstDataRow.textContent ?? '').toContain('scheduled rotation');
    expect(firstDataRow.textContent ?? '').toContain('Active');
    expect(firstDataRow.textContent ?? '').toContain('Applied');
    expect(within(firstDataRow).getByText('import-job-1')).toBeTruthy();

    expect(secondDataRow.textContent ?? '').toContain('government');
    expect(secondDataRow.textContent ?? '').toContain('client-gov');
    expect(secondDataRow.textContent ?? '').not.toContain('Active');
    expect(secondDataRow.textContent ?? '').toContain('—');
  });

  it('shows active government credentials and protocol defaults when available', async () => {
    const statusResponse: DeviceStatusResponse = {
      device: {
        uuid: 'device-uuid-1',
        imei: '123456789012345',
        status: 'active',
        configurationStatus: 'configured',
        lastTelemetryAt: null,
        lastHeartbeatAt: null,
        connectivityStatus: 'online',
        connectivityUpdatedAt: '2025-05-02T10:00:00.000Z',
        offlineThresholdMs: 43_200_000,
        offlineNotificationChannelCount: 1,
        originImportJobId: 'device-import-42',
        protocolVersion: {
          id: 'protocol-1',
          version: '1.0.0',
          name: 'MSEDCL Base',
          metadata: {
            governmentCredentials: {
              endpointDefaults: [
                {
                  protocol: 'mqtts',
                  host: 'gov.example',
                  port: 8883,
                  url: 'mqtts://gov.example:8883',
                },
              ],
              topics: {
                publish: ['123456789012345/heartbeat'],
                subscribe: ['123456789012345/ondemand'],
              },
            },
          },
        },
      },
      telemetry: [],
      recentEvents: [],
      credentialsHistory: [],
      activeCredentials: {
        local: null,
        government: {
          type: 'government',
          clientId: 'gov-client',
          username: 'gov-user',
          password: 'gov-pass',
          endpoints: [
            {
              protocol: 'mqtt',
              host: 'gov.local',
              port: 1886,
              url: 'mqtt://gov.local:1886',
            },
          ],
          topics: {
            publish: ['123456789012345/heartbeat'],
            subscribe: ['123456789012345/ondemand'],
          },
          validFrom: '2025-05-02T09:00:00.000Z',
          issuedBy: '507f1f77bcf86cd799439022',
          lifecycle: 'active',
          originImportJobId: 'gov-job-99',
          protocolSelector: {
            stateId: 'state-1',
            stateAuthorityId: 'authority-1',
            projectId: 'project-1',
            serverVendorId: 'server-1',
            protocolVersionId: 'proto-1',
            version: '1.0.0',
          },
        },
      },
      mqttProvisioning: null,
      thresholds: {
        effective: {},
        installation: null,
        override: null,
      },
    };

    fetchDeviceStatus.mockResolvedValue(statusResponse);

    await renderPage();

    const section = screen.getByRole('region', { name: 'Queue Device Configuration' });

    const user = userEvent.setup();
    const deviceUuidInput = within(section).getByLabelText('Device UUID');

    await user.type(deviceUuidInput, 'device-uuid-1');

    await waitFor(() => {
      expect(fetchDeviceStatus).toHaveBeenCalledWith('device-uuid-1');
    });

    const governmentHeading = await within(section).findByText('Government Credentials');
    expect(governmentHeading).toBeTruthy();

    expect(within(section).getByText('Active Bundle')).toBeTruthy();
    expect(within(section).getByText('gov-client')).toBeTruthy();
    expect(within(section).getByText('gov-user')).toBeTruthy();
    expect(within(section).getByText('gov-pass')).toBeTruthy();
    expect(within(section).getByText('Protocol Defaults')).toBeTruthy();
    const mqttEndpointMatches = within(section).getAllByText(/mqtt:\/\/gov\.local:1886/);
    expect(mqttEndpointMatches.length).toBeGreaterThan(0);
  expect(within(section).getByText(/mqtts:\/\/gov\.example:8883/)).toBeTruthy();

    const activeImportLink = within(section).getByRole('link', {
      name: /device-import-42/i,
    });
    expect(activeImportLink).toHaveAttribute('href', '/devices/import/jobs?jobId=device-import-42');

    const historyImportLink = within(section).getByRole('link', {
      name: /gov-job-99/i,
    });
    expect(historyImportLink).toHaveAttribute('href', '/devices/import/jobs?jobId=gov-job-99');
  });
});

describe('DeviceConfigurationPage credential capability guard', () => {
  it('disables credential management controls without super-admin privileges', () => {
    authController.setCapabilities(
      DEFAULT_CAPABILITIES.filter(
        (capability) => capability !== 'admin:all' && capability !== 'devices:credentials',
      ),
    );

    renderPage();

    const guardMessage =
      'Device credential management requires the devices:credentials capability or super-admin access.';

    const rotationSection = screen.getByRole('region', { name: 'Regenerate Device Credentials' });
    expect(within(rotationSection).getByText(guardMessage)).toBeInTheDocument();
    const rotateButton = within(rotationSection).getByRole('button', {
      name: 'Rotate Credentials',
    });
    expect(rotateButton).toBeDisabled();

    const revokeSection = screen.getByRole('region', { name: 'Revoke Device Credentials' });
    expect(within(revokeSection).getByText(guardMessage)).toBeInTheDocument();
    const revokeButton = within(revokeSection).getByRole('button', { name: 'Revoke Credentials' });
    expect(revokeButton).toBeDisabled();

    const governmentSection = screen.getByRole('region', { name: 'Manage Government Credentials' });
    expect(within(governmentSection).getByText(guardMessage)).toBeInTheDocument();
    const saveButton = within(governmentSection).getByRole('button', {
      name: 'Save Government Credentials',
    });
    expect(saveButton).toBeDisabled();

    expect(rotateDeviceCredentials).not.toHaveBeenCalled();
    expect(revokeDeviceCredentials).not.toHaveBeenCalled();
    expect(upsertDeviceGovernmentCredentials).not.toHaveBeenCalled();
  });
});

describe('DeviceConfigurationPage credential revocation', () => {
  it('requires a device UUID before revoking credentials', async () => {
    await renderPage();

    const section = screen.getByRole('region', { name: 'Revoke Device Credentials' });
    const revokeButton = within(section).getByRole('button', { name: 'Revoke Credentials' });
    const user = userEvent.setup();

    await user.click(revokeButton);

    expect(
      await within(section).findByText('Provide a device UUID before revoking credentials.'),
    ).toBeInTheDocument();
    expect(revokeDeviceCredentials).not.toHaveBeenCalled();
  });

  it('revokes credentials and shows lifecycle transitions', async () => {
    await renderPage();

    const section = screen.getByRole('region', { name: 'Revoke Device Credentials' });
    const deviceUuidInput = within(section).getByLabelText('Device UUID');
    const reasonField = within(section).getByLabelText('Revocation Reason (optional)');
    const revokeButton = within(section).getByRole('button', { name: 'Revoke Credentials' });
    const user = userEvent.setup();

    await user.type(deviceUuidInput, 'uuid-1');
    await user.type(reasonField, 'Compromised credential bundle');
    await user.click(revokeButton);

    await waitFor(() => {
      expect(revokeDeviceCredentials).toHaveBeenCalledWith(
        'uuid-1',
        expect.objectContaining({
          type: 'local',
          reason: 'Compromised credential bundle',
        }),
      );
    });

    expect(await within(section).findByText(/Revocation complete/i)).toBeInTheDocument();
    expect(within(section).getByText(/Local: Active -> Revoked \(1\)/i)).toBeInTheDocument();
  });
});

describe('DeviceConfigurationPage broker resync', () => {
  it('queues a broker resync with optional reason', async () => {
    fetchDeviceStatus.mockResolvedValue({
      device: {
        uuid: 'uuid-1',
        imei: '123456789012345',
        status: 'active',
        configurationStatus: 'pending',
        lastTelemetryAt: null,
        lastHeartbeatAt: null,
        connectivityStatus: 'unknown',
        connectivityUpdatedAt: null,
        offlineThresholdMs: 86_400_000,
        offlineNotificationChannelCount: 0,
        originImportJobId: null,
        protocolVersion: null,
      },
      telemetry: [],
      recentEvents: [],
      credentialsHistory: [],
      activeCredentials: {
        local: null,
        government: null,
      },
      mqttProvisioning: {
        status: 'applied',
        jobId: 'job-1',
        attemptCount: 1,
        maxAttempts: 5,
        baseRetryDelayMs: 30000,
        lastAttemptAt: new Date().toISOString(),
        nextAttemptAt: new Date().toISOString(),
        lastError: null,
      },
    });

    await renderPage();

    const form = screen.getByRole('form', { name: 'Queue Broker Resync' });
    const deviceUuidInput = within(form).getByLabelText('Device UUID (Broker Resync)');
    const reasonField = within(form).getByLabelText('Resync Reason (optional)');
    const user = userEvent.setup();

    await user.type(deviceUuidInput, 'uuid-1');
    await waitFor(() => expect(fetchDeviceStatus).toHaveBeenCalled());
    await user.type(reasonField, 'Broker maintenance window');
    await user.click(within(form).getByRole('button', { name: 'Queue Resync' }));

    await waitFor(() => {
      expect(resyncDeviceMqttProvisioning).toHaveBeenCalledWith({
        deviceUuid: 'uuid-1',
        reason: 'Broker maintenance window',
      });
    });

    expect(await within(form).findByText(/Broker resync queued/i)).toBeInTheDocument();
    expect(within(form).getByText(/Job job-2 queued/i)).toBeInTheDocument();
  });
});

describe('DeviceConfigurationPage VFD model catalogue', () => {
  it('renders RS485 metadata and command dictionary when a model is selected', async () => {
    const vfdModel: VfdModel = {
      id: 'model-abb-acq580',
      manufacturerId: 'manufacturer-abb',
      manufacturer: 'ABB',
      manufacturerName: 'ABB',
      model: 'ACQ580',
      version: 'v1.0',
      rs485: {
        baudRate: 9600,
        dataBits: 8,
        stopBits: 1,
        parity: 'EVEN',
        flowControl: 'NONE',
        metadata: {
          wiring: '2-wire',
        },
      },
      realtimeParameters: [],
      faultMap: [],
      commandDictionary: [
        {
          commandName: 'START',
          address: 40001,
          functionCode: '0x06',
          description: 'Start pump',
          metadata: { requiresInterlock: true },
        },
      ],
      metadata: null,
      assignments: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    fetchVfdModels.mockResolvedValue([vfdModel]);

    await renderPage();

    const user = userEvent.setup();
    const select = screen.getByLabelText('VFD Model');
    await screen.findByRole('option', { name: 'ABB — ACQ580 v1.0' });
    await user.selectOptions(select, vfdModel.id);

    await waitFor(() => {
      expect(screen.getByText('Selected VFD Model')).toBeTruthy();
    });

    expect(screen.getByText('Command Dictionary')).toBeTruthy();
    expect(screen.getByText('START')).toBeTruthy();
    expect(screen.getByText('Metadata keys: requiresInterlock')).toBeTruthy();
    expect(screen.getByText('RS485 Metadata')).toBeTruthy();
  });
});

describe('DeviceConfigurationPage government credentials', () => {
  it('applies protocol defaults to the government credential form', async () => {
    const statusResponse: DeviceStatusResponse = {
      device: {
        uuid: 'device-uuid-1',
        imei: '123456789012345',
        status: 'active',
        configurationStatus: 'configured',
        lastTelemetryAt: null,
        lastHeartbeatAt: null,
        connectivityStatus: 'online',
        connectivityUpdatedAt: null,
        offlineThresholdMs: 86_400_000,
        offlineNotificationChannelCount: 1,
        originImportJobId: null,
        protocolVersion: {
          id: 'protocol-1',
          version: '1.0.0',
          name: 'MSEDCL Base',
          metadata: {
            governmentCredentials: {
              endpointDefaults: [
                {
                  protocol: 'mqtts',
                  host: 'gov.example',
                  port: 8883,
                  url: 'mqtts://gov.example:8883',
                },
              ],
              topics: {
                publish: ['PUB-TOPIC'],
                subscribe: ['SUB-TOPIC'],
              },
            },
          },
        },
      },
      telemetry: [],
      recentEvents: [],
      credentialsHistory: [],
      activeCredentials: {
        local: null,
        government: null,
      },
      mqttProvisioning: null,
      thresholds: {
        effective: {},
        installation: null,
        override: null,
      },
    };

    fetchDeviceStatus.mockResolvedValue(statusResponse);

    await renderPage();

    const section = screen.getByRole('region', { name: 'Manage Government Credentials' });

    const user = userEvent.setup();
    const deviceUuidInput = within(section).getByLabelText('Device UUID') as HTMLInputElement;
    await user.type(deviceUuidInput, 'device-uuid-1');

    await waitFor(() => {
      expect(fetchDeviceStatus).toHaveBeenCalledWith('device-uuid-1');
    });

    const defaultsButton = within(section).getByRole('button', { name: 'Apply Protocol Defaults' });
    await user.click(defaultsButton);

    const endpointsTextarea = within(section).getByLabelText(
      'Endpoints JSON (array)',
    ) as HTMLTextAreaElement;
    await waitFor(() => {
      expect(endpointsTextarea.value).toContain('gov.example');
    });

    const publishTextarea = within(section).getByLabelText(
      'Publish Topics (newline or comma separated)',
    ) as HTMLTextAreaElement;
    const subscribeTextarea = within(section).getByLabelText(
      'Subscribe Topics (newline or comma separated)',
    ) as HTMLTextAreaElement;

    expect(publishTextarea.value).toContain('PUB-TOPIC');
    expect(subscribeTextarea.value).toContain('SUB-TOPIC');
  });

  it('prefills fields from the active government bundle when requested', async () => {
    const statusResponse: DeviceStatusResponse = {
      device: {
        uuid: 'device-uuid-1',
        imei: '123456789012345',
        status: 'active',
        configurationStatus: 'configured',
        lastTelemetryAt: null,
        lastHeartbeatAt: null,
        connectivityStatus: 'online',
        connectivityUpdatedAt: null,
        offlineThresholdMs: 86_400_000,
        offlineNotificationChannelCount: 1,
        originImportJobId: null,
        protocolVersion: {
          id: 'protocol-1',
          version: '1.0.0',
          name: 'MSEDCL Base',
          metadata: null,
        },
      },
      telemetry: [],
      recentEvents: [],
      credentialsHistory: [],
      activeCredentials: {
        local: null,
        government: {
          type: 'government',
          clientId: 'active-client',
          username: 'active-user',
          password: 'active-pass',
          endpoints: [
            {
              protocol: 'mqtt',
              host: 'gov.active',
              port: 1886,
              url: 'mqtt://gov.active:1886',
            },
          ],
          topics: {
            publish: ['PUB-ACTIVE'],
            subscribe: ['SUB-ACTIVE'],
          },
          validFrom: '2025-05-02T09:00:00.000Z',
          issuedBy: '507f1f77bcf86cd799439099',
          lifecycle: 'active',
          originImportJobId: 'gov-active-job',
          protocolSelector: {
            stateId: 'state-1',
            stateAuthorityId: 'authority-1',
            projectId: 'project-1',
            serverVendorId: 'server-1',
            protocolVersionId: 'proto-1',
            version: '1.0.0',
          },
        },
      },
      mqttProvisioning: null,
      thresholds: {
        effective: {},
        installation: null,
        override: null,
      },
    };

    fetchDeviceStatus.mockResolvedValue(statusResponse);

    await renderPage();

    const section = screen.getByRole('region', { name: 'Manage Government Credentials' });

    const user = userEvent.setup();
    await user.type(within(section).getByLabelText('Device UUID'), 'device-uuid-1');

    await waitFor(() => {
      expect(fetchDeviceStatus).toHaveBeenCalledWith('device-uuid-1');
    });

    const activeButton = within(section).getByRole('button', { name: 'Use Active Bundle' });
    await user.click(activeButton);

    expect((within(section).getByLabelText('Client ID') as HTMLInputElement).value).toBe(
      'active-client',
    );
    expect((within(section).getByLabelText('Username') as HTMLInputElement).value).toBe(
      'active-user',
    );
    expect((within(section).getByLabelText('Password') as HTMLInputElement).value).toBe(
      'active-pass',
    );

    const endpointsTextarea = within(section).getByLabelText(
      'Endpoints JSON (array)',
    ) as HTMLTextAreaElement;
    expect(endpointsTextarea.value).toContain('mqtt://gov.active:1886');
  });

  it('submits updated government credentials and refreshes device status', async () => {
    const statusResponse: DeviceStatusResponse = {
      device: {
        uuid: 'device-uuid-1',
        imei: '123456789012345',
        status: 'active',
        configurationStatus: 'configured',
        lastTelemetryAt: null,
        lastHeartbeatAt: null,
        connectivityStatus: 'online',
        connectivityUpdatedAt: null,
        offlineThresholdMs: 86_400_000,
        offlineNotificationChannelCount: 1,
        originImportJobId: null,
        protocolVersion: {
          id: 'protocol-1',
          version: '1.0.0',
          name: 'MSEDCL Base',
          metadata: null,
        },
      },
      telemetry: [],
      recentEvents: [],
      credentialsHistory: [],
      activeCredentials: {
        local: null,
        government: null,
      },
      mqttProvisioning: null,
      thresholds: {
        effective: {},
        installation: null,
        override: null,
      },
    };

    const mutationResponse = {
      device: { id: 'device-uuid-1', imei: '123456789012345' },
      credentials: {
        clientId: 'gov-client',
        username: 'gov-user',
        password: 'new-pass',
        endpoints: [
          {
            protocol: 'mqtt',
            host: 'gov.host',
            port: 1886,
            url: 'mqtt://gov.host:1886',
          },
        ],
        topics: {
          publish: ['pub-topic'],
          subscribe: ['sub-topic'],
        },
      },
    };

    fetchDeviceStatus.mockResolvedValue(statusResponse);
    upsertDeviceGovernmentCredentials.mockResolvedValue(mutationResponse);

    await renderPage();

    const section = screen.getByRole('region', { name: 'Manage Government Credentials' });

    const user = userEvent.setup();
    const deviceUuidInput = within(section).getByLabelText('Device UUID') as HTMLInputElement;
    await user.type(deviceUuidInput, 'device-uuid-1');

    await waitFor(() => {
      expect(fetchDeviceStatus).toHaveBeenCalledWith('device-uuid-1');
    });

    fetchDeviceStatus.mockClear();

    const clientIdInput = within(section).getByLabelText('Client ID') as HTMLInputElement;
    const usernameInput = within(section).getByLabelText('Username') as HTMLInputElement;
    const passwordInput = within(section).getByLabelText('Password') as HTMLInputElement;
    const issuedByInput = within(section).getByLabelText(
      'Issued By (optional)',
    ) as HTMLInputElement;
    const endpointsTextarea = within(section).getByLabelText(
      'Endpoints JSON (array)',
    ) as HTMLTextAreaElement;
    const publishTextarea = within(section).getByLabelText(
      'Publish Topics (newline or comma separated)',
    ) as HTMLTextAreaElement;
    const subscribeTextarea = within(section).getByLabelText(
      'Subscribe Topics (newline or comma separated)',
    ) as HTMLTextAreaElement;
    const metadataTextarea = within(section).getByLabelText(
      'Metadata JSON (optional)',
    ) as HTMLTextAreaElement;

    await user.type(clientIdInput, 'gov-client');
    await user.type(usernameInput, 'gov-user');
    await user.type(passwordInput, 'secret-pass');
    await user.type(issuedByInput, 'operator-1');
    await user.click(endpointsTextarea);
    await user.paste(
      '[{"protocol":"mqtt","host":"gov.host","port":1886,"url":"mqtt://gov.host:1886"}]',
    );
    await user.type(publishTextarea, 'pub-topic');
    await user.type(subscribeTextarea, 'sub-topic');
    await user.click(metadataTextarea);
    await user.paste('{"source":"ui-test"}');

    const submitButton = within(section).getByRole('button', {
      name: 'Save Government Credentials',
    });

    await user.click(submitButton);

    await waitFor(() => {
      expect(upsertDeviceGovernmentCredentials).toHaveBeenCalledWith('device-uuid-1', {
        clientId: 'gov-client',
        username: 'gov-user',
        password: 'secret-pass',
        endpoints: [
          {
            protocol: 'mqtt',
            host: 'gov.host',
            port: 1886,
            url: 'mqtt://gov.host:1886',
          },
        ],
        topics: {
          publish: ['pub-topic'],
          subscribe: ['sub-topic'],
        },
        metadata: { source: 'ui-test' },
        issuedBy: 'operator-1',
      });
    });

    await waitFor(() => {
      expect(fetchDeviceStatus).toHaveBeenCalledWith('device-uuid-1');
    });

    const successBanner = await within(section).findByText(
      'Government credentials saved successfully.',
    );
    expect(successBanner).toBeTruthy();
  });
});
