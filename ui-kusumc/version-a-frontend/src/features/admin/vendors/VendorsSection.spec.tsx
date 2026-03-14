import { notifyManager, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  beforeAll,
  afterAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type MockInstance,
} from 'vitest';

import type { AdminVendor, VendorCollectionKey } from '../../../api/admin';
import type { CapabilityKey } from '../../../api/capabilities';
import { VendorsSection } from './VendorsSection';

const mocks = vi.hoisted(() => ({
  fetchAdminVendorsMock: vi.fn(),
  createAdminVendorMock: vi.fn(),
  updateAdminVendorMock: vi.fn(),
  deleteAdminVendorMock: vi.fn(),
}));

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const authEnv = vi.hoisted(() => {
  const defaults: CapabilityKey[] = ['admin:all', 'vendors:manage'];

  const controller = {
    defaults,
    capabilities: new Set<CapabilityKey>(defaults),
    setCapabilities(next: CapabilityKey[]) {
      controller.capabilities = new Set(next);
    },
    reset() {
      controller.capabilities = new Set(controller.defaults);
    },
  };

  return { defaults, controller } as const;
});

const authController = authEnv.controller;

vi.mock('../../../api/admin', async () => {
  const actual = await vi.importActual<typeof import('../../../api/admin')>('../../../api/admin');

  return {
    ...actual,
    fetchAdminVendors: mocks.fetchAdminVendorsMock,
    createAdminVendor: mocks.createAdminVendorMock,
    updateAdminVendor: mocks.updateAdminVendorMock,
    deleteAdminVendor: mocks.deleteAdminVendorMock,
  };
});

vi.mock('../../../auth', () => {
  const login = vi.fn(async () => {
    throw new Error('login not implemented in tests');
  });
  const logout = vi.fn(async () => undefined);

  return {
    useAuth: () => {
      const capabilities = [...authController.capabilities];
      return {
        session: null,
        user: { username: 'admin', displayName: 'Admin' },
        isAuthenticated: true,
        login,
        logout,
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
  fetchAdminVendorsMock,
  createAdminVendorMock,
  updateAdminVendorMock,
  deleteAdminVendorMock,
} = mocks;

let queryClient: QueryClient;
let vendorDataset: Partial<Record<VendorCollectionKey, AdminVendor[]>>;

type ConsoleErrorMock = MockInstance<
  Parameters<typeof console.error>,
  ReturnType<typeof console.error>
>;
let consoleErrorSpy: ConsoleErrorMock | undefined;
const originalConsoleError = console.error;

const defaultNotify = (callback: () => void) => {
  callback();
};

const defaultBatchNotify = (callback: () => void) => {
  callback();
};

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
  // React Query invalidate cycles trigger a benign act warning; filter it to keep test output focused.
  consoleErrorSpy = vi
    .spyOn(console, 'error')
    .mockImplementation((message?: unknown, ...rest: unknown[]) => {
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

async function renderSectionReady() {
  render(
    <QueryClientProvider client={queryClient}>
      <VendorsSection />
    </QueryClientProvider>,
  );

  await waitFor(() => {
    expect(fetchAdminVendorsMock).toHaveBeenCalled();
  });
  await waitFor(() => {
    expect(queryClient.isFetching()).toBe(0);
  });
}

async function waitForVendorsIdle() {
  await waitFor(() => {
    expect(queryClient.isFetching()).toBe(0);
  });
}

describe('VendorsSection', () => {
  let confirmSpy: MockInstance<
    Parameters<typeof window.confirm>,
    ReturnType<typeof window.confirm>
  >;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    vendorDataset = {
      server: [],
      solarPump: [],
      vfdManufacturer: [],
      rmsManufacturer: [],
    };

    fetchAdminVendorsMock.mockImplementation(async (collection: VendorCollectionKey) => {
      return vendorDataset[collection] ?? [];
    });

    createAdminVendorMock.mockReset();
    updateAdminVendorMock.mockReset();
    deleteAdminVendorMock.mockReset();
    confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    authController.reset();
  });

  afterEach(() => {
    queryClient.clear();
    fetchAdminVendorsMock.mockReset();
    confirmSpy.mockRestore();
  });

  it('creates a vendor and shows success feedback', async () => {
    const createdVendor: AdminVendor = {
      id: 'vendor-1',
      name: 'EMQX Ops',
      metadata: { supportEmail: 'ops@example.com' },
      createdAt: '2024-01-04T00:00:00.000Z',
      updatedAt: '2024-01-04T00:00:00.000Z',
    } as AdminVendor;

    createAdminVendorMock.mockResolvedValue(createdVendor);

    await renderSectionReady();
    await screen.findByText('Vendors');

    const serverCard = screen.getByRole('region', { name: 'Server Vendors' });
    const user = userEvent.setup();

    const nameInput = within(serverCard).getByPlaceholderText('Vendor name');
    await user.type(nameInput, 'EMQX Ops');

    const metadataInput = within(serverCard).getByPlaceholderText(
      '{"supportEmail": "ops@example.com"}',
    );
    fireEvent.change(metadataInput, {
      target: { value: '{"supportEmail":"ops@example.com"}' },
    });

    await act(async () => {
      await user.click(within(serverCard).getByRole('button', { name: 'Add Vendor' }));
    });

    await waitFor(() => {
      expect(createAdminVendorMock).toHaveBeenCalledTimes(1);
    });

    const [collectionArg, payloadArg] = createAdminVendorMock.mock.calls[0];
    expect(collectionArg).toBe('server');
    expect(payloadArg).toEqual({
      name: 'EMQX Ops',
      metadata: { supportEmail: 'ops@example.com' },
    });

    const successBanner = await within(serverCard).findByText('EMQX Ops created.');
    expect(successBanner).toBeInTheDocument();

    await waitForVendorsIdle();
  });

  it('updates a vendor and shows success feedback', async () => {
    const originalVendor: AdminVendor = {
      id: 'vendor-1',
      name: 'EMQX Ops',
      metadata: { supportEmail: 'ops@example.com' },
      createdAt: '2024-01-04T00:00:00.000Z',
      updatedAt: '2024-01-04T00:00:00.000Z',
    } as AdminVendor;

    const updatedVendor: AdminVendor = {
      ...originalVendor,
      name: 'EMQX Ops Renewed',
      metadata: { supportEmail: 'support@example.com' },
    };

    vendorDataset.server = [originalVendor];
    updateAdminVendorMock.mockResolvedValue(updatedVendor);

    await renderSectionReady();
    await screen.findByText('Vendors');

    const serverCard = screen.getByRole('region', { name: 'Server Vendors' });
    const user = userEvent.setup();

    const editButton = await within(serverCard).findByRole('button', { name: 'Edit' });
    await act(async () => {
      await user.click(editButton);
    });

    const nameInput = within(serverCard).getByPlaceholderText('Vendor name');
    await user.clear(nameInput);
    await user.type(nameInput, 'EMQX Ops Renewed');

    const metadataInput = within(serverCard).getByPlaceholderText(
      '{"supportEmail": "ops@example.com"}',
    );
    fireEvent.change(metadataInput, {
      target: { value: '{"supportEmail":"support@example.com"}' },
    });

    await act(async () => {
      await user.click(within(serverCard).getByRole('button', { name: 'Update Vendor' }));
    });

    await waitFor(() => {
      expect(updateAdminVendorMock).toHaveBeenCalledTimes(1);
    });

    const [collectionArg, entityIdArg, payloadArg] = updateAdminVendorMock.mock.calls[0];
    expect(collectionArg).toBe('server');
    expect(entityIdArg).toBe(originalVendor.id);
    expect(payloadArg).toEqual({
      name: 'EMQX Ops Renewed',
      metadata: { supportEmail: 'support@example.com' },
    });

    const successBanner = await within(serverCard).findByText('EMQX Ops Renewed updated.');
    expect(successBanner).toBeInTheDocument();

    await waitForVendorsIdle();
  });

  it('deletes a vendor after confirmation', async () => {
    const vendor: AdminVendor = {
      id: 'vendor-1',
      name: 'EMQX Ops',
      metadata: { supportEmail: 'ops@example.com' },
      createdAt: '2024-01-04T00:00:00.000Z',
      updatedAt: '2024-01-04T00:00:00.000Z',
    } as AdminVendor;

    vendorDataset.server = [vendor];
    deleteAdminVendorMock.mockResolvedValue(undefined);

    await renderSectionReady();
    await screen.findByText('Vendors');

    const serverCard = screen.getByRole('region', { name: 'Server Vendors' });
    const user = userEvent.setup();

    const deleteButton = await within(serverCard).findByRole('button', { name: 'Delete' });
    await act(async () => {
      await user.click(deleteButton);
    });

    expect(confirmSpy).toHaveBeenCalledWith('Delete vendor "EMQX Ops"?');

    await waitFor(() => {
      expect(deleteAdminVendorMock).toHaveBeenCalledTimes(1);
    });

    const successBanner = await within(serverCard).findByText('EMQX Ops deleted.');
    expect(successBanner).toBeInTheDocument();

    await waitForVendorsIdle();
  });

  it('disables vendor deletion without super-admin capability', async () => {
    const vendor: AdminVendor = {
      id: 'vendor-1',
      name: 'EMQX Ops',
      metadata: { supportEmail: 'ops@example.com' },
      createdAt: '2024-01-04T00:00:00.000Z',
      updatedAt: '2024-01-04T00:00:00.000Z',
    } as AdminVendor;

    vendorDataset.server = [vendor];
    authController.setCapabilities(['catalog:protocols']);

    await renderSectionReady();
    await screen.findByText('Vendors');

    const serverCard = screen.getByRole('region', { name: 'Server Vendors' });
    const deleteButton = await within(serverCard).findByRole('button', { name: 'Delete' });

    expect(deleteButton).toBeDisabled();
    expect(deleteButton).toHaveAttribute(
      'title',
      'Deleting vendor records requires the vendors:manage capability or super-admin access.',
    );
  });
});
