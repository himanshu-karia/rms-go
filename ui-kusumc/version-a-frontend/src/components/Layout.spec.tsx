import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { Layout } from './Layout';
import { ActiveProjectProvider } from '../activeProject';
import { hasCapabilities, type CapabilityKey } from '../api/capabilities';
import type { AuthContextValue } from '../auth';
import { SessionTimersProvider } from '../session/SessionTimersProvider';
import type { SessionSnapshot } from '../api/session';

type CapabilityPreset = CapabilityKey[];

const useAuthMock = vi.fn<[], AuthContextValue>();

vi.mock('../auth', () => ({
  useAuth: () => useAuthMock(),
}));

const TELEMETRY_ANALYST_CAPS: CapabilityPreset = [
  'telemetry:read',
  'telemetry:export',
  'reports:manage',
];

const SUPPORT_AGENT_CAPS: CapabilityPreset = [
  'telemetry:read',
  'telemetry:live:device',
  'devices:read',
  'audit:read',
  'support:manage',
];

const AUTHORITY_MANAGER_CAPS: CapabilityPreset = [
  'telemetry:read',
  'telemetry:live:device',
  'devices:read',
  'devices:write',
  'devices:credentials',
  'devices:commands',
  'devices:bulk_import',
  'installations:manage',
  'beneficiaries:manage',
  'reports:manage',
];

function buildSession(capabilities: CapabilityPreset): SessionSnapshot {
  const now = Date.now();
  return {
    token: 'test-token',
    username: 'tester',
    displayName: 'Test User',
    expiresAt: new Date(now + 60 * 60 * 1000).toISOString(),
    sessionId: 'session-test',
    capabilities,
  };
}

function buildAuthContext(capabilities: CapabilityPreset): AuthContextValue {
  const session = buildSession(capabilities);
  return {
    session,
    user: { username: 'tester', displayName: 'Test User' },
    isAuthenticated: true,
    login: vi.fn(),
    logout: vi.fn(async () => {}),
    refresh: vi.fn(async () => session),
    capabilities,
    hasCapability: (required, options) => hasCapabilities(capabilities, required, options),
  } as AuthContextValue;
}

function renderLayout(initialPath: string) {
  render(
    <MemoryRouter
      initialEntries={[initialPath]}
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <SessionTimersProvider>
        <ActiveProjectProvider>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route index element={<div>Dashboard</div>} />
              <Route path="telemetry" element={<div>Telemetry</div>} />
              <Route path="telemetry/v2" element={<div>Telemetry v2</div>} />
              <Route path="live/device-inventory" element={<div>Device Inventory</div>} />
              <Route path="devices/enroll" element={<div>Enroll Device</div>} />
              <Route path="devices/configuration" element={<div>Device Configuration</div>} />
              <Route path="devices/import" element={<div>Import CSVs</div>} />
              <Route path="devices/import/jobs" element={<div>Import Jobs</div>} />
              <Route path="operations/installations" element={<div>Installations</div>} />
              <Route path="admin/hierarchy" element={<div>Admin</div>} />
            </Route>
            <Route path="/login" element={<div>Login Page</div>} />
            <Route path="/simulator" element={<div>Simulator Page</div>} />
          </Routes>
        </ActiveProjectProvider>
      </SessionTimersProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  useAuthMock.mockReset();
});

describe('Layout navigation filtering', () => {
  it('shows telemetry-only navigation for telemetry analyst role', () => {
    useAuthMock.mockReturnValue(buildAuthContext(TELEMETRY_ANALYST_CAPS));
    renderLayout('/telemetry');

    expect(screen.getAllByRole('link', { name: /RMS Dashboard/i })).not.toHaveLength(0);
    expect(screen.getAllByRole('link', { name: /Telemetry Monitor/i })).not.toHaveLength(0);
    expect(screen.getAllByRole('link', { name: /Telemetry v2/i })).not.toHaveLength(0);
    expect(screen.queryByRole('button', { name: /Administration/i })).toBeNull();
    expect(screen.queryAllByRole('link', { name: /Enroll Device/i })).toHaveLength(0);
    expect(screen.queryAllByRole('link', { name: /^Simulator$/i })).toHaveLength(0);
  });

  it('hides admin and simulator links for support agent', () => {
    useAuthMock.mockReturnValue(buildAuthContext(SUPPORT_AGENT_CAPS));
    renderLayout('/live/device-inventory');

    expect(screen.getAllByRole('link', { name: /Device Inventory/i })).not.toHaveLength(0);
    expect(screen.queryByRole('button', { name: /Administration/i })).toBeNull();
    expect(screen.queryAllByRole('link', { name: /Enroll Device/i })).toHaveLength(0);
    expect(screen.queryAllByRole('link', { name: /^Simulator$/i })).toHaveLength(0);
  });

  it('exposes provisioning tools to authority manager roles', () => {
    useAuthMock.mockReturnValue(buildAuthContext(AUTHORITY_MANAGER_CAPS));
    renderLayout('/devices/enroll');

    expect(screen.getAllByRole('link', { name: /Enroll Device/i })).not.toHaveLength(0);
    expect(
      screen.getAllByRole('link', { name: /Manage Government Credentials/i }),
    ).not.toHaveLength(0);
    expect(screen.getAllByRole('link', { name: /Installations/i })).not.toHaveLength(0);
    expect(screen.getAllByRole('link', { name: /Import CSVs/i })).not.toHaveLength(0);
    expect(screen.queryByRole('button', { name: /Administration/i })).toBeNull();
  });
});
