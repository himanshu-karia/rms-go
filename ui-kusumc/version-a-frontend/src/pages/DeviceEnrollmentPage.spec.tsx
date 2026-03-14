import { notifyManager, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
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

import type {
  DeviceStatusResponse,
  MqttProvisioningInfo,
  RegisterDeviceResponse,
} from '../api/devices';
import type {
  AuthorityOption,
  ProjectOption,
  ProtocolVersionOption,
  StateOption,
} from '../api/lookups';
import { DeviceEnrollmentPage } from './DeviceEnrollmentPage';

const mocks = vi.hoisted(() => ({
  fetchStatesMock: vi.fn(),
  fetchAuthoritiesMock: vi.fn(),
  fetchProjectsMock: vi.fn(),
  registerDeviceMock: vi.fn(),
  fetchDeviceStatusMock: vi.fn(),
  downloadJsonMock: vi.fn(),
}));

const sessionMocks = vi.hoisted(() => ({
  usePollingGateMock: vi.fn(),
}));

vi.mock('../api/lookups', async () => {
  const actual = await vi.importActual<typeof import('../api/lookups')>('../api/lookups');

  return {
    ...actual,
    fetchStates: mocks.fetchStatesMock,
    fetchAuthorities: mocks.fetchAuthoritiesMock,
    fetchProjects: mocks.fetchProjectsMock,
  };
});

vi.mock('../api/devices', async () => {
  const actual = await vi.importActual<typeof import('../api/devices')>('../api/devices');

  return {
    ...actual,
    registerDevice: mocks.registerDeviceMock,
    fetchDeviceStatus: mocks.fetchDeviceStatusMock,
  };
});

vi.mock('../utils/download', () => ({
  downloadJsonFile: mocks.downloadJsonMock,
}));

vi.mock('../session', () => ({
  usePollingGate: sessionMocks.usePollingGateMock,
}));

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const {
  fetchStatesMock,
  fetchAuthoritiesMock,
  fetchProjectsMock,
  registerDeviceMock,
  fetchDeviceStatusMock,
} = mocks;
const { usePollingGateMock } = sessionMocks;

const stateOption: StateOption = {
  id: 'state-1',
  name: 'Maharashtra',
  isoCode: 'MH',
  authorityCount: 1,
};

const authorityOption: AuthorityOption = {
  id: 'authority-1',
  name: 'MSEDCL',
  stateId: stateOption.id,
  projectCount: 1,
};

const protocolOption: ProtocolVersionOption = {
  id: 'protocol-1',
  version: '1.0.0',
  serverVendorId: 'server-1',
  serverVendorName: 'RMS Vendor',
};

const projectOption: ProjectOption = {
  id: 'project-1',
  name: 'PM KUSUM RMS',
  authorityId: authorityOption.id,
  protocolVersions: [protocolOption],
};

const defaultProvisioning: MqttProvisioningInfo = {
  status: 'pending',
  jobId: 'job-1',
  attemptCount: 0,
  maxAttempts: 3,
  baseRetryDelayMs: 3000,
  lastAttemptAt: null,
  nextAttemptAt: '2024-01-01T00:00:00.000Z',
  lastError: null,
};

const baseLocalCredentials: NonNullable<RegisterDeviceResponse['credentials']> = {
  clientId: 'client-local',
  username: 'mqtt-user',
  password: 'mqtt-pass',
  endpoints: [
    {
      protocol: 'mqtts',
      host: 'edge-broker',
      port: 8883,
      url: 'mqtts://edge-broker:8883',
    },
  ],
  topics: {
    publish: ['device/heartbeat'],
    subscribe: ['device/commands'],
  },
  mqttAccess: {
    applied: false,
  },
};

function createDeviceStatusResponse(
  overrides?: Partial<DeviceStatusResponse>,
): DeviceStatusResponse {
  const base: DeviceStatusResponse = {
    device: {
      uuid: 'device-uuid-1',
      imei: '123456789012345',
      status: null,
      configurationStatus: null,
      lastTelemetryAt: null,
      lastHeartbeatAt: null,
      connectivityStatus: 'unknown',
      connectivityUpdatedAt: null,
      offlineThresholdMs: 600000,
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
    thresholds: {
      effective: {},
      installation: null,
      override: null,
    },
  };

  return {
    ...base,
    ...overrides,
    device: {
      ...base.device,
      ...(overrides?.device ?? {}),
    },
    activeCredentials: {
      ...base.activeCredentials,
      ...(overrides?.activeCredentials ?? {}),
    },
    mqttProvisioning: overrides?.mqttProvisioning ?? base.mqttProvisioning,
  };
}

let queryClient: QueryClient;

const defaultNotify = (callback: () => void) => {
  callback();
};

const defaultBatchNotify = (callback: () => void) => {
  callback();
};

type ConsoleErrorMock = MockInstance<
  Parameters<typeof console.error>,
  ReturnType<typeof console.error>
>;

let consoleErrorSpy: ConsoleErrorMock | undefined;
const originalConsoleError = console.error;

beforeAll(() => {
  notifyManager.setNotifyFunction((callback) => {
    act(() => {
      callback();
    });
  });
  notifyManager.setBatchNotifyFunction((callback) => {
    act(() => {
      callback();
    });
  });
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((message?: unknown, ...rest) => {
    if (typeof message === 'string' && message.includes('not wrapped in act')) {
      return;
    }
    originalConsoleError(message as never, ...rest);
  });
});

afterAll(() => {
  notifyManager.setNotifyFunction(defaultNotify);
  notifyManager.setBatchNotifyFunction(defaultBatchNotify);
  consoleErrorSpy?.mockRestore();
});

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  fetchStatesMock.mockResolvedValue([stateOption]);
  fetchAuthoritiesMock.mockImplementation(async (stateId: string) => {
    return stateId === stateOption.id ? [authorityOption] : [];
  });
  fetchProjectsMock.mockImplementation(
    async (params: { stateId: string; stateAuthorityId: string }) => {
      return params.stateId === stateOption.id && params.stateAuthorityId === authorityOption.id
        ? [projectOption]
        : [];
    },
  );
  registerDeviceMock.mockReset();
  fetchDeviceStatusMock.mockReset();
  usePollingGateMock.mockReset();
  usePollingGateMock.mockImplementation(() => ({
    enabled: true,
    isIdle: false,
    remainingMs: null,
    resume: vi.fn(),
  }));
});

afterEach(() => {
  queryClient.clear();
  fetchStatesMock.mockReset();
  fetchAuthoritiesMock.mockReset();
  fetchProjectsMock.mockReset();
  usePollingGateMock.mockReset();
});

function renderPage() {
  const user = userEvent.setup();
  const view = render(
    <QueryClientProvider client={queryClient}>
      <DeviceEnrollmentPage />
    </QueryClientProvider>,
  );
  return { user, view };
}

type RenderContext = ReturnType<typeof renderPage>;

async function completeRequiredFields(user: RenderContext['user']) {
  const imeiInput = await screen.findByLabelText('IMEI');
  await user.type(imeiInput, '123456789012345');

  const stateSelect = screen.getByLabelText('State') as HTMLSelectElement;
  await waitFor(() => expect(stateSelect.disabled).toBe(false));
  await user.selectOptions(stateSelect, stateOption.id);

  await waitFor(() => expect(fetchAuthoritiesMock).toHaveBeenCalledWith(stateOption.id));

  const authoritySelect = screen.getByLabelText('State Authority') as HTMLSelectElement;
  await waitFor(() => expect(authoritySelect.disabled).toBe(false));
  await user.selectOptions(authoritySelect, authorityOption.id);

  await waitFor(() =>
    expect(fetchProjectsMock).toHaveBeenCalledWith({
      stateId: stateOption.id,
      stateAuthorityId: authorityOption.id,
    }),
  );

  const projectSelect = screen.getByLabelText('Project') as HTMLSelectElement;
  await waitFor(() => expect(projectSelect.disabled).toBe(false));
  await user.selectOptions(projectSelect, projectOption.id);

  const protocolSelect = screen.getByLabelText('Protocol Version') as HTMLSelectElement;
  await waitFor(() => expect(protocolSelect.disabled).toBe(false));
  await user.selectOptions(protocolSelect, protocolOption.id);

  const vendorInput = screen.getByLabelText('Solar Pump Vendor ID');
  await user.type(vendorInput, 'abcdef123456abcdef123456');
}

describe('DeviceEnrollmentPage provisioning copy', () => {
  it('shows refreshed pending copy while broker sync is in progress', async () => {
    const { user } = renderPage();

    const registerResponse: RegisterDeviceResponse = {
      device: {
        id: 'device-1',
        imei: '123456789012345',
      },
      credentials: baseLocalCredentials,
      mqttProvisioning: defaultProvisioning,
      governmentCredentials: null,
    };
    registerDeviceMock.mockResolvedValue(registerResponse);

    let resolveStatus: ((value: DeviceStatusResponse) => void) | undefined;
    let statusPromise: Promise<DeviceStatusResponse> | undefined;
    fetchDeviceStatusMock.mockImplementation(() => {
      statusPromise = new Promise<DeviceStatusResponse>((resolve) => {
        resolveStatus = resolve;
      });
      return statusPromise;
    });

    await completeRequiredFields(user);

    await user.click(screen.getByRole('button', { name: 'Enroll Device' }));
    await waitFor(() => expect(registerDeviceMock).toHaveBeenCalledTimes(1));

    expect(await screen.findByText('Provisioning in progress…')).toBeInTheDocument();
    expect(screen.getByText(/This may take up to 30 seconds\./)).toBeInTheDocument();
    expect(
      screen.getByText(
        'Initial credentials are ready below if installers need to proceed while the broker catches up.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/Refreshing status/)).toBeInTheDocument();

    await waitFor(() => expect(fetchDeviceStatusMock).toHaveBeenCalledTimes(1));

    expect(resolveStatus).toBeDefined();
    expect(statusPromise).toBeDefined();
    resolveStatus!(
      createDeviceStatusResponse({
        mqttProvisioning: defaultProvisioning,
      }),
    );
    await statusPromise!;
  }, 15_000);

  it('renders confirmation copy once provisioning is applied', async () => {
    const { user } = renderPage();

    const inProgressResponse: RegisterDeviceResponse = {
      device: {
        id: 'device-1',
        imei: '123456789012345',
      },
      credentials: baseLocalCredentials,
      mqttProvisioning: {
        ...defaultProvisioning,
        status: 'in_progress',
      },
      governmentCredentials: null,
    };
    registerDeviceMock.mockResolvedValue(inProgressResponse);

    fetchDeviceStatusMock.mockResolvedValue(
      createDeviceStatusResponse({
        mqttProvisioning: {
          ...defaultProvisioning,
          status: 'applied',
        },
        activeCredentials: {
          local: {
            type: 'local',
            clientId: baseLocalCredentials.clientId,
            username: baseLocalCredentials.username,
            password: baseLocalCredentials.password,
            endpoints: baseLocalCredentials.endpoints,
            topics: baseLocalCredentials.topics,
            validFrom: '2024-01-01T00:00:00.000Z',
            issuedBy: 'operator-1',
            lifecycle: 'active',
            originImportJobId: null,
            protocolSelector: null,
            mqttAccess: {
              applied: true,
            },
          },
          government: null,
        },
      }),
    );

    await completeRequiredFields(user);

    await user.click(screen.getByRole('button', { name: 'Enroll Device' }));
    await waitFor(() => expect(registerDeviceMock).toHaveBeenCalledTimes(1));

    expect(await screen.findByText('Provisioning complete.')).toBeInTheDocument();
    expect(
      screen.getByText('Credentials now reflect the broker-confirmed bundle.'),
    ).toBeInTheDocument();
  }, 15_000);

  it('updates the provisioning badge and download CTA once polling reports an applied status', async () => {
    const { user } = renderPage();

    const registerResponse: RegisterDeviceResponse = {
      device: {
        id: 'device-1',
        imei: '123456789012345',
      },
      credentials: baseLocalCredentials,
      mqttProvisioning: defaultProvisioning,
      governmentCredentials: null,
    };
    registerDeviceMock.mockResolvedValue(registerResponse);

    fetchDeviceStatusMock.mockResolvedValue(
      createDeviceStatusResponse({
        mqttProvisioning: {
          ...defaultProvisioning,
          status: 'applied',
        },
        activeCredentials: {
          local: {
            type: 'local',
            clientId: baseLocalCredentials.clientId,
            username: baseLocalCredentials.username,
            password: baseLocalCredentials.password,
            endpoints: baseLocalCredentials.endpoints,
            topics: baseLocalCredentials.topics,
            validFrom: '2024-01-01T00:00:00.000Z',
            issuedBy: 'operator-1',
            lifecycle: 'active',
            originImportJobId: null,
            protocolSelector: null,
            mqttAccess: {
              applied: true,
            },
          },
          government: null,
        },
      }),
    );

    await completeRequiredFields(user);

    await user.click(screen.getByRole('button', { name: 'Enroll Device' }));
    await waitFor(() => expect(fetchDeviceStatusMock).toHaveBeenCalledTimes(1));

    const appliedLabels = await screen.findAllByText('Applied');
    expect(appliedLabels.length).toBeGreaterThan(0);
    expect(
      await screen.findByRole('button', { name: 'Download credentials JSON' }),
    ).toBeInTheDocument();
  }, 15_000);

  it('surfaces broker failure context when provisioning ends in a failed state', async () => {
    const { user } = renderPage();

    const registerResponse: RegisterDeviceResponse = {
      device: {
        id: 'device-1',
        imei: '123456789012345',
      },
      credentials: baseLocalCredentials,
      mqttProvisioning: defaultProvisioning,
      governmentCredentials: null,
    };
    registerDeviceMock.mockResolvedValue(registerResponse);

    fetchDeviceStatusMock.mockResolvedValue(
      createDeviceStatusResponse({
        mqttProvisioning: {
          ...defaultProvisioning,
          status: 'failed',
          lastError: {
            message: 'Forbidden',
            status: 403,
            endpoint: '/mqtt/dynsec',
          },
        },
      }),
    );

    await completeRequiredFields(user);

    await user.click(screen.getByRole('button', { name: 'Enroll Device' }));

    expect(await screen.findByText('Broker synchronization failed.')).toBeInTheDocument();
    expect(screen.getAllByText('Forbidden').length).toBeGreaterThan(0);
    expect(screen.getAllByText(/HTTP 403/).length).toBeGreaterThan(0);
    expect(screen.getAllByText('/mqtt/dynsec').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Failed').length).toBeGreaterThan(0);
  });
});
