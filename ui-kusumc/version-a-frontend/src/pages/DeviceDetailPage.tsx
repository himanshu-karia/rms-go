import { FormEvent, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  deleteDevice,
  fetchDevice,
  fetchDeviceCredentialHistory,
  fetchDeviceStatus,
  issueCredentialDownloadToken,
  retryDeviceMqttProvisioning,
  rotateDeviceCredentials,
  revokeDeviceCredentials,
  updateDevice,
  type DeviceCredentialHistoryItem,
  type DeviceSummary,
} from '../api/devices';
import {
  deleteTelemetryThresholds,
  fetchTelemetryThresholds,
  upsertTelemetryThresholds,
  type TelemetryThresholdEntry,
  type TelemetryThresholdResponse,
} from '../api/telemetry';
import { queueDeviceConfiguration, fetchPendingDeviceConfiguration } from '../api/deviceConfigurations';
import { AdminStatusBanner, type AdminStatusMessage } from '../features/admin/components/AdminStatusBanner';

function safeJsonParse(input: string): { ok: true; value: unknown } | { ok: false; error: string } {
  try {
    const parsed = JSON.parse(input);
    return { ok: true, value: parsed };
  } catch (error) {
    return { ok: false, error: error instanceof Error ? error.message : 'Invalid JSON' };
  }
}

function jsonPretty(value: unknown) {
  return JSON.stringify(value, null, 2);
}

function toNumberOrNull(value: string): number | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  const num = Number(trimmed);
  return Number.isFinite(num) ? num : null;
}

type ThresholdDraft = {
  parameter: string;
  min: string;
  max: string;
  warnLow: string;
  warnHigh: string;
  alertLow: string;
  alertHigh: string;
  target: string;
  unit: string;
  decimalPlaces: string;
};

function emptyThresholdDraft(): ThresholdDraft {
  return {
    parameter: '',
    min: '',
    max: '',
    warnLow: '',
    warnHigh: '',
    alertLow: '',
    alertHigh: '',
    target: '',
    unit: '',
    decimalPlaces: '',
  };
}

export function DeviceDetailPage() {
  const params = useParams();
  const idOrUuid = params.idOrUuid ?? '';
  const queryClient = useQueryClient();

  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [devicePatchJson, setDevicePatchJson] = useState<string>('{}');
  const [queueConfigJson, setQueueConfigJson] = useState<string>('{}');

  const [thresholdScope, setThresholdScope] = useState<'override' | 'installation'>('override');
  const [thresholdReason, setThresholdReason] = useState<string>('');
  const [thresholdDraft, setThresholdDraft] = useState<ThresholdDraft>(() => emptyThresholdDraft());

  const deviceQuery = useQuery<DeviceSummary, Error>({
    queryKey: ['device-detail', idOrUuid],
    queryFn: () => fetchDevice(idOrUuid),
    enabled: Boolean(idOrUuid.trim()),
    refetchOnWindowFocus: false,
  });

  const statusQuery = useQuery({
    queryKey: ['device-detail', idOrUuid, 'status'],
    queryFn: () => fetchDeviceStatus(idOrUuid),
    enabled: Boolean(idOrUuid.trim()),
    refetchOnWindowFocus: false,
  });

  const credentialHistoryQuery = useQuery<{ items: DeviceCredentialHistoryItem[] }, Error>({
    queryKey: ['device-detail', idOrUuid, 'credential-history'],
    queryFn: () => fetchDeviceCredentialHistory(idOrUuid),
    enabled: Boolean(idOrUuid.trim()),
    refetchOnWindowFocus: false,
  });

  const pendingConfigQuery = useQuery({
    queryKey: ['device-detail', idOrUuid, 'pending-config'],
    queryFn: () => fetchPendingDeviceConfiguration(idOrUuid),
    enabled: Boolean(idOrUuid.trim()),
    refetchOnWindowFocus: false,
  });

  const thresholdsQuery = useQuery<TelemetryThresholdResponse, Error>({
    queryKey: ['device-detail', idOrUuid, 'thresholds'],
    queryFn: () => fetchTelemetryThresholds(idOrUuid),
    enabled: Boolean(idOrUuid.trim()),
    refetchOnWindowFocus: false,
  });

  const updateDeviceMutation = useMutation({
    mutationFn: async () => {
      const parsed = safeJsonParse(devicePatchJson);
      if (!parsed.ok) {
        throw new Error(parsed.error);
      }
      if (!parsed.value || typeof parsed.value !== 'object' || Array.isArray(parsed.value)) {
        throw new Error('Device update must be a JSON object');
      }
      return updateDevice(idOrUuid, parsed.value as Record<string, unknown>);
    },
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Device updated.' });
      await queryClient.invalidateQueries({ queryKey: ['device-detail', idOrUuid] });
      await queryClient.invalidateQueries({ queryKey: ['device-inventory'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to update device.' });
    },
  });

  const deleteDeviceMutation = useMutation({
    mutationFn: () => deleteDevice(idOrUuid),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Device deleted.' });
      await queryClient.invalidateQueries({ queryKey: ['device-inventory'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to delete device.' });
    },
  });

  const rotateCredsMutation = useMutation({
    mutationFn: () => rotateDeviceCredentials(idOrUuid, {}),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Credentials rotated.' });
      await queryClient.invalidateQueries({ queryKey: ['device-detail', idOrUuid, 'credential-history'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to rotate credentials.' });
    },
  });

  const revokeCredsMutation = useMutation({
    mutationFn: () => revokeDeviceCredentials(idOrUuid, { type: 'local' }),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Credentials revoked.' });
      await queryClient.invalidateQueries({ queryKey: ['device-detail', idOrUuid, 'credential-history'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to revoke credentials.' });
    },
  });

  const retryProvisioningMutation = useMutation({
    mutationFn: () => retryDeviceMqttProvisioning(idOrUuid),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Provisioning retry queued.' });
      await queryClient.invalidateQueries({ queryKey: ['device-detail', idOrUuid, 'credential-history'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to retry provisioning.' });
    },
  });

  const downloadTokenMutation = useMutation({
    mutationFn: () => issueCredentialDownloadToken(idOrUuid),
    onSuccess: (result) => {
      setStatus({ type: 'success', message: `Download token issued (expires ${new Date(result.expiresAt).toLocaleString()}).` });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to issue download token.' });
    },
  });

  const queueConfigMutation = useMutation({
    mutationFn: async () => {
      const parsed = safeJsonParse(queueConfigJson);
      if (!parsed.ok) throw new Error(parsed.error);
      if (!parsed.value || typeof parsed.value !== 'object' || Array.isArray(parsed.value)) {
        throw new Error('Configuration payload must be a JSON object');
      }
      return queueDeviceConfiguration(idOrUuid, parsed.value as any);
    },
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Configuration queued.' });
      await queryClient.invalidateQueries({ queryKey: ['device-detail', idOrUuid, 'pending-config'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to queue configuration.' });
    },
  });

  const upsertThresholdMutation = useMutation({
    mutationFn: async () => {
      const parameter = thresholdDraft.parameter.trim();
      if (!parameter) throw new Error('Parameter is required');

      const decimalPlacesNum = thresholdDraft.decimalPlaces.trim()
        ? Number(thresholdDraft.decimalPlaces)
        : null;
      if (thresholdDraft.decimalPlaces.trim() && !Number.isFinite(decimalPlacesNum)) {
        throw new Error('decimalPlaces must be a number');
      }

      return upsertTelemetryThresholds(idOrUuid, {
        scope: thresholdScope,
        reason: thresholdReason.trim() || undefined,
        thresholds: [
          {
            parameter,
            min: toNumberOrNull(thresholdDraft.min),
            max: toNumberOrNull(thresholdDraft.max),
            warnLow: toNumberOrNull(thresholdDraft.warnLow),
            warnHigh: toNumberOrNull(thresholdDraft.warnHigh),
            alertLow: toNumberOrNull(thresholdDraft.alertLow),
            alertHigh: toNumberOrNull(thresholdDraft.alertHigh),
            target: toNumberOrNull(thresholdDraft.target),
            unit: thresholdDraft.unit.trim() || null,
            decimalPlaces: decimalPlacesNum === null ? null : decimalPlacesNum,
          },
        ],
      });
    },
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Threshold saved.' });
      setThresholdDraft(emptyThresholdDraft());
      await queryClient.invalidateQueries({ queryKey: ['device-detail', idOrUuid, 'thresholds'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to save threshold.' });
    },
  });

  const deleteThresholdMutation = useMutation({
    mutationFn: () => deleteTelemetryThresholds(idOrUuid, { scope: thresholdScope, reason: thresholdReason.trim() || undefined }),
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Threshold layer deleted.' });
      await queryClient.invalidateQueries({ queryKey: ['device-detail', idOrUuid, 'thresholds'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to delete thresholds.' });
    },
  });

  const effective = thresholdsQuery.data?.thresholds.effective ?? [];
  const overrideLayer = thresholdsQuery.data?.thresholds.override ?? null;
  const installationLayer = thresholdsQuery.data?.thresholds.installation ?? null;

  const scopeEntries: TelemetryThresholdEntry[] =
    thresholdScope === 'override'
      ? overrideLayer?.entries ?? []
      : installationLayer?.entries ?? [];

  const busy =
    deviceQuery.isFetching ||
    updateDeviceMutation.isPending ||
    deleteDeviceMutation.isPending ||
    rotateCredsMutation.isPending ||
    revokeCredsMutation.isPending ||
    retryProvisioningMutation.isPending ||
    downloadTokenMutation.isPending ||
    queueConfigMutation.isPending ||
    upsertThresholdMutation.isPending ||
    deleteThresholdMutation.isPending;

  const headerTitle = deviceQuery.data?.imei
    ? `Device ${deviceQuery.data.imei}`
    : idOrUuid
      ? `Device ${idOrUuid}`
      : 'Device';

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-2xl font-semibold text-slate-800">{headerTitle}</h2>
          <Link className="text-sm font-medium text-emerald-700 hover:underline" to="/live/device-inventory">
            ← Back to inventory
          </Link>
        </div>
        <p className="text-sm text-slate-600">Lifecycle admin view: status, credentials, configuration, telemetry thresholds.</p>
      </header>

      <AdminStatusBanner status={status} />

      {deviceQuery.isError ? (
        <p className="text-sm text-rose-600" role="alert">
          {deviceQuery.error.message}
        </p>
      ) : null}

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Overview</h3>
          <button
            type="button"
            onClick={() => {
              setStatus(null);
              deviceQuery.refetch();
              statusQuery.refetch();
              credentialHistoryQuery.refetch();
              pendingConfigQuery.refetch();
              thresholdsQuery.refetch();
            }}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={busy}
          >
            Refresh
          </button>
        </div>

        {deviceQuery.data ? (
          <div className="mt-3 grid gap-3 md:grid-cols-3">
            <div className="rounded border border-slate-200 bg-slate-50 p-3">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Device UUID</p>
              <p className="mt-1 break-all font-mono text-xs text-slate-700">{deviceQuery.data.id}</p>
            </div>
            <div className="rounded border border-slate-200 bg-slate-50 p-3">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Project</p>
              <p className="mt-1 break-all font-mono text-xs text-slate-700">{deviceQuery.data.projectId}</p>
            </div>
            <div className="rounded border border-slate-200 bg-slate-50 p-3">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Connectivity</p>
              <p className="mt-1 text-sm font-medium text-slate-800">
                {deviceQuery.data.connectivityStatus ?? deviceQuery.data.connectivity_status ?? '—'}
              </p>
              <p className="mt-1 text-xs text-slate-500">
                {deviceQuery.data.connectivityUpdatedAt || deviceQuery.data.connectivity_updated_at
                  ? new Date(
                      (deviceQuery.data.connectivityUpdatedAt ??
                        deviceQuery.data.connectivity_updated_at) as string,
                    ).toLocaleString()
                  : '—'}
              </p>
            </div>
          </div>
        ) : (
          <p className="mt-3 text-sm text-slate-500">{deviceQuery.isLoading ? 'Loading…' : '—'}</p>
        )}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Device status (raw)</h3>
        <pre className="mt-3 max-h-72 overflow-auto rounded border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
          {statusQuery.data ? jsonPretty(statusQuery.data) : statusQuery.isLoading ? 'Loading…' : '—'}
        </pre>
      </section>

      <section className="grid gap-6 md:grid-cols-2">
        <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <h3 className="text-lg font-semibold text-slate-900">Update device</h3>
          <p className="mt-1 text-xs text-slate-500">Paste a JSON object with fields like name/status/projectId/metadata.</p>
          <textarea
            className="mt-3 h-40 w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
            value={devicePatchJson}
            onChange={(e) => setDevicePatchJson(e.target.value)}
            disabled={updateDeviceMutation.isPending}
          />
          <div className="mt-3 flex gap-2">
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                updateDeviceMutation.mutate();
              }}
              className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={updateDeviceMutation.isPending}
            >
              {updateDeviceMutation.isPending ? 'Saving…' : 'Save'}
            </button>
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                deleteDeviceMutation.mutate();
              }}
              className="rounded border border-rose-200 bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-100 disabled:opacity-60"
              disabled={deleteDeviceMutation.isPending}
            >
              {deleteDeviceMutation.isPending ? 'Deleting…' : 'Delete'}
            </button>
          </div>
        </div>

        <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <h3 className="text-lg font-semibold text-slate-900">Credentials & provisioning</h3>
          <div className="mt-3 flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                rotateCredsMutation.mutate();
              }}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
              disabled={rotateCredsMutation.isPending}
            >
              Rotate
            </button>
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                revokeCredsMutation.mutate();
              }}
              className="rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-100 disabled:opacity-60"
              disabled={revokeCredsMutation.isPending}
            >
              Revoke
            </button>
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                retryProvisioningMutation.mutate();
              }}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
              disabled={retryProvisioningMutation.isPending}
            >
              Retry provisioning
            </button>
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                downloadTokenMutation.mutate();
              }}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
              disabled={downloadTokenMutation.isPending}
            >
              Issue download token
            </button>
          </div>

          <p className="mt-4 text-xs font-semibold uppercase tracking-wide text-slate-500">Credential history</p>
          <pre className="mt-2 max-h-56 overflow-auto rounded border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
            {credentialHistoryQuery.data
              ? jsonPretty(credentialHistoryQuery.data)
              : credentialHistoryQuery.isLoading
                ? 'Loading…'
                : '—'}
          </pre>
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="text-lg font-semibold text-slate-900">Device configuration</h3>
        <p className="mt-1 text-xs text-slate-500">Queue a configuration record by posting a JSON object to the device configuration endpoint.</p>

        <div className="mt-4 grid gap-6 md:grid-cols-2">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Pending</p>
            <pre className="mt-2 max-h-64 overflow-auto rounded border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
              {pendingConfigQuery.data === null
                ? 'No pending configuration.'
                : pendingConfigQuery.data
                  ? jsonPretty(pendingConfigQuery.data)
                  : pendingConfigQuery.isLoading
                    ? 'Loading…'
                    : '—'}
            </pre>
          </div>

          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Queue</p>
            <textarea
              className="mt-2 h-40 w-full rounded border border-slate-300 bg-white p-3 font-mono text-xs text-slate-800"
              value={queueConfigJson}
              onChange={(e) => setQueueConfigJson(e.target.value)}
              disabled={queueConfigMutation.isPending}
            />
            <button
              type="button"
              onClick={() => {
                setStatus(null);
                queueConfigMutation.mutate();
              }}
              className="mt-3 rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
              disabled={queueConfigMutation.isPending}
            >
              {queueConfigMutation.isPending ? 'Queuing…' : 'Queue configuration'}
            </button>
          </div>
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Telemetry thresholds</h3>
          <div className="flex items-center gap-2">
            <label className="text-xs font-semibold uppercase tracking-wide text-slate-500">Scope</label>
            <select
              value={thresholdScope}
              onChange={(e) => setThresholdScope(e.target.value as typeof thresholdScope)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              <option value="override">Override</option>
              <option value="installation">Installation</option>
            </select>
          </div>
        </div>

        <p className="mt-2 text-xs text-slate-500">
          Effective thresholds are computed from project defaults + device overrides. Use scope to edit a layer.
        </p>

        <div className="mt-4 grid gap-6 md:grid-cols-2">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Effective</p>
            <div className="mt-2 max-h-64 overflow-auto rounded border border-slate-200">
              <table className="min-w-full divide-y divide-slate-200 text-xs">
                <thead className="bg-slate-50 text-slate-600">
                  <tr>
                    <th className="px-3 py-2 text-left">Param</th>
                    <th className="px-3 py-2 text-left">Min</th>
                    <th className="px-3 py-2 text-left">Max</th>
                    <th className="px-3 py-2 text-left">Source</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 bg-white">
                  {effective.map((e) => (
                    <tr key={`${e.parameter}-${e.source}`}>
                      <td className="px-3 py-2 font-mono">{e.parameter}</td>
                      <td className="px-3 py-2">{e.min ?? '—'}</td>
                      <td className="px-3 py-2">{e.max ?? '—'}</td>
                      <td className="px-3 py-2">{e.source}</td>
                    </tr>
                  ))}
                  {!effective.length ? (
                    <tr>
                      <td colSpan={4} className="px-3 py-4 text-slate-500">
                        {thresholdsQuery.isLoading ? 'Loading…' : 'No thresholds.'}
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </div>
          </div>

          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Selected scope entries</p>
            <div className="mt-2 max-h-64 overflow-auto rounded border border-slate-200">
              <table className="min-w-full divide-y divide-slate-200 text-xs">
                <thead className="bg-slate-50 text-slate-600">
                  <tr>
                    <th className="px-3 py-2 text-left">Param</th>
                    <th className="px-3 py-2 text-left">Min</th>
                    <th className="px-3 py-2 text-left">Max</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 bg-white">
                  {scopeEntries.map((e) => (
                    <tr
                      key={`${e.parameter}-${e.source}`}
                      className="cursor-pointer hover:bg-slate-50"
                      onClick={() => {
                        setThresholdDraft({
                          parameter: e.parameter,
                          min: e.min == null ? '' : String(e.min),
                          max: e.max == null ? '' : String(e.max),
                          warnLow: e.warnLow == null ? '' : String(e.warnLow),
                          warnHigh: e.warnHigh == null ? '' : String(e.warnHigh),
                          alertLow: e.alertLow == null ? '' : String(e.alertLow),
                          alertHigh: e.alertHigh == null ? '' : String(e.alertHigh),
                          target: e.target == null ? '' : String(e.target),
                          unit: e.unit ?? '',
                          decimalPlaces: e.decimalPlaces == null ? '' : String(e.decimalPlaces),
                        });
                      }}
                    >
                      <td className="px-3 py-2 font-mono">{e.parameter}</td>
                      <td className="px-3 py-2">{e.min ?? '—'}</td>
                      <td className="px-3 py-2">{e.max ?? '—'}</td>
                    </tr>
                  ))}
                  {!scopeEntries.length ? (
                    <tr>
                      <td colSpan={3} className="px-3 py-4 text-slate-500">
                        No entries in this scope.
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div className="mt-6 rounded border border-slate-200 bg-slate-50 p-4">
          <div className="flex items-center justify-between gap-3">
            <p className="font-semibold text-slate-900">Upsert threshold ({thresholdScope})</p>
            <button
              type="button"
              onClick={() => setThresholdDraft(emptyThresholdDraft())}
              className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100"
            >
              Clear
            </button>
          </div>

          <div className="mt-4 grid gap-4 md:grid-cols-4">
            <label className="flex flex-col gap-1 text-sm md:col-span-2">
              <span className="font-medium text-slate-700">Parameter</span>
              <input
                value={thresholdDraft.parameter}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, parameter: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
                placeholder="e.g. pv_voltage"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Min</span>
              <input
                value={thresholdDraft.min}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, min: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Max</span>
              <input
                value={thresholdDraft.max}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, max: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Warn low</span>
              <input
                value={thresholdDraft.warnLow}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, warnLow: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Warn high</span>
              <input
                value={thresholdDraft.warnHigh}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, warnHigh: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Alert low</span>
              <input
                value={thresholdDraft.alertLow}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, alertLow: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Alert high</span>
              <input
                value={thresholdDraft.alertHigh}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, alertHigh: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Target</span>
              <input
                value={thresholdDraft.target}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, target: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Unit</span>
              <input
                value={thresholdDraft.unit}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, unit: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium text-slate-700">Decimal places</span>
              <input
                value={thresholdDraft.decimalPlaces}
                onChange={(e) => setThresholdDraft((p) => ({ ...p, decimalPlaces: e.target.value }))}
                className="rounded border border-slate-300 px-3 py-2 font-mono text-xs"
              />
            </label>

            <label className="flex flex-col gap-1 text-sm md:col-span-2">
              <span className="font-medium text-slate-700">Reason (optional)</span>
              <input
                value={thresholdReason}
                onChange={(e) => setThresholdReason(e.target.value)}
                className="rounded border border-slate-300 px-3 py-2 text-sm"
              />
            </label>

            <div className="flex items-end gap-2 md:col-span-2">
              <button
                type="button"
                onClick={() => {
                  setStatus(null);
                  upsertThresholdMutation.mutate();
                }}
                className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:opacity-60"
                disabled={upsertThresholdMutation.isPending}
              >
                {upsertThresholdMutation.isPending ? 'Saving…' : 'Save threshold'}
              </button>
              <button
                type="button"
                onClick={() => {
                  setStatus(null);
                  deleteThresholdMutation.mutate();
                }}
                className="rounded border border-rose-200 bg-rose-50 px-4 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-100 disabled:opacity-60"
                disabled={deleteThresholdMutation.isPending}
              >
                {deleteThresholdMutation.isPending ? 'Deleting…' : `Delete ${thresholdScope} layer`}
              </button>
            </div>
          </div>
        </div>

        {thresholdsQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {thresholdsQuery.error.message}
          </p>
        ) : null}
      </section>
    </div>
  );
}
