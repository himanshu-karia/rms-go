import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import {
  createSimulatorSession,
  listSimulatorSessions,
  revokeSimulatorSession,
  type SimulatorSessionRecord,
} from '../api/simulatorSessions';
import { AdminStatusBanner, type AdminStatusMessage } from '../features/admin/components/AdminStatusBanner';

export function AdminSimulatorSessionsPage() {
  const [filters, setFilters] = useState<{ status: string; limit: string }>({ status: '', limit: '25' });
  const [cursor, setCursor] = useState<string | null>(null);

  const [createForm, setCreateForm] = useState<{ deviceUuid: string; expiresInMinutes: string }>({
    deviceUuid: '',
    expiresInMinutes: '60',
  });

  const [statusBanner, setStatusBanner] = useState<AdminStatusMessage | null>(null);

  const limitNumber = Math.max(1, Math.min(100, Number(filters.limit) || 25));

  const sessionsQuery = useQuery({
    queryKey: ['admin', 'simulator-sessions', { ...filters, cursor, limitNumber }],
    queryFn: () =>
      listSimulatorSessions({
        status: filters.status.trim() || undefined,
        limit: limitNumber,
        cursor,
      }),
    refetchOnWindowFocus: false,
  });

  const sessions = useMemo<SimulatorSessionRecord[]>(
    () => sessionsQuery.data?.sessions ?? [],
    [sessionsQuery.data?.sessions],
  );

  const createMutation = useMutation({
    mutationFn: () =>
      createSimulatorSession({
        deviceUuid: createForm.deviceUuid.trim(),
        expiresInMinutes: Number(createForm.expiresInMinutes) || 60,
      }),
    onSuccess: async () => {
      setStatusBanner({ type: 'success', message: 'Simulator session created.' });
      setCreateForm((p) => ({ ...p, deviceUuid: '' }));
      await sessionsQuery.refetch();
    },
    onError: (error: Error) => {
      setStatusBanner({ type: 'error', message: error.message ?? 'Unable to create session.' });
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (sessionId: string) => revokeSimulatorSession(sessionId),
    onSuccess: async () => {
      setStatusBanner({ type: 'success', message: 'Session revoked.' });
      await sessionsQuery.refetch();
    },
    onError: (error: Error) => {
      setStatusBanner({ type: 'error', message: error.message ?? 'Unable to revoke session.' });
    },
  });

  function applyFilters(event: FormEvent) {
    event.preventDefault();
    setStatusBanner(null);
    setCursor(null);
    sessionsQuery.refetch();
  }

  function submitCreate(event: FormEvent) {
    event.preventDefault();
    setStatusBanner(null);

    if (!createForm.deviceUuid.trim()) {
      setStatusBanner({ type: 'error', message: 'deviceUuid is required.' });
      return;
    }

    createMutation.mutate();
  }

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-800">Simulator Sessions</h2>
        <p className="text-sm text-slate-600">Create, list, and revoke device simulator sessions.</p>
      </header>

      <AdminStatusBanner status={statusBanner} />

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Create session</h3>
        <form onSubmit={submitCreate} className="mt-4 grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="font-medium text-slate-700">Device UUID</span>
            <input
              value={createForm.deviceUuid}
              onChange={(e) => setCreateForm((p) => ({ ...p, deviceUuid: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="device UUID"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Expires (minutes)</span>
            <input
              type="number"
              min={5}
              max={480}
              value={createForm.expiresInMinutes}
              onChange={(e) => setCreateForm((p) => ({ ...p, expiresInMinutes: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>
          <div className="flex items-end md:col-span-3">
            <button
              type="submit"
              className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Sessions</h3>
          <button
            type="button"
            onClick={() => sessionsQuery.refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={sessionsQuery.isFetching}
          >
            {sessionsQuery.isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        <form onSubmit={applyFilters} className="mt-4 grid gap-4 md:grid-cols-4">
          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Status</span>
            <select
              value={filters.status}
              onChange={(e) => setFilters((p) => ({ ...p, status: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
            >
              <option value="">All</option>
              <option value="active">Active</option>
              <option value="revoked">Revoked</option>
              <option value="expired">Expired</option>
            </select>
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Limit</span>
            <input
              type="number"
              min={1}
              max={100}
              value={filters.limit}
              onChange={(e) => setFilters((p) => ({ ...p, limit: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="w-full rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500"
            >
              Apply
            </button>
          </div>
        </form>

        {sessionsQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {sessionsQuery.error instanceof Error ? sessionsQuery.error.message : 'Unable to load sessions'}
          </p>
        ) : null}

        <div className="mt-4 overflow-x-auto rounded border border-slate-200">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-3 py-2">ID</th>
                <th className="px-3 py-2">Device</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Expires</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 bg-white">
              {sessions.map((s) => (
                <tr key={s.id}>
                  <td className="px-3 py-2 font-mono text-[0.8rem] text-slate-700">{s.id}</td>
                  <td className="px-3 py-2 font-mono text-[0.8rem] text-slate-700">{s.deviceUuid}</td>
                  <td className="px-3 py-2 text-xs text-slate-600">{s.status}</td>
                  <td className="px-3 py-2 text-xs text-slate-600">
                    {s.expiresAt ? new Date(s.expiresAt).toLocaleString() : '—'}
                  </td>
                  <td className="px-3 py-2 text-right">
                    <button
                      type="button"
                      onClick={() => revokeMutation.mutate(s.id)}
                      className="rounded border border-rose-200 bg-rose-50 px-3 py-1 text-xs font-semibold text-rose-700 hover:bg-rose-100 disabled:opacity-60"
                      disabled={revokeMutation.isPending}
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              ))}
              {!sessions.length ? (
                <tr>
                  <td colSpan={5} className="px-3 py-6 text-center text-sm text-slate-500">
                    {sessionsQuery.isLoading ? 'Loading…' : 'No sessions found.'}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        {sessionsQuery.data?.nextCursor ? (
          <div className="mt-4 flex gap-2">
            <button
              type="button"
              onClick={() => setCursor(sessionsQuery.data?.nextCursor ?? null)}
              className="rounded border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            >
              Next page
            </button>
          </div>
        ) : null}
      </section>
    </div>
  );
}
