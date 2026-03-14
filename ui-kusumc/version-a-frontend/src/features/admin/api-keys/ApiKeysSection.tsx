import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  createApiKey,
  fetchApiKeys,
  revokeApiKey,
  type ApiKeyRecord,
  type ApiKeyCreateResponse,
} from '../../../api/apiKeys';
import { useAuth } from '../../../auth';
import { useActiveProject } from '../../../activeProject';
import { IconActionButton } from '../../../components/IconActionButton';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';

type ApiKeyFormState = {
  name: string;
  scopesCsv: string;
  projectId: string;
  orgId: string;
};

export function ApiKeysSection() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const { activeProjectId } = useActiveProject();
  const canManage = hasCapability('hierarchy:manage') || hasCapability('admin:all');

  const [form, setForm] = useState<ApiKeyFormState>(() => ({
    name: '',
    scopesCsv: '',
    projectId: activeProjectId ?? '',
    orgId: '',
  }));
  const [formError, setFormError] = useState<string | null>(null);
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const keysQuery = useQuery<ApiKeyRecord[], Error>({
    queryKey: ['admin', 'apikeys'],
    queryFn: fetchApiKeys,
    enabled: canManage,
  });

  const keys = useMemo(() => keysQuery.data ?? [], [keysQuery.data]);

  const createMutation = useMutation<ApiKeyCreateResponse, Error, { name: string; scopes: string[]; project_id?: string | null; org_id?: string | null }>({
    mutationFn: (payload) => createApiKey(payload),
    onSuccess: (res) => {
      setCreatedSecret(res.secret);
      setStatus({ type: 'success', message: 'API key created. Copy the secret now — it will not be shown again.' });
      queryClient.invalidateQueries({ queryKey: ['admin', 'apikeys'] });
      setForm((prev) => ({ ...prev, name: '' }));
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create API key.' });
    },
  });

  const revokeMutation = useMutation<void, Error, { id: string }>({
    mutationFn: ({ id }) => revokeApiKey(id),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'API key revoked.' });
      await queryClient.invalidateQueries({ queryKey: ['admin', 'apikeys'] });
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to revoke API key.' });
    },
    onSettled: () => {
      setRevokingId(null);
    },
  });

  function parseScopes(csv: string): string[] {
    return csv
      .split(',')
      .map((v) => v.trim())
      .filter((v) => v.length > 0);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);
    setStatus(null);
    setCreatedSecret(null);

    if (!canManage) {
      setFormError('Requires hierarchy:manage capability.');
      return;
    }

    if (!form.name.trim()) {
      setFormError('Name is required.');
      return;
    }

    const scopes = parseScopes(form.scopesCsv);

    createMutation.mutate({
      name: form.name.trim(),
      scopes,
      project_id: form.projectId.trim() ? form.projectId.trim() : null,
      org_id: form.orgId.trim() ? form.orgId.trim() : null,
    });
  }

  function handleRevoke(id: string) {
    setStatus(null);
    setFormError(null);
    setCreatedSecret(null);
    setRevokingId(id);
    revokeMutation.mutate({ id });
  }

  const busy = createMutation.isPending;

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-xl font-semibold text-slate-900">API Keys</h2>
        <p className="text-sm text-slate-600">Create and revoke API keys used for northbound ingestion and automation clients.</p>
      </header>

      <AdminStatusBanner status={status} />

      {createdSecret ? (
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
          <p className="font-semibold">New API key secret</p>
          <p className="mt-2 break-all font-mono text-xs">{createdSecret}</p>
        </div>
      ) : null}

      <form onSubmit={handleSubmit} className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Create API key</h3>

        <div className="mt-4 grid gap-4 md:grid-cols-2">
          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Name</span>
            <input
              value={form.name}
              onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. Timescale Ingest Client"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Scopes (comma-separated)</span>
            <input
              value={form.scopesCsv}
              onChange={(e) => setForm((prev) => ({ ...prev, scopesCsv: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. ingest:write, telemetry:read"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Project ID (optional)</span>
            <input
              value={form.projectId}
              onChange={(e) => setForm((prev) => ({ ...prev, projectId: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="defaults to global scope"
              disabled={!canManage || busy}
            />
            <p className="text-xs text-slate-500">Prefilled from the Active Project selector when available.</p>
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Org ID (optional)</span>
            <input
              value={form.orgId}
              onChange={(e) => setForm((prev) => ({ ...prev, orgId: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="uuid"
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
            {busy ? 'Creating…' : 'Create API key'}
          </button>
        </div>
      </form>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Existing keys</h3>
          <button
            type="button"
            onClick={() => keysQuery.refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={!keysQuery.isFetched || keysQuery.isFetching}
          >
            {keysQuery.isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {keysQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {keysQuery.error instanceof Error ? keysQuery.error.message : 'Unable to load API keys'}
          </p>
        ) : null}

        <p className="mt-2 text-xs text-slate-500">Loaded: {keys.length} key(s).</p>

        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-100 text-slate-700">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold">ID</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Name</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Prefix</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Project</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Org</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Active</th>
                <th className="px-3 py-2 text-right text-xs font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {keys.map((key) => (
                <tr key={key.id}>
                  <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">{key.id}</td>
                  <td className="px-3 py-2">{key.name}</td>
                  <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">{key.prefix}</td>
                  <td className="px-3 py-2 text-xs text-slate-700">{String(key.project_id ?? '—')}</td>
                  <td className="px-3 py-2 text-xs text-slate-700">{String(key.org_id ?? '—')}</td>
                  <td className="px-3 py-2">{key.is_active ? 'Yes' : 'No'}</td>
                  <td className="px-3 py-2 text-right">
                    <IconActionButton
                      label="Revoke API key"
                      onClick={() => handleRevoke(key.id)}
                      disabled={!canManage || !key.is_active || revokingId === key.id}
                      variant="danger"
                    />
                  </td>
                </tr>
              ))}
              {!keys.length ? (
                <tr>
                  <td className="px-3 py-3 text-sm text-slate-500" colSpan={7}>
                    No API keys found.
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
