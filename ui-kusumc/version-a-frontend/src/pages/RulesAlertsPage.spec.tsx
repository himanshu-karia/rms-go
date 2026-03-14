import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { RulesAlertsPage } from './RulesAlertsPage';
import { ActiveProjectProvider } from '../activeProject';

type RulesApi = typeof import('../api/rules');
type AlertsApi = typeof import('../api/alerts');

const mocks = vi.hoisted(() => ({
  fetchRules: vi.fn<Parameters<RulesApi['fetchRules']>, ReturnType<RulesApi['fetchRules']>>(),
  createRule: vi.fn<Parameters<RulesApi['createRule']>, ReturnType<RulesApi['createRule']>>(),
  deleteRule: vi.fn<Parameters<RulesApi['deleteRule']>, ReturnType<RulesApi['deleteRule']>>(),
  fetchAlerts: vi.fn<
    Parameters<AlertsApi['fetchAlerts']>,
    ReturnType<AlertsApi['fetchAlerts']>
  >(),
  ackAlert: vi.fn<Parameters<AlertsApi['ackAlert']>, ReturnType<AlertsApi['ackAlert']>>(),
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

vi.mock('../api/rules', async () => {
  const actual: RulesApi = await vi.importActual('../api/rules');
  return {
    ...actual,
    fetchRules: mocks.fetchRules,
    createRule: mocks.createRule,
    deleteRule: mocks.deleteRule,
  } satisfies Partial<RulesApi>;
});

vi.mock('../api/alerts', async () => {
  const actual: AlertsApi = await vi.importActual('../api/alerts');
  return {
    ...actual,
    fetchAlerts: mocks.fetchAlerts,
    ackAlert: mocks.ackAlert,
  } satisfies Partial<AlertsApi>;
});

let queryClient: QueryClient;

function renderPage() {
  return render(
    <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <QueryClientProvider client={queryClient}>
        <ActiveProjectProvider>
          <RulesAlertsPage />
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

  mocks.fetchRules.mockResolvedValue([]);
  mocks.fetchAlerts.mockResolvedValue([]);
  mocks.createRule.mockResolvedValue({ id: 'rule-1' });
  mocks.deleteRule.mockResolvedValue();
  mocks.ackAlert.mockResolvedValue();
});

afterEach(() => {
  queryClient.clear();
  vi.clearAllMocks();
});

describe('RulesAlertsPage', () => {
  it('renders and loads rules/alerts after setting project id', async () => {
    renderPage();
    const user = userEvent.setup();

    expect(screen.getByRole('heading', { name: /rules\s*\/\s*alerts/i })).toBeInTheDocument();

    await act(async () => {
      await user.type(screen.getByLabelText(/project id \(filters\)/i), 'rms-pump-01');
      await user.click(screen.getByRole('button', { name: /^load$/i }));
    });

    await waitFor(() => {
      expect(mocks.fetchRules).toHaveBeenCalledTimes(1);
      expect(mocks.fetchAlerts).toHaveBeenCalledTimes(1);
    });
  });
});
