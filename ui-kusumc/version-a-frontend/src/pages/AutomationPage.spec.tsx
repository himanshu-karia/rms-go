import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AutomationPage } from './AutomationPage';
import { ActiveProjectProvider } from '../activeProject';

type AutomationApi = typeof import('../api/automation');

const mocks = vi.hoisted(() => ({
  getAutomationFlow: vi.fn<
    Parameters<AutomationApi['getAutomationFlow']>,
    ReturnType<AutomationApi['getAutomationFlow']>
  >(),
  saveAutomationFlow: vi.fn<
    Parameters<AutomationApi['saveAutomationFlow']>,
    ReturnType<AutomationApi['saveAutomationFlow']>
  >(),
}));

vi.mock('../auth', () => ({
  useAuth: () => ({
    isAuthenticated: true,
    user: { username: 'admin', displayName: 'Admin' },
    logout: vi.fn(async () => {}),
    login: vi.fn(async () => {
      throw new Error('not implemented');
    }),
    refresh: vi.fn(async () => {
      throw new Error('not implemented');
    }),
    session: null,
    capabilities: ['alerts:manage'],
    hasCapability: () => true,
  }),
}));

vi.mock('../api/automation', async () => {
  const actual: AutomationApi = await vi.importActual('../api/automation');
  return {
    ...actual,
    getAutomationFlow: mocks.getAutomationFlow,
    saveAutomationFlow: mocks.saveAutomationFlow,
  } satisfies Partial<AutomationApi>;
});

let queryClient: QueryClient;

function renderPage() {
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <QueryClientProvider client={queryClient}>
        <ActiveProjectProvider>
          <AutomationPage />
        </ActiveProjectProvider>
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

  mocks.getAutomationFlow.mockResolvedValue({
    nodes: [],
    edges: [],
    compiled_rules: [],
    schema_version: '1.0.0',
  });

  mocks.saveAutomationFlow.mockResolvedValue({
    saved: true,
    project_id: 'rms-pump-01',
    schema_version: '1.0.0',
    compiled_count: 0,
    errors: [],
    warnings: [],
    issues: [],
  });
});

afterEach(() => {
  queryClient.clear();
  vi.clearAllMocks();
});

describe('AutomationPage', () => {
  it('loads automation flow for a project', async () => {
    renderPage();
    const user = userEvent.setup();

    expect(screen.getByRole('heading', { name: /automation/i })).toBeInTheDocument();

    await act(async () => {
      await user.type(screen.getByLabelText(/project id/i), 'rms-pump-01');
      await user.click(screen.getByRole('button', { name: /^load$/i }));
    });

    await waitFor(() => {
      expect(mocks.getAutomationFlow).toHaveBeenCalledTimes(1);
    });
  });
});
