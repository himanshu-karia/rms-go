import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import {
  deleteCommandCatalogItem,
  listCommandCatalogAdmin,
  upsertCommandCatalog,
  type CommandCatalogItem,
} from '../api/commandCatalog';
import { useActiveProject } from '../activeProject';
import { AdminStatusBanner, type AdminStatusMessage } from '../features/admin/components/AdminStatusBanner';

function safeJsonParse(input: string): { ok: true; value: unknown } | { ok: false; error: string } {
  try {
    return { ok: true, value: JSON.parse(input) };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : 'Invalid JSON' };
  }
}

function jsonPretty(value: unknown) {
  return JSON.stringify(value, null, 2);
}

type FiltersState = {
  projectId: string;
  deviceId: string;
};

type EditorState = {
  id: string;
  name: string;
  scope: string;
  transport: string;
  protocolId: string;
  modelId: string;
  projectId: string;
  deviceIds: string;
  payloadSchema: string;
};

function emptyEditor(projectId?: string): EditorState {
  return {
    id: '',
    name: '',
    scope: 'project',
    transport: 'mqtt',
    protocolId: '',
    modelId: '',
    projectId: projectId ?? '',
    deviceIds: '',
    payloadSchema: '{\n  "type": "object",\n  "properties": {}\n}',
  };
}

export function CommandCatalogAdminPage() {
  const { activeProjectId } = useActiveProject();

  const [filters, setFilters] = useState<FiltersState>(() => ({
    projectId: activeProjectId ?? '',
    deviceId: '',
  }));

  const [editor, setEditor] = useState<EditorState>(() => emptyEditor(activeProjectId ?? ''));
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);

  const queryKey = useMemo(() => ['ops', 'command-catalog-admin', filters] as const, [filters]);

  const catalogQuery = useQuery<CommandCatalogItem[], Error>({
    queryKey,
    queryFn: () => listCommandCatalogAdmin(filters),
    enabled: Boolean(filters.projectId.trim()) && Boolean(filters.deviceId.trim()),
    refetchOnWindowFocus: false,
  });

  const upsertMutation = useMutation({
    mutationFn: async () => {
      const parsed = safeJsonParse(editor.payloadSchema);
      if (!parsed.ok) throw new Error(parsed.error);
      if (!parsed.value || typeof parsed.value !== 'object' || Array.isArray(parsed.value)) {
        throw new Error('payloadSchema must be a JSON object');
      }

      const deviceIds = editor.deviceIds
        .split(',')
        .map((d) => d.trim())
        .filter(Boolean);

      return upsertCommandCatalog({
        id: editor.id.trim() || undefined,
        name: editor.name.trim(),
        scope: editor.scope.trim(),
        transport: editor.transport.trim(),
        protocolId: editor.protocolId.trim() || null,
        modelId: editor.modelId.trim() || null,
        projectId: editor.projectId.trim() || null,
        deviceIds: deviceIds.length ? deviceIds : undefined,
        payloadSchema: parsed.value as Record<string, unknown>,
      });
    },
    onSuccess: async (result) => {
      setStatus({ type: 'success', message: `Saved catalog item ${result.id}.` });
      await catalogQuery.refetch();
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to save catalog item.' });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteCommandCatalogItem(id),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Catalog item deleted.' });
      await catalogQuery.refetch();
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to delete catalog item.' });
    },
  });

  function onLoad(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    catalogQuery.refetch();
  }

  function loadIntoEditor(item: CommandCatalogItem) {
    setStatus(null);
    setEditor({
      id: item.id ?? '',
      name: item.name ?? '',
      scope: item.scope ?? 'project',
      transport: item.transport ?? 'mqtt',
      protocolId: (item as any).protocolId ?? '',
      modelId: (item as any).modelId ?? '',
      projectId: (item as any).projectId ?? filters.projectId,
      deviceIds: Array.isArray((item as any).deviceIds) ? (item as any).deviceIds.join(', ') : '',
      payloadSchema: jsonPretty((item as any).payloadSchema ?? {}),
    });
  }

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-800">Command Catalog (Admin)</h2>
        <p className="text-sm text-slate-600">
          Author command definitions. Listing requires both projectId and deviceId to scope capabilities.
        </p>
      </header>

      <AdminStatusBanner status={status} />

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <form onSubmit={onLoad} className="grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="font-medium text-slate-700">Project ID</span>
            <input
              value={filters.projectId}
              onChange={(e) => setFilters((p) => ({ ...p, projectId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="Set Active Project to autofill"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Device ID / IMEI</span>
            <input
              value={filters.deviceId}
              onChange={(e) => setFilters((p) => ({ ...p, deviceId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="required"
            />
          </label>
          <div className="flex items-end md:col-span-3">
            <button
              type="submit"
              className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={!filters.projectId.trim() || !filters.deviceId.trim() || catalogQuery.isFetching}
            >
              {catalogQuery.isFetching ? 'Loading…' : 'Load catalog'}
            </button>
          </div>
        </form>

        {catalogQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {catalogQuery.error.message}
          </p>
        ) : null}

        <div className="mt-4 overflow-x-auto rounded border border-slate-200">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-3 py-2">Name</th>
                <th className="px-3 py-2">Scope</th>
                <th className="px-3 py-2">Transport</th>
                <th className="px-3 py-2">ID</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 bg-white">
              {(catalogQuery.data ?? []).map((item) => (
                <tr key={item.id}>
                  <td className="px-3 py-2 font-medium text-slate-800">{item.name}</td>
                  <td className="px-3 py-2 text-xs text-slate-600">{item.scope}</td>
                  <td className="px-3 py-2 text-xs text-slate-600">{item.transport}</td>
                  <td className="px-3 py-2 font-mono text-[0.8rem] text-slate-700">{item.id}</td>
                  <td className="px-3 py-2 text-right">
                    <div className="flex justify-end gap-2">
                      <button
                        type="button"
                        onClick={() => loadIntoEditor(item)}
                        className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        onClick={() => deleteMutation.mutate(item.id)}
                        className="rounded border border-rose-200 bg-rose-50 px-3 py-1 text-xs font-semibold text-rose-700 hover:bg-rose-100 disabled:opacity-60"
                        disabled={deleteMutation.isPending}
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {!catalogQuery.data?.length ? (
                <tr>
                  <td colSpan={5} className="px-3 py-6 text-center text-sm text-slate-500">
                    {catalogQuery.isLoading ? 'Loading…' : 'No catalog items loaded.'}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Upsert catalog item</h3>

        <div className="mt-4 grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">ID (optional)</span>
            <input
              value={editor.id}
              onChange={(e) => setEditor((p) => ({ ...p, id: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="font-medium text-slate-700">Name</span>
            <input
              value={editor.name}
              onChange={(e) => setEditor((p) => ({ ...p, name: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Scope</span>
            <input
              value={editor.scope}
              onChange={(e) => setEditor((p) => ({ ...p, scope: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
              placeholder="project / model / protocol / device"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Transport</span>
            <input
              value={editor.transport}
              onChange={(e) => setEditor((p) => ({ ...p, transport: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
              placeholder="mqtt"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">ProjectId (optional)</span>
            <input
              value={editor.projectId}
              onChange={(e) => setEditor((p) => ({ ...p, projectId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">ProtocolId (optional)</span>
            <input
              value={editor.protocolId}
              onChange={(e) => setEditor((p) => ({ ...p, protocolId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">ModelId (optional)</span>
            <input
              value={editor.modelId}
              onChange={(e) => setEditor((p) => ({ ...p, modelId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm md:col-span-3">
            <span className="font-medium text-slate-700">Device IDs (optional, comma-separated)</span>
            <input
              value={editor.deviceIds}
              onChange={(e) => setEditor((p) => ({ ...p, deviceIds: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="uuid1, uuid2"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm md:col-span-3">
            <span className="font-medium text-slate-700">Payload schema (JSON)</span>
            <textarea
              value={editor.payloadSchema}
              onChange={(e) => setEditor((p) => ({ ...p, payloadSchema: e.target.value }))}
              className="h-64 w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
              spellCheck={false}
            />
          </label>

          <div className="flex gap-2 md:col-span-3">
            <button
              type="button"
              onClick={() => {
                const parsed = safeJsonParse(editor.payloadSchema);
                if (parsed.ok) {
                  setEditor((p) => ({ ...p, payloadSchema: jsonPretty(parsed.value) }));
                  setStatus({ type: 'success', message: 'Formatted payloadSchema.' });
                } else {
                  setStatus({ type: 'error', message: parsed.error });
                }
              }}
              className="rounded border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            >
              Format JSON
            </button>
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                upsertMutation.mutate();
              }}
              className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={upsertMutation.isPending}
            >
              {upsertMutation.isPending ? 'Saving…' : 'Save'}
            </button>
            <button
              type="button"
              onClick={() => setEditor(emptyEditor(filters.projectId))}
              className="rounded border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            >
              New
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
