import { Link, useLocation } from 'react-router-dom';
import { useMemo, useState } from 'react';

type SidebarLink = {
  id: string;
  label: string;
  to: string;
  exact?: boolean;
};

type SidebarGroup = {
  id: string;
  label: string;
  items: SidebarLink[];
};

type SidebarNavProps = {
  standaloneLinks: SidebarLink[];
  groups: SidebarGroup[];
  onNavigate?: () => void;
};

function parseLinkTarget(to: string): { pathname: string; search: string } {
  try {
    const url = new URL(to, window.location.origin);
    return { pathname: url.pathname, search: url.search };
  } catch {
    const queryIndex = to.indexOf('?');
    const hashIndex = to.indexOf('#');
    let end = to.length;
    if (queryIndex >= 0) {
      end = Math.min(end, queryIndex);
    }
    if (hashIndex >= 0) {
      end = Math.min(end, hashIndex);
    }

    const search =
      queryIndex >= 0
        ? to.slice(queryIndex, hashIndex >= 0 && hashIndex > queryIndex ? hashIndex : undefined)
        : '';

    return {
      pathname: to.slice(0, end),
      search,
    };
  }
}

function normalizeSearch(search: string): string {
  if (!search || search === '?') {
    return '';
  }

  const normalized = search.startsWith('?') ? search.slice(1) : search;
  if (!normalized) {
    return '';
  }

  const params = new URLSearchParams(normalized);
  const entries = Array.from(params.entries()).sort(([aKey, aValue], [bKey, bValue]) => {
    const keyCompare = aKey.localeCompare(bKey);
    if (keyCompare !== 0) {
      return keyCompare;
    }
    return aValue.localeCompare(bValue);
  });

  return entries.map(([key, value]) => `${key}=${value}`).join('&');
}

function ChevronIcon({ expanded }: { expanded: boolean }) {
  return (
    <span aria-hidden="true" className="text-xs font-semibold">
      {expanded ? 'v' : '>'}
    </span>
  );
}

function SidebarNavLink({ link, onNavigate }: { link: SidebarLink; onNavigate?: () => void }) {
  const location = useLocation();

  const target = useMemo(() => parseLinkTarget(link.to), [link.to]);
  const locationSearchKey = useMemo(
    () => normalizeSearch(location.search ?? ''),
    [location.search],
  );
  const targetSearchKey = useMemo(() => normalizeSearch(target.search), [target.search]);

  const isActive = useMemo(() => {
    if (link.exact) {
      return location.pathname === target.pathname && locationSearchKey === targetSearchKey;
    }
    return location.pathname.startsWith(target.pathname);
  }, [link.exact, location.pathname, locationSearchKey, target.pathname, targetSearchKey]);

  const className = `block rounded-md px-3 py-2 text-sm font-medium transition-colors ${
    isActive
      ? 'bg-emerald-600 text-white shadow-sm'
      : 'text-slate-500 hover:bg-slate-100 hover:text-slate-700'
  }`;

  return (
    <Link
      to={link.to}
      onClick={onNavigate}
      className={className}
      aria-current={isActive ? 'page' : undefined}
    >
      {link.label}
    </Link>
  );
}

function extractPathname(to: string): string {
  return parseLinkTarget(to).pathname;
}

export function SidebarNav({ standaloneLinks, groups, onNavigate }: SidebarNavProps) {
  const location = useLocation();
  const [openGroupId, setOpenGroupId] = useState<string | null>(null);

  const activeGroupId = useMemo(() => {
    for (const group of groups) {
      if (group.items.some((item) => location.pathname.startsWith(extractPathname(item.to)))) {
        return group.id;
      }
    }
    return null;
  }, [groups, location.pathname]);

  const resolvedOpenGroupId = openGroupId ?? activeGroupId;

  return (
    <nav className="flex h-full flex-col gap-6 py-6">
      {standaloneLinks.length > 0 && (
        <div className="flex flex-col gap-1">
          {standaloneLinks.map((link) => (
            <SidebarNavLink key={link.id} link={link} onNavigate={onNavigate} />
          ))}
        </div>
      )}

      {groups.map((group) => {
        const isOpen = resolvedOpenGroupId === group.id;
        const hasActiveChild = group.items.some((item) =>
          location.pathname.startsWith(extractPathname(item.to)),
        );

        return (
          <div key={group.id} className="flex flex-col gap-2">
            <button
              type="button"
              onClick={() => setOpenGroupId((current) => (current === group.id ? null : group.id))}
              aria-expanded={isOpen}
              className={`flex items-center justify-between rounded-md px-3 py-2 text-sm font-semibold transition-colors ${
                hasActiveChild
                  ? 'bg-emerald-50 text-emerald-700'
                  : 'text-slate-600 hover:bg-slate-100'
              }`}
            >
              <span>{group.label}</span>
              <ChevronIcon expanded={isOpen} />
            </button>

            {isOpen && (
              <div className="flex flex-col gap-1 pl-3">
                {group.items.map((item) => (
                  <SidebarNavLink key={item.id} link={item} onNavigate={onNavigate} />
                ))}
              </div>
            )}
          </div>
        );
      })}
    </nav>
  );
}
