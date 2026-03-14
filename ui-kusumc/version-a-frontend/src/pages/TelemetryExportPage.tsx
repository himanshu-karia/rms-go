import { FormEvent, useMemo, useState } from 'react';

import { API_BASE_URL } from '../api/config';
import { downloadWithAuth } from '../api/download';
import { useActiveProject } from '../activeProject';

type ExportFormState = {
  projectId: string;
  imei: string;
  start: string;
  end: string;
  packetType: string;
  quality: string;
  excludeQuality: string;
  format: 'csv' | 'xlsx' | 'pdf';
};

function buildExportUrl(state: ExportFormState): string {
  const query = new URLSearchParams();

  if (state.imei.trim()) query.set('imei', state.imei.trim());
  if (state.projectId.trim()) query.set('projectId', state.projectId.trim());

  if (state.start.trim()) query.set('start', state.start.trim());
  if (state.end.trim()) query.set('end', state.end.trim());

  if (state.packetType.trim()) query.set('packetType', state.packetType.trim());
  if (state.quality.trim()) query.set('quality', state.quality.trim());
  if (state.excludeQuality.trim()) query.set('exclude_quality', state.excludeQuality.trim());

  query.set('format', state.format);

  return `${API_BASE_URL}/telemetry/export?${query.toString()}`;
}

export function TelemetryExportPage() {
  const { activeProjectId } = useActiveProject();

  const [form, setForm] = useState<ExportFormState>(() => ({
    projectId: activeProjectId ?? '',
    imei: '',
    start: '',
    end: '',
    packetType: '',
    quality: '',
    excludeQuality: '',
    format: 'csv',
  }));

  const [error, setError] = useState<string | null>(null);

  const exportUrl = useMemo(() => buildExportUrl(form), [form]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);

    if (!form.imei.trim() && !form.projectId.trim()) {
      setError('Provide IMEI or Project ID.');
      return;
    }

    downloadWithAuth({
      url: exportUrl,
      filenameFallback: `telemetry_export.${form.format}`,
      accept:
        form.format === 'pdf'
          ? 'application/pdf'
          : form.format === 'xlsx'
            ? 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
            : 'text/csv',
    }).catch((e: unknown) => {
      const message = e instanceof Error ? e.message : 'Download failed';
      setError(message);
    });
  }

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-800">Telemetry Export</h2>
        <p className="text-sm text-slate-600">Download telemetry as CSV/XLSX/PDF with basic filters.</p>
      </header>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <form className="grid gap-4 md:grid-cols-3" onSubmit={onSubmit}>
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="font-medium text-slate-700">Project ID</span>
            <input
              value={form.projectId}
              onChange={(e) => setForm((p) => ({ ...p, projectId: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="defaults to Active Project"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">IMEI (optional)</span>
            <input
              value={form.imei}
              onChange={(e) => setForm((p) => ({ ...p, imei: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="device IMEI"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Start (optional)</span>
            <input
              value={form.start}
              onChange={(e) => setForm((p) => ({ ...p, start: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="e.g. 2026-02-18T00:00:00Z"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">End (optional)</span>
            <input
              value={form.end}
              onChange={(e) => setForm((p) => ({ ...p, end: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="e.g. 2026-02-18T23:59:59Z"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Packet type (optional)</span>
            <input
              value={form.packetType}
              onChange={(e) => setForm((p) => ({ ...p, packetType: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
              placeholder="topic suffix / packet type"
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Quality (optional)</span>
            <input
              value={form.quality}
              onChange={(e) => setForm((p) => ({ ...p, quality: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
              placeholder=""
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Exclude quality (optional)</span>
            <input
              value={form.excludeQuality}
              onChange={(e) => setForm((p) => ({ ...p, excludeQuality: e.target.value }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
              placeholder=""
            />
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Format</span>
            <select
              value={form.format}
              onChange={(e) => setForm((p) => ({ ...p, format: e.target.value as ExportFormState['format'] }))}
              className="rounded border border-slate-300 px-3 py-2 text-sm"
            >
              <option value="csv">CSV</option>
              <option value="xlsx">XLSX</option>
              <option value="pdf">PDF</option>
            </select>
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="w-full rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500"
            >
              Download
            </button>
          </div>
        </form>

        {error ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {error}
          </p>
        ) : null}

        <div className="mt-4 rounded border border-slate-200 bg-slate-50 p-3">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Request URL</p>
          <p className="mt-1 break-all font-mono text-xs text-slate-700">{exportUrl}</p>
        </div>
      </section>
    </div>
  );
}
