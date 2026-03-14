import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import {
  buildVfdModelsExportUrl,
  importVfdCommandDictionary,
  importVfdModelsCsv,
  listVfdCommandImportJobs,
  type VfdCommandImportJob,
} from '../api/vfd';
import { useActiveProject } from '../activeProject';
import { AdminStatusBanner, type AdminStatusMessage } from '../features/admin/components/AdminStatusBanner';
import { downloadWithAuth } from '../api/download';

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

export function AdminVfdCatalogOpsPage() {
  const { activeProjectId } = useActiveProject();

  const [projectId, setProjectId] = useState(activeProjectId ?? '');
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);

  const [importModelsForm, setImportModelsForm] = useState({
    manufacturerId: '',
    protocolVersionId: '',
    csv: '',
  });

  const [importCommandsForm, setImportCommandsForm] = useState({
    modelId: '',
    mergeStrategy: 'replace',
    mode: 'json' as 'json' | 'csv',
    json: '[]',
    csv: '',
  });

  const exportUrl = useMemo(() => {
    try {
      return projectId.trim() ? buildVfdModelsExportUrl(projectId) : '';
    } catch {
      return '';
    }
  }, [projectId]);

  const jobsQuery = useQuery<{ jobs: VfdCommandImportJob[]; count: number }, Error>({
    queryKey: ['admin', 'vfd', 'import-jobs', projectId],
    queryFn: () => listVfdCommandImportJobs({ projectId, limit: 25 }),
    enabled: Boolean(projectId.trim()),
    refetchOnWindowFocus: false,
  });

  const importModelsMutation = useMutation({
    mutationFn: () =>
      importVfdModelsCsv({
        projectId,
        manufacturerId: importModelsForm.manufacturerId.trim() || undefined,
        protocolVersionId: importModelsForm.protocolVersionId.trim() || undefined,
        csv: importModelsForm.csv,
      }),
    onSuccess: async (result) => {
      setStatus({ type: 'success', message: `Imported ${result.count} model(s).` });
      await jobsQuery.refetch();
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to import VFD models.' });
    },
  });

  const importCommandsMutation = useMutation({
    mutationFn: async () => {
      if (!importCommandsForm.modelId.trim()) {
        throw new Error('modelId is required');
      }
      if (importCommandsForm.mode === 'json') {
        const parsed = safeJsonParse(importCommandsForm.json);
        if (!parsed.ok) throw new Error(parsed.error);
        if (!Array.isArray(parsed.value)) {
          throw new Error('Command dictionary JSON must be an array');
        }
        return importVfdCommandDictionary({
          projectId,
          modelId: importCommandsForm.modelId.trim(),
          mergeStrategy: importCommandsForm.mergeStrategy.trim() || 'replace',
          json: importCommandsForm.json,
        });
      }

      if (!importCommandsForm.csv.trim()) {
        throw new Error('CSV payload is required');
      }

      return importVfdCommandDictionary({
        projectId,
        modelId: importCommandsForm.modelId.trim(),
        mergeStrategy: importCommandsForm.mergeStrategy.trim() || 'replace',
        csv: importCommandsForm.csv,
      });
    },
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Command dictionary import completed.' });
      await jobsQuery.refetch();
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to import command dictionary.' });
    },
  });

  function downloadExport() {
    setStatus(null);
    if (!exportUrl) return;
    downloadWithAuth({
      url: exportUrl,
      filenameFallback: 'vfd-models.csv',
      accept: 'text/csv',
    }).catch((e: unknown) => {
      const message = e instanceof Error ? e.message : 'Download failed';
      setStatus({ type: 'error', message });
    });
  }

  function submitModels(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    if (!projectId.trim()) {
      setStatus({ type: 'error', message: 'projectId is required.' });
      return;
    }
    if (!importModelsForm.csv.trim()) {
      setStatus({ type: 'error', message: 'CSV payload is required.' });
      return;
    }
    importModelsMutation.mutate();
  }

  function submitCommands(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    if (!projectId.trim()) {
      setStatus({ type: 'error', message: 'projectId is required.' });
      return;
    }
    importCommandsMutation.mutate();
  }

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-800">VFD Catalog Ops</h2>
        <p className="text-sm text-slate-600">Export/import VFD models and command dictionaries for a project.</p>
      </header>

      <AdminStatusBanner status={status} />

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <label className="flex flex-col gap-1 text-sm">
          <span className="font-medium text-slate-700">Project ID</span>
          <input
            value={projectId}
            onChange={(e) => setProjectId(e.target.value)}
            className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            placeholder="Set Active Project to autofill"
          />
        </label>

        <div className="mt-4 flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={downloadExport}
            className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
            disabled={!exportUrl}
          >
            Export models CSV
          </button>
          {exportUrl ? (
            <p className="break-all font-mono text-[0.7rem] text-slate-500">{exportUrl}</p>
          ) : null}
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Import VFD models</h3>
        <p className="mt-1 text-xs text-slate-500">
          Backend expects a JSON payload with a CSV string. CSV headers can include manufacturer_id/model/version/rs485/etc.
        </p>

        <form onSubmit={submitModels} className="mt-4 grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Manufacturer ID (optional)</span>
            <input
              value={importModelsForm.manufacturerId}
              onChange={(e) => setImportModelsForm((p) => ({ ...p, manufacturerId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Protocol Version ID (optional)</span>
            <input
              value={importModelsForm.protocolVersionId}
              onChange={(e) => setImportModelsForm((p) => ({ ...p, protocolVersionId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
            />
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="w-full rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={importModelsMutation.isPending}
            >
              {importModelsMutation.isPending ? 'Importing…' : 'Import models'}
            </button>
          </div>

          <label className="flex flex-col gap-1 text-sm md:col-span-3">
            <span className="font-medium text-slate-700">CSV (string)</span>
            <textarea
              value={importModelsForm.csv}
              onChange={(e) => setImportModelsForm((p) => ({ ...p, csv: e.target.value }))}
              className="h-48 w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
              spellCheck={false}
              placeholder="id,manufacturer_id,model,version,rs485,realtime_parameters,fault_map,command_dictionary,metadata"
            />
          </label>
        </form>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Import command dictionary</h3>

        <form onSubmit={submitCommands} className="mt-4 grid gap-4 md:grid-cols-4">
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="font-medium text-slate-700">Model ID</span>
            <input
              value={importCommandsForm.modelId}
              onChange={(e) => setImportCommandsForm((p) => ({ ...p, modelId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="required"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Merge strategy</span>
            <input
              value={importCommandsForm.mergeStrategy}
              onChange={(e) => setImportCommandsForm((p) => ({ ...p, mergeStrategy: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
              placeholder="replace/merge"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Mode</span>
            <select
              value={importCommandsForm.mode}
              onChange={(e) => setImportCommandsForm((p) => ({ ...p, mode: e.target.value as any }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
            >
              <option value="json">JSON array</option>
              <option value="csv">CSV string</option>
            </select>
          </label>

          {importCommandsForm.mode === 'json' ? (
            <label className="flex flex-col gap-1 text-sm md:col-span-4">
              <span className="font-medium text-slate-700">JSON</span>
              <textarea
                value={importCommandsForm.json}
                onChange={(e) => setImportCommandsForm((p) => ({ ...p, json: e.target.value }))}
                className="h-48 w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
                spellCheck={false}
              />
            </label>
          ) : (
            <label className="flex flex-col gap-1 text-sm md:col-span-4">
              <span className="font-medium text-slate-700">CSV</span>
              <textarea
                value={importCommandsForm.csv}
                onChange={(e) => setImportCommandsForm((p) => ({ ...p, csv: e.target.value }))}
                className="h-48 w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
                spellCheck={false}
              />
            </label>
          )}

          <div className="flex items-end md:col-span-4">
            <button
              type="submit"
              className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={importCommandsMutation.isPending}
            >
              {importCommandsMutation.isPending ? 'Importing…' : 'Import command dictionary'}
            </button>
          </div>
        </form>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Import jobs</h3>
          <button
            type="button"
            onClick={() => jobsQuery.refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={jobsQuery.isFetching}
          >
            {jobsQuery.isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {jobsQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {jobsQuery.error.message}
          </p>
        ) : null}

        <pre className="mt-3 max-h-80 overflow-auto rounded border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
          {jobsQuery.data ? jsonPretty(jobsQuery.data) : jobsQuery.isLoading ? 'Loading…' : '—'}
        </pre>
      </section>
    </div>
  );
}
