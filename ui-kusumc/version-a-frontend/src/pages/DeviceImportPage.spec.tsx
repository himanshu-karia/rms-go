import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { DeviceImportPage } from './DeviceImportPage';

vi.mock('../auth', () => {
  return {
    useAuth: () => ({
      session: null,
      user: { username: 'importer', displayName: 'Importer' },
      isAuthenticated: true,
      login: vi.fn(async () => {
        throw new Error('login not implemented in tests');
      }),
      logout: vi.fn(async () => {}),
      refresh: vi.fn(async () => {
        throw new Error('refresh not implemented in tests');
      }),
      capabilities: ['devices:bulk_import', 'devices:credentials'],
      hasCapability: () => true,
    }),
  };
});

type DevicesApi = typeof import('../api/devices');
type ImportDevicesFn = DevicesApi['importDevicesCsv'];
type ImportGovernmentFn = DevicesApi['importGovernmentCredentialsCsv'];

const apiMocks = vi.hoisted(() => ({
  importDevicesCsv: vi.fn<Parameters<ImportDevicesFn>, ReturnType<ImportDevicesFn>>(),
  importGovernmentCredentialsCsv: vi.fn<
    Parameters<ImportGovernmentFn>,
    ReturnType<ImportGovernmentFn>
  >(),
}));

vi.mock('../api/devices', async () => {
  const actual: DevicesApi = await vi.importActual('../api/devices');
  return {
    ...actual,
    importDevicesCsv: apiMocks.importDevicesCsv,
    importGovernmentCredentialsCsv: apiMocks.importGovernmentCredentialsCsv,
  } satisfies Partial<DevicesApi>;
});

let queryClient: QueryClient;

function createCsvFile(content: string, name: string) {
  const file = new File([content], name, { type: 'text/csv' });
  if (typeof file.text !== 'function') {
    Object.assign(file, {
      text: async () => content,
    });
  }
  return file;
}

function renderPage() {
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <QueryClientProvider client={queryClient}>
        <DeviceImportPage />
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  apiMocks.importDevicesCsv.mockReset();
  apiMocks.importGovernmentCredentialsCsv.mockReset();
});

afterEach(() => {
  queryClient.clear();
});

async function runWithAct(callback: () => Promise<void>) {
  await act(async () => {
    await callback();
  });
}

describe('DeviceImportPage', () => {
  it('shows download template links', () => {
    renderPage();

    const fullLink = screen.getByRole('link', { name: /full enrollment template/i });
    const govLink = screen.getByRole('link', { name: /government credential template/i });
    const historyLink = screen.getByRole('link', { name: /open the import history view/i });

    expect(fullLink).toHaveAttribute('href', '/csv-templates/device-full-enrollment.csv');
    expect(govLink).toHaveAttribute('href', '/csv-templates/government-credentials.csv');
    expect(historyLink).toHaveAttribute('href', '/devices/import/jobs');
  });

  it('requires a file before submitting the full enrollment form', async () => {
    renderPage();
    const user = userEvent.setup();

    await runWithAct(async () => {
      await user.click(screen.getByRole('button', { name: /upload full enrollment csv/i }));
    });

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Select a CSV file before uploading.');
    expect(apiMocks.importDevicesCsv).not.toHaveBeenCalled();
  });

  it('submits the full enrollment CSV and shows the summary', async () => {
    const summary = {
      jobId: 'job-device-123',
      processed: 2,
      enrolled: 2,
      failed: 0,
      errors: [],
      stateId: 'state-1',
      stateAuthorityId: 'authority-1',
      projectId: 'project-1',
    };
    apiMocks.importDevicesCsv.mockResolvedValue(summary);

    renderPage();
    const user = userEvent.setup();

    const file = createCsvFile(
      'imei,stateId,stateAuthorityId,projectId,serverVendorId,protocolVersionId,solarPumpVendorId\n123456789012345,state,stateAuth,project,serverVendor,protocol,solarVendor',
      'full.csv',
    );

    await runWithAct(async () => {
      await user.upload(screen.getByLabelText(/full enrollment csv/i), file);
      await user.type(screen.getByLabelText(/provisioning history for traceability/i), 'csv-admin');
      await user.click(screen.getByRole('button', { name: /upload full enrollment csv/i }));
    });

    await waitFor(() => {
      expect(apiMocks.importDevicesCsv).toHaveBeenCalledTimes(1);
    });

    const [payload] = apiMocks.importDevicesCsv.mock.calls[0];
    expect(payload.csv).toContain('imei,stateId');
    expect(payload.issuedBy).toBe('csv-admin');

    const summaryRegion = await screen.findByRole('region', {
      name: /enrollment import complete/i,
    });
    const list = within(summaryRegion);
    expect(list.getByText(/job-device-123/)).toBeInTheDocument();
    expect(list.getByText(/State state-1/)).toBeInTheDocument();
    expect(list.getByText(/Authority authority-1/)).toBeInTheDocument();
    expect(list.getByText(/Project project-1/)).toBeInTheDocument();
    expect(list.getByText(/processed rows/i)).toBeInTheDocument();
  });

  it('submits the government credential CSV and shows the summary', async () => {
    const summary = {
      jobId: 'job-gov-456',
      processed: 1,
      updated: 1,
      failed: 0,
      errors: [],
      stateId: 'state-2',
      stateAuthorityId: null,
      projectId: 'project-9',
    };
    apiMocks.importGovernmentCredentialsCsv.mockResolvedValue(summary);

    renderPage();
    const user = userEvent.setup();

    const file = createCsvFile(
      'imei,clientId,username,password,endpoints\n123456789012345,gov-client,gov-user,secret,mqtt://broker:1886',
      'gov.csv',
    );

    await runWithAct(async () => {
      await user.upload(screen.getByLabelText(/government credential csv/i), file);
      await user.type(
        screen.getByLabelText(/stored alongside credential rotation history/i),
        'gov-admin',
      );
      await user.click(screen.getByRole('button', { name: /upload government credentials csv/i }));
    });

    await waitFor(() => {
      expect(apiMocks.importGovernmentCredentialsCsv).toHaveBeenCalledTimes(1);
    });

    const [payload] = apiMocks.importGovernmentCredentialsCsv.mock.calls[0];
    expect(payload.csv).toContain('clientId');
    expect(payload.issuedBy).toBe('gov-admin');

    const summaryRegion = await screen.findByRole('region', {
      name: /government credential import complete/i,
    });
    const list = within(summaryRegion);
    expect(list.getByText(/job-gov-456/)).toBeInTheDocument();
    expect(list.getByText(/State state-2/)).toBeInTheDocument();
    expect(list.queryByText(/Authority/)).toBeNull();
    expect(list.getByText(/Project project-9/)).toBeInTheDocument();
    expect(list.getByText(/updated bundles/i)).toBeInTheDocument();
  });

  it('shows enrollment preview issues and exposes the error report download', async () => {
    renderPage();
    const user = userEvent.setup();

    const file = createCsvFile('imei,stateId\n,maharashtra', 'invalid.csv');

    await runWithAct(async () => {
      await user.upload(screen.getByLabelText(/full enrollment csv/i), file);
    });

    expect(await screen.findByText(/previewed rows have issues/i)).toBeInTheDocument();
    expect(screen.getAllByText(/imei is required/i).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /download error report/i })).toBeInTheDocument();
  });

  it('shows government preview issues and highlights missing endpoints', async () => {
    renderPage();
    const user = userEvent.setup();

    const file = createCsvFile(
      'imei,clientId,username,password\n123,client,user,secret',
      'gov-invalid.csv',
    );

    await runWithAct(async () => {
      await user.upload(screen.getByLabelText(/government credential csv/i), file);
    });

    expect(await screen.findByText(/previewed rows have issues/i)).toBeInTheDocument();
    expect(screen.getAllByText(/endpoints is required/i).length).toBeGreaterThan(0);
  });
});
