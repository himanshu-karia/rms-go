import { ChangeEvent, FormEvent, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';

import { fetchDeviceList, type DeviceListItem } from '../api/devices';
import { StatusBadge } from '../components/StatusBadge';

export function DeviceInventoryPage() {
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all');
  const [limit, setLimit] = useState('30');
  const [includeInactive, setIncludeInactive] = useState(false);

  const parsedLimit = Math.max(5, Math.min(200, Number(limit) || 30));

  const deviceListQuery = useQuery({
    queryKey: ['device-inventory', { statusFilter, limit: parsedLimit, includeInactive }],
    queryFn: () =>
      fetchDeviceList({
        limit: parsedLimit,
        includeInactive,
        status: statusFilter === 'all' ? undefined : statusFilter,
      }),
    refetchOnWindowFocus: false,
  });

  const devices = useMemo<DeviceListItem[]>(
    () => deviceListQuery.data?.devices ?? [],
    [deviceListQuery.data?.devices],
  );

  const paginationInfo = deviceListQuery.data?.pagination ?? null;

  const handleFiltersSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    deviceListQuery.refetch();
  };

  const handleLimitChange = (event: ChangeEvent<HTMLInputElement>) => {
    setLimit(event.target.value);
  };

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-1">
        <h2 className="text-2xl font-semibold text-slate-800">Device Inventory</h2>
        <p className="text-sm text-slate-600">
          Read-only view of enrolled devices with quick filters for status and activity.
        </p>
      </header>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <form className="grid gap-4 md:grid-cols-4" onSubmit={handleFiltersSubmit}>
          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Status</span>
            <select
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value as typeof statusFilter)}
              className="rounded border border-slate-300 px-3 py-2"
            >
              <option value="all">All</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </select>
          </label>

          <label className="flex flex-col gap-1 text-sm">
            <span className="font-medium text-slate-700">Limit</span>
            <input
              type="number"
              min={5}
              max={200}
              value={limit}
              onChange={handleLimitChange}
              className="rounded border border-slate-300 px-3 py-2"
            />
          </label>

          <label className="flex items-center gap-2 text-sm text-slate-700">
            <input
              type="checkbox"
              checked={includeInactive}
              onChange={(event) => setIncludeInactive(event.target.checked)}
              className="size-4 rounded border border-slate-300"
            />
            Include inactive devices
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:cursor-not-allowed"
              disabled={deviceListQuery.isFetching}
            >
              {deviceListQuery.isFetching ? 'Refreshing…' : 'Apply Filters'}
            </button>
          </div>
        </form>
        {paginationInfo && (
          <p className="mt-3 text-xs text-slate-500">
            Showing up to {paginationInfo.limit} devices.
            {paginationInfo.status ? ` Status filter: ${paginationInfo.status}.` : ''}
            {paginationInfo.includeInactive ? ' Inactive devices included.' : ''}
          </p>
        )}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-3 py-2">IMEI</th>
                <th className="px-3 py-2">UUID</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Config Status</th>
                <th className="px-3 py-2">Connectivity</th>
                <th className="px-3 py-2">Last Telemetry</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {devices.map((device) => (
                <tr key={device.uuid} className="bg-white">
                  <td className="px-3 py-2 font-medium text-slate-800">
                    <Link
                      to={`/live/device-inventory/${encodeURIComponent(device.uuid)}`}
                      className="text-emerald-700 hover:underline"
                    >
                      {device.imei}
                    </Link>
                  </td>
                  <td className="px-3 py-2 text-xs text-slate-500">
                    <Link
                      to={`/live/device-inventory/${encodeURIComponent(device.uuid)}`}
                      className="hover:underline"
                    >
                      {device.uuid}
                    </Link>
                  </td>
                  <td className="px-3 py-2">
                    <StatusBadge status={device.status ?? 'unknown'} />
                  </td>
                  <td className="px-3 py-2 text-xs text-slate-500">
                    {device.configurationStatus ?? '—'}
                  </td>
                  <td className="px-3 py-2 text-xs text-slate-500">
                    <div className="flex flex-col">
                      <span className="font-medium text-slate-700">
                        {device.connectivityStatus}
                      </span>
                      {device.connectivityUpdatedAt && (
                        <span className="text-xs text-slate-500">
                          {new Date(device.connectivityUpdatedAt).toLocaleString()}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-3 py-2 text-xs text-slate-500">
                    {device.lastTelemetryAt
                      ? new Date(device.lastTelemetryAt).toLocaleString()
                      : '—'}
                  </td>
                </tr>
              ))}
              {!devices.length && (
                <tr>
                  <td colSpan={6} className="px-3 py-6 text-center text-sm text-slate-500">
                    {deviceListQuery.isLoading ? 'Loading devices…' : 'No devices found.'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
