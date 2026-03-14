import { FormEvent, useMemo, useState } from 'react';
import { useMutation } from '@tanstack/react-query';

import { fetchAuditEvents, type AuditEvent, type AuditListResponse } from '../../../api/audit';
import { useAuth } from '../../../auth';
import { useActiveProject } from '../../../activeProject';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';

function safeString(value: unknown): string {
  if (value == null) return '';
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

export function AuditSection() {
  const { hasCapability } = useAuth();
  const { activeProjectId } = useActiveProject();
  const canRead = hasCapability(['audit:read', 'admin:all'], { match: 'any' });

  const [filters, setFilters] = useState(() => ({
    projectId: activeProjectId ?? '',
    actorId: '',
    action: '',
    limit: 100,
  }));

  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);

  const loadMutation = useMutation<AuditListResponse, Error, { mode: 'reset' | 'append' }>({
    mutationFn: async ({ mode }) => {
      const afterId = mode === 'append' ? nextCursor ?? undefined : undefined;
      return fetchAuditEvents({
        limit: filters.limit,
        afterId,
        projectId: filters.projectId.trim() || undefined,
        actorId: filters.actorId.trim() || undefined,
        action: filters.action.trim() || undefined,
      });
    },
    onSuccess: (res, vars) => {
      setStatus(null);
      setFormError(null);
      setNextCursor(res.nextCursor);
      setEvents((prev) => (vars.mode === 'append' ? [...prev, ...(res.events ?? [])] : res.events ?? []));
    },
    onError: (err) => {
      setStatus({ type: 'error', message: err.message ?? 'Unable to load audit logs.' });
    },
  });

  const canLoadMore = Boolean(nextCursor) && !loadMutation.isPending;

  const metrics = useMemo(
    () => ({ loaded: events.length, hasMore: Boolean(nextCursor) }),
    [events.length, nextCursor],
  );

  function handleLoad(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    setFormError(null);

    if (!canRead) {
      setFormError('Requires audit:read capability.');
      return;
    }

    setEvents([]);
    setNextCursor(null);
    loadMutation.mutate({ mode: 'reset' });
  }

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-xl font-semibold text-slate-900">Audit Logs</h2>
        <p className="text-sm text-slate-600">Query server audit events for investigations and compliance.</p>
      </header>

      <AdminStatusBanner status={status} />

      <form onSubmit={handleLoad} className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="grid gap-4 md:grid-cols-4">
          <label className="flex flex-col gap-2 text-sm md:col-span-2">
            <span className="font-medium text-slate-800">Project ID (optional)</span>
            <input
              value={filters.projectId}
              onChange={(e) => setFilters((p) => ({ ...p, projectId: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. rms-pump-01"
              disabled={!canRead}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Actor ID (optional)</span>
            <input
              value={filters.actorId}
              onChange={(e) => setFilters((p) => ({ ...p, actorId: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="user uuid"
              disabled={!canRead}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Action (optional)</span>
            <input
              value={filters.action}
              onChange={(e) => setFilters((p) => ({ ...p, action: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. devices.create"
              disabled={!canRead}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Limit</span>
            <select
              value={String(filters.limit)}
              onChange={(e) => setFilters((p) => ({ ...p, limit: Number(e.target.value) }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canRead}
            >
              <option value="50">50</option>
              <option value="100">100</option>
              <option value="200">200</option>
            </select>
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canRead || loadMutation.isPending}
            >
              {loadMutation.isPending ? 'Loading…' : 'Load'}
            </button>
          </div>
        </div>

        {formError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {formError}
          </p>
        ) : null}

        <p className="mt-3 text-xs text-slate-500">
          Loaded: {metrics.loaded} event(s){metrics.hasMore ? ' • more available' : ''}
        </p>
      </form>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Events</h3>
          <button
            type="button"
            onClick={() => loadMutation.mutate({ mode: 'reset' })}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={!canRead || loadMutation.isPending}
          >
            {loadMutation.isPending ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-100 text-slate-700">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold">Created</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Action</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Actor</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Metadata</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {events.map((evt) => (
                <tr key={evt.id}>
                  <td className="px-3 py-2 text-xs text-slate-700">{String(evt.createdAt ?? '—')}</td>
                  <td className="px-3 py-2">{String(evt.action ?? '—')}</td>
                  <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">
                    {String(evt.actor?.id ?? 'system')}
                  </td>
                  <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">
                    {safeString(evt.metadata ?? '').slice(0, 220) || '—'}
                  </td>
                </tr>
              ))}
              {!events.length ? (
                <tr>
                  <td className="px-3 py-3 text-sm text-slate-500" colSpan={4}>
                    No events loaded.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <div className="mt-4 flex justify-end">
          <button
            type="button"
            onClick={() => loadMutation.mutate({ mode: 'append' })}
            className="rounded border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={!canRead || !canLoadMore}
          >
            Load more
          </button>
        </div>
      </section>
    </section>
  );
}
