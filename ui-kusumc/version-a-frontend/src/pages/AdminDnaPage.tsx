import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import { fetchDna, upsertDna } from '../api/dna';
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

export function AdminDnaPage() {
  const { activeProjectId } = useActiveProject();
  const [projectId, setProjectId] = useState(activeProjectId ?? '');
  const [editor, setEditor] = useState<string>('');
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);

  useEffect(() => {
    if (activeProjectId && !projectId) {
      setProjectId(activeProjectId);
    }
  }, [activeProjectId, projectId]);

  const dnaQuery = useQuery({
    queryKey: ['admin', 'dna', projectId],
    queryFn: () => fetchDna(projectId),
    enabled: Boolean(projectId.trim()),
    refetchOnWindowFocus: false,
  });

  const initialPayload = useMemo(() => {
    if (dnaQuery.data) return dnaQuery.data;
    if (!projectId.trim()) return null;
    return { projectId };
  }, [dnaQuery.data, projectId]);

  useEffect(() => {
    if (dnaQuery.isFetched && editor.trim().length === 0 && initialPayload) {
      setEditor(jsonPretty(initialPayload));
    }
    // intentionally only set default once
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dnaQuery.isFetched]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      const parsed = safeJsonParse(editor);
      if (!parsed.ok) throw new Error(parsed.error);
      if (!parsed.value || typeof parsed.value !== 'object' || Array.isArray(parsed.value)) {
        throw new Error('DNA payload must be a JSON object');
      }
      await upsertDna(projectId, parsed.value as Record<string, unknown>);
    },
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'DNA saved.' });
      await dnaQuery.refetch();
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to save DNA.' });
    },
  });

  function onLoad(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    dnaQuery.refetch();
  }

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-800">DNA Config</h2>
        <p className="text-sm text-slate-600">
          Project DNA record (single-project workflow via Active Project). Load and update JSON.
        </p>
      </header>

      <AdminStatusBanner status={status} />

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <form onSubmit={onLoad} className="grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="font-medium text-slate-700">Project ID</span>
            <input
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
              className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              placeholder="Set Active Project to autofill"
            />
          </label>
          <div className="flex items-end gap-2">
            <button
              type="submit"
              className="w-full rounded border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              disabled={!projectId.trim() || dnaQuery.isFetching}
            >
              {dnaQuery.isFetching ? 'Loading…' : 'Load'}
            </button>
          </div>
        </form>

        {dnaQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {dnaQuery.error instanceof Error ? dnaQuery.error.message : 'Unable to load DNA'}
          </p>
        ) : null}

        <div className="mt-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Editor</p>
          <textarea
            value={editor}
            onChange={(e) => setEditor(e.target.value)}
            className="mt-2 h-[420px] w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
            spellCheck={false}
            placeholder="Load DNA first, then edit JSON here"
          />
        </div>

        <div className="mt-4 flex gap-2">
          <button
            type="button"
            onClick={() => {
              const parsed = safeJsonParse(editor);
              if (parsed.ok) {
                setEditor(jsonPretty(parsed.value));
                setStatus({ type: 'success', message: 'Formatted JSON.' });
              } else {
                setStatus({ type: 'error', message: parsed.error });
              }
            }}
            className="rounded border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          >
            Format
          </button>
          <button
            type="button"
            onClick={() => {
              setStatus(null);
              saveMutation.mutate();
            }}
            className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
            disabled={!projectId.trim() || saveMutation.isPending}
          >
            {saveMutation.isPending ? 'Saving…' : 'Save'}
          </button>
        </div>
      </section>
    </div>
  );
}
