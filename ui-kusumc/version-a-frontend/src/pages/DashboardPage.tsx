import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';

import { fetchDeviceList, type DeviceListItem } from '../api/devices';
import { StatusBadge } from '../components/StatusBadge';

function formatRelativeTime(value: string | null) {
  if (!value) {
    return '—';
  }

  const date = new Date(value);
  const now = Date.now();
  const diffMs = now - date.getTime();
  if (Number.isNaN(diffMs)) {
    return '—';
  }

  const diffMinutes = Math.floor(diffMs / 60000);
  if (diffMinutes < 1) {
    return 'just now';
  }
  if (diffMinutes < 60) {
    return `${diffMinutes} min ago`;
  }
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return `${diffHours} hr ago`;
  }
  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays} day${diffDays === 1 ? '' : 's'} ago`;
}

function isApproachingOffline(device: DeviceListItem) {
  if (!device.lastTelemetryAt || device.connectivityStatus === 'offline') {
    return false;
  }

  const threshold = device.offlineThresholdMs;
  if (!threshold || threshold <= 0) {
    return false;
  }

  const lastTelemetry = new Date(device.lastTelemetryAt).getTime();
  if (Number.isNaN(lastTelemetry)) {
    return false;
  }

  const elapsed = Date.now() - lastTelemetry;
  return elapsed >= threshold * 0.75 && elapsed < threshold;
}

export function DashboardPage() {
  const deviceListQuery = useQuery({
    queryKey: ['device-list'],
    queryFn: () => fetchDeviceList({ limit: 50 }),
    refetchInterval: false,
  });

  const devices = useMemo(() => deviceListQuery.data?.devices ?? [], [deviceListQuery.data]);
  const totalDevices = deviceListQuery.data?.pagination.total ?? 0;

  const summary = useMemo(() => {
    const offline = devices.filter((device) => device.connectivityStatus === 'offline').length;
    const online = devices.filter((device) => device.connectivityStatus === 'online').length;
    const approaching = devices.filter((device) => isApproachingOffline(device)).length;

    return {
      offline,
      online,
      approaching,
    };
  }, [devices]);

  return (
    <div className="space-y-6">
      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold text-slate-900">Fleet Overview</h2>
            <p className="text-sm text-slate-600">
              Track active devices and surface connectivity risks sourced from the offline monitor
              workflow.
            </p>
          </div>
          <Link
            to="/telemetry"
            className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
          >
            Open Telemetry Monitor
          </Link>
        </div>

        {deviceListQuery.isFetching && !devices.length && (
          <p className="mt-4 text-sm text-slate-500">Loading devices…</p>
        )}
        {deviceListQuery.isError && (
          <p className="mt-4 text-sm text-red-600">
            {(deviceListQuery.error as Error).message || 'Unable to load devices'}
          </p>
        )}

        <div className="mt-6 grid gap-4 md:grid-cols-4">
          <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-slate-500">
              Active Devices
            </p>
            <p className="mt-2 text-2xl font-semibold text-slate-900">{totalDevices}</p>
            <p className="text-xs text-slate-500">
              Includes devices provisioned and not marked inactive.
            </p>
          </div>
          <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-emerald-700">Online</p>
            <p className="mt-2 text-2xl font-semibold text-emerald-700">{summary.online}</p>
            <p className="text-xs text-emerald-700">
              Devices reporting telemetry within protocol thresholds.
            </p>
          </div>
          <div className="rounded-lg border border-amber-200 bg-amber-50 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-amber-700">
              Approaching Offline
            </p>
            <p className="mt-2 text-2xl font-semibold text-amber-700">{summary.approaching}</p>
            <p className="text-xs text-amber-700">
              Telemetry lagging past 75% of the offline threshold.
            </p>
          </div>
          <div className="rounded-lg border border-red-200 bg-red-50 p-4">
            <p className="text-xs font-medium uppercase tracking-wide text-red-700">Offline</p>
            <p className="mt-2 text-2xl font-semibold text-red-700">{summary.offline}</p>
            <p className="text-xs text-red-700">Flagged by the offline monitor workflow.</p>
          </div>
        </div>

        {devices.length > 0 && (
          <div className="mt-6 overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="p-3 text-left">Device</th>
                  <th className="p-3 text-left">Status</th>
                  <th className="p-3 text-left">Connectivity</th>
                  <th className="p-3 text-left">Last Telemetry</th>
                  <th className="p-3 text-left">Offline Threshold</th>
                  <th className="p-3 text-left">Notifications</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {devices.slice(0, 12).map((device) => (
                  <tr key={device.uuid} className="hover:bg-slate-50">
                    <td className="px-3 py-2 align-top text-xs text-slate-700">
                      <div className="font-semibold text-slate-900">{device.uuid}</div>
                      <div className="text-[11px] text-slate-500">IMEI {device.imei}</div>
                    </td>
                    <td className="px-3 py-2 align-top text-xs text-slate-700">
                      <div className="flex items-center gap-2">
                        <StatusBadge status={device.status} />
                        {device.protocolVersion && (
                          <span className="text-[11px] text-slate-500">
                            v{device.protocolVersion.version}
                          </span>
                        )}
                      </div>
                      {isApproachingOffline(device) && device.connectivityStatus !== 'offline' && (
                        <div className="mt-1 text-[11px] text-amber-600">
                          Telemetry nearing offline threshold
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-2 align-top text-xs text-slate-700">
                      <div className="flex items-center gap-2">
                        <StatusBadge status={device.connectivityStatus} />
                        <span className="text-[11px] text-slate-500">
                          {formatRelativeTime(device.connectivityUpdatedAt)}
                        </span>
                      </div>
                    </td>
                    <td className="px-3 py-2 align-top text-xs text-slate-700">
                      {formatRelativeTime(device.lastTelemetryAt)}
                    </td>
                    <td className="px-3 py-2 align-top text-xs text-slate-700">
                      {Math.round(device.offlineThresholdMs / 3600000)} hr
                    </td>
                    <td className="px-3 py-2 align-top text-xs text-slate-700">
                      {device.offlineNotificationChannelCount}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
