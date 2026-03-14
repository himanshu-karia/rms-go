import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import { usePollingGate } from '../session';
import {
  fetchTelemetryHistory,
  subscribeToTelemetryStream,
  type TelemetryRecord,
} from '../api/telemetry';
import {
  fetchDeviceStatus,
  lookupDevice,
  type DeviceLookupResponse,
  type DeviceStatusResponse,
} from '../api/devices';
import { StatusBadge } from '../components/StatusBadge';
import { formatRelativeDuration } from '../utils/datetime';

const HISTORY_LIMIT_OPTIONS = [25, 50, 100, 200, 500];
const REFRESH_INTERVAL_OPTIONS = [1, 5, 10, 15, 30, 45, 60];
const LIVE_TICKET_WARNING_THRESHOLD_MS = 2 * 60 * 1000;
const TELEMETRY_PACKET_TABS = [
  {
    id: 'heartbeat',
    label: 'Heartbeat',
    topicSuffix: 'heartbeat',
    description: 'Connectivity, GPS, and controller vitals from periodic heartbeat packets.',
    columns: [
      'VD',
      'TIMESTAMP',
      'DATE',
      'IMEI',
      'ASN',
      'RTCDATE',
      'RTCTIME',
      'LAT',
      'LONG',
      'RSSI',
      'STINTERVAL',
      'POTP',
      'COTP',
      'GSM',
      'SIM',
      'NET',
      'GPRS',
      'SD',
      'ONLINE',
      'GPS',
      'GPSLOC',
      'RF',
      'TEMP',
      'SIMSLOT',
      'SIMCHNGCNT',
      'FLASH',
      'BATTST',
      'VBATT',
      'PST',
    ],
  },
  {
    id: 'pump',
    label: 'Pump Data (Data Topic)',
    topicSuffix: 'data',
    description: 'Performance and production metrics emitted on the canonical data topic.',
    columns: [
      'VD',
      'TIMESTAMP',
      'DATE',
      'IMEI',
      'ASN',
      'PDKWH1',
      'PTOTKWH1',
      'POPDWD1',
      'POPTOTWD1',
      'PDHR1',
      'PTOTHR1',
      'POPKW1',
      'MAXINDEX',
      'INDEX',
      'LOAD',
      'STINTERVAL',
      'POTP',
      'COTP',
      'PMAXFREQ1',
      'PFREQLSP1',
      'PFREQHSP1',
      'PCNTRMODE1',
      'PRUNST1',
      'POPFREQ1',
      'POPI1',
      'POPV1',
      'PDC1V1',
      'PDC1I1',
      'PDCVOC1',
      'POPFLW1',
    ],
  },
  {
    id: 'daq',
    label: 'DAQ',
    topicSuffix: 'daq',
    description: 'Analog and digital channel snapshots captured via the DAQ topic.',
    columns: [
      'VD',
      'TIMESTAMP',
      'MAXINDEX',
      'INDEX',
      'LOAD',
      'STINTERVAL',
      'MSGID',
      'DATE',
      'IMEI',
      'ASN',
      'POTP',
      'COTP',
      'AI11',
      'AI21',
      'AI31',
      'AI41',
      'DI11',
      'DI21',
      'DI31',
      'DI41',
      'DO11',
      'DO21',
      'DO31',
      'DO41',
    ],
  },
  {
    id: 'ondemand',
    label: 'On-demand',
    topicSuffix: 'ondemand',
    description: 'Command requests and acknowledgements exchanged via the ondemand channel.',
    columns: ['msgid', 'COTP', 'POTP', 'timestamp', 'type', 'cmd', 'status', 'DO1', 'PRUNST1'],
  },
] as const;

const TELEMETRY_FIELD_METADATA: Record<string, { label: string; unit?: string }> = {
  VD: { label: 'Virtual Device Index' },
  TIMESTAMP: { label: 'RTC Timestamp' },
  DATE: { label: 'Local Storage Date' },
  IMEI: { label: 'IMEI' },
  ASN: { label: 'Application Serial #' },
  RTCDATE: { label: 'RTC Date' },
  RTCTIME: { label: 'RTC Time' },
  LAT: { label: 'Latitude', unit: '°' },
  LONG: { label: 'Longitude', unit: '°' },
  RSSI: { label: 'Signal Strength' },
  STINTERVAL: { label: 'Periodic Interval', unit: 'min' },
  POTP: { label: 'Previous OTP' },
  COTP: { label: 'Current OTP' },
  GSM: { label: 'GSM Connected' },
  SIM: { label: 'SIM Detected' },
  NET: { label: 'Network Status' },
  GPRS: { label: 'GPRS Connected' },
  SD: { label: 'SD Card Detected' },
  ONLINE: { label: 'Device Online' },
  GPS: { label: 'GPS Module' },
  GPSLOC: { label: 'GPS Lock' },
  RF: { label: 'RF Module' },
  TEMP: { label: 'Temperature', unit: '°C' },
  SIMSLOT: { label: 'SIM Slot' },
  SIMCHNGCNT: { label: 'SIM Change Count' },
  FLASH: { label: 'Flash Status' },
  BATTST: { label: 'Battery Input Status' },
  VBATT: { label: 'Battery Voltage', unit: 'V' },
  PST: { label: 'Power Supply Status' },
  PDKWH1: { label: 'Today Generated Energy', unit: 'kWh' },
  PTOTKWH1: { label: 'Cumulative Generated Energy', unit: 'kWh' },
  POPDWD1: { label: 'Daily Water Discharge', unit: 'L' },
  POPTOTWD1: { label: 'Total Water Discharge', unit: 'L' },
  PDHR1: { label: 'Pump Day Run Hours', unit: 'h' },
  PTOTHR1: { label: 'Pump Total Run Hours', unit: 'h' },
  POPKW1: { label: 'Output Active Power', unit: 'kW' },
  MAXINDEX: { label: 'Max Storage Index' },
  INDEX: { label: 'Storage Index' },
  LOAD: { label: 'Load Status' },
  PMAXFREQ1: { label: 'Maximum Frequency', unit: 'Hz' },
  PFREQLSP1: { label: 'Lower Frequency Limit', unit: 'Hz' },
  PFREQHSP1: { label: 'Upper Frequency Limit', unit: 'Hz' },
  PCNTRMODE1: { label: 'Control Mode' },
  PRUNST1: { label: 'Run Status' },
  POPFREQ1: { label: 'Output Frequency', unit: 'Hz' },
  POPI1: { label: 'Output Current', unit: 'A' },
  POPV1: { label: 'Output Voltage', unit: 'V' },
  PDC1V1: { label: 'DC Input Voltage', unit: 'V DC' },
  PDC1I1: { label: 'DC Input Current', unit: 'A DC' },
  PDCVOC1: { label: 'DC Open Circuit Voltage', unit: 'V DC' },
  POPFLW1: { label: 'Flow Speed', unit: 'LPM' },
  MSGID: { label: 'Message ID' },
  AI11: { label: 'Analog Input 1' },
  AI21: { label: 'Analog Input 2' },
  AI31: { label: 'Analog Input 3' },
  AI41: { label: 'Analog Input 4' },
  DI11: { label: 'Digital Input 1' },
  DI21: { label: 'Digital Input 2' },
  DI31: { label: 'Digital Input 3' },
  DI41: { label: 'Digital Input 4' },
  DO11: { label: 'Digital Output 1' },
  DO21: { label: 'Digital Output 2' },
  DO31: { label: 'Digital Output 3' },
  DO41: { label: 'Digital Output 4' },
  msgid: { label: 'Message ID' },
  timestamp: { label: 'Timestamp' },
  type: { label: 'Command Type' },
  cmd: { label: 'Command' },
  status: { label: 'Status' },
  DO1: { label: 'Digital Output 1' },
};

type TelemetryPacketTabKey = (typeof TELEMETRY_PACKET_TABS)[number]['id'];

type TelemetryPacketTabConfig = (typeof TELEMETRY_PACKET_TABS)[number];

const TELEMETRY_PACKET_TAB_MAP: Record<TelemetryPacketTabKey, TelemetryPacketTabConfig> =
  TELEMETRY_PACKET_TABS.reduce(
    (acc, tab) => {
      acc[tab.id] = tab;
      return acc;
    },
    {} as Record<TelemetryPacketTabKey, TelemetryPacketTabConfig>,
  );

function formatTimestamp(value: string) {
  return new Date(value).toLocaleString();
}

function extractAsn(payload: Record<string, unknown> | null | undefined) {
  if (!payload) {
    return null;
  }

  const upper = payload['ASN'];
  if (typeof upper === 'string' && upper.trim()) {
    return upper;
  }

  const lower = payload['asn'];
  if (typeof lower === 'string' && lower.trim()) {
    return lower;
  }

  return null;
}

function describeTelemetryField(key: string) {
  return TELEMETRY_FIELD_METADATA[key] ?? { label: key };
}

function formatOptionalTimestamp(value: string | null) {
  return value ? new Date(value).toLocaleString() : '—';
}

function formatDurationMs(value: number | null | undefined) {
  if (!value || value <= 0) {
    return '—';
  }

  const minutes = Math.round(value / 60000);
  if (minutes < 60) {
    return `${minutes} min${minutes === 1 ? '' : 's'}`;
  }

  const hours = value / 3600000;
  if (hours < 48) {
    return `${hours.toFixed(hours % 1 === 0 ? 0 : 1)} hr`;
  }

  const days = value / (24 * 3600000);
  return `${days.toFixed(days % 1 === 0 ? 0 : 1)} day${days >= 2 ? 's' : ''}`;
}

const UUID_REGEX =
  /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/;

function buildLookupParams(identifier: string): { deviceUuid?: string; imei?: string } {
  if (UUID_REGEX.test(identifier)) {
    return { deviceUuid: identifier };
  }

  return { imei: identifier };
}

type MergeTelemetryRecordsParams = {
  historyRecords: TelemetryRecord[];
  liveRecords: TelemetryRecord[];
  topicSuffix: string;
  limit: number;
};

function mergeTelemetryRecords({
  historyRecords,
  liveRecords,
  topicSuffix,
  limit,
}: MergeTelemetryRecordsParams) {
  if (!topicSuffix) {
    return [] as TelemetryRecord[];
  }

  const combined = [...liveRecords, ...historyRecords].filter(
    (record) => record.topicSuffix === topicSuffix,
  );

  combined.sort((a, b) => Date.parse(b.receivedAt) - Date.parse(a.receivedAt));

  const seen = new Set<string>();
  const result: TelemetryRecord[] = [];

  for (const record of combined) {
    if (seen.has(record.telemetryId)) {
      continue;
    }
    seen.add(record.telemetryId);
    result.push(record);
    if (result.length >= limit) {
      break;
    }
  }

  return result;
}

function pluckPayloadValue(payload: Record<string, unknown> | null | undefined, key: string) {
  if (!payload || !key) {
    return undefined;
  }

  if (Object.prototype.hasOwnProperty.call(payload, key)) {
    return payload[key];
  }

  const trimmed = key.trim();
  const lowerKey = trimmed.toLowerCase();
  const upperKey = trimmed.toUpperCase();

  if (lowerKey !== key && Object.prototype.hasOwnProperty.call(payload, lowerKey)) {
    return payload[lowerKey];
  }

  if (upperKey !== key && Object.prototype.hasOwnProperty.call(payload, upperKey)) {
    return payload[upperKey];
  }

  return undefined;
}

function formatPacketValue(value: unknown) {
  if (value === null || value === undefined) {
    return '—';
  }

  if (typeof value === 'number') {
    if (Number.isNaN(value)) {
      return 'NaN';
    }
    if (!Number.isFinite(value)) {
      return value > 0 ? '∞' : '-∞';
    }
    return value.toString();
  }

  if (typeof value === 'boolean') {
    return value ? 'True' : 'False';
  }

  if (typeof value === 'string') {
    return value || '—';
  }

  if (Array.isArray(value) || typeof value === 'object') {
    try {
      return JSON.stringify(value);
    } catch (error) {
      console.warn('Failed to stringify payload field', error);
      return '[object]';
    }
  }

  return String(value);
}

export function TelemetryMonitorPage() {
  const [deviceIdentifierInput, setDeviceIdentifierInput] = useState('');
  const [selectedDeviceUuid, setSelectedDeviceUuid] = useState('');
  const [selectedDevice, setSelectedDevice] = useState<DeviceLookupResponse['device'] | null>(null);
  const [lookupError, setLookupError] = useState<string | null>(null);
  const [activePacketTab, setActivePacketTab] = useState<TelemetryPacketTabKey>('heartbeat');
  const [limit, setLimit] = useState(50);
  const [refreshIntervalSeconds, setRefreshIntervalSeconds] = useState(60);
  const [liveEvents, setLiveEvents] = useState<TelemetryRecord[]>([]);
  const [liveStreamError, setLiveStreamError] = useState<string | null>(null);
  const [liveTicketMetadata, setLiveTicketMetadata] = useState<{
    deviceUuid: string | null;
    expiresAt: string | null;
  }>({ deviceUuid: null, expiresAt: null });
  const [ticketClockMs, setTicketClockMs] = useState(() => Date.now());
  const activeTabConfig = TELEMETRY_PACKET_TAB_MAP[activePacketTab];
  const activeTopicSuffix = activeTabConfig.topicSuffix;
  const liveTicketExpiresAt =
    liveTicketMetadata.deviceUuid === selectedDeviceUuid ? liveTicketMetadata.expiresAt : null;
  const pollingGate = usePollingGate('telemetry-monitor', {
    isActive: Boolean(selectedDeviceUuid),
  });

  useEffect(() => {
    return () => {
      setSelectedDevice(null);
      setSelectedDeviceUuid('');
      setLiveEvents([]);
      setLiveStreamError(null);
      setLiveTicketMetadata({ deviceUuid: null, expiresAt: null });
      setTicketClockMs(Date.now());
    };
  }, []);

  const resetLiveStream = () => {
    setLiveEvents([]);
    setLiveStreamError(null);
    setLiveTicketMetadata({ deviceUuid: null, expiresAt: null });
  };

  const lookupMutation = useMutation({
    mutationFn: lookupDevice,
  });

  const telemetryQuery = useQuery({
    queryKey: ['telemetry-history', selectedDeviceUuid, activeTopicSuffix, limit],
    queryFn: () =>
      fetchTelemetryHistory({
        deviceUuid: selectedDeviceUuid,
        topicSuffix: activeTopicSuffix,
        limit,
      }),
    enabled: Boolean(selectedDeviceUuid) && pollingGate.enabled,
    refetchInterval: pollingGate.enabled ? refreshIntervalSeconds * 1000 : false,
  });

  const statusQuery = useQuery<DeviceStatusResponse, Error>({
    queryKey: ['device-status', selectedDeviceUuid],
    queryFn: () => fetchDeviceStatus(selectedDeviceUuid),
    enabled: Boolean(selectedDeviceUuid) && pollingGate.enabled,
    refetchInterval: pollingGate.enabled ? refreshIntervalSeconds * 1000 : false,
  });

  useEffect(() => {
    if (!liveTicketExpiresAt) {
      return;
    }

    const id = window.setInterval(() => {
      setTicketClockMs(Date.now());
    }, 5_000);

    return () => {
      window.clearInterval(id);
    };
  }, [liveTicketExpiresAt]);

  const liveTicketRemainingMs = useMemo(() => {
    if (!liveTicketExpiresAt) {
      return null;
    }

    const expiresAtMs = Date.parse(liveTicketExpiresAt);
    if (Number.isNaN(expiresAtMs)) {
      return null;
    }

    return Math.max(0, expiresAtMs - ticketClockMs);
  }, [liveTicketExpiresAt, ticketClockMs]);

  const liveTicketCountdownLabel =
    liveTicketRemainingMs !== null ? formatRelativeDuration(liveTicketRemainingMs) : null;
  const liveTicketAbsoluteLabel = liveTicketExpiresAt ? formatTimestamp(liveTicketExpiresAt) : null;
  const isLiveTicketExpiringSoon =
    liveTicketRemainingMs !== null && liveTicketRemainingMs <= LIVE_TICKET_WARNING_THRESHOLD_MS;

  useEffect(() => {
    if (!selectedDeviceUuid) {
      return;
    }

    let closed = false;

    const unsubscribe = subscribeToTelemetryStream(
      selectedDeviceUuid,
      (event) => {
        if (event.topicSuffix !== activeTopicSuffix) {
          return;
        }

        setLiveEvents((prev) => {
          if (prev.some((existing) => existing.telemetryId === event.telemetryId)) {
            return prev;
          }
          return [event, ...prev].slice(0, Math.max(1, limit));
        });
      },
      {
        topicSuffix: activeTopicSuffix,
        historyLimit: limit,
        historyHours: 3,
        includeHistory: true,
        onTicketIssued: ({ expiresAt }) => {
          if (closed) {
            return;
          }
          setLiveStreamError(null);
          setLiveTicketMetadata({ deviceUuid: selectedDeviceUuid, expiresAt: expiresAt ?? null });
          setTicketClockMs(Date.now());
        },
        onError: () => {
          if (closed) {
            return;
          }
          setLiveStreamError(
            'Live telemetry stream disconnected. If this persists, ensure no other tab is streaming a different device.',
          );
          setLiveTicketMetadata({ deviceUuid: selectedDeviceUuid, expiresAt: null });
        },
      },
    );

    return () => {
      closed = true;
      unsubscribe();
    };
  }, [selectedDeviceUuid, activeTopicSuffix, limit]);

  const telemetryHistoryRecords = telemetryQuery.data?.records;
  const historyRecords = useMemo(() => telemetryHistoryRecords ?? [], [telemetryHistoryRecords]);
  const deviceStatus = statusQuery.data ?? null;
  const mergedRecords = useMemo(() => {
    if (!selectedDeviceUuid) {
      return [] as TelemetryRecord[];
    }

    return mergeTelemetryRecords({
      historyRecords,
      liveRecords: liveEvents,
      topicSuffix: activeTopicSuffix,
      limit,
    });
  }, [selectedDeviceUuid, historyRecords, liveEvents, activeTopicSuffix, limit]);
  const latestPacket = mergedRecords[0] ?? null;
  const deviceInfo = deviceStatus?.device ?? selectedDevice;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = deviceIdentifierInput.trim();

    if (!trimmed) {
      setSelectedDevice(null);
      setSelectedDeviceUuid('');
      resetLiveStream();
      setLookupError(null);
      if (!lookupMutation.isPending) {
        lookupMutation.reset();
      }
      return;
    }

    const lookupParams = buildLookupParams(trimmed);
    const matchesCurrent =
      selectedDevice &&
      ((lookupParams.deviceUuid && selectedDevice.uuid === lookupParams.deviceUuid) ||
        (lookupParams.imei && selectedDevice.imei === lookupParams.imei));

    if (matchesCurrent && selectedDeviceUuid) {
      void telemetryQuery.refetch();
      void statusQuery.refetch();
      return;
    }

    setLookupError(null);
    lookupMutation.mutate(lookupParams, {
      onSuccess: (result) => {
        setDeviceIdentifierInput(result.device.imei ?? trimmed);
        setSelectedDevice(result.device);
        setSelectedDeviceUuid(result.device.uuid);
        setLookupError(null);
        resetLiveStream();
      },
      onError: (error) => {
        const message =
          error instanceof Error && error.message
            ? error.message
            : 'Device lookup failed. Verify the identifier and try again.';
        setLookupError(message);
        setSelectedDevice(null);
        setSelectedDeviceUuid('');
        resetLiveStream();
      },
    });
  };

  const telemetrySummary = useMemo(() => {
    if (!latestPacket) {
      return null;
    }

    return {
      topicSuffix: latestPacket.topicSuffix,
      receivedAt: formatTimestamp(latestPacket.receivedAt),
      msgid: latestPacket.metadata.msgid ?? 'N/A',
      asn: extractAsn(latestPacket.payload),
    };
  }, [latestPacket]);

  return (
    <div className="w-full px-4 sm:px-6 lg:px-8">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-6">
        <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="text-lg font-semibold">Telemetry Monitor</h2>
          <p className="mt-1 text-sm text-slate-600">
            Provide a device IMEI (preferred) or UUID to observe live MQTT telemetry and pull recent
            historical records.
          </p>
          <form className="mt-4 flex flex-wrap items-end gap-4" onSubmit={handleSubmit}>
            <div className="min-w-[220px] flex-1">
              <label
                className="block text-xs font-medium text-slate-600"
                htmlFor="deviceIdentifier"
              >
                Device IMEI or UUID
              </label>
              <input
                id="deviceIdentifier"
                name="deviceIdentifier"
                value={deviceIdentifierInput}
                onChange={(event) => {
                  setDeviceIdentifierInput(event.target.value);
                  if (lookupError) {
                    setLookupError(null);
                  }
                  if (!lookupMutation.isPending && lookupMutation.isError) {
                    lookupMutation.reset();
                  }
                }}
                className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                placeholder="Enter IMEI (preferred) or UUID"
                autoComplete="off"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600" htmlFor="historyLimit">
                History Limit
              </label>
              <select
                id="historyLimit"
                value={limit}
                onChange={(event) => {
                  setLimit(Number(event.target.value));
                  resetLiveStream();
                }}
                className="mt-1 w-32 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              >
                {HISTORY_LIMIT_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-600" htmlFor="refreshInterval">
                Update Frequency (sec)
              </label>
              <select
                id="refreshInterval"
                value={refreshIntervalSeconds}
                onChange={(event) => setRefreshIntervalSeconds(Number(event.target.value))}
                className="mt-1 w-32 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              >
                {REFRESH_INTERVAL_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </select>
            </div>
            <button
              type="submit"
              className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={lookupMutation.isPending}
            >
              {lookupMutation.isPending ? 'Looking up…' : 'Load Telemetry'}
            </button>
            {selectedDeviceUuid && (
              <button
                type="button"
                onClick={() => telemetryQuery.refetch()}
                className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              >
                Refresh
              </button>
            )}
          </form>
          {lookupError && <p className="mt-3 text-sm text-red-600">{lookupError}</p>}
          {lookupMutation.isPending && !lookupError && (
            <p className="mt-3 text-xs text-slate-500">Looking up device…</p>
          )}
          {telemetryQuery.error instanceof Error && (
            <p className="mt-3 text-sm text-red-600">{telemetryQuery.error.message}</p>
          )}
          {selectedDeviceUuid && statusQuery.isFetching && !deviceStatus && (
            <p className="mt-3 text-xs text-slate-500">Fetching device status…</p>
          )}
          {selectedDeviceUuid && statusQuery.error && (
            <p className="mt-3 text-sm text-red-600">{statusQuery.error.message}</p>
          )}
          {selectedDeviceUuid && deviceInfo && (
            <div className="mt-4 grid gap-3 rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700 md:grid-cols-4">
              <div>
                <span className="font-medium">IMEI:</span> {deviceInfo.imei}
              </div>
              <div>
                <span className="font-medium">UUID:</span> {deviceInfo.uuid}
              </div>
              <div>
                <span className="font-medium">Status:</span> {deviceInfo.status ?? '—'}
              </div>
              <div>
                <span className="font-medium">Connectivity:</span> {deviceInfo.connectivityStatus}
              </div>
              <div>
                <span className="font-medium">Last Telemetry:</span>{' '}
                {formatOptionalTimestamp(deviceInfo.lastTelemetryAt ?? null)}
              </div>
              <div>
                <span className="font-medium">Last Heartbeat:</span>{' '}
                {formatOptionalTimestamp(deviceInfo.lastHeartbeatAt ?? null)}
              </div>
              <div>
                <span className="font-medium">Protocol:</span>{' '}
                {deviceInfo.protocolVersion ? deviceInfo.protocolVersion.version : '—'}
              </div>
              <div>
                <span className="font-medium">Connectivity Updated:</span>{' '}
                {formatOptionalTimestamp(deviceInfo.connectivityUpdatedAt ?? null)}
              </div>
            </div>
          )}
          {selectedDeviceUuid && telemetrySummary && (
            <div className="mt-4 grid gap-3 rounded-md border border-emerald-100 bg-emerald-50 p-4 text-sm text-emerald-900 md:grid-cols-4">
              <div>
                <span className="font-medium">Last Topic:</span> {telemetrySummary.topicSuffix}
              </div>
              <div>
                <span className="font-medium">Received:</span> {telemetrySummary.receivedAt}
              </div>
              <div>
                <span className="font-medium">Message ID:</span> {telemetrySummary.msgid}
              </div>
              <div>
                <span className="font-medium">Application Serial #:</span>{' '}
                {telemetrySummary.asn ?? 'N/A'}
              </div>
            </div>
          )}
          {selectedDeviceUuid && deviceStatus && (
            <div className="mt-6">
              <section className="grid gap-3 rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700 md:grid-cols-3">
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    IMEI
                  </span>
                  {deviceStatus.device.imei}
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Device Status
                  </span>
                  <StatusBadge status={deviceStatus.device.status} />
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Configuration Status
                  </span>
                  <StatusBadge status={deviceStatus.device.configurationStatus} />
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Connectivity
                  </span>
                  <StatusBadge status={deviceStatus.device.connectivityStatus} />
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Connectivity Updated
                  </span>
                  {formatOptionalTimestamp(deviceStatus.device.connectivityUpdatedAt)}
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Last Telemetry
                  </span>
                  {formatOptionalTimestamp(deviceStatus.device.lastTelemetryAt)}
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Last Heartbeat
                  </span>
                  {formatOptionalTimestamp(deviceStatus.device.lastHeartbeatAt)}
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Offline Threshold
                  </span>
                  {formatDurationMs(deviceStatus.device.offlineThresholdMs)}
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Notification Channels
                  </span>
                  {deviceStatus.device.offlineNotificationChannelCount}
                </div>
                <div>
                  <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Protocol Version
                  </span>
                  {deviceStatus.device.protocolVersion
                    ? `${deviceStatus.device.protocolVersion.version} (${deviceStatus.device.protocolVersion.name ?? 'unnamed'})`
                    : '—'}
                </div>
              </section>
            </div>
          )}
        </section>
        <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <h3 className="text-base font-semibold text-slate-800">Telemetry Packets</h3>
              <p className="text-xs text-slate-500">
                Review the most recent MQTT payloads grouped by topic. Newest entries appear first.
              </p>
            </div>
            <div className="text-xs text-slate-500">Showing up to {limit} packets per tab.</div>
          </div>

          <div className="mt-6 border-b border-slate-200">
            <div className="flex flex-wrap gap-4 text-sm font-semibold text-slate-500">
              {TELEMETRY_PACKET_TABS.map((tab) => {
                const isActive = tab.id === activePacketTab;
                return (
                  <button
                    key={tab.id}
                    type="button"
                    onClick={() => {
                      setActivePacketTab(tab.id as TelemetryPacketTabKey);
                      resetLiveStream();
                    }}
                    className={`border-b-2 px-1 py-2 transition-colors ${isActive ? 'border-emerald-500 text-emerald-700' : 'border-transparent text-slate-500 hover:text-slate-700'}`}
                  >
                    {tab.label}
                  </button>
                );
              })}
            </div>
          </div>

          <p className="mt-3 text-sm text-slate-600">{activeTabConfig.description}</p>

          {selectedDeviceUuid ? (
            <div className="mt-4 space-y-4">
              {liveTicketCountdownLabel && (
                <div
                  className={`rounded border px-3 py-2 text-xs ${isLiveTicketExpiringSoon ? 'border-amber-200 bg-amber-50 text-amber-800' : 'border-emerald-100 bg-emerald-50 text-emerald-800'}`}
                >
                  <p>
                    Live telemetry token expires in{' '}
                    <span className="font-semibold">{liveTicketCountdownLabel}</span>
                    {liveTicketAbsoluteLabel ? ` (${liveTicketAbsoluteLabel})` : ''}.{' '}
                    {isLiveTicketExpiringSoon
                      ? 'Lookup the device again soon to renew before the stream disconnects.'
                      : 'Re-run the lookup whenever you need to renew the stream.'}
                  </p>
                </div>
              )}

              {liveStreamError && <p className="text-sm text-red-600">{liveStreamError}</p>}

              <TelemetryPacketTable
                records={mergedRecords}
                columns={activeTabConfig.columns}
                isLoading={telemetryQuery.isFetching && mergedRecords.length === 0}
                emptyMessage={`No ${activeTabConfig.label.toLowerCase()} packets captured yet.`}
              />
            </div>
          ) : (
            <p className="mt-4 text-sm text-slate-600">
              Lookup a device above to inspect telemetry packets.
            </p>
          )}
        </section>
      </div>
    </div>
  );
}

type TelemetryPacketTableProps = {
  records: TelemetryRecord[];
  columns: readonly string[];
  isLoading: boolean;
  emptyMessage: string;
};

function TelemetryPacketTable({
  records,
  columns,
  isLoading,
  emptyMessage,
}: TelemetryPacketTableProps) {
  if (isLoading) {
    return <p className="text-sm text-slate-500">Loading packets…</p>;
  }

  if (!records.length) {
    return <p className="text-sm text-slate-500">{emptyMessage}</p>;
  }

  const latestTelemetryId = records[0]?.telemetryId ?? null;

  return (
    <div className="w-full overflow-hidden rounded-md border border-slate-200">
      <div className="max-h-[70vh]">
        <div className="overflow-auto">
          <table className="min-w-max divide-y divide-slate-200 text-sm">
            <thead className="sticky top-0 z-10 bg-slate-50 text-xs uppercase tracking-wide text-slate-500 shadow">
              <tr>
                <th className="px-3 py-2 text-left">
                  <div className="text-slate-800">Received</div>
                  <div className="text-[11px] uppercase tracking-wide text-slate-400">UTC</div>
                </th>
                <th className="px-3 py-2 text-left">
                  <div className="text-slate-800">Source</div>
                  <div className="text-[11px] uppercase tracking-wide text-slate-400">
                    live/history
                  </div>
                </th>
                <th className="px-3 py-2 text-left">
                  <div className="text-slate-800">Message ID</div>
                  <div className="text-[11px] uppercase tracking-wide text-slate-400">msgid</div>
                </th>
                {columns.map((column) => {
                  const meta = describeTelemetryField(column);
                  return (
                    <th key={column} className="px-3 py-2 text-left">
                      <div className="text-slate-800">{meta.label}</div>
                      <div className="text-[11px] uppercase tracking-wide text-slate-400">
                        {meta.unit ? `${column} · ${meta.unit}` : column}
                      </div>
                    </th>
                  );
                })}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {records.map((record) => {
                const isLatest = record.telemetryId === latestTelemetryId;
                const sourceLabel = record.source === 'history' ? 'History' : 'Live';
                return (
                  <tr
                    key={record.telemetryId}
                    className={isLatest ? 'bg-emerald-50/50' : undefined}
                  >
                    <td className="px-3 py-2 align-top text-xs text-slate-600">
                      {formatTimestamp(record.receivedAt)}
                    </td>
                    <td className="px-3 py-2 align-top text-xs text-slate-600">
                      <span
                        className={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${sourceLabel === 'History' ? 'bg-slate-200 text-slate-600' : 'bg-emerald-100 text-emerald-700'}`}
                      >
                        {sourceLabel}
                      </span>
                      {isLatest && (
                        <span className="ml-2 rounded bg-emerald-600/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-emerald-700">
                          Latest
                        </span>
                      )}
                    </td>
                    <td className="px-3 py-2 align-top text-xs text-slate-600">
                      {record.metadata.msgid ?? '—'}
                    </td>
                    {columns.map((column) => {
                      const meta = describeTelemetryField(column);
                      const title = meta.unit ? `${meta.label} (${meta.unit})` : meta.label;
                      return (
                        <td
                          key={`${record.telemetryId}-${column}`}
                          className="px-3 py-2 align-top text-xs text-slate-600"
                          title={title}
                        >
                          {formatPacketValue(pluckPayloadValue(record.payload, column))}
                        </td>
                      );
                    })}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
