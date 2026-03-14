import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { MemoryRouter } from 'react-router-dom';

import type { CapabilityKey } from '../api/capabilities';
import type {
  ApiError,
  Beneficiary,
  CreateBeneficiaryPayload,
  UpdateBeneficiaryPayload,
  Installation,
  InstallationAssignment,
} from '../api/installations';
import { InstallationsPage } from './InstallationsPage';
import { SessionTimersProvider } from '../session/SessionTimersProvider';
import type { SessionSnapshot } from '../api/session';

const mocks = vi.hoisted(() => ({
  fetchInstallationsMock: vi.fn(),
  fetchInstallationAssignmentsMock: vi.fn(),
  fetchBeneficiariesMock: vi.fn(),
  assignBeneficiaryMock: vi.fn(),
  removeAssignmentMock: vi.fn(),
  createBeneficiaryMock: vi.fn(),
  updateBeneficiaryMock: vi.fn(),
}));

const authEnv = vi.hoisted(() => {
  const defaults: CapabilityKey[] = ['admin:all', 'installations:manage', 'beneficiaries:manage'];

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

vi.mock('../api/installations', async () => {
  const actual =
    await vi.importActual<typeof import('../api/installations')>('../api/installations');
  return {
    ...actual,
    fetchInstallations: mocks.fetchInstallationsMock,
    fetchInstallationAssignments: mocks.fetchInstallationAssignmentsMock,
    fetchBeneficiaries: mocks.fetchBeneficiariesMock,
    assignBeneficiaryToInstallation: mocks.assignBeneficiaryMock,
    removeBeneficiaryAssignment: mocks.removeAssignmentMock,
    createBeneficiary: mocks.createBeneficiaryMock,
    updateBeneficiary: mocks.updateBeneficiaryMock,
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
        sessionId: 'session-installations',
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
  fetchInstallationsMock,
  fetchInstallationAssignmentsMock,
  fetchBeneficiariesMock,
  assignBeneficiaryMock,
  removeAssignmentMock,
  createBeneficiaryMock,
  updateBeneficiaryMock,
} = mocks;

let queryClient: QueryClient;
let consoleErrorSpy:
  | MockInstance<Parameters<typeof console.error>, ReturnType<typeof console.error>>
  | undefined;

function renderPage() {
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <SessionTimersProvider>
        <QueryClientProvider client={queryClient}>
          <InstallationsPage />
        </QueryClientProvider>
      </SessionTimersProvider>
    </MemoryRouter>,
  );
}

async function waitForQueriesToIdle() {
  await waitFor(() => {
    expect(queryClient.isFetching()).toBe(0);
    expect(queryClient.isMutating()).toBe(0);
  });
}

const LONG_MUTATION_TIMEOUT = 20_000;

describe('InstallationsPage', () => {
  const ownerBeneficiary: Beneficiary = {
    id: 'b-1',
    uuid: 'ben-owner',
    name: 'Asha Solanki',
    email: 'asha@example.com',
    phoneNumber: '+91-9876543210',
    address: 'Pune, Maharashtra',
    contacts: [
      {
        type: 'phone',
        value: '+91-9876543210',
        isPrimary: true,
        proxyBeneficiaryUuid: null,
      },
      {
        type: 'email',
        value: 'asha@example.com',
        isPrimary: true,
        proxyBeneficiaryUuid: null,
      },
      {
        type: 'phone',
        value: '+91-1111222233',
        isPrimary: false,
        proxyBeneficiaryUuid: 'proxy-uuid-1',
      },
    ],
    location: {
      state: 'Maharashtra',
      district: 'Pune',
      tehsil: null,
      village: null,
      geoLocation: null,
    },
    metadata: null,
    accountStatus: 'active',
    deletedAt: null,
    createdAt: '2024-01-01T00:00:00.000Z',
    updatedAt: '2024-01-01T00:00:00.000Z',
  };

  const availableBeneficiary: Beneficiary = {
    id: 'b-2',
    uuid: 'ben-available',
    name: 'Vikram Deshmukh',
    email: 'vikram@example.com',
    phoneNumber: null,
    address: null,
    contacts: [
      {
        type: 'email',
        value: 'vikram@example.com',
        isPrimary: true,
        proxyBeneficiaryUuid: null,
      },
    ],
    location: {
      state: null,
      district: null,
      tehsil: null,
      village: null,
      geoLocation: null,
    },
    metadata: null,
    accountStatus: 'active',
    deletedAt: null,
    createdAt: '2024-01-02T00:00:00.000Z',
    updatedAt: '2024-01-02T00:00:00.000Z',
  };

  const installation: Installation = {
    id: 'i-1',
    uuid: 'installation-1',
    deviceId: 'device-1',
    deviceUuid: 'device-uuid-1',
    imei: '123456789012345',
    stateId: 'state-1',
    stateAuthorityId: 'authority-1',
    projectId: 'project-1',
    serverVendorId: 'vendor-1',
    solarPumpVendorId: 'pump-1',
    protocolVersionId: 'protocol-1',
    vfdDriveModelId: null,
    status: 'active',
    metadata: { rows: 4 },
    notes: 'Primary installation',
    geoLocation: {
      latitude: 19.076,
      longitude: 72.8777,
      accuracyMeters: 10,
      source: 'survey',
      capturedAt: '2024-01-03T00:00:00.000Z',
    },
    activatedAt: '2024-01-04T00:00:00.000Z',
    decommissionedAt: null,
    createdAt: '2024-01-04T00:00:00.000Z',
    updatedAt: '2024-01-05T00:00:00.000Z',
    beneficiaryCount: 1,
  };

  const ownerAssignment: InstallationAssignment = {
    id: 'assign-1',
    installationId: 'i-1',
    installationUuid: installation.uuid,
    beneficiaryId: ownerBeneficiary.id,
    beneficiaryUuid: ownerBeneficiary.uuid,
    role: 'owner',
    assignmentStatus: 'active',
    createdAt: '2024-01-05T00:00:00.000Z',
    updatedAt: '2024-01-05T00:00:00.000Z',
    removedAt: null,
    beneficiary: ownerBeneficiary,
  };

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });

    authController.reset();
    fetchInstallationsMock.mockReset();
    fetchInstallationAssignmentsMock.mockReset();
    fetchBeneficiariesMock.mockReset();
    assignBeneficiaryMock.mockReset();
    removeAssignmentMock.mockReset();
    createBeneficiaryMock.mockReset();
    updateBeneficiaryMock.mockReset();

    fetchInstallationsMock.mockResolvedValue([installation]);
    fetchInstallationAssignmentsMock.mockResolvedValue([ownerAssignment]);
    fetchBeneficiariesMock.mockResolvedValue([ownerBeneficiary, availableBeneficiary]);
    assignBeneficiaryMock.mockResolvedValue({
      ...ownerAssignment,
      beneficiaryId: availableBeneficiary.id,
      beneficiaryUuid: availableBeneficiary.uuid,
      beneficiary: availableBeneficiary,
      role: 'secondary',
    });
    removeAssignmentMock.mockResolvedValue({
      ...ownerAssignment,
      assignmentStatus: 'removed',
      removedAt: '2024-02-01T00:00:00.000Z',
    });
    updateBeneficiaryMock.mockResolvedValue(ownerBeneficiary);
  });

  afterEach(() => {
    queryClient.clear();
  });

  beforeAll(() => {
    const originalError = console.error;
    consoleErrorSpy = vi
      .spyOn(console, 'error')
      .mockImplementation((...args: Parameters<typeof console.error>) => {
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

  it(
    'renders installations and beneficiary assignments, allowing removal',
    async () => {
      const user = userEvent.setup();
      renderPage();
      await screen.findByRole('button', { name: installation.imei });
      await screen.findByText(ownerBeneficiary.name);
      await screen.findByText('Phone: +91-9876543210 (primary)');
      await screen.findByText('Email: asha@example.com (primary)');

      const removeButton = await screen.findByRole('button', { name: 'Remove' });
      await user.click(removeButton);

      await waitFor(() => {
        expect(removeAssignmentMock).toHaveBeenCalledWith(installation.uuid, ownerBeneficiary.uuid);
      });

      await waitForQueriesToIdle();
    },
    LONG_MUTATION_TIMEOUT,
  );

  it(
    'disables beneficiary removal without super-admin capability',
    async () => {
      const limitedCapabilities = DEFAULT_CAPABILITIES.filter(
        (cap) => !['admin:all', 'installations:manage', 'beneficiaries:manage'].includes(cap),
      );
      authController.setCapabilities(limitedCapabilities);

      renderPage();
      await screen.findByRole('button', { name: installation.imei });

      const warning = await screen.findByText(
        'Beneficiary removal requires both installations:manage and beneficiaries:manage capabilities or super-admin access.',
      );
      expect(warning).toBeInTheDocument();

      const removeButton = await screen.findByRole('button', { name: 'Remove' });
      expect(removeButton).toBeDisabled();
      expect(removeButton).toHaveAttribute(
        'title',
        'Installations:manage and beneficiaries:manage capabilities or super-admin access required to remove assignments',
      );
      expect(removeAssignmentMock).not.toHaveBeenCalled();

      await waitForQueriesToIdle();
    },
    LONG_MUTATION_TIMEOUT,
  );

  it('assigns a selected beneficiary with the chosen role', async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole('button', { name: installation.imei });
    await screen.findByRole('option', { name: availableBeneficiary.name });

    await user.selectOptions(screen.getByLabelText('Beneficiary'), availableBeneficiary.uuid);
    await user.selectOptions(screen.getByLabelText('Assignment role'), 'owner');

    const assignButton = screen.getByRole('button', { name: 'Assign beneficiary' });
    await user.click(assignButton);

    await waitFor(() => {
      expect(assignBeneficiaryMock).toHaveBeenCalledWith(installation.uuid, {
        beneficiaryUuid: availableBeneficiary.uuid,
        role: 'owner',
      });
    });

    await waitForQueriesToIdle();
  });

  it(
    'creates a beneficiary with contact and location details',
    async () => {
      const user = userEvent.setup();
      const capturedAtInputValue = '2024-02-01T10:00';
      const expectedCapturedAt = new Date(capturedAtInputValue).toISOString();

      const createdBeneficiary: Beneficiary = {
        id: 'b-3',
        uuid: 'ben-new',
        name: 'Savita',
        email: 'savita@example.com',
        phoneNumber: '+91 1234567890',
        address: 'Village Square',
        contacts: [
          {
            type: 'phone',
            value: '+91 1234567890',
            isPrimary: true,
            proxyBeneficiaryUuid: null,
          },
          {
            type: 'email',
            value: 'savita@example.com',
            isPrimary: true,
            proxyBeneficiaryUuid: null,
          },
        ],
        location: {
          state: 'Maharashtra',
          district: 'Pune',
          tehsil: 'Mulshi',
          village: 'Lavale',
          geoLocation: {
            latitude: 18.52,
            longitude: 73.85,
            accuracyMeters: 15,
            source: 'survey',
            capturedAt: expectedCapturedAt,
          },
        },
        metadata: null,
        accountStatus: 'active',
        deletedAt: null,
        createdAt: '2024-02-10T00:00:00.000Z',
        updatedAt: '2024-02-10T00:00:00.000Z',
      };

      createBeneficiaryMock.mockResolvedValue(createdBeneficiary);

      renderPage();
      await screen.findByRole('button', { name: installation.imei });

      await user.click(screen.getByRole('button', { name: 'Create beneficiary' }));

      await user.type(screen.getByPlaceholderText('Beneficiary name'), ' Savita ');
      await user.type(screen.getByPlaceholderText('Email (optional)'), 'savita@example.com');
      await user.type(screen.getByPlaceholderText('Phone number (optional)'), '+91 1234567890');
      await user.type(screen.getByPlaceholderText('Address (optional)'), 'Village Square');
      await user.type(screen.getByPlaceholderText('State (optional)'), 'Maharashtra');
      await user.type(screen.getByPlaceholderText('District (optional)'), 'Pune');
      await user.type(screen.getByPlaceholderText('Tehsil (optional)'), 'Mulshi');
      await user.type(screen.getByPlaceholderText('Village (optional)'), 'Lavale');
      await user.type(screen.getByPlaceholderText('Latitude (optional)'), '18.52');
      await user.type(screen.getByPlaceholderText('Longitude (optional)'), '73.85');
      await user.type(screen.getByPlaceholderText('Accuracy (meters, optional)'), '15');
      await user.type(screen.getByPlaceholderText('Location source (optional)'), 'survey');
      await user.type(screen.getByLabelText('Geo capture time (optional)'), capturedAtInputValue);

      const saveButton = screen.getByRole('button', { name: 'Save beneficiary' });
      await user.click(saveButton);

      await waitFor(() => {
        expect(createBeneficiaryMock).toHaveBeenCalledWith(
          expect.objectContaining({
            name: 'Savita',
            email: 'savita@example.com',
            phoneNumber: '+91 1234567890',
            address: 'Village Square',
            contacts: expect.arrayContaining([
              expect.objectContaining({ type: 'phone', value: '+91 1234567890', isPrimary: true }),
              expect.objectContaining({
                type: 'email',
                value: 'savita@example.com',
                isPrimary: true,
              }),
            ]),
            location: expect.objectContaining({
              state: 'Maharashtra',
              district: 'Pune',
              tehsil: 'Mulshi',
              village: 'Lavale',
              geoLocation: expect.objectContaining({
                latitude: 18.52,
                longitude: 73.85,
                accuracyMeters: 15,
                source: 'survey',
                capturedAt: expectedCapturedAt,
              }),
            }),
          }),
        );
      });

      const [createPayload] = createBeneficiaryMock.mock.calls[0] as [CreateBeneficiaryPayload];
      expect(createPayload.contacts).toEqual([
        { type: 'phone', value: '+91 1234567890', isPrimary: true },
        { type: 'email', value: 'savita@example.com', isPrimary: true },
      ]);
      expect(createPayload.location).toEqual({
        state: 'Maharashtra',
        district: 'Pune',
        tehsil: 'Mulshi',
        village: 'Lavale',
        geoLocation: {
          latitude: 18.52,
          longitude: 73.85,
          accuracyMeters: 15,
          source: 'survey',
          capturedAt: expectedCapturedAt,
        },
      });

      await waitForQueriesToIdle();
    },
    LONG_MUTATION_TIMEOUT,
  );

  it(
    'allows linking a conflicting phone contact as a proxy during creation',
    async () => {
      const user = userEvent.setup();
      const conflictError = new Error(
        'Phone number is already in use by another beneficiary',
      ) as ApiError;
      conflictError.status = 409;
      conflictError.details = {
        type: 'phone',
        value: '+91 1234567890',
        beneficiaryUuid: 'conflict-beneficiary-uuid',
      };

      const createdBeneficiary: Beneficiary = {
        ...ownerBeneficiary,
        uuid: 'new-beneficiary',
        name: 'Savita',
        phoneNumber: '+91 1234567890',
        email: null,
        contacts: [
          {
            type: 'phone',
            value: '+91 1234567890',
            isPrimary: true,
            proxyBeneficiaryUuid: 'conflict-beneficiary-uuid',
          },
        ],
      };

      createBeneficiaryMock.mockRejectedValueOnce(conflictError);
      createBeneficiaryMock.mockResolvedValueOnce(createdBeneficiary);

      renderPage();
      await screen.findByRole('button', { name: installation.imei });

      await user.click(screen.getByRole('button', { name: 'Create beneficiary' }));

      await user.type(screen.getByPlaceholderText('Beneficiary name'), ' Savita ');
      await user.type(screen.getByPlaceholderText('Phone number (optional)'), '+91 1234567890');

      const saveButton = screen.getByRole('button', { name: 'Save beneficiary' });
      await user.click(saveButton);

      const conflictAlert = await screen.findByText(
        'Phone number is already in use by another beneficiary',
      );
      expect(conflictAlert).toBeInTheDocument();

      const linkButton = screen.getByRole('button', { name: 'Link as proxy' });
      await user.click(linkButton);

      await screen.findByText(/Linked as proxy to conflict/);

      await user.click(saveButton);

      await waitFor(() => {
        expect(createBeneficiaryMock).toHaveBeenCalledTimes(2);
      });

      const secondCall = createBeneficiaryMock.mock.calls[1] as [CreateBeneficiaryPayload];
      expect(secondCall[0].contacts).toEqual([
        {
          type: 'phone',
          value: '+91 1234567890',
          isPrimary: true,
          proxyBeneficiaryUuid: 'conflict-beneficiary-uuid',
        },
      ]);

      await waitForQueriesToIdle();
    },
    LONG_MUTATION_TIMEOUT,
  );

  it('edits beneficiary and preserves secondary contacts', async () => {
    const user = userEvent.setup();
    updateBeneficiaryMock.mockResolvedValue({
      ...ownerBeneficiary,
      email: 'asha.updated@example.com',
      phoneNumber: '+91-9999999999',
      contacts: [
        {
          type: 'phone',
          value: '+91-1111222233',
          isPrimary: false,
          proxyBeneficiaryUuid: 'proxy-uuid-1',
        },
        {
          type: 'phone',
          value: '+91-9999999999',
          isPrimary: true,
          proxyBeneficiaryUuid: null,
        },
        {
          type: 'email',
          value: 'asha.updated@example.com',
          isPrimary: true,
          proxyBeneficiaryUuid: null,
        },
      ],
    });

    renderPage();
    await screen.findByRole('button', { name: installation.imei });

    const editButton = await screen.findByRole('button', { name: 'Edit' });
    await user.click(editButton);

    const phoneInput = screen.getByPlaceholderText('Phone number (optional)');
    expect(phoneInput).toHaveValue('+91-9876543210');
    await user.clear(phoneInput);
    await user.type(phoneInput, '+91-9999999999');

    const emailInput = screen.getByPlaceholderText('Email (optional)');
    expect(emailInput).toHaveValue('asha@example.com');
    await user.clear(emailInput);
    await user.type(emailInput, 'asha.updated@example.com');

    const saveButton = screen.getByRole('button', { name: 'Save changes' });
    await user.click(saveButton);

    await waitFor(() => {
      expect(updateBeneficiaryMock).toHaveBeenCalledWith(ownerBeneficiary.uuid, expect.any(Object));
    });

    const updatePayload = (
      updateBeneficiaryMock.mock.calls[0] as [string, UpdateBeneficiaryPayload]
    )[1];

    expect(updatePayload.name).toBe(ownerBeneficiary.name);
    expect(updatePayload.phoneNumber).toBe('+91-9999999999');
    expect(updatePayload.email).toBe('asha.updated@example.com');
    expect(updatePayload.contacts).toBeDefined();
    expect(updatePayload.contacts).toHaveLength(3);
    expect(updatePayload.contacts).toEqual(
      expect.arrayContaining([
        {
          type: 'phone',
          value: '+91-9999999999',
          isPrimary: true,
          proxyBeneficiaryUuid: null,
        },
        {
          type: 'phone',
          value: '+91-1111222233',
          isPrimary: false,
          proxyBeneficiaryUuid: 'proxy-uuid-1',
        },
        {
          type: 'email',
          value: 'asha.updated@example.com',
          isPrimary: true,
          proxyBeneficiaryUuid: null,
        },
      ]),
    );

    await waitForQueriesToIdle();
  });

  it('allows linking a conflicting email contact as a proxy during update', async () => {
    const user = userEvent.setup();
    const conflictError = new Error('Email is already in use by another beneficiary') as ApiError;
    conflictError.status = 409;
    conflictError.details = {
      type: 'email',
      value: 'asha.proxy@example.com',
      beneficiaryUuid: 'conflict-email-beneficiary',
    };

    updateBeneficiaryMock.mockRejectedValueOnce(conflictError);
    updateBeneficiaryMock.mockResolvedValueOnce({
      ...ownerBeneficiary,
      email: 'asha.proxy@example.com',
      contacts: [
        {
          type: 'phone',
          value: '+91-9876543210',
          isPrimary: true,
          proxyBeneficiaryUuid: null,
        },
        {
          type: 'email',
          value: 'asha.proxy@example.com',
          isPrimary: true,
          proxyBeneficiaryUuid: 'conflict-email-beneficiary',
        },
      ],
    });

    renderPage();
    await screen.findByRole('button', { name: installation.imei });

    const editButton = await screen.findByRole('button', { name: 'Edit' });
    await user.click(editButton);

    const emailInput = screen.getByPlaceholderText('Email (optional)');
    await user.clear(emailInput);
    await user.type(emailInput, 'asha.proxy@example.com');

    const saveButton = screen.getByRole('button', { name: 'Save changes' });
    await user.click(saveButton);

    await screen.findByText('Email is already in use by another beneficiary');

    const linkButton = screen.getByRole('button', { name: 'Link as proxy' });
    await user.click(linkButton);

    await screen.findByText(/Linked as proxy to conflict/);

    await user.click(saveButton);

    await waitFor(() => {
      expect(updateBeneficiaryMock).toHaveBeenCalledTimes(2);
    });

    const updateCall = updateBeneficiaryMock.mock.calls[1] as [string, UpdateBeneficiaryPayload];
    const updatePayload = updateCall[1];
    const emailContact = updatePayload.contacts?.find((contact) => contact.type === 'email');
    expect(emailContact).toMatchObject({
      value: 'asha.proxy@example.com',
      proxyBeneficiaryUuid: 'conflict-email-beneficiary',
    });

    await waitForQueriesToIdle();
  });
});
