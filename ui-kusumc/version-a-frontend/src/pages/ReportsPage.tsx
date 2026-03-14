import { FormEvent, useMemo, useState } from 'react';

import { API_BASE_URL } from '../api/config';
import { downloadWithAuth } from '../api/download';

export function ReportsPage() {
  const [reportId, setReportId] = useState('');
  const [error, setError] = useState<string | null>(null);

  const complianceUrl = useMemo(() => {
    const trimmed = reportId.trim();
    if (!trimmed) return '';
    return `${API_BASE_URL}/reports/${encodeURIComponent(trimmed)}/compliance`;
  }, [reportId]);

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError(null);

    if (!reportId.trim()) {
      setError('Report ID is required.');
      return;
    }

    downloadWithAuth({
      url: complianceUrl,
      filenameFallback: `compliance_${reportId.trim()}.bin`,
    }).catch((e: unknown) => {
      const message = e instanceof Error ? e.message : 'Download failed';
      setError(message);
    });
  }

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-800">Reports</h2>
        <p className="text-sm text-slate-600">Minimal report launcher (download endpoints).</p>
      </header>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Compliance report</h3>
        <p className="mt-1 text-xs text-slate-500">Calls GET /reports/:id/compliance and downloads in a new tab.</p>

        <form onSubmit={onSubmit} className="mt-4 grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="font-medium text-slate-700">Report ID</span>
            <input
              value={reportId}
              onChange={(e) => setReportId(e.target.value)}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="report id"
            />
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="w-full rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500"
              disabled={!reportId.trim()}
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

        {complianceUrl ? (
          <div className="mt-4 rounded border border-slate-200 bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Request URL</p>
            <p className="mt-1 break-all font-mono text-xs text-slate-700">{complianceUrl}</p>
          </div>
        ) : null}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Advanced query builder</h3>
        <p className="mt-1 text-sm text-slate-600">
          Not implemented here. If you want it, we can build a constrained GUI for specific report types/endpoints
          (to avoid a full SQL-like builder surface).
        </p>
      </section>
    </div>
  );
}
