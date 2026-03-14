import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  createSchedule,
  fetchSchedules,
  toggleSchedule,
  type SchedulerRecord,
} from '../../../api/scheduler';
import { useAuth } from '../../../auth';
import { useActiveProject } from '../../../activeProject';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';

function describeJson(value: unknown): string {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

export function SchedulerSection() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const { activeProjectId } = useActiveProject();

  const canManage = hasCapability(['reports:manage', 'admin:all'], { match: 'any' });

  const [form, setForm] = useState(() => ({
    projectId: activeProjectId ?? '',
    time: '',
    commandJson: '{\n  "cmd": "pump_on"\n}',
  }));

  const [formError, setFormError] = useState<string | null>(null);
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  const schedulesQuery = useQuery<SchedulerRecord[], Error>({
    queryKey: ['admin', 'scheduler', 'schedules'],
    queryFn: fetchSchedules,
    enabled: canManage,
  });

  const schedules = useMemo(() => schedulesQuery.data ?? [], [schedulesQuery.data]);

  const createMutation = useMutation<void, Error, { project_id: string; time?: string; command: unknown }>({
    mutationFn: (payload) => createSchedule(payload),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Schedule created.' });
      setFormError(null);
      await queryClient.invalidateQueries({ queryKey: ['admin', 'scheduler', 'schedules'] });
    },
    onError: (err) => {
      setStatus({ type: 'error', message: err.message ?? 'Unable to create schedule.' });
    },
  });

  const toggleMutation = useMutation<void, Error, { id: string }>({
    mutationFn: ({ id }) => toggleSchedule(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['admin', 'scheduler', 'schedules'] });
    },
    onError: (err) => {
      setStatus({ type: 'error', message: err.message ?? 'Unable to toggle schedule.' });
    },
    onSettled: () => {
      setTogglingId(null);
    },
  });

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus(null);
    setFormError(null);

    if (!canManage) {
      setFormError('Requires reports:manage capability.');
      return;
    }

    const project_id = form.projectId.trim();
    if (!project_id) {
      setFormError('Project ID is required.');
      return;
    }

    if (!form.time.trim()) {
      setFormError('Time is required.');
      return;
    }

    let command: unknown;
    try {
      command = JSON.parse(form.commandJson);
    } catch (e) {
      setFormError(e instanceof Error ? e.message : 'Invalid command JSON');
      return;
    }

    createMutation.mutate({ project_id, time: form.time, command });
  }

  function handleToggle(id: string) {
    setStatus(null);
    setFormError(null);
    setTogglingId(id);
    toggleMutation.mutate({ id });
  }

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-xl font-semibold text-slate-900">Scheduler</h2>
        <p className="text-sm text-slate-600">Create and toggle scheduled commands.</p>
      </header>

      <AdminStatusBanner status={status} />

      <form onSubmit={handleSubmit} className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Create schedule</h3>

        <div className="mt-4 grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-2 text-sm md:col-span-1">
            <span className="font-medium text-slate-800">Project ID</span>
            <input
              value={form.projectId}
              onChange={(e) => setForm((p) => ({ ...p, projectId: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. rms-pump-01"
              disabled={!canManage || createMutation.isPending}
            />
            <p className="text-xs text-slate-500">Prefilled from the Active Project selector when available.</p>
          </label>

          <label className="flex flex-col gap-2 text-sm md:col-span-1">
            <span className="font-medium text-slate-800">Time</span>
            <input
              value={form.time}
              onChange={(e) => setForm((p) => ({ ...p, time: e.target.value }))}
              type="datetime-local"
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || createMutation.isPending}
            />
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canManage || createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating…' : 'Create'}
            </button>
          </div>

          <label className="flex flex-col gap-2 text-sm md:col-span-3">
            <span className="font-medium text-slate-800">Command (JSON)</span>
            <textarea
              value={form.commandJson}
              onChange={(e) => setForm((p) => ({ ...p, commandJson: e.target.value }))}
              rows={5}
              className="rounded border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || createMutation.isPending}
            />
          </label>
        </div>

        {formError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {formError}
          </p>
        ) : null}
      </form>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Schedules</h3>
          <button
            type="button"
            onClick={() => schedulesQuery.refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={!schedulesQuery.isFetched || schedulesQuery.isFetching}
          >
            {schedulesQuery.isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {schedulesQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {schedulesQuery.error instanceof Error ? schedulesQuery.error.message : 'Unable to load schedules'}
          </p>
        ) : null}

        <p className="mt-2 text-xs text-slate-500">Loaded: {schedules.length} schedule(s).</p>

        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-100 text-slate-700">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold">ID</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Project</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Time</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Active</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Command</th>
                <th className="px-3 py-2 text-right text-xs font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {schedules.map((sch, idx) => {
                const id = String(sch.id ?? sch.ID ?? `row-${idx}`);
                const active = Boolean(sch.is_active ?? sch.isActive);
                return (
                  <tr key={id}>
                    <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">{id}</td>
                    <td className="px-3 py-2 text-xs text-slate-700">{String(sch.project_id ?? sch.projectId ?? '—')}</td>
                    <td className="px-3 py-2 text-xs text-slate-700">{String(sch.time ?? '—')}</td>
                    <td className="px-3 py-2">{active ? 'Yes' : 'No'}</td>
                    <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">{describeJson(sch.command ?? '—').slice(0, 200)}</td>
                    <td className="px-3 py-2 text-right">
                      <button
                        type="button"
                        onClick={() => handleToggle(id)}
                        className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
                        disabled={!canManage || togglingId === id}
                      >
                        {togglingId === id ? 'Toggling…' : 'Toggle'}
                      </button>
                    </td>
                  </tr>
                );
              })}
              {!schedules.length ? (
                <tr>
                  <td className="px-3 py-3 text-sm text-slate-500" colSpan={6}>
                    No schedules found.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </section>
  );
}
