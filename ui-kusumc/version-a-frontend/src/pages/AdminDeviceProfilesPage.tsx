import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import { createDeviceProfile, fetchDeviceProfiles, type DeviceProfileRecord } from '../api/deviceProfiles';
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

export function AdminDeviceProfilesPage() {
  const [editor, setEditor] = useState<string>('{}');
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);

  const profilesQuery = useQuery<DeviceProfileRecord[], Error>({
    queryKey: ['admin', 'device-profiles'],
    queryFn: fetchDeviceProfiles,
    refetchOnWindowFocus: false,
  });

  const createMutation = useMutation({
    mutationFn: async () => {
      const parsed = safeJsonParse(editor);
      if (!parsed.ok) throw new Error(parsed.error);
      if (!parsed.value || typeof parsed.value !== 'object' || Array.isArray(parsed.value)) {
        throw new Error('Profile payload must be a JSON object');
      }
      await createDeviceProfile(parsed.value as Record<string, unknown>);
    },
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Device profile created.' });
      await profilesQuery.refetch();
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create device profile.' });
    },
  });

  const profilesPretty = useMemo(() => {
    return profilesQuery.data ? jsonPretty(profilesQuery.data) : '';
  }, [profilesQuery.data]);

  function onCreate(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    createMutation.mutate();
  }

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-800">Device Profiles</h2>
        <p className="text-sm text-slate-600">List and create device profiles stored by the backend config service.</p>
      </header>

      <AdminStatusBanner status={status} />

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Profiles (raw)</h3>
          <button
            type="button"
            onClick={() => profilesQuery.refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={profilesQuery.isFetching}
          >
            {profilesQuery.isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {profilesQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {profilesQuery.error.message}
          </p>
        ) : null}

        <pre className="mt-3 max-h-80 overflow-auto rounded border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
          {profilesQuery.isLoading ? 'Loading…' : profilesPretty || '—'}
        </pre>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Create profile</h3>
        <p className="mt-1 text-xs text-slate-500">Paste a JSON object. Backend will store as-is.</p>

        <form onSubmit={onCreate} className="mt-4 space-y-3">
          <textarea
            value={editor}
            onChange={(e) => setEditor(e.target.value)}
            className="h-64 w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
            spellCheck={false}
          />
          <div className="flex gap-2">
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
              type="submit"
              className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}
