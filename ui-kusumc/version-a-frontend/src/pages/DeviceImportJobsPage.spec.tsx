import { QueryClient } from '@tanstack/react-query';
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

import type { CsvImportError, DeviceImportRetrySummary, ImportJob } from '../api/devices';
import type * as devicesApi from '../api/devices';
import { DeviceImportJobsPage } from './DeviceImportJobsPage';
import { TestProviders, createTestQueryClient } from '../../tests/test-utils';

const sessionMocks = vi.hoisted(() => ({
  usePollingGateMock: vi.fn(),
}));

vi.mock('../session', async () => {
  const actual = await vi.importActual<typeof import('../session')>('../session');
  return {
    ...actual,
    usePollingGate: sessionMocks.usePollingGateMock,
  };
});

const iso = (value: string) => new Date(value).toISOString();

function createDefaultErrors(): CsvImportError[] {
  return [
    { row: 3, message: 'Missing column', payload: null },
    { row: 7, message: 'Invalid identifier', payload: null },
  ];
}

function createImportJob(overrides: Partial<ImportJob> = {}): ImportJob {
  const errors = overrides.errors ?? createDefaultErrors();
  const createdAt = overrides.createdAt ?? iso('2025-02-01T10:00:00Z');
  const completedAt = overrides.completedAt ?? (overrides.status === 'pending' ? null : createdAt);
  return {
    id: 'job-default',
    type: 'device',
    status: 'completed',
    processed: 12,
    succeeded: 11,
    failed: 1,
    errorCount: overrides.errorCount ?? errors.length,
    errors,
    issuedBy: 'operator-1',
    stateId: 'maharashtra',
    stateAuthorityId: 'msedcl',
    projectId: 'pm_kusum_solarpump_rms',
    createdAt,
    completedAt,
    metadata: null,
    ...overrides,
  };
}

function createPagedResult(jobs: ImportJob[], nextCursor: string | null = null) {
  return {
    jobs,
    nextCursor,
  };
}

type FetchImportJobsFn = typeof devicesApi.fetchImportJobs;
type FetchImportJobFn = typeof devicesApi.fetchImportJob;
type DownloadImportJobErrorsCsvFn = typeof devicesApi.downloadImportJobErrorsCsv;
type RetryDeviceImportJobRowsFn = typeof devicesApi.retryDeviceImportJobRows;

type MutableURL = typeof URL & {
  createObjectURL: (blob: Blob) => string;
  revokeObjectURL: (url: string) => void;
};

const apiMocks = vi.hoisted(() => ({
  fetchImportJobs: vi.fn<Parameters<FetchImportJobsFn>, ReturnType<FetchImportJobsFn>>(),
  fetchImportJob: vi.fn<Parameters<FetchImportJobFn>, ReturnType<FetchImportJobFn>>(),
  downloadImportJobErrorsCsv: vi.fn<
    Parameters<DownloadImportJobErrorsCsvFn>,
    ReturnType<DownloadImportJobErrorsCsvFn>
  >(),
  retryDeviceImportJobRows: vi.fn<
    Parameters<RetryDeviceImportJobRowsFn>,
    ReturnType<RetryDeviceImportJobRowsFn>
  >(),
}));

vi.mock('../api/devices', async () => {
  const actual = await vi.importActual<typeof import('../api/devices')>('../api/devices');
  return {
    ...actual,
    fetchImportJobs: apiMocks.fetchImportJobs,
    fetchImportJob: apiMocks.fetchImportJob,
    downloadImportJobErrorsCsv: apiMocks.downloadImportJobErrorsCsv,
    retryDeviceImportJobRows: apiMocks.retryDeviceImportJobRows,
  } satisfies Partial<typeof actual>;
});

let queryClient: QueryClient;
let consoleErrorSpy: MockInstance | undefined;

function renderPage(initialEntries: string[] = ['/devices/import/jobs']) {
  return render(<DeviceImportJobsPage />, {
    wrapper: ({ children }) => (
      <TestProviders queryClient={queryClient} routerProps={{ initialEntries }}>
        {children}
      </TestProviders>
    ),
  });
}

async function waitForReactQueryIdle() {
  await waitFor(() => {
    expect(queryClient.isFetching()).toBe(0);
    expect(queryClient.isMutating()).toBe(0);
  });
}

beforeEach(() => {
  queryClient = createTestQueryClient();

  apiMocks.fetchImportJobs.mockReset();
  apiMocks.fetchImportJob.mockReset();
  apiMocks.downloadImportJobErrorsCsv.mockReset();
  apiMocks.retryDeviceImportJobRows.mockReset();
  sessionMocks.usePollingGateMock.mockReset();
  sessionMocks.usePollingGateMock.mockImplementation(() => ({
    enabled: true,
    isIdle: false,
    remainingMs: null,
    resume: vi.fn(),
  }));

  apiMocks.fetchImportJobs.mockResolvedValue(createPagedResult([]));
  apiMocks.fetchImportJob.mockResolvedValue(createImportJob());
  apiMocks.downloadImportJobErrorsCsv.mockResolvedValue({
    blob: new Blob([], { type: 'text/csv' }),
    filename: 'errors.csv',
  });
  apiMocks.retryDeviceImportJobRows.mockResolvedValue({
    jobId: 'retry-default',
    sourceJobId: 'job-default',
    rows: [],
    processed: 0,
    enrolled: 0,
    failed: 0,
    errors: [],
    stateId: null,
    stateAuthorityId: null,
    projectId: null,
  });
});

afterEach(() => {
  queryClient.clear();
  sessionMocks.usePollingGateMock.mockReset();
});

beforeAll(() => {
  const originalError = console.error;
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((...args) => {
    const [message] = args;
    if (typeof message === 'string' && message.includes('not wrapped in act(')) {
      return;
    }
    originalError(...args);
  });
});

afterAll(() => {
  consoleErrorSpy?.mockRestore();
});

describe('DeviceImportJobsPage', () => {
  it('renders import jobs from the API and loads additional pages', async () => {
    const firstJob = createImportJob({ id: 'job-1', errors: [], errorCount: 0 });
    const secondJob = createImportJob({
      id: 'job-2',
      type: 'government_credentials',
      status: 'pending',
      completedAt: null,
      errors: [],
      errorCount: 0,
      issuedBy: null,
    });

    apiMocks.fetchImportJobs
      .mockResolvedValueOnce(createPagedResult([firstJob], 'cursor-1'))
      .mockResolvedValueOnce(createPagedResult([secondJob], null));

    renderPage();

    const table = await screen.findByRole('table');
    await within(table).findByText(firstJob.id);
    await within(table).findByText('Device enrollment');

    const loadMoreButton = screen.getByRole('button', { name: 'Load more' });
    expect(loadMoreButton).toBeEnabled();

    const user = userEvent.setup();
    await user.click(loadMoreButton);

    await within(table).findByText(secondJob.id);
    await within(table).findByText('Government credential');
  });

  it('focuses on a job when a jobId query parameter is present', async () => {
    const focusedJob = createImportJob({ id: 'job-focused' });
    apiMocks.fetchImportJobs.mockResolvedValueOnce(createPagedResult([focusedJob], null));

    renderPage(['/devices/import/jobs?jobId=job-focused']);

    await screen.findByText(focusedJob.id);

    expect(screen.getByText(/Focusing on import job/)).toHaveTextContent('job-focused');
    await screen.findByText('Linked job');

    const loadMoreButton = screen.getByRole('button', { name: 'Load more' });
    expect(loadMoreButton).toBeDisabled();

    await waitFor(() => {
      const callArgs = apiMocks.fetchImportJobs.mock.calls[0]?.[0];
      expect(callArgs).toMatchObject({ jobId: 'job-focused' });
    });
  });

  it('opens the detail panel, refreshes live data, and downloads row issues', async () => {
    const rowErrors: CsvImportError[] = [
      { row: 5, message: 'Duplicate IMEI', payload: null },
      { row: 8, message: 'State mismatch', payload: null },
    ];

    const initialJob = createImportJob({ id: 'job-details', errors: rowErrors, errorCount: 2 });
    const liveJob = createImportJob({
      ...initialJob,
      errorCount: 3,
      errors: [...rowErrors, { row: 12, message: 'Project missing', payload: null }],
    });

    apiMocks.fetchImportJobs.mockResolvedValueOnce(createPagedResult([initialJob], null));
    apiMocks.fetchImportJob.mockResolvedValueOnce(liveJob);
    apiMocks.downloadImportJobErrorsCsv.mockResolvedValueOnce({
      blob: new Blob(['row,data'], { type: 'text/csv' }),
      filename: 'job-details-errors.csv',
    });

    renderPage();

    const user = userEvent.setup();
    const detailsButton = await screen.findByRole('button', { name: 'View details' });
    await user.click(detailsButton);

    const detailHeading = await screen.findByRole('heading', { name: 'Import job details' });
    expect(detailHeading).toBeInTheDocument();

    await waitForReactQueryIdle();
    expect(apiMocks.fetchImportJob).toHaveBeenCalledWith('job-details');

    const downloadButton = screen.getByRole('button', { name: 'Download row issues CSV' });

    const mutableURL = URL as MutableURL;
    const originalCreateObjectURL = mutableURL.createObjectURL;
    const originalRevokeObjectURL = mutableURL.revokeObjectURL;
    const createObjectURLSpy = vi.fn(() => 'blob:job-details');
    const revokeObjectURLSpy = vi.fn();
    mutableURL.createObjectURL = createObjectURLSpy;
    mutableURL.revokeObjectURL = revokeObjectURLSpy;

    const anchorClickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined);

    try {
      await user.click(downloadButton);

      await waitFor(() => {
        expect(apiMocks.downloadImportJobErrorsCsv).toHaveBeenCalledWith('job-details');
        expect(createObjectURLSpy).toHaveBeenCalledTimes(1);
        expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:job-details');
        expect(anchorClickSpy).toHaveBeenCalledTimes(1);
      });

      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    } finally {
      mutableURL.createObjectURL = originalCreateObjectURL;
      mutableURL.revokeObjectURL = originalRevokeObjectURL;
      anchorClickSpy.mockRestore();
    }
  });

  it('validates retry input and queues a retry job for failed rows', async () => {
    const job = createImportJob({
      id: 'job-retry',
      errors: [
        { row: 12, message: 'IMEI missing', payload: null },
        { row: 18, message: 'Duplicate pump binding', payload: null },
      ],
      errorCount: 2,
    });

    const retrySummary: DeviceImportRetrySummary = {
      jobId: 'retry-job-123',
      sourceJobId: job.id,
      rows: [12, 30, 31],
      processed: 3,
      enrolled: 2,
      failed: 1,
      errors: [{ row: 31, message: 'Row still failing', payload: null }],
      stateId: job.stateId,
      stateAuthorityId: job.stateAuthorityId,
      projectId: job.projectId,
    };

    apiMocks.fetchImportJobs.mockResolvedValue(createPagedResult([job], null));
    apiMocks.fetchImportJob.mockResolvedValue(job);
    apiMocks.retryDeviceImportJobRows.mockResolvedValueOnce(retrySummary);

    renderPage();

    const user = userEvent.setup();
    const detailsButton = await screen.findByRole('button', { name: 'View details' });
    await user.click(detailsButton);

    await screen.findByRole('heading', { name: 'Import job details' });
    const retryHeading = await screen.findByRole('heading', { name: 'Retry failed rows' });
    expect(retryHeading).toBeInTheDocument();

    const rowsIndicator = screen.getByText(/Rows retried:/);
    expect(rowsIndicator).toHaveTextContent('Rows retried: 2');

    const clearButton = screen.getByRole('button', { name: 'Clear' });
    await user.click(clearButton);

    await waitFor(() => expect(rowsIndicator).toHaveTextContent('Rows retried: 0'));

    const submitButton = screen.getByRole('button', { name: 'Create retry job' });
    await user.click(submitButton);

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Select or enter at least one row to retry');

    const rowCheckbox = screen.getByRole('checkbox', { name: /Row 12:/ });
    await user.click(rowCheckbox);

    const additionalRowsInput = await screen.findByLabelText(/Additional row numbers/i);
    await user.type(additionalRowsInput, '30 31');

    const issuedByInput = screen.getByLabelText('Issued by (optional)');
    await user.type(issuedByInput, 'operator-99');

    await user.click(submitButton);

    await waitFor(() => {
      expect(apiMocks.retryDeviceImportJobRows).toHaveBeenCalledWith('job-retry', {
        rows: [12, 30, 31],
        issuedBy: 'operator-99',
      });
    });

    await waitForReactQueryIdle();

    const retryLink = await screen.findByRole('link', { name: retrySummary.jobId });
    const successParagraph = await screen.findByText((_, element) => {
      if (!element || element.tagName !== 'P') {
        return false;
      }
      return element.textContent?.includes('Retry job') ?? false;
    });
    expect(successParagraph).toBeInTheDocument();
    expect(successParagraph).toContainElement(retryLink);
    await waitFor(() =>
      expect(screen.getByText(/Rows retried:/)).toHaveTextContent('Rows retried: 1'),
    );
    expect(additionalRowsInput).toHaveValue('');
    expect(issuedByInput).toHaveValue('');
  }, 15_000);
});
