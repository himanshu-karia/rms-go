import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { createOrg, fetchOrgs, updateOrg, type OrgRecord } from '../../../api/orgs';
import { useAuth } from '../../../auth';
import { IconActionButton } from '../../../components/IconActionButton';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';
import { parseMetadata, stringifyMetadata } from '../utils/metadata';

type OrgFormState = {
  name: string;
  type: string;
  path: string;
  parentId: string;
  metadata: string;
};

const emptyForm: OrgFormState = {
  name: '',
  type: '',
  path: '',
  parentId: '',
  metadata: '',
};

export function OrgsSection() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const canManage = hasCapability('hierarchy:manage') || hasCapability('admin:all');

  const [form, setForm] = useState<OrgFormState>(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);

  const orgsQuery = useQuery<OrgRecord[], Error>({
    queryKey: ['admin', 'orgs'],
    queryFn: fetchOrgs,
    enabled: canManage,
  });

  const orgs = useMemo(() => orgsQuery.data ?? [], [orgsQuery.data]);

  const createMutation = useMutation<OrgRecord, Error, ReturnType<typeof buildPayload>>({
    mutationFn: (payload) => createOrg(payload),
    onSuccess: (org) => {
      setStatus({ type: 'success', message: `Organization "${org.name}" created.` });
      queryClient.invalidateQueries({ queryKey: ['admin', 'orgs'] });
      resetForm();
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create organization.' });
    },
  });

  const updateMutation = useMutation<OrgRecord, Error, { id: string; payload: ReturnType<typeof buildPayload> }>({
    mutationFn: ({ id, payload }) => updateOrg({ id, payload }),
    onSuccess: (org) => {
      setStatus({ type: 'success', message: `Organization "${org.name}" updated.` });
      queryClient.invalidateQueries({ queryKey: ['admin', 'orgs'] });
      resetForm();
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to update organization.' });
    },
  });

  function resetForm() {
    setForm(emptyForm);
    setEditingId(null);
    setFormError(null);
  }

  function buildPayload() {
    const { metadata, error } = parseMetadata(form.metadata);
    if (error) {
      throw new Error(error);
    }

    return {
      name: form.name.trim(),
      type: form.type.trim(),
      path: form.path.trim(),
      parent_id: form.parentId.trim() ? form.parentId.trim() : null,
      metadata,
    };
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);
    setStatus(null);

    if (!canManage) {
      setFormError('Requires hierarchy:manage capability.');
      return;
    }

    if (!form.name.trim() || !form.type.trim()) {
      setFormError('Name and type are required.');
      return;
    }

    let payload: ReturnType<typeof buildPayload>;
    try {
      payload = buildPayload();
    } catch (e) {
      setFormError(e instanceof Error ? e.message : 'Invalid metadata');
      return;
    }

    if (editingId) {
      updateMutation.mutate({ id: editingId, payload });
      return;
    }

    createMutation.mutate(payload);
  }

  function handleEdit(org: OrgRecord) {
    setEditingId(org.id);
    setStatus(null);
    setFormError(null);
    setForm({
      name: org.name ?? '',
      type: org.type ?? '',
      path: String(org.path ?? ''),
      parentId: String(org.parent_id ?? org.parentId ?? ''),
      metadata: stringifyMetadata((org.metadata as Record<string, unknown> | null | undefined) ?? null),
    });
  }

  const busy = createMutation.isPending || updateMutation.isPending;

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-xl font-semibold text-slate-900">Organizations</h2>
        <p className="text-sm text-slate-600">Manage organizations used for hierarchy and API key scoping.</p>
      </header>

      <AdminStatusBanner status={status} />

      <form onSubmit={handleSubmit} className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">{editingId ? 'Edit organization' : 'Create organization'}</h3>
          {editingId ? (
            <button
              type="button"
              onClick={resetForm}
              className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100"
            >
              Cancel
            </button>
          ) : null}
        </div>

        <div className="mt-4 grid gap-4 md:grid-cols-2">
          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Name</span>
            <input
              value={form.name}
              onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. State Department"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Type</span>
            <input
              value={form.type}
              onChange={(e) => setForm((prev) => ({ ...prev, type: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. gov, vendor"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Path (optional)</span>
            <input
              value={form.path}
              onChange={(e) => setForm((prev) => ({ ...prev, path: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. state.maharashtra"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Parent org ID (optional)</span>
            <input
              value={form.parentId}
              onChange={(e) => setForm((prev) => ({ ...prev, parentId: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="uuid"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm md:col-span-2">
            <span className="font-medium text-slate-800">Metadata (JSON)</span>
            <textarea
              value={form.metadata}
              onChange={(e) => setForm((prev) => ({ ...prev, metadata: e.target.value }))}
              rows={4}
              className="rounded border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder='e.g. {"category":"server"}'
              disabled={!canManage || busy}
            />
          </label>
        </div>

        {formError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {formError}
          </p>
        ) : null}

        <div className="mt-4">
          <button
            type="submit"
            className="inline-flex items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={!canManage || busy}
          >
            {busy ? 'Saving…' : editingId ? 'Update organization' : 'Create organization'}
          </button>
        </div>
      </form>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Existing organizations</h3>
          <button
            type="button"
            onClick={() => orgsQuery.refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={!orgsQuery.isFetched || orgsQuery.isFetching}
          >
            {orgsQuery.isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {orgsQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {orgsQuery.error instanceof Error ? orgsQuery.error.message : 'Unable to load organizations'}
          </p>
        ) : null}

        <p className="mt-2 text-xs text-slate-500">Loaded: {orgs.length} org(s).</p>

        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-100 text-slate-700">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold">ID</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Name</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Type</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Path</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Parent</th>
                <th className="px-3 py-2 text-right text-xs font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {orgs.map((org) => (
                <tr key={org.id}>
                  <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">{org.id}</td>
                  <td className="px-3 py-2">{org.name}</td>
                  <td className="px-3 py-2">{org.type}</td>
                  <td className="px-3 py-2 text-xs text-slate-700">{String(org.path ?? '—')}</td>
                  <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">{String(org.parent_id ?? org.parentId ?? '—')}</td>
                  <td className="px-3 py-2 text-right">
                    <IconActionButton
                      label="Edit organization"
                      onClick={() => handleEdit(org)}
                      disabled={!canManage}
                    />
                  </td>
                </tr>
              ))}
              {!orgs.length ? (
                <tr>
                  <td className="px-3 py-3 text-sm text-slate-500" colSpan={6}>
                    No organizations found.
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
