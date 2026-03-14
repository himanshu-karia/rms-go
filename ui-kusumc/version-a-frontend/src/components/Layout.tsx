import { useEffect, useState } from 'react';
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom';

import { useAuth } from '../auth';
import { useActiveProject } from '../activeProject';
import { SidebarNav } from './SidebarNav';
import { SessionIdleBanner, SessionStatusIndicators } from './SessionStatusWidgets';
import type { CapabilityKey, CapabilityMatchMode } from '../api/capabilities';

type CapabilityChecker = (
  required: CapabilityKey | CapabilityKey[],
  options?: { match?: CapabilityMatchMode },
) => boolean;

type NavLinkConfig = {
  id: string;
  label: string;
  to: string;
  exact?: boolean;
  requiredCapabilities?: CapabilityKey | CapabilityKey[];
  match?: CapabilityMatchMode;
};

type NavGroupConfig = {
  id: string;
  label: string;
  items: NavLinkConfig[];
  requiredCapabilities?: CapabilityKey | CapabilityKey[];
  match?: CapabilityMatchMode;
};

type SidebarLinkShape = {
  id: string;
  label: string;
  to: string;
  exact?: boolean;
};

type SidebarGroupShape = {
  id: string;
  label: string;
  items: SidebarLinkShape[];
};

const AUTHENTICATED_STANDALONE_LINKS: NavLinkConfig[] = [
  {
    id: 'dashboard',
    label: 'RMS Dashboard',
    to: '/',
    exact: true,
    requiredCapabilities: ['telemetry:read', 'admin:all'],
    match: 'any',
  },
];

const UNAUTHENTICATED_STANDALONE_LINKS: NavLinkConfig[] = [
  { id: 'simulator', label: 'Simulator', to: '/simulator' },
  { id: 'login', label: 'Login', to: '/login', exact: true },
];

const NAV_GROUP_CONFIGS: NavGroupConfig[] = [
  {
    id: 'administration',
    label: 'Administration',
    requiredCapabilities: [
      'admin:all',
      'hierarchy:manage',
      'users:manage',
      'vendors:manage',
      'catalog:protocols',
      'catalog:drives',
    ],
    match: 'any',
    items: [
      {
        id: 'admin-orgs',
        label: 'Organizations',
        to: '/admin/orgs',
        requiredCapabilities: 'hierarchy:manage',
      },
      {
        id: 'admin-apikeys',
        label: 'API Keys',
        to: '/admin/apikeys',
        requiredCapabilities: 'hierarchy:manage',
      },
      {
        id: 'admin-audit',
        label: 'Audit Logs',
        to: '/admin/audit',
        requiredCapabilities: 'admin:all',
      },
      {
        id: 'admin-scheduler',
        label: 'Scheduler',
        to: '/admin/scheduler',
        requiredCapabilities: 'admin:all',
      },
      {
        id: 'admin-dna',
        label: 'DNA Config',
        to: '/admin/dna',
        requiredCapabilities: ['admin:all', 'hierarchy:manage'],
        match: 'any',
      },
      {
        id: 'admin-device-profiles',
        label: 'Device Profiles',
        to: '/admin/device-profiles',
        requiredCapabilities: 'admin:all',
      },
      {
        id: 'admin-simulator-sessions',
        label: 'Simulator Sessions',
        to: '/admin/simulator-sessions',
        requiredCapabilities: 'simulator:launch',
      },
      {
        id: 'admin-states',
        label: 'States',
        to: '/admin/states',
        requiredCapabilities: 'hierarchy:manage',
      },
      {
        id: 'admin-authorities',
        label: 'State Authorities',
        to: '/admin/state-authorities',
        requiredCapabilities: 'hierarchy:manage',
      },
      {
        id: 'admin-projects',
        label: 'Projects',
        to: '/admin/projects',
        requiredCapabilities: 'hierarchy:manage',
      },
      {
        id: 'admin-user-groups',
        label: 'User Groups',
        to: '/admin/user-groups',
        requiredCapabilities: 'users:manage',
      },
      {
        id: 'admin-users',
        label: 'Users',
        to: '/admin/users',
        requiredCapabilities: 'users:manage',
      },
      {
        id: 'admin-server-vendors',
        label: 'Server Vendors',
        to: '/admin/server-vendors',
        requiredCapabilities: 'vendors:manage',
      },
      {
        id: 'admin-protocols',
        label: 'Protocol Versions',
        to: '/admin/protocol-versions',
        requiredCapabilities: 'catalog:protocols',
      },
      {
        id: 'admin-drive-manufacturers',
        label: 'Drive Manufacturers',
        to: '/admin/drive-manufacturers',
        requiredCapabilities: 'catalog:drives',
      },
      {
        id: 'admin-pump-vendors',
        label: 'Pump Vendors',
        to: '/admin/pump-vendors',
        requiredCapabilities: 'vendors:manage',
      },
      {
        id: 'admin-rms-manufacturers',
        label: 'RMS Manufacturers',
        to: '/admin/rms-manufacturers',
        requiredCapabilities: 'vendors:manage',
      },
      {
        id: 'admin-vfd-models',
        label: 'VFD Drive Models',
        to: '/admin/vfd-models',
        requiredCapabilities: 'catalog:drives',
      },
      {
        id: 'admin-vfd-ops',
        label: 'VFD Catalog Ops',
        to: '/admin/vfd-catalog-ops',
        requiredCapabilities: 'catalog:drives',
      },
    ],
  },
  {
    id: 'operations',
    label: 'Operations',
    items: [
      {
        id: 'ops-enroll-device',
        label: 'Enroll Device',
        to: '/devices/enroll',
        requiredCapabilities: 'devices:write',
      },
      {
        id: 'ops-government-credentials',
        label: 'Manage Government Credentials',
        to: '/devices/configuration/government',
        exact: true,
        requiredCapabilities: 'devices:credentials',
      },
      {
        id: 'ops-internal-credentials',
        label: 'Internal Credentials',
        to: '/devices/configuration/internal',
        exact: true,
        requiredCapabilities: 'devices:credentials',
      },
      {
        id: 'ops-rms-drive-config',
        label: 'RMS & Drive Config',
        to: '/devices/configuration/drive',
        exact: true,
        requiredCapabilities: 'devices:credentials',
      },
      {
        id: 'ops-installations',
        label: 'Installations',
        to: '/operations/installations',
        requiredCapabilities: ['installations:manage', 'beneficiaries:manage'],
        match: 'all',
      },
      {
        id: 'ops-import-csvs',
        label: 'Import CSVs',
        to: '/devices/import',
        requiredCapabilities: 'devices:bulk_import',
      },
      {
        id: 'ops-command-center',
        label: 'Command Center',
        to: '/operations/command-center',
        requiredCapabilities: 'devices:commands',
      },
      {
        id: 'ops-command-catalog',
        label: 'Command Catalog',
        to: '/operations/command-catalog',
        requiredCapabilities: 'devices:commands',
      },
      {
        id: 'ops-rules-alerts',
        label: 'Rules / Alerts',
        to: '/operations/rules-alerts',
        requiredCapabilities: ['alerts:manage', 'admin:all'],
        match: 'any',
      },
      {
        id: 'ops-automation',
        label: 'Automation',
        to: '/operations/automation',
        requiredCapabilities: ['alerts:manage', 'admin:all'],
        match: 'any',
      },
      {
        id: 'ops-import-history',
        label: 'Import History',
        to: '/devices/import/jobs',
        requiredCapabilities: 'devices:bulk_import',
      },
    ],
  },
  {
    id: 'live',
    label: 'Live',
    items: [
      {
        id: 'live-device-inventory',
        label: 'Device Inventory',
        to: '/live/device-inventory',
        requiredCapabilities: 'devices:read',
      },
      {
        id: 'live-telemetry',
        label: 'Telemetry Monitor',
        to: '/telemetry',
        exact: true,
        requiredCapabilities: 'telemetry:read',
      },
      {
        id: 'live-telemetry-export',
        label: 'Telemetry Export',
        to: '/telemetry/export',
        requiredCapabilities: 'telemetry:export',
      },
      {
        id: 'live-telemetry-v2',
        label: 'Telemetry v2',
        to: '/telemetry/v2',
        requiredCapabilities: 'telemetry:read',
      },
      {
        id: 'live-reports',
        label: 'Reports',
        to: '/reports',
        requiredCapabilities: ['reports:manage', 'admin:all'],
        match: 'any',
      },
      {
        id: 'live-simulator',
        label: 'Simulator',
        to: '/simulator',
        requiredCapabilities: 'simulator:launch',
      },
    ],
  },
];

function canAccess(
  checker: CapabilityChecker,
  required?: CapabilityKey | CapabilityKey[],
  match?: CapabilityMatchMode,
) {
  if (!required) {
    return true;
  }

  const options = match ? { match } : undefined;
  return checker(required, options);
}

function filterNavLinks(links: NavLinkConfig[], checker: CapabilityChecker): NavLinkConfig[] {
  return links.filter((link) => canAccess(checker, link.requiredCapabilities, link.match));
}

function toSidebarLinks(links: NavLinkConfig[]): SidebarLinkShape[] {
  return links.map(({ id, label, to, exact }) => ({ id, label, to, exact }));
}

export function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { isAuthenticated, user, logout, hasCapability } = useAuth();
  const { activeProjectId, setActiveProjectId, clearActiveProjectId } = useActiveProject();
  const [projectIdInput, setProjectIdInput] = useState(activeProjectId);
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = useState(false);

  const capabilityChecker: CapabilityChecker = hasCapability;

  const standaloneLinks: SidebarLinkShape[] = isAuthenticated
    ? toSidebarLinks(filterNavLinks(AUTHENTICATED_STANDALONE_LINKS, capabilityChecker))
    : toSidebarLinks(UNAUTHENTICATED_STANDALONE_LINKS);

  const navGroups: SidebarGroupShape[] = isAuthenticated
    ? NAV_GROUP_CONFIGS.map((group) => {
        if (!canAccess(capabilityChecker, group.requiredCapabilities, group.match)) {
          return null;
        }

        const filteredItems = filterNavLinks(group.items, capabilityChecker);
        if (!filteredItems.length) {
          return null;
        }

        return {
          id: group.id,
          label: group.label,
          items: toSidebarLinks(filteredItems),
        } satisfies SidebarGroupShape;
      }).filter((group): group is SidebarGroupShape => Boolean(group))
    : [];

  async function handleLogout() {
    setIsLoggingOut(true);
    try {
      await logout();
      navigate('/login');
    } finally {
      setIsLoggingOut(false);
    }
  }

  useEffect(() => {
    setIsMobileSidebarOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    setProjectIdInput(activeProjectId);
  }, [activeProjectId]);

  return (
    <div className="min-h-screen bg-slate-100 text-slate-900">
      {isAuthenticated && (
        <>
          <div
            className={`fixed inset-0 z-30 bg-slate-900/50 transition-opacity md:hidden ${
              isMobileSidebarOpen ? 'opacity-100' : 'pointer-events-none opacity-0'
            }`}
            aria-hidden={!isMobileSidebarOpen}
            onClick={() => setIsMobileSidebarOpen(false)}
          />
          <div
            className={`fixed inset-y-0 left-0 z-40 w-72 bg-white shadow-xl transition-transform duration-200 md:hidden ${
              isMobileSidebarOpen ? 'translate-x-0' : '-translate-x-full'
            }`}
          >
            <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
              <span className="text-sm font-semibold text-slate-700">Navigation</span>
              <button
                type="button"
                onClick={() => setIsMobileSidebarOpen(false)}
                className="flex size-9 items-center justify-center rounded-full border border-slate-300 text-slate-600 transition-colors hover:bg-slate-100"
              >
                <CloseIcon />
                <span className="sr-only">Close navigation</span>
              </button>
            </div>
            <div className="h-[calc(100%-3.5rem)] overflow-y-auto px-4 pb-6 pt-4">
              <SidebarNav
                standaloneLinks={standaloneLinks}
                groups={navGroups}
                onNavigate={() => setIsMobileSidebarOpen(false)}
              />
            </div>
          </div>
        </>
      )}
      <div className="mx-auto flex min-h-screen w-full max-w-[1440px]">
        {isAuthenticated && (
          <aside className="hidden w-64 border-r border-slate-200 bg-white md:block">
            <div className="h-full overflow-y-auto px-4">
              <SidebarNav standaloneLinks={standaloneLinks} groups={navGroups} />
            </div>
          </aside>
        )}
        <div className="flex min-h-screen flex-1 flex-col">
          <header className="border-b border-slate-200 bg-white">
            <div className="flex items-center justify-between px-6 py-4">
              <div className="flex items-center gap-3">
                {isAuthenticated && (
                  <button
                    type="button"
                    onClick={() => setIsMobileSidebarOpen(true)}
                    className="flex size-10 items-center justify-center rounded-md border border-slate-300 text-slate-600 transition-colors hover:bg-slate-100 md:hidden"
                  >
                    <HamburgerIcon />
                    <span className="sr-only">Open navigation</span>
                  </button>
                )}
                <h1 className="text-xl font-semibold">PM KUSUM RMS</h1>
                {isAuthenticated && <p className="text-xs text-slate-500">{location.pathname}</p>}
              </div>
              <div className="flex flex-col items-end gap-2 sm:flex-row sm:items-center">
                {isAuthenticated && (
                  <div className="hidden sm:block">
                    <SessionStatusIndicators />
                  </div>
                )}
                {isAuthenticated ? (
                  <div className="hidden items-center gap-2 rounded border border-slate-200 bg-slate-50 px-3 py-2 sm:flex">
                    <span className="text-xs font-semibold text-slate-600">Project</span>
                    <input
                      value={projectIdInput}
                      onChange={(e) => setProjectIdInput(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault();
                          setActiveProjectId(projectIdInput);
                        }
                      }}
                      className="w-44 rounded border border-slate-300 bg-white px-2 py-1 text-xs text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                      placeholder="e.g. rms-pump-01"
                      aria-label="Active project id"
                    />
                    <button
                      type="button"
                      onClick={() => setActiveProjectId(projectIdInput)}
                      className="rounded bg-emerald-600 px-2 py-1 text-xs font-semibold text-white hover:bg-emerald-700"
                    >
                      Set
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setProjectIdInput('');
                        clearActiveProjectId();
                      }}
                      className="rounded border border-slate-300 bg-white px-2 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100"
                      disabled={!activeProjectId}
                    >
                      Clear
                    </button>
                  </div>
                ) : null}
                {isAuthenticated && user ? (
                  <>
                    <span className="text-sm text-slate-500">
                      Signed in as{' '}
                      <span className="font-medium text-slate-700">{user.displayName}</span>
                    </span>
                    <button
                      type="button"
                      className="rounded bg-slate-800 px-3 py-1 text-sm font-medium text-white transition-colors hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-70"
                      onClick={handleLogout}
                      disabled={isLoggingOut}
                    >
                      {isLoggingOut ? 'Signing out…' : 'Sign out'}
                    </button>
                  </>
                ) : (
                  <div className="flex items-center gap-3 text-sm">
                    <Link
                      to="/simulator"
                      className="rounded border border-slate-300 px-3 py-1 font-medium text-slate-600 hover:bg-slate-100"
                    >
                      Simulator
                    </Link>
                    <Link
                      to="/login"
                      className="rounded bg-emerald-600 px-3 py-1 font-medium text-white hover:bg-emerald-500"
                    >
                      Login
                    </Link>
                  </div>
                )}
              </div>
            </div>
            {isAuthenticated && (
              <div className="px-6 pb-3 sm:hidden">
                <SessionStatusIndicators />
              </div>
            )}
          </header>
          {isAuthenticated && <SessionIdleBanner />}
          <main className="flex-1 overflow-y-auto px-6 py-8">
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  );
}

function HamburgerIcon() {
  return (
    <svg
      aria-hidden="true"
      className="size-5"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
    >
      <path d="M4 6h16" />
      <path d="M4 12h16" />
      <path d="M4 18h16" />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg
      aria-hidden="true"
      className="size-5"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
    >
      <path d="m6 6 12 12" />
      <path d="m18 6-12 12" />
    </svg>
  );
}
