import { expect, test as base } from '@playwright/test';
import type { Page } from '@playwright/test';

import type { CapabilityKey } from '../../src/api/capabilities';

const ONE_HOUR_MS = 60 * 60 * 1000;

type RoleProfile = {
  username: string;
  displayName: string;
  roleKey: string;
  roleName: string;
  capabilities: CapabilityKey[];
  scope?: Record<string, unknown> | null;
};

export type RoleName = 'superAdmin' | 'operationsManager' | 'telemetryViewer';

const roleProfiles: Record<RoleName, RoleProfile> = {
  superAdmin: {
    username: 'suryadev',
    displayName: 'Surya Dev (Super Admin)',
    roleKey: 'super_admin',
    roleName: 'Super Admin',
    capabilities: [
      'admin:all',
      'telemetry:read',
      'telemetry:live:all',
      'telemetry:live:device',
      'devices:read',
      'devices:write',
      'devices:credentials',
      'devices:bulk_import',
      'alerts:manage',
      'reports:manage',
      'simulator:launch',
      'hierarchy:manage',
      'vendors:manage',
      'catalog:protocols',
      'catalog:drives',
      'installations:manage',
      'beneficiaries:manage',
      'users:manage',
      'audit:read',
    ],
  },
  operationsManager: {
    username: 'ops_rani',
    displayName: 'Ops Rani',
    roleKey: 'operations_manager',
    roleName: 'Operations Manager',
    capabilities: [
      'telemetry:read',
      'devices:read',
      'devices:write',
      'devices:credentials',
      'devices:bulk_import',
      'installations:manage',
      'beneficiaries:manage',
      'simulator:launch',
    ],
    scope: { stateId: 'RJ' },
  },
  telemetryViewer: {
    username: 'viewer_amit',
    displayName: 'Viewer Amit',
    roleKey: 'telemetry_viewer',
    roleName: 'Telemetry Viewer',
    capabilities: ['telemetry:read'],
  },
};

function buildIntrospectionResponse(profile: RoleProfile) {
  const issuedAt = new Date(Date.now() - 30_000).toISOString();
  const expiresAt = new Date(Date.now() + ONE_HOUR_MS).toISOString();

  return {
    session: {
      id: `session-${profile.username}`,
      issuedAt,
      expiresAt,
      remainingSeconds: Math.floor((Date.parse(expiresAt) - Date.now()) / 1000),
    },
    user: {
      id: profile.username,
      username: profile.username,
      displayName: profile.displayName,
      capabilities: profile.capabilities,
      mustRotatePassword: false,
      roles: [
        {
          id: `role-${profile.roleKey}`,
          key: profile.roleKey,
          name: profile.roleName,
          capabilities: profile.capabilities,
          scope: profile.scope ?? null,
        },
      ],
    },
  };
}

function buildLoginResponse(profile: RoleProfile) {
  const accessExpiresAt = new Date(Date.now() + ONE_HOUR_MS).toISOString();
  const refreshExpiresAt = new Date(Date.now() + ONE_HOUR_MS * 12).toISOString();

  return {
    user: {
      username: profile.username,
      displayName: profile.displayName,
      capabilities: profile.capabilities,
    },
    session: {
      id: `session-${profile.username}`,
    },
    tokens: {
      access: {
        token: `access-${profile.username}`,
        expiresAt: accessExpiresAt,
      },
      refresh: {
        token: `refresh-${profile.username}`,
        expiresAt: refreshExpiresAt,
      },
    },
  };
}

function buildSessionSnapshot(profile: RoleProfile) {
  const expiresAt = new Date(Date.now() + ONE_HOUR_MS).toISOString();

  return {
    token: `access-${profile.username}`,
    username: profile.username,
    displayName: profile.displayName,
    expiresAt,
    sessionId: `session-${profile.username}`,
    capabilities: profile.capabilities,
  };
}

function buildDeviceListResponse() {
  return {
    devices: [
      {
        uuid: 'device-test-001',
        imei: '359868000000001',
        status: 'active',
        configurationStatus: 'ready',
        connectivityStatus: 'online',
        connectivityUpdatedAt: new Date().toISOString(),
        lastTelemetryAt: new Date().toISOString(),
        lastHeartbeatAt: new Date().toISOString(),
        offlineThresholdMs: 4 * ONE_HOUR_MS,
        offlineNotificationChannelCount: 1,
        protocolVersion: {
          id: 'proto-1',
          version: '1.0.0',
          name: 'Baseline',
        },
      },
    ],
    pagination: {
      total: 1,
      limit: 50,
      includeInactive: false,
      status: null,
    },
  };
}

function jsonResponse(body: unknown, status = 200) {
  return {
    status,
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
  } as const;
}

async function performUiLogin(page: Page, profile: RoleProfile) {
  const snapshot = buildSessionSnapshot(profile);
  await page.goto('/login');
  await page.evaluate((value) => {
    window.localStorage.setItem('pmkusum.session.v1', JSON.stringify(value));
  }, snapshot);
}

async function registerApiStubs(page: Page, getProfile: () => RoleProfile | null) {
  await page.route('**/api/**', async (route, request) => {
    const url = new URL(request.url());
    const path = url.pathname.replace(/.*\/api/, '') || '/';
    const profile = getProfile();

    if (path.startsWith('/auth/login') && request.method() === 'POST') {
      if (!profile) {
        await route.fulfill(jsonResponse({ message: 'No role selected' }, 400));
        return;
      }
      await route.fulfill(jsonResponse(buildLoginResponse(profile)));
      return;
    }

    if (path.startsWith('/auth/logout') && request.method() === 'POST') {
      await route.fulfill(jsonResponse({ success: true }));
      return;
    }

    if (path.startsWith('/auth/refresh') && request.method() === 'POST') {
      if (!profile) {
        await route.fulfill(jsonResponse({ message: 'Not authenticated' }, 401));
        return;
      }
      await route.fulfill(jsonResponse(buildLoginResponse(profile)));
      return;
    }

    if (path.startsWith('/auth/session')) {
      if (!profile) {
        await route.fulfill(jsonResponse({ message: 'Not authenticated' }, 401));
        return;
      }
      await route.fulfill(jsonResponse(buildIntrospectionResponse(profile)));
      return;
    }

    if (path.startsWith('/devices') && request.method() === 'GET') {
      await route.fulfill(jsonResponse(buildDeviceListResponse()));
      return;
    }

    await route.fulfill(jsonResponse({}));
  });
}

export const test = base.extend<{ authenticateAs: (role: RoleName) => Promise<void> }>({
  authenticateAs: async ({ page }, use) => {
    let activeProfile: RoleProfile | null = null;
    await registerApiStubs(page, () => activeProfile);

    await use(async (role: RoleName) => {
      const profile = roleProfiles[role];
      activeProfile = profile;
      await performUiLogin(page, profile);
    });
  },
});

export { expect };
