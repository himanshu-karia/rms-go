import { FormEvent, useCallback, useEffect, useMemo, useRef, useState, useId } from 'react';
import {
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type UseMutationResult,
} from '@tanstack/react-query';

import {
  fetchTelemetryHistory,
  fetchTelemetryThresholds,
  subscribeToTelemetryStream,
  upsertTelemetryThresholds,
  deleteTelemetryThresholds,
  type TelemetryRecord,
  type TelemetryThresholdResponse,
  type TelemetryThresholdEntry,
  type TelemetryThresholdUpsertPayload,
  type TelemetryThresholdDeletePayload,
} from '../api/telemetry';
import {
  fetchDeviceStatus,
  lookupDevice,
  type DeviceLookupResponse,
  type DeviceStatusResponse,
  type TelemetryThresholdConfig,
} from '../api/devices';
import {
  fetchDeviceCommandHistory,
  issueDeviceCommand,
  type DeviceCommandHistoryRecord,
  type DeviceCommandStatus,
  type IssueDeviceCommandPayload,
  type IssueDeviceCommandResponse,
} from '../api/deviceCommands';
import { StatusBadge } from '../components/StatusBadge';
import { TelemetryChart } from '../components/TelemetryChart';
import { SemiCircularGauge, evaluateGaugeStatus } from '../components/SemiCircularGauge';
import { formatDuration, useMqttBudget, usePollingGate, useSessionTimers } from '../session';
import { buildTelemetrySeries, parseTelemetryNumeric } from '../utils/telemetry';
import {
  formatDateTimeShort,
  formatDateTimeWithSeconds,
  formatRelativeDuration,
} from '../utils/datetime';
import { useAuth } from '../auth';

// See RMS_Server_Preprompt.txt § "MQTT & Telemetry Handling" for canonical topic suffix set.
const TELEMETRY_SUFFIXES = ['data', 'daq', 'heartbeat'] as const;

const TELEMETRY_SUFFIX_INDEX: Record<TopicSuffix, number> = {
  data: 0,
  daq: 1,
  heartbeat: 2,
};

function scheduleStateUpdate(callback: () => void) {
  if (typeof queueMicrotask === 'function') {
    queueMicrotask(callback);
    return;
  }

  void Promise.resolve().then(callback);
}

const rawEnv = import.meta.env ?? {};

const ENABLE_THRESHOLD_OVERRIDES = rawEnv.VITE_ENABLE_TELEMETRY_THRESHOLD_OVERRIDES !== 'false';
const ENABLE_ADVANCED_ANALYTICS = rawEnv.VITE_ENABLE_TELEMETRY_ANALYTICS !== 'false';
const ENABLE_RAW_DIFF_TOOLING = rawEnv.VITE_ENABLE_TELEMETRY_RAW_DIFF === 'true';

type TopicSuffix = (typeof TELEMETRY_SUFFIXES)[number];

type TelemetryBySuffix = Record<TopicSuffix, TelemetryRecord[]>;

type DeviceSummary = DeviceLookupResponse['device'];

type TabKey = 'dashboard' | 'graphs' | 'data' | 'commands';

export type DashboardGaugeConfig = {
  id: string;
  label: string;
  payloadKey: string;
  unit?: string;
  min?: number;
  max?: number;
  warnLow?: number;
  warnHigh?: number;
  alertLow?: number;
  alertHigh?: number;
  target?: number;
  suffix: TopicSuffix;
  decimalPlaces?: number;
};

type DashboardKpiConfig = {
  id: string;
  label: string;
  description?: string;
  payloadKey: string;
  suffix: TopicSuffix;
  unit?: string;
  fractionDigits?: number;
};

type TelemetryTableGroup = {
  id: string;
  title: string;
  rows: Array<{
    id: string;
    label: string;
    payloadKey: string;
    suffix: TopicSuffix;
    unit?: string;
    fractionDigits?: number;
    description?: string;
  }>;
};

type TelemetryChartConfig = {
  id: string;
  label: string;
  payloadKey: string;
  suffix: TopicSuffix;
  unit?: string;
  description?: string;
  scale?: number;
};

type ParameterMetadata = {
  description: string;
  unit?: string;
};

type ThresholdView = {
  effective: Map<string, TelemetryThresholdEntry>;
  installation: {
    entries: Map<string, TelemetryThresholdEntry>;
    templateId: string | null;
    updatedAt: string | null;
    updatedBy: { id: string; displayName: string | null } | null;
    metadata: Record<string, unknown> | null;
  } | null;
  overrideEntries: Map<string, TelemetryThresholdEntry>;
  overrideMeta: {
    reason: string | null;
    updatedAt: string | null;
    updatedBy: { id: string; displayName: string | null } | null;
  } | null;
};

type CommandHistoryFilter = 'all' | DeviceCommandStatus;

type AnalyticsCallout = {
  id: string;
  label: string;
  value: string;
  description?: string;
  emphasis?: 'positive' | 'negative';
};

// Command composer UX (Increment 3) comes from RMS_Server_Preprompt.txt § "Device lifecycle APIs".
const COMMAND_HISTORY_PAGE_SIZE = 25;
const COMMAND_STATUS_FILTERS: Array<{ id: CommandHistoryFilter; label: string }> = [
  { id: 'all', label: 'All' },
  { id: 'pending', label: 'Pending' },
  { id: 'acknowledged', label: 'Acknowledged' },
  { id: 'failed', label: 'Failed' },
];

const HISTORY_LIMIT_OPTIONS = [50, 100, 200, 500];
const DEFAULT_HISTORY_LIMIT = 200;
const LIVE_BUFFER_LIMIT = 1000;
const LIVE_BUFFER_RECENT_UNTOUCHED = 200;
const LIVE_BUFFER_SOFT_LIMIT = Math.floor(LIVE_BUFFER_LIMIT * 0.9);
const LIVE_TICKET_WARNING_THRESHOLD_MS = 2 * 60 * 1000;

const TABS: Array<{ id: TabKey; label: string }> = [
  { id: 'dashboard', label: 'Dashboard' },
  { id: 'graphs', label: 'Graphs' },
  { id: 'data', label: 'Data Table' },
  { id: 'commands', label: 'Commands & Raw' },
];

const GAUGE_CONFIGS: DashboardGaugeConfig[] = [
  {
    id: 'pump-frequency',
    label: 'Pump Frequency',
    payloadKey: 'POPFREQ1',
    unit: 'Hz',
    min: 0,
    max: 60,
    warnLow: 5,
    warnHigh: 55,
    suffix: 'data',
    decimalPlaces: 1,
  },
  {
    id: 'pump-output',
    label: 'Pump Output Power',
    payloadKey: 'POPKW1',
    unit: 'kW',
    min: 0,
    max: 10,
    warnHigh: 9,
    suffix: 'data',
    decimalPlaces: 2,
  },
  {
    id: 'array-voltage',
    label: 'Array Voltage',
    payloadKey: 'PDC1V1',
    unit: 'V',
    min: 0,
    max: 900,
    warnLow: 300,
    warnHigh: 750,
    suffix: 'data',
    decimalPlaces: 0,
  },
  {
    id: 'array-current',
    label: 'Array Current',
    payloadKey: 'PDC1I1',
    unit: 'A',
    min: 0,
    max: 45,
    warnHigh: 40,
    suffix: 'data',
    decimalPlaces: 1,
  },
  {
    id: 'flow-rate',
    label: 'Flow Rate',
    payloadKey: 'PFLWRT1',
    unit: 'L/min',
    min: 0,
    max: 700,
    warnLow: 50,
    suffix: 'data',
    decimalPlaces: 0,
  },
  {
    id: 'water-level',
    label: 'Water Level',
    payloadKey: 'PTWTLV1',
    unit: 'm',
    min: 0,
    max: 120,
    warnLow: 5,
    suffix: 'data',
    decimalPlaces: 1,
  },
];

const KPI_CONFIGS: DashboardKpiConfig[] = [
  {
    id: 'daily-energy',
    label: 'Daily Energy',
    description: 'Energy delivered since midnight',
    payloadKey: 'PTDAYE1',
    suffix: 'data',
    unit: 'kWh',
    fractionDigits: 2,
  },
  {
    id: 'total-hours',
    label: 'Total Run Hours',
    description: 'Lifetime accumulated run time',
    payloadKey: 'PTOTHR1',
    suffix: 'data',
    unit: 'hr',
    fractionDigits: 1,
  },
  {
    id: 'fault-count',
    label: 'Active Faults',
    description: 'Latest fault code count',
    payloadKey: 'PFAULT1',
    suffix: 'daq',
    fractionDigits: 0,
  },
  {
    id: 'last-heartbeat',
    label: 'Heartbeat Age',
    description: 'Minutes since last heartbeat packet',
    payloadKey: 'heartbeatAgeMinutes',
    suffix: 'heartbeat',
    unit: 'min',
    fractionDigits: 0,
  },
];

const TABLE_GROUPS: TelemetryTableGroup[] = [
  {
    id: 'pump-performance',
    title: 'Pump Performance',
    rows: [
      {
        id: 'pump-output-power',
        label: 'Pump Output Power',
        payloadKey: 'POPKW1',
        suffix: 'data',
        unit: 'kW',
        fractionDigits: 2,
      },
      {
        id: 'pump-frequency',
        label: 'Pump Frequency',
        payloadKey: 'POPFREQ1',
        suffix: 'data',
        unit: 'Hz',
        fractionDigits: 1,
      },
      {
        id: 'pump-voltage',
        label: 'Output Voltage',
        payloadKey: 'POPV1',
        suffix: 'data',
        unit: 'V',
        fractionDigits: 0,
      },
      {
        id: 'pump-current',
        label: 'Output Current',
        payloadKey: 'POPI1',
        suffix: 'data',
        unit: 'A',
        fractionDigits: 1,
      },
      {
        id: 'pump-temperature',
        label: 'Drive Temperature',
        payloadKey: 'PDRVTM1',
        suffix: 'data',
        unit: '°C',
        fractionDigits: 1,
      },
    ],
  },
  {
    id: 'array-health',
    title: 'Array Health',
    rows: [
      {
        id: 'array-voltage',
        label: 'Array Voltage',
        payloadKey: 'PDC1V1',
        suffix: 'data',
        unit: 'V',
        fractionDigits: 0,
      },
      {
        id: 'array-current',
        label: 'Array Current',
        payloadKey: 'PDC1I1',
        suffix: 'data',
        unit: 'A',
        fractionDigits: 1,
      },
      {
        id: 'insolation',
        label: 'Irradiance',
        payloadKey: 'PSOLIR1',
        suffix: 'data',
        unit: 'W/m²',
        fractionDigits: 0,
      },
      {
        id: 'dc-bus-voltage',
        label: 'DC Bus Voltage',
        payloadKey: 'PDCBUS1',
        suffix: 'daq',
        unit: 'V',
        fractionDigits: 0,
      },
    ],
  },
  {
    id: 'water-system',
    title: 'Water System',
    rows: [
      {
        id: 'flow-rate',
        label: 'Flow Rate',
        payloadKey: 'PFLWRT1',
        suffix: 'data',
        unit: 'L/min',
        fractionDigits: 0,
      },
      {
        id: 'water-yield',
        label: 'Daily Water Yield',
        payloadKey: 'PTDAYW1',
        suffix: 'data',
        unit: 'kl',
        fractionDigits: 2,
      },
      {
        id: 'water-level',
        label: 'Water Level',
        payloadKey: 'PTWTLV1',
        suffix: 'data',
        unit: 'm',
        fractionDigits: 1,
      },
    ],
  },
  {
    id: 'faults',
    title: 'Faults & Alerts',
    rows: [
      {
        id: 'fault-code',
        label: 'Fault Code',
        payloadKey: 'PFAULT1',
        suffix: 'daq',
        fractionDigits: 0,
        description: 'Non-zero indicates an active fault',
      },
      {
        id: 'fault-reset',
        label: 'Reset Required',
        payloadKey: 'PRESET1',
        suffix: 'daq',
        description: '1 indicates manual reset required',
      },
    ],
  },
];

const CHART_CONFIGS: TelemetryChartConfig[] = [
  {
    id: 'pump-output-chart',
    label: 'Pump Output Power',
    payloadKey: 'POPKW1',
    suffix: 'data',
    unit: 'kW',
  },
  {
    id: 'array-voltage-chart',
    label: 'Array Voltage',
    payloadKey: 'PDC1V1',
    suffix: 'data',
    unit: 'V',
  },
  {
    id: 'array-current-chart',
    label: 'Array Current',
    payloadKey: 'PDC1I1',
    suffix: 'data',
    unit: 'A',
  },
];

// Schema hints derived from RMS JSON MQTT Topics MDs/JSON_PARAMETERS.md (spec reference).
const COMMON_PARAMETER_METADATA: Record<string, ParameterMetadata> = {
  IMEI: { description: 'Device IMEI identifier' },
  TIMESTAMP: { description: 'Device-reported timestamp' },
  DATE: { description: 'Device local storage date' },
  ASN: { description: 'Application serial number' },
  POTP: { description: 'Previous one-time password' },
  COTP: { description: 'Current one-time password' },
};

const PARAMETER_METADATA_BY_SUFFIX: Record<TopicSuffix, Record<string, ParameterMetadata>> = {
  heartbeat: {
    ...COMMON_PARAMETER_METADATA,
    LAT: { description: 'GPS latitude', unit: 'Degrees' },
    LONG: { description: 'GPS longitude', unit: 'Degrees' },
    RSSI: { description: 'Signal strength' },
    TEMP: { description: 'Device temperature', unit: '°C' },
    VBATT: { description: 'Battery voltage', unit: 'V' },
    ONLINE: { description: 'Device online indicator' },
    GPS: { description: 'GPS module status' },
    NET: { description: 'Network connectivity indicator' },
    STINTERVAL: { description: 'Periodic heartbeat interval', unit: 'Minutes' },
  },
  data: {
    ...COMMON_PARAMETER_METADATA,
    POPKW1: { description: 'Output active power', unit: 'kW' },
    POPFREQ1: { description: 'Output frequency', unit: 'Hz' },
    POPI1: { description: 'Output current', unit: 'A' },
    POPV1: { description: 'Output voltage', unit: 'V' },
    PDC1V1: { description: 'DC input voltage', unit: 'V' },
    PDC1I1: { description: 'DC input current', unit: 'A' },
    PTOTHR1: { description: 'Cumulative run hours', unit: 'hr' },
    PTDAYE1: { description: 'Energy delivered since midnight', unit: 'kWh' },
    PDRVTM1: { description: 'Drive temperature', unit: '°C' },
    PDCBUS1: { description: 'DC bus voltage', unit: 'V' },
    PFAULT1: { description: 'Fault code' },
    PRESET1: { description: 'Reset required indicator' },
    PFLWRT1: { description: 'Flow rate', unit: 'L/min' },
    PTWTLV1: { description: 'Water level', unit: 'm' },
    PTDAYW1: { description: 'Daily water yield', unit: 'kl' },
    POPFLW1: { description: 'Flow speed', unit: 'L/min' },
    PSOLIR1: { description: 'Solar irradiance', unit: 'W/m²' },
  },
  daq: {
    ...COMMON_PARAMETER_METADATA,
    AI11: { description: 'Analog input 1' },
    AI21: { description: 'Analog input 2' },
    AI31: { description: 'Analog input 3' },
    AI41: { description: 'Analog input 4' },
    DI11: { description: 'Digital input 1' },
    DI21: { description: 'Digital input 2' },
    DI31: { description: 'Digital input 3' },
    DI41: { description: 'Digital input 4' },
    DO11: { description: 'Digital output 1' },
    DO21: { description: 'Digital output 2' },
    DO31: { description: 'Digital output 3' },
    DO41: { description: 'Digital output 4' },
    PFAULT1: { description: 'Fault code' },
    PRESET1: { description: 'Reset required indicator' },
  },
};

const UUID_REGEX =
  /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/;

function buildLookupParams(identifier: string): { deviceUuid?: string; imei?: string } {
  if (UUID_REGEX.test(identifier)) {
    return { deviceUuid: identifier };
  }

  return { imei: identifier };
}

function toRecordKey(record: TelemetryRecord): string {
  if (record.telemetryId) {
    return record.telemetryId;
  }

  const receivedAt = record.receivedAt ?? '';
  const suffix = record.topicSuffix ?? '';
  const msgid = record.metadata?.msgid ?? '';
  return `${suffix}-${receivedAt}-${msgid}`;
}

function compactLiveBuffer(events: TelemetryRecord[]): TelemetryRecord[] {
  if (events.length <= LIVE_BUFFER_SOFT_LIMIT) {
    return events;
  }

  const untouchedTail = events.slice(0, LIVE_BUFFER_RECENT_UNTOUCHED);
  const older = events.slice(LIVE_BUFFER_RECENT_UNTOUCHED);
  const targetBudget = Math.max(LIVE_BUFFER_SOFT_LIMIT - untouchedTail.length, 0);

  if (targetBudget === 0 || older.length === 0) {
    return untouchedTail.slice(0, LIVE_BUFFER_LIMIT);
  }

  const step = Math.max(1, Math.ceil(older.length / targetBudget));
  const compactedOlder = older.filter((_, idx) => idx % step === 0);

  return [...untouchedTail, ...compactedOlder].slice(0, LIVE_BUFFER_LIMIT);
}

function mergeTelemetry(live: TelemetryRecord[], history: TelemetryRecord[]): TelemetryRecord[] {
  const merged = new Map<string, TelemetryRecord>();

  history.forEach((record) => {
    merged.set(toRecordKey(record), record);
  });

  live.forEach((record) => {
    merged.set(toRecordKey(record), record);
  });

  return Array.from(merged.values()).sort((a, b) => {
    const aTime = new Date(a.receivedAt).getTime();
    const bTime = new Date(b.receivedAt).getTime();
    if (Number.isNaN(aTime) || Number.isNaN(bTime)) {
      return 0;
    }
    return bTime - aTime;
  });
}

function formatNumber(value: number | null, unit?: string, fractionDigits = 1): string {
  if (value === null || Number.isNaN(value)) {
    return '—';
  }

  const formatted = value.toLocaleString(undefined, {
    maximumFractionDigits: fractionDigits,
    minimumFractionDigits: fractionDigits > 0 ? Math.min(1, fractionDigits) : 0,
  });

  return unit ? `${formatted} ${unit}` : formatted;
}

function describeRecord(record: TelemetryRecord): string {
  const payload = record.payload ?? {};
  const suffix = record.topicSuffix;

  if (suffix === 'data') {
    const freq = parseTelemetryNumeric(payload['POPFREQ1']);
    if (freq !== null) {
      return `Pump frequency ${freq.toFixed(1)} Hz`;
    }
    const power = parseTelemetryNumeric(payload['POPKW1']);
    if (power !== null) {
      return `Pump power ${power.toFixed(2)} kW`;
    }
  }

  if (suffix === 'data') {
    const flow = parseTelemetryNumeric(payload['PFLWRT1']);
    if (flow !== null) {
      return `Flow rate ${flow.toFixed(0)} L/min`;
    }
    const waterLevel = parseTelemetryNumeric(payload['PTWTLV1']);
    if (waterLevel !== null) {
      return `Water level ${waterLevel.toFixed(1)} m`;
    }
  }

  if (suffix === 'daq') {
    const fault = parseTelemetryNumeric(payload['PFAULT1']);
    if (fault !== null && fault > 0) {
      return `Fault code ${fault.toFixed(0)} reported`;
    }
  }

  if (suffix === 'heartbeat') {
    return 'Heartbeat received';
  }

  return `Received ${suffix} packet`;
}

function computeTrend(records: TelemetryRecord[], payloadKey: string): number | null {
  if (records.length < 2) {
    return null;
  }

  const sorted = [...records].sort(
    (a, b) => new Date(b.receivedAt).getTime() - new Date(a.receivedAt).getTime(),
  );
  const currentValue = parseTelemetryNumeric(sorted[0]?.payload?.[payloadKey]);
  if (currentValue === null) {
    return null;
  }

  const previousRecord = sorted.find(
    (record) =>
      parseTelemetryNumeric(record.payload?.[payloadKey]) !== null && record !== sorted[0],
  );
  if (!previousRecord) {
    return null;
  }

  const previousValue = parseTelemetryNumeric(previousRecord.payload?.[payloadKey]);
  if (previousValue === null) {
    return null;
  }

  return currentValue - previousValue;
}

function resolveHeartbeatAgeMinutes(record: TelemetryRecord | null): number | null {
  if (!record) {
    return null;
  }

  const received = new Date(record.receivedAt).getTime();
  if (Number.isNaN(received)) {
    return null;
  }

  const now = Date.now();
  const diffMinutes = Math.floor((now - received) / 60000);
  return diffMinutes >= 0 ? diffMinutes : 0;
}

export function TelemetryV2Page() {
  const { hasCapability } = useAuth();
  const [deviceIdentifierInput, setDeviceIdentifierInput] = useState('');
  const [selectedDeviceUuid, setSelectedDeviceUuid] = useState('');
  const [selectedDevice, setSelectedDevice] = useState<DeviceSummary | null>(null);
  const [lookupError, setLookupError] = useState<string | null>(null);
  const [historyLimit, setHistoryLimit] = useState<number>(DEFAULT_HISTORY_LIMIT);
  const [liveEvents, setLiveEvents] = useState<TelemetryRecord[]>([]);
  const [liveStreamError, setLiveStreamError] = useState<string | null>(null);
  const [liveTicketMetadata, setLiveTicketMetadata] = useState<{
    deviceUuid: string | null;
    expiresAt: string | null;
  }>({ deviceUuid: null, expiresAt: null });
  const liveTicketExpiresAt =
    liveTicketMetadata.deviceUuid === selectedDeviceUuid ? liveTicketMetadata.expiresAt : null;
  const [liveTicketClockMs, setLiveTicketClockMs] = useState(() => Date.now());
  const [topicSuffix, setTopicSuffix] = useState<TopicSuffix | ''>('');
  const [activeTab, setActiveTab] = useState<TabKey>('dashboard');
  const [isThresholdDrawerOpen, setThresholdDrawerOpen] = useState(false);
  const [commandStatusFilter, setCommandStatusFilter] = useState<CommandHistoryFilter>('all');
  const pollingGate = usePollingGate('telemetry-v2', {
    isActive: Boolean(selectedDeviceUuid),
  });
  const {
    enabled: isPollingEnabled,
    isIdle: isPollingIdle,
    remainingMs: pollingIdleRemainingMs,
    resume: resumePolling,
  } = pollingGate;
  const { isSessionIdle, isTabVisible, sessionGeneration } = useSessionTimers();
  const mqttScope = selectedDeviceUuid ? `telemetry-v2:${selectedDeviceUuid}` : 'telemetry-v2';
  const {
    summary: mqttSummary,
    cap: mqttPacketCap,
    recordPacket: recordMqttPacket,
    reset: resetMqttBudget,
    pauseForIdle,
    pauseForCap,
    pauseForHidden,
  } = useMqttBudget(mqttScope);
  const resetMqttBudgetRef = useRef(resetMqttBudget);

  useEffect(() => {
    resetMqttBudgetRef.current = resetMqttBudget;
  }, [resetMqttBudget]);
  const queryClient = useQueryClient();
  const livePauseReason = useMemo<'cap' | 'idle' | 'hidden' | null>(() => {
    if (!selectedDeviceUuid) {
      return null;
    }

    if (mqttSummary.isPaused) {
      return mqttSummary.reason ?? null;
    }

    if (!isTabVisible) {
      return 'hidden';
    }

    if (isPollingIdle) {
      return 'idle';
    }

    return null;
  }, [selectedDeviceUuid, mqttSummary.isPaused, mqttSummary.reason, isPollingIdle, isTabVisible]);

  useEffect(() => {
    return () => {
      setSelectedDevice(null);
      setSelectedDeviceUuid('');
      setLiveEvents([]);
      setLiveStreamError(null);
      setLiveTicketMetadata({ deviceUuid: null, expiresAt: null });
      setLiveTicketClockMs(Date.now());
      resetMqttBudgetRef.current();
    };
  }, []);

  useEffect(() => {
    if (!liveTicketExpiresAt) {
      return;
    }

    const id = window.setInterval(() => {
      setLiveTicketClockMs(Date.now());
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

    return Math.max(0, expiresAtMs - liveTicketClockMs);
  }, [liveTicketExpiresAt, liveTicketClockMs]);

  const liveTicketCountdownLabel =
    liveTicketRemainingMs !== null ? formatRelativeDuration(liveTicketRemainingMs) : null;
  const liveTicketAbsoluteLabel = liveTicketExpiresAt
    ? formatDateTimeShort(liveTicketExpiresAt)
    : null;
  const isLiveTicketExpiringSoon =
    liveTicketRemainingMs !== null && liveTicketRemainingMs <= LIVE_TICKET_WARNING_THRESHOLD_MS;

  const livePauseMessage = useMemo(() => {
    if (!livePauseReason) {
      return null;
    }

    if (livePauseReason === 'hidden') {
      return 'Live stream paused while this tab is hidden. Focus the tab and press resume to reconnect.';
    }

    if (livePauseReason === 'idle') {
      if (pollingIdleRemainingMs && pollingIdleRemainingMs > 0) {
        return `Live stream paused while idle. Activity resumes automatically after ${formatDuration(pollingIdleRemainingMs)} or when you press resume.`;
      }

      return 'Live stream paused due to inactivity. Resume to reconnect to the MQTT feed.';
    }

    const consumed = Math.min(mqttSummary.count, mqttPacketCap);
    return `Live stream paused after receiving ${consumed} packets (budget ${mqttPacketCap}). Resume to continue streaming.`;
  }, [livePauseReason, pollingIdleRemainingMs, mqttSummary.count, mqttPacketCap]);

  const handleResumeLiveFeed = useCallback(() => {
    resumePolling();
    resetMqttBudget();
    setLiveEvents([]);
    setLiveStreamError(null);
  }, [resumePolling, resetMqttBudget]);

  const lookupMutation = useMutation({
    mutationFn: lookupDevice,
  });

  const historyQueries = useQueries({
    queries: TELEMETRY_SUFFIXES.map((suffix) => ({
      queryKey: ['telemetry-history-v2', selectedDeviceUuid, suffix, historyLimit],
      queryFn: () =>
        fetchTelemetryHistory({
          deviceUuid: selectedDeviceUuid,
          topicSuffix: suffix,
          limit: historyLimit,
        }),
      enabled: Boolean(selectedDeviceUuid) && isPollingEnabled,
      refetchInterval: isPollingEnabled ? 60_000 : (false as const),
      refetchOnWindowFocus: false,
    })),
  });

  const statusQuery = useQuery<DeviceStatusResponse, Error>({
    queryKey: ['device-status-v2', selectedDeviceUuid],
    queryFn: () => fetchDeviceStatus(selectedDeviceUuid),
    enabled: Boolean(selectedDeviceUuid) && isPollingEnabled,
    refetchInterval: isPollingEnabled ? 60_000 : false,
    refetchOnWindowFocus: false,
  });

  const canManageTelemetryThresholds =
    ENABLE_THRESHOLD_OVERRIDES &&
    hasCapability(['alerts:manage', 'admin:all'], {
      match: 'any',
    });

  const thresholdsQuery = useQuery<TelemetryThresholdResponse, Error>({
    queryKey: ['telemetry-thresholds-v2', selectedDeviceUuid],
    queryFn: () => fetchTelemetryThresholds(selectedDeviceUuid),
    enabled: Boolean(selectedDeviceUuid) && isPollingEnabled && canManageTelemetryThresholds,
    refetchOnWindowFocus: false,
  });

  useEffect(() => {
    if (thresholdsQuery.error) {
      console.warn(
        '[telemetry-v2] threshold fetch failed; gauges will use built-in defaults until backend support is available.',
        thresholdsQuery.error,
      );
    }
  }, [thresholdsQuery.error]);

  const commandHistoryQuery = useInfiniteQuery({
    queryKey: ['device-command-history', selectedDeviceUuid, commandStatusFilter],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      if (!selectedDeviceUuid) {
        throw new Error('Select a device before loading command history');
      }

      return fetchDeviceCommandHistory({
        deviceUuid: selectedDeviceUuid,
        limit: COMMAND_HISTORY_PAGE_SIZE,
        cursor: typeof pageParam === 'string' ? pageParam : undefined,
        statuses: commandStatusFilter === 'all' ? undefined : [commandStatusFilter],
      });
    },
    enabled: Boolean(selectedDeviceUuid),
    refetchOnWindowFocus: false,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    refetchInterval: activeTab === 'commands' && selectedDeviceUuid ? 15_000 : false,
  });

  const upsertThresholdMutation = useMutation({
    mutationFn: ({
      deviceUuid,
      payload,
    }: {
      deviceUuid: string;
      payload: TelemetryThresholdUpsertPayload;
    }) => upsertTelemetryThresholds(deviceUuid, payload),
    onSuccess: () => {
      if (canManageTelemetryThresholds) {
        void thresholdsQuery.refetch();
      }
    },
  });

  const deleteThresholdMutation = useMutation({
    mutationFn: ({
      deviceUuid,
      payload,
    }: {
      deviceUuid: string;
      payload?: TelemetryThresholdDeletePayload;
    }) => deleteTelemetryThresholds(deviceUuid, payload),
    onSuccess: () => {
      if (canManageTelemetryThresholds) {
        void thresholdsQuery.refetch();
      }
    },
  });

  const commandHistoryRecords = useMemo(() => {
    return (commandHistoryQuery.data?.pages ?? []).flatMap((page) => page.commands);
  }, [commandHistoryQuery.data?.pages]);
  const commandHistoryErrorMessage =
    commandHistoryQuery.error instanceof Error ? commandHistoryQuery.error.message : null;
  const isCommandHistoryLoading = commandHistoryQuery.isLoading;
  const isCommandHistoryRefreshing =
    commandHistoryQuery.isFetching && !commandHistoryQuery.isFetchingNextPage;
  const hasCommandHistoryNextPage = Boolean(commandHistoryQuery.hasNextPage);
  const isFetchingNextCommandPage = commandHistoryQuery.isFetchingNextPage;

  const issueCommandMutation = useMutation({
    mutationFn: async (payload: IssueDeviceCommandPayload) => {
      if (!selectedDeviceUuid) {
        throw new Error('Select a device before issuing commands');
      }
      return issueDeviceCommand(selectedDeviceUuid, payload);
    },
    onSuccess: () => {
      if (!selectedDeviceUuid) {
        return;
      }
      void queryClient.invalidateQueries({
        queryKey: ['device-command-history', selectedDeviceUuid],
      });
      void commandHistoryQuery.refetch();
    },
  });

  const handleThresholdSave = useCallback(
    async (payload: TelemetryThresholdUpsertPayload) => {
      if (!selectedDeviceUuid) {
        throw new Error('Select a device before updating thresholds');
      }
      await upsertThresholdMutation.mutateAsync({ deviceUuid: selectedDeviceUuid, payload });
    },
    [selectedDeviceUuid, upsertThresholdMutation],
  );

  const handleThresholdDelete = useCallback(
    async (payload?: TelemetryThresholdDeletePayload) => {
      if (!selectedDeviceUuid) {
        throw new Error('Select a device before updating thresholds');
      }
      await deleteThresholdMutation.mutateAsync({ deviceUuid: selectedDeviceUuid, payload });
    },
    [selectedDeviceUuid, deleteThresholdMutation],
  );

  useEffect(() => {
    if (!selectedDeviceUuid) {
      return;
    }

    if (!isTabVisible) {
      pauseForHidden();
      scheduleStateUpdate(() => {
        setLiveStreamError('Live stream stopped because the tab is hidden.');
      });
    }
  }, [isTabVisible, selectedDeviceUuid, pauseForHidden]);

  useEffect(() => {
    if (!selectedDeviceUuid || !isPollingEnabled) {
      return;
    }

    if (isSessionIdle) {
      pauseForIdle();
      return;
    }

    if (
      mqttSummary.isPaused &&
      (mqttSummary.reason === 'idle' || mqttSummary.reason === 'hidden')
    ) {
      return;
    }

    let closed = false;

    const unsubscribe = subscribeToTelemetryStream(
      selectedDeviceUuid,
      (event) => {
        if (!TELEMETRY_SUFFIXES.includes(event.topicSuffix as TopicSuffix)) {
          return;
        }

        if (topicSuffix && event.topicSuffix !== topicSuffix) {
          return;
        }

        setLiveEvents((prev) => {
          const withoutDuplicate = prev.filter(
            (existing) => toRecordKey(existing) !== toRecordKey(event),
          );
          const next = [event, ...withoutDuplicate];
          return compactLiveBuffer(next);
        });

        const budgetState = recordMqttPacket();
        if (budgetState.isPaused && budgetState.reason === 'cap') {
          pauseForCap();
          // Note: Do not disconnect the live stream on budget cap to maintain connection stability
          // The budget pause prevents further recording but allows live updates to continue
        }
      },
      {
        topicSuffix: topicSuffix || undefined,
        historyLimit,
        historyHours: 6,
        includeHistory: true,
        onTicketIssued: ({ expiresAt }) => {
          if (closed) {
            return;
          }
          setLiveTicketMetadata({ deviceUuid: selectedDeviceUuid, expiresAt: expiresAt ?? null });
          setLiveTicketClockMs(Date.now());
        },
        onError: (error) => {
          if (closed) {
            return;
          }
          setLiveStreamError(
            error.message ||
              'Live telemetry stream disconnected. If this persists, retry device lookup.',
          );
          setLiveTicketMetadata({ deviceUuid: selectedDeviceUuid, expiresAt: null });
        },
      },
    );

    return () => {
      if (!closed) {
        unsubscribe();
      }
      closed = true;
      scheduleStateUpdate(() => {
        setLiveTicketMetadata((prev) =>
          prev.deviceUuid === selectedDeviceUuid
            ? { deviceUuid: selectedDeviceUuid, expiresAt: null }
            : prev,
        );
      });
    };
  }, [
    selectedDeviceUuid,
    isPollingEnabled,
    historyLimit,
    topicSuffix,
    isSessionIdle,
    sessionGeneration,
    mqttSummary.isPaused,
    mqttSummary.reason,
    recordMqttPacket,
    pauseForCap,
    pauseForIdle,
  ]);

  useEffect(() => {
    scheduleStateUpdate(() => {
      setLiveEvents([]);
      setLiveStreamError(null);
      setLiveTicketMetadata({ deviceUuid: null, expiresAt: null });
    });
    resetMqttBudget();
  }, [selectedDeviceUuid, topicSuffix, resetMqttBudget]);

  useEffect(() => {
    scheduleStateUpdate(() => {
      setCommandStatusFilter('all');
    });
  }, [selectedDeviceUuid]);

  const historyErrors = historyQueries
    .map((query) => query.error)
    .filter((value): value is Error => value instanceof Error);

  const eventsBySuffix = useMemo<TelemetryBySuffix>(() => {
    const dataHistory = historyQueries[TELEMETRY_SUFFIX_INDEX['data']]?.data?.records ?? [];
    const daqHistory = historyQueries[TELEMETRY_SUFFIX_INDEX['daq']]?.data?.records ?? [];
    const heartbeatHistory =
      historyQueries[TELEMETRY_SUFFIX_INDEX['heartbeat']]?.data?.records ?? [];

    const liveBySuffix = TELEMETRY_SUFFIXES.reduce(
      (acc, suffix) => {
        acc[suffix] = liveEvents.filter((event) => event.topicSuffix === suffix);
        return acc;
      },
      Object.create(null) as Record<TopicSuffix, TelemetryRecord[]>,
    );

    return {
      data: mergeTelemetry(liveBySuffix.data ?? [], dataHistory),
      daq: mergeTelemetry(liveBySuffix.daq ?? [], daqHistory),
      heartbeat: mergeTelemetry(liveBySuffix.heartbeat ?? [], heartbeatHistory),
    } satisfies TelemetryBySuffix;
  }, [liveEvents, historyQueries]);

  const allTelemetryRecords = useMemo(() => {
    const combined = [
      ...eventsBySuffix.data,
      ...eventsBySuffix.daq,
      ...eventsBySuffix.heartbeat,
    ];
    return combined.sort(
      (a, b) => new Date(b.receivedAt).getTime() - new Date(a.receivedAt).getTime(),
    );
  }, [eventsBySuffix]);

  const latestRecordBySuffix: Record<TopicSuffix, TelemetryRecord | null> = useMemo(
    () => ({
      data: eventsBySuffix.data[0] ?? null,
      daq: eventsBySuffix.daq[0] ?? null,
      heartbeat: eventsBySuffix.heartbeat[0] ?? null,
    }),
    [eventsBySuffix],
  );

  const chartSeries = useMemo(
    () =>
      CHART_CONFIGS.map((config) => ({
        config,
        data: buildTelemetrySeries(eventsBySuffix[config.suffix], config.payloadKey, {
          sortOrder: 'asc',
          scale: config.scale,
        }),
      })),
    [eventsBySuffix],
  );

  const deviceStatus = statusQuery.data?.device ?? null;
  const protocolVersion = deviceStatus?.protocolVersion ?? selectedDevice?.protocolVersion ?? null;
  const heartbeatAgeMinutes = resolveHeartbeatAgeMinutes(latestRecordBySuffix.heartbeat);

  const analyticsCallouts = useMemo(
    () => computeAnalyticsCallouts(eventsBySuffix, heartbeatAgeMinutes),
    [eventsBySuffix, heartbeatAgeMinutes],
  );

  const kpiValues = useMemo(() => {
    const entries = KPI_CONFIGS.map((config) => {
      if (config.payloadKey === 'heartbeatAgeMinutes') {
        return {
          config,
          value: heartbeatAgeMinutes ?? null,
          updatedAt: latestRecordBySuffix.heartbeat?.receivedAt ?? null,
        } as const;
      }

      const latest = latestRecordBySuffix[config.suffix];
      const value = latest ? parseTelemetryNumeric(latest.payload?.[config.payloadKey]) : null;
      return {
        config,
        value,
        updatedAt: latest?.receivedAt ?? null,
      } as const;
    });

    return entries;
  }, [heartbeatAgeMinutes, latestRecordBySuffix]);

  const thresholdView = useMemo<ThresholdView | null>(() => {
    if (!selectedDeviceUuid) {
      return null;
    }

    if (thresholdsQuery.data) {
      const effective = new Map(
        thresholdsQuery.data.thresholds.effective.map((entry) => [entry.parameter, entry]),
      );
      const installationInfo = thresholdsQuery.data.thresholds.installation;
      const installationEntries = installationInfo?.entries
        ? new Map(installationInfo.entries.map((entry) => [entry.parameter, entry]))
        : new Map<string, TelemetryThresholdEntry>();
      const overrideEntries = thresholdsQuery.data.thresholds.override
        ? new Map(
            thresholdsQuery.data.thresholds.override.entries.map((entry) => [
              entry.parameter,
              entry,
            ]),
          )
        : new Map<string, TelemetryThresholdEntry>();

      return {
        effective,
        installation: installationInfo
          ? {
              entries: installationEntries,
              templateId: installationInfo.templateId,
              updatedAt: installationInfo.updatedAt,
              updatedBy: installationInfo.updatedBy ?? null,
              metadata: installationInfo.metadata ?? null,
            }
          : null,
        overrideEntries,
        overrideMeta: thresholdsQuery.data.thresholds.override
          ? {
              reason: thresholdsQuery.data.thresholds.override.reason,
              updatedAt: thresholdsQuery.data.thresholds.override.updatedAt,
              updatedBy: thresholdsQuery.data.thresholds.override.updatedBy ?? null,
            }
          : null,
      };
    }

    const statusThresholds = statusQuery.data?.thresholds;
    if (statusThresholds) {
      const effective = recordToEntryMap(statusThresholds.effective, 'effective');
      const installationEntries = recordToEntryMap(
        statusThresholds.installation?.thresholds,
        'installation',
      );
      const overrideEntries = statusThresholds.override
        ? recordToEntryMap(statusThresholds.override.thresholds, 'override')
        : new Map<string, TelemetryThresholdEntry>();

      return {
        effective,
        installation: statusThresholds.installation
          ? {
              entries: installationEntries,
              templateId: statusThresholds.installation.templateId,
              updatedAt: statusThresholds.installation.updatedAt,
              updatedBy: statusThresholds.installation.updatedBy
                ? { id: statusThresholds.installation.updatedBy, displayName: null }
                : null,
              metadata: statusThresholds.installation.metadata ?? null,
            }
          : null,
        overrideEntries,
        overrideMeta: statusThresholds.override
          ? {
              reason: statusThresholds.override.reason,
              updatedAt: statusThresholds.override.updatedAt,
              updatedBy: statusThresholds.override.updatedBy
                ? { id: statusThresholds.override.updatedBy, displayName: null }
                : null,
            }
          : null,
      };
    }

    return null;
  }, [selectedDeviceUuid, thresholdsQuery.data, statusQuery.data?.thresholds]);

  const resolvedGaugeConfigs = useMemo(() => {
    return GAUGE_CONFIGS.map((config) => {
      const effective = thresholdView?.effective.get(config.payloadKey);
      if (!effective) {
        return config;
      }

      return {
        ...config,
        min: effective.min ?? config.min,
        max: effective.max ?? config.max,
        warnLow: effective.warnLow ?? config.warnLow,
        warnHigh: effective.warnHigh ?? config.warnHigh,
        alertLow: effective.alertLow ?? config.alertLow,
        alertHigh: effective.alertHigh ?? config.alertHigh,
        target: effective.target ?? config.target,
        unit: effective.unit ?? config.unit,
        decimalPlaces: effective.decimalPlaces ?? config.decimalPlaces,
      };
    });
  }, [thresholdView]);

  const canEditThresholds = canManageTelemetryThresholds && !thresholdsQuery.error;
  const thresholdLoadError = (() => {
    if (!ENABLE_THRESHOLD_OVERRIDES) {
      return null;
    }

    if (!canManageTelemetryThresholds) {
      return 'Viewing/editing thresholds requires the alerts:manage capability.';
    }

    if (thresholdsQuery.error) {
      return 'Threshold overrides unavailable. Gauges are using built-in defaults until backend support ships.';
    }

    return null;
  })();
  const thresholdMutationError =
    (upsertThresholdMutation.error instanceof Error
      ? upsertThresholdMutation.error.message
      : null) ??
    (deleteThresholdMutation.error instanceof Error
      ? deleteThresholdMutation.error.message
      : null) ??
    null;

  const activityFeed = useMemo(() => {
    return allTelemetryRecords.slice(0, 12).map((record) => ({
      id: toRecordKey(record),
      description: describeRecord(record),
      receivedAt: record.receivedAt,
      suffix: record.topicSuffix,
    }));
  }, [allTelemetryRecords]);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = deviceIdentifierInput.trim();

    if (!trimmed) {
      setSelectedDevice(null);
      setSelectedDeviceUuid('');
      setLookupError(null);
      setLiveEvents([]);
      setLiveStreamError(null);
      lookupMutation.reset();
      return;
    }

    const lookupParams = buildLookupParams(trimmed);

    if (
      selectedDevice &&
      ((lookupParams.deviceUuid && selectedDevice.uuid === lookupParams.deviceUuid) ||
        (lookupParams.imei && selectedDevice.imei === lookupParams.imei))
    ) {
      historyQueries.forEach((query) => {
        void query.refetch?.();
      });
      void statusQuery.refetch();
      if (canManageTelemetryThresholds) {
        void thresholdsQuery.refetch();
      }
      void commandHistoryQuery.refetch();
      return;
    }

    setLookupError(null);
    lookupMutation.mutate(lookupParams, {
      onSuccess: (result) => {
        setSelectedDevice(result.device);
        setSelectedDeviceUuid(result.device.uuid);
        setDeviceIdentifierInput(result.device.imei ?? trimmed);
        setLiveEvents([]);
        setLiveStreamError(null);
      },
      onError: (error) => {
        const message =
          error instanceof Error && error.message
            ? error.message
            : 'Device lookup failed. Verify the identifier and try again.';
        setLookupError(message);
        setSelectedDevice(null);
        setSelectedDeviceUuid('');
        setLiveEvents([]);
        setLiveStreamError(null);
      },
    });
  };

  const liveSummary = useMemo(() => {
    if (!allTelemetryRecords.length) {
      return null;
    }

    const latest = allTelemetryRecords[0];
    return {
      suffix: latest.topicSuffix,
      receivedAt: latest.receivedAt,
      count: allTelemetryRecords.length,
    };
  }, [allTelemetryRecords]);

  const overrideUpdatedAt = thresholdView?.overrideMeta?.updatedAt ?? 'none';
  const overrideReason = thresholdView?.overrideMeta?.reason ?? '';
  const installationUpdatedAt = thresholdView?.installation?.updatedAt ?? 'none';
  const overrideCount = thresholdView?.overrideEntries.size ?? 0;
  const effectiveCount = thresholdView?.effective.size ?? 0;

  const thresholdDrawerKey = [
    isThresholdDrawerOpen ? 'open' : 'closed',
    selectedDeviceUuid ?? 'none',
    overrideUpdatedAt,
    overrideReason,
    installationUpdatedAt,
    overrideCount,
    effectiveCount,
  ].join('|');

  return (
    <div className="space-y-6">
      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold">Telemetry v2</h2>
        <p className="mt-1 text-sm text-slate-600">
          Look up a device by IMEI or UUID to review live MQTT telemetry, aggregated insights, and
          recent packet history without leaving the page.
        </p>
        <form className="mt-4 flex flex-wrap items-end gap-4" onSubmit={handleSubmit}>
          <div className="min-w-[240px] flex-1">
            <label
              className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
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
                if (lookupMutation.isError) {
                  lookupMutation.reset();
                }
              }}
              placeholder="Enter IMEI (preferred) or UUID"
              autoComplete="off"
              className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
            />
          </div>
          <div>
            <label
              className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
              htmlFor="historyLimit"
            >
              History Limit (per topic)
            </label>
            <select
              id="historyLimit"
              value={historyLimit}
              onChange={(event) =>
                setHistoryLimit(Number(event.target.value) || DEFAULT_HISTORY_LIMIT)
              }
              className="mt-1 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
            >
              {HISTORY_LIMIT_OPTIONS.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label
              className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
              htmlFor="topicSuffix"
            >
              Live Topic Filter
            </label>
            <select
              id="topicSuffix"
              value={topicSuffix}
              onChange={(event) => setTopicSuffix((event.target.value || '') as TopicSuffix | '')}
              className="mt-1 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
            >
              <option value="">All topics</option>
              {TELEMETRY_SUFFIXES.map((suffix) => (
                <option key={suffix} value={suffix}>
                  {suffix}
                </option>
              ))}
            </select>
          </div>
          <button
            type="submit"
            className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-70"
            disabled={lookupMutation.isPending}
          >
            {lookupMutation.isPending ? 'Searching…' : 'Find Device'}
          </button>
        </form>
        {lookupError && (
          <p className="mt-3 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {lookupError}
          </p>
        )}
        {!selectedDevice && !lookupError && (
          <p className="mt-4 text-sm text-slate-500">
            Select a device to load telemetry insights. Live streaming will attach automatically
            once the device is found.
          </p>
        )}
      </section>

      {selectedDevice && (
        <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h3 className="text-base font-semibold text-slate-800">Device Summary</h3>
              <dl className="mt-3 grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
                <div>
                  <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    IMEI
                  </dt>
                  <dd className="mt-1 font-medium text-slate-800">{selectedDevice.imei || '—'}</dd>
                </div>
                <div>
                  <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Device UUID
                  </dt>
                  <dd className="mt-1 break-all font-mono text-xs text-slate-700">
                    {selectedDevice.uuid}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Connectivity
                  </dt>
                  <dd className="mt-1">
                    <StatusBadge status={deviceStatus?.connectivityStatus} />
                    {deviceStatus?.lastTelemetryAt && (
                      <span className="ml-2 text-xs text-slate-500">
                        Last telemetry {formatDateTimeWithSeconds(deviceStatus.lastTelemetryAt)}
                      </span>
                    )}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Protocol Version
                  </dt>
                  <dd className="mt-1 text-sm text-slate-700">
                    {protocolVersion
                      ? `${protocolVersion.version}${protocolVersion.name ? ` · ${protocolVersion.name}` : ''}`
                      : '—'}
                  </dd>
                </div>
              </dl>
            </div>
            <div className="lg:w-64">
              <div className="rounded border border-slate-200 bg-slate-50 p-4">
                <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                  Live Stream
                </h4>
                <dl className="mt-3 space-y-2 text-sm">
                  <div className="flex items-center justify-between">
                    <dt className="text-slate-500">Packets buffered</dt>
                    <dd className="font-semibold text-slate-800">{allTelemetryRecords.length}</dd>
                  </div>
                  <div className="flex items-center justify-between">
                    <dt className="text-slate-500">Latest topic</dt>
                    <dd className="font-semibold capitalize text-slate-800">
                      {liveSummary?.suffix ?? '—'}
                    </dd>
                  </div>
                  <div className="flex items-center justify-between">
                    <dt className="text-slate-500">Last update</dt>
                    <dd className="font-semibold text-slate-800">
                      {liveSummary?.receivedAt ? formatDateTimeShort(liveSummary.receivedAt) : '—'}
                    </dd>
                  </div>
                  <div className="flex items-center justify-between">
                    <dt className="text-slate-500">Topic filter</dt>
                    <dd className="font-semibold capitalize text-slate-800">
                      {topicSuffix ? topicSuffix : 'all'}
                    </dd>
                  </div>
                  <div className="flex items-center justify-between">
                    <dt className="text-slate-500">Ticket expires</dt>
                    <dd
                      className={`font-semibold ${
                        isLiveTicketExpiringSoon ? 'text-amber-600' : 'text-slate-800'
                      }`}
                    >
                      {liveTicketCountdownLabel ?? '—'}
                    </dd>
                  </div>
                </dl>
                <p className="mt-2 text-xs text-slate-500">
                  Auto-flushes when {LIVE_BUFFER_LIMIT} packets are captured so the buffer stays hot
                  and ready for new telemetry.
                </p>
              </div>
            </div>
          </div>
          {livePauseMessage && (
            <div className="mt-4 rounded border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <p>{livePauseMessage}</p>
                <button
                  type="button"
                  onClick={handleResumeLiveFeed}
                  className="inline-flex items-center justify-center rounded bg-emerald-600 px-3 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-1"
                >
                  Resume live stream
                </button>
              </div>
              {livePauseReason === 'cap' && (
                <p className="mt-2 text-xs text-amber-700">
                  Packets received this session: {mqttSummary.count.toLocaleString()} /{' '}
                  {mqttPacketCap.toLocaleString()}. Budget resets when you resume.
                </p>
              )}
              {livePauseReason === 'idle' && (
                <p className="mt-2 text-xs text-amber-700">
                  Interaction or the resume button reactivates streaming immediately.
                </p>
              )}
            </div>
          )}
          {selectedDeviceUuid && liveTicketCountdownLabel && (
            <div
              className={`mt-4 rounded border px-4 py-3 text-sm ${
                isLiveTicketExpiringSoon
                  ? 'border-amber-200 bg-amber-50 text-amber-800'
                  : 'border-emerald-100 bg-emerald-50 text-emerald-800'
              }`}
            >
              <p>
                Live telemetry token expires in{' '}
                <span className="font-semibold">{liveTicketCountdownLabel}</span>
                {liveTicketAbsoluteLabel ? ` (${liveTicketAbsoluteLabel})` : ''}.{' '}
                {isLiveTicketExpiringSoon
                  ? 'Resume or re-run the lookup soon to renew before the stream disconnects.'
                  : 'Re-open the stream whenever you need to renew the token.'}
              </p>
            </div>
          )}
          {liveStreamError && (
            <p className="mt-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-700">
              {liveStreamError}
            </p>
          )}
          {historyErrors.length > 0 && (
            <p className="mt-4 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {historyErrors[0]?.message ??
                'Historical telemetry failed to load. Try again shortly.'}
            </p>
          )}
        </section>
      )}

      {selectedDevice && (
        <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-wrap items-center gap-2">
              {TABS.map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  onClick={() => setActiveTab(tab.id)}
                  className={`rounded-full px-4 py-1 text-sm font-semibold transition-colors ${
                    activeTab === tab.id
                      ? 'bg-emerald-600 text-white'
                      : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                  }`}
                >
                  {tab.label}
                </button>
              ))}
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => setThresholdDrawerOpen(true)}
                className="rounded-full border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 transition-colors hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
              >
                View thresholds
              </button>
            </div>
          </div>

          <div className="mt-6">
            {activeTab === 'dashboard' && (
              <DashboardTab
                gauges={resolvedGaugeConfigs}
                kpis={kpiValues}
                eventsBySuffix={eventsBySuffix}
                latestRecordBySuffix={latestRecordBySuffix}
                activityFeed={activityFeed}
                thresholds={thresholdView}
              />
            )}
            {activeTab === 'graphs' && (
              <GraphsTab
                chartSeries={chartSeries}
                analyticsCallouts={analyticsCallouts}
                enableAnalytics={ENABLE_ADVANCED_ANALYTICS}
              />
            )}
            {activeTab === 'data' && (
              <DataTableTab
                groups={TABLE_GROUPS}
                latestRecordBySuffix={latestRecordBySuffix}
                eventsBySuffix={eventsBySuffix}
              />
            )}
            {activeTab === 'commands' && (
              <CommandsTab
                key={selectedDevice?.uuid ?? 'none'}
                device={selectedDevice}
                commandStatusFilter={commandStatusFilter}
                onCommandStatusFilterChange={setCommandStatusFilter}
                commandHistoryRecords={commandHistoryRecords}
                isHistoryLoading={isCommandHistoryLoading}
                isHistoryRefreshing={isCommandHistoryRefreshing}
                historyError={commandHistoryErrorMessage}
                hasNextPage={hasCommandHistoryNextPage}
                onLoadMore={() => {
                  void commandHistoryQuery.fetchNextPage();
                }}
                isFetchingNextPage={isFetchingNextCommandPage}
                onRefreshHistory={() => {
                  void commandHistoryQuery.refetch();
                }}
                issueCommandMutation={issueCommandMutation}
                telemetryRecords={allTelemetryRecords}
                enableRawDiff={ENABLE_RAW_DIFF_TOOLING}
              />
            )}
          </div>
        </section>
      )}
      <ThresholdDrawer
        key={thresholdDrawerKey}
        open={isThresholdDrawerOpen}
        onClose={() => setThresholdDrawerOpen(false)}
        gaugeConfigs={GAUGE_CONFIGS}
        thresholds={thresholdView}
        onSave={handleThresholdSave}
        onDelete={handleThresholdDelete}
        isLoading={thresholdsQuery.isLoading}
        isSaving={upsertThresholdMutation.isPending}
        isDeleting={deleteThresholdMutation.isPending}
        loadError={thresholdLoadError}
        mutationError={thresholdMutationError}
        canEdit={canEditThresholds}
      />
    </div>
  );
}

type DashboardTabProps = {
  gauges: DashboardGaugeConfig[];
  kpis: Array<{
    config: DashboardKpiConfig;
    value: number | null;
    updatedAt: string | null;
  }>;
  eventsBySuffix: TelemetryBySuffix;
  latestRecordBySuffix: Record<TopicSuffix, TelemetryRecord | null>;
  activityFeed: Array<{
    id: string;
    description: string;
    receivedAt: string;
    suffix: string;
  }>;
  thresholds: ThresholdView | null;
};

function DashboardTab({
  gauges,
  kpis,
  eventsBySuffix,
  latestRecordBySuffix,
  activityFeed,
  thresholds,
}: DashboardTabProps) {
  return (
    <div className="space-y-6">
      <section>
        <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
          Live Gauges
        </h3>
        <div className="mt-3 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {gauges.map((config) => {
            const records = eventsBySuffix[config.suffix];
            const latest = records[0] ?? null;
            const previous = records.find((record) => record !== latest);
            const value = latest
              ? parseTelemetryNumeric(latest.payload?.[config.payloadKey])
              : null;
            const delta = previous
              ? parseTelemetryNumeric(previous.payload?.[config.payloadKey])
              : null;
            const trend = computeTrend(records, config.payloadKey);
            const threshold = thresholds?.effective.get(config.payloadKey) ?? null;
            const overrideEntry = thresholds?.overrideEntries?.get(config.payloadKey) ?? null;
            const installationEntry =
              thresholds?.installation?.entries?.get(config.payloadKey) ?? null;
            const thresholdOrigin: GaugeThresholdOrigin = overrideEntry
              ? 'override'
              : installationEntry
                ? 'installation'
                : threshold
                  ? 'protocol'
                  : null;
            return (
              <GaugeCard
                key={config.id}
                config={config}
                value={value}
                previousValue={delta}
                trend={trend}
                updatedAt={latest?.receivedAt ?? null}
                threshold={threshold}
                thresholdOrigin={thresholdOrigin}
              />
            );
          })}
        </div>
      </section>

      <section>
        <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
          Key Metrics
        </h3>
        <div className="mt-3 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {kpis.map(({ config, value, updatedAt }) => (
            <div key={config.id} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                {config.label}
              </p>
              <p className="mt-2 text-2xl font-semibold text-slate-800">
                {formatNumber(value, config.unit, config.fractionDigits ?? 1)}
              </p>
              {config.description && (
                <p className="mt-1 text-xs text-slate-500">{config.description}</p>
              )}
              {updatedAt && (
                <p className="mt-2 text-xs text-slate-400">
                  Updated {formatDateTimeShort(updatedAt)}
                </p>
              )}
            </div>
          ))}
        </div>
      </section>

      <section>
        <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
          Recent Activity
        </h3>
        <ul className="mt-3 space-y-3">
          {activityFeed.length === 0 && (
            <li className="rounded border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
              Waiting for telemetry packets…
            </li>
          )}
          {activityFeed.map((item) => (
            <li
              key={item.id}
              className="flex items-start justify-between gap-3 rounded border border-slate-200 bg-white px-4 py-3 shadow-sm"
            >
              <div>
                <p className="text-sm font-medium text-slate-800">{item.description}</p>
                <p className="mt-1 text-xs capitalize text-slate-500">{item.suffix}</p>
              </div>
              <span className="text-xs text-slate-500">{formatDateTimeShort(item.receivedAt)}</span>
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
          Latest Packets
        </h3>
        <div className="mt-3 grid gap-4 md:grid-cols-2">
          {TELEMETRY_SUFFIXES.map((suffix) => {
            const record = latestRecordBySuffix[suffix];
            return (
              <div key={suffix} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                  {suffix}
                </p>
                <p className="mt-1 text-xs text-slate-500">
                  {record?.receivedAt ? formatDateTimeShort(record.receivedAt) : 'No packets yet'}
                </p>
                <pre className="mt-3 max-h-40 overflow-y-auto rounded bg-slate-900/90 p-3 text-xs text-emerald-100">
                  {record ? JSON.stringify(record.payload ?? {}, null, 2) : '// awaiting payload'}
                </pre>
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}

type GaugeCardProps = {
  config: DashboardGaugeConfig;
  value: number | null;
  previousValue: number | null;
  trend: number | null;
  updatedAt: string | null;
  threshold: TelemetryThresholdEntry | null;
  thresholdOrigin: GaugeThresholdOrigin;
};

type GaugeThresholdOrigin = 'override' | 'installation' | 'protocol' | null;

function GaugeCard({
  config,
  value,
  previousValue,
  trend,
  updatedAt,
  threshold,
  thresholdOrigin,
}: GaugeCardProps) {
  const min = config.min ?? 0;
  const max = config.max ?? (config.min ?? 0) + 1;
  const severity = evaluateGaugeStatus(value, {
    min,
    max,
    warnLow: config.warnLow,
    warnHigh: config.warnHigh,
    alertLow: config.alertLow,
    alertHigh: config.alertHigh,
  });
  const diff =
    trend !== null
      ? `${trend > 0 ? '+' : ''}${trend.toFixed(config.decimalPlaces ?? 1)}${config.unit ? ` ${config.unit}` : ''}`
      : null;

  const severityBadge =
    severity.status === 'alert' ? (
      <span className="rounded-full bg-rose-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-rose-700">
        Alert
      </span>
    ) : severity.status === 'warn' ? (
      <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-amber-700">
        Warning
      </span>
    ) : null;

  const severityMessage = (() => {
    if (value === null || severity.status === 'normal' || severity.status === 'idle') {
      return null;
    }
    const directionLabel =
      severity.direction === 'low' ? 'below' : severity.direction === 'high' ? 'above' : 'outside';
    const thresholdLabel =
      severity.threshold !== null
        ? formatNumber(severity.threshold, config.unit, config.decimalPlaces ?? 1)
        : 'the configured limit';

    return `${severity.status === 'alert' ? 'Alert' : 'Warning'}: reading ${directionLabel} ${thresholdLabel}`;
  })();

  const fallbackThreshold: TelemetryThresholdConfig = {
    min: config.min ?? null,
    max: config.max ?? null,
    warnLow: config.warnLow ?? null,
    warnHigh: config.warnHigh ?? null,
    alertLow: config.alertLow ?? null,
    alertHigh: config.alertHigh ?? null,
    target: config.target ?? null,
    unit: config.unit ?? null,
    decimalPlaces: config.decimalPlaces ?? null,
  };
  const thresholdSummary = formatThresholdSummary(threshold ?? fallbackThreshold);
  const thresholdOriginLabelMap: Record<Exclude<GaugeThresholdOrigin, null>, string> = {
    override: 'Override thresholds active',
    installation: 'Installation template thresholds',
    protocol: 'Protocol defaults',
  };
  const thresholdOriginLabel =
    thresholdOrigin && thresholdOriginLabelMap[thresholdOrigin]
      ? thresholdOriginLabelMap[thresholdOrigin]
      : null;

  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
          {config.label}
        </p>
        <div className="flex flex-col items-end gap-1 text-right">
          {updatedAt && (
            <span className="text-xs text-slate-400">Updated {formatDateTimeShort(updatedAt)}</span>
          )}
          {severityBadge}
        </div>
      </div>
      <div className="mt-1">
        <SemiCircularGauge
          value={value}
          min={min}
          max={max}
          warnLow={config.warnLow}
          warnHigh={config.warnHigh}
          alertLow={config.alertLow}
          alertHigh={config.alertHigh}
          unit={config.unit}
          decimalPlaces={config.decimalPlaces ?? 1}
        />
      </div>
      <div className="mt-3 flex items-center justify-between text-xs text-slate-500">
        <span>
          Range {min.toLocaleString()} - {max.toLocaleString()} {config.unit ?? ''}
        </span>
        {diff && (
          <span className={trend !== null && trend < 0 ? 'text-rose-600' : 'text-emerald-600'}>
            {diff}
          </span>
        )}
      </div>
      {previousValue !== null && value !== null && (
        <p className="mt-1 text-xs text-slate-500">
          Prev {formatNumber(previousValue, config.unit, config.decimalPlaces ?? 1)}
        </p>
      )}
      {severityMessage && (
        <p
          className={`mt-1 text-xs font-semibold ${severity.status === 'alert' ? 'text-rose-600' : 'text-amber-600'}`}
        >
          {severityMessage}
        </p>
      )}
      {value === null && (
        <p className="mt-1 text-xs text-slate-500">
          No recent packets on the {config.suffix} topic.
        </p>
      )}
      {thresholdSummary && thresholdSummary !== '—' && (
        <p className="mt-2 text-[11px] text-slate-500">Thresholds: {thresholdSummary}</p>
      )}
      {thresholdOriginLabel && (
        <p className="text-[11px] text-slate-400">Source: {thresholdOriginLabel}</p>
      )}
    </div>
  );
}

type GraphsTabProps = {
  chartSeries: Array<{
    config: TelemetryChartConfig;
    data: ReturnType<typeof buildTelemetrySeries>;
  }>;
  analyticsCallouts: AnalyticsCallout[];
  enableAnalytics: boolean;
};

function GraphsTab({ chartSeries, analyticsCallouts, enableAnalytics }: GraphsTabProps) {
  return (
    <div className="space-y-6">
      {enableAnalytics && analyticsCallouts.length > 0 && (
        <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
          <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
            Analytics Callouts
          </h3>
          <div className="mt-3 grid gap-3 md:grid-cols-3">
            {analyticsCallouts.map((callout) => (
              <div
                key={callout.id}
                className={`rounded border p-3 text-sm shadow-sm ${
                  callout.emphasis === 'negative'
                    ? 'border-rose-200 bg-rose-50'
                    : callout.emphasis === 'positive'
                      ? 'border-emerald-200 bg-emerald-50'
                      : 'border-slate-200 bg-slate-50'
                }`}
              >
                <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                  {callout.label}
                </p>
                <p className="mt-1 text-xl font-semibold text-slate-800">{callout.value}</p>
                {callout.description && (
                  <p className="mt-1 text-xs text-slate-600">{callout.description}</p>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      {chartSeries.map(({ config, data }) => (
        <div key={config.id} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
          <div className="flex flex-col gap-2 pb-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="text-sm font-semibold text-slate-700">{config.label}</h3>
              {config.description && <p className="text-xs text-slate-500">{config.description}</p>}
            </div>
            <span className="text-xs capitalize text-slate-500">Topic: {config.suffix}</span>
          </div>
          <TelemetryChart data={data} parameterLabel={config.label} unit={config.unit} />
        </div>
      ))}
    </div>
  );
}

type DataTableTabProps = {
  groups: TelemetryTableGroup[];
  latestRecordBySuffix: Record<TopicSuffix, TelemetryRecord | null>;
  eventsBySuffix: TelemetryBySuffix;
};

function DataTableTab({ groups, latestRecordBySuffix, eventsBySuffix }: DataTableTabProps) {
  return (
    <div className="space-y-6">
      {groups.map((group) => (
        <section key={group.id} className="rounded-lg border border-slate-200 bg-white shadow-sm">
          <header className="border-b border-slate-200 bg-slate-50 px-4 py-3">
            <h3 className="text-sm font-semibold text-slate-700">{group.title}</h3>
          </header>
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th scope="col" className="px-4 py-2 text-left font-semibold">
                    Parameter
                  </th>
                  <th scope="col" className="px-4 py-2 text-left font-semibold">
                    Value
                  </th>
                  <th scope="col" className="px-4 py-2 text-left font-semibold">
                    Updated
                  </th>
                  <th scope="col" className="px-4 py-2 text-left font-semibold">
                    Trend (Δ)
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 bg-white">
                {group.rows.map((row) => {
                  const latest = latestRecordBySuffix[row.suffix];
                  const records = eventsBySuffix[row.suffix];
                  const value = latest
                    ? parseTelemetryNumeric(latest.payload?.[row.payloadKey])
                    : null;
                  const trend = computeTrend(records, row.payloadKey);
                  return (
                    <tr key={row.id}>
                      <td className="px-4 py-3">
                        <div className="font-medium text-slate-800">{row.label}</div>
                        {row.description && (
                          <div className="text-xs text-slate-500">{row.description}</div>
                        )}
                      </td>
                      <td className="px-4 py-3 font-semibold text-slate-800">
                        {formatNumber(value, row.unit, row.fractionDigits ?? 1)}
                      </td>
                      <td className="px-4 py-3 text-sm text-slate-500">
                        {latest?.receivedAt ? formatDateTimeShort(latest.receivedAt) : '—'}
                      </td>
                      <td className="px-4 py-3 text-sm">
                        {trend === null ? (
                          <span className="text-slate-400">—</span>
                        ) : (
                          <span className={trend < 0 ? 'text-rose-600' : 'text-emerald-600'}>
                            {trend > 0 ? '+' : ''}
                            {trend.toFixed(row.fractionDigits ?? 1)}
                            {row.unit ? ` ${row.unit}` : ''}
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </section>
      ))}
    </div>
  );
}

type RawViewTabProps = {
  records: TelemetryRecord[];
  enableDiffTooling: boolean;
};

type CommandsTabProps = {
  device: DeviceSummary;
  commandStatusFilter: CommandHistoryFilter;
  onCommandStatusFilterChange: (value: CommandHistoryFilter) => void;
  commandHistoryRecords: DeviceCommandHistoryRecord[];
  isHistoryLoading: boolean;
  isHistoryRefreshing: boolean;
  historyError: string | null;
  hasNextPage: boolean;
  onLoadMore: () => void;
  isFetchingNextPage: boolean;
  onRefreshHistory: () => void;
  issueCommandMutation: UseMutationResult<
    IssueDeviceCommandResponse,
    Error,
    IssueDeviceCommandPayload
  >;
  telemetryRecords: TelemetryRecord[];
  enableRawDiff: boolean;
};

export function CommandsTab({
  device,
  commandStatusFilter,
  onCommandStatusFilterChange,
  commandHistoryRecords,
  isHistoryLoading,
  isHistoryRefreshing,
  historyError,
  hasNextPage,
  onLoadMore,
  isFetchingNextPage,
  onRefreshHistory,
  issueCommandMutation,
  telemetryRecords,
  enableRawDiff,
}: CommandsTabProps) {
  const [commandName, setCommandName] = useState('');
  const [payloadInput, setPayloadInput] = useState('{}');
  const [qosInput, setQosInput] = useState('');
  const [timeoutInput, setTimeoutInput] = useState('30');
  const [issuedByInput, setIssuedByInput] = useState('');
  const [simulatorTokenInput, setSimulatorTokenInput] = useState('');
  const [formError, setFormError] = useState<string | null>(null);
  const [lastIssued, setLastIssued] = useState<IssueDeviceCommandResponse | null>(null);

  useEffect(() => {
    issueCommandMutation.reset();
  }, [issueCommandMutation]);

  const handleSubmitCommand = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setFormError(null);

      const trimmedName = commandName.trim();
      if (!trimmedName) {
        setFormError('Command name is required');
        return;
      }

      let parsedPayload: Record<string, unknown> | undefined;
      const rawPayload = payloadInput.trim();
      if (rawPayload) {
        try {
          const parsed = JSON.parse(rawPayload);
          if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') {
            setFormError('Command payload must be a JSON object');
            return;
          }
          parsedPayload = parsed as Record<string, unknown>;
        } catch {
          setFormError('Command payload must be valid JSON');
          return;
        }
      }

      let qosValue: number | undefined;
      const qosTrimmed = qosInput.trim();
      if (qosTrimmed) {
        const parsedQos = Number(qosTrimmed);
        if (!Number.isInteger(parsedQos) || parsedQos < 0 || parsedQos > 2) {
          setFormError('QoS must be 0, 1 or 2');
          return;
        }
        qosValue = parsedQos;
      }

      let timeoutSeconds: number | undefined;
      const timeoutTrimmed = timeoutInput.trim();
      if (timeoutTrimmed) {
        const parsedTimeout = Number(timeoutTrimmed);
        if (!Number.isInteger(parsedTimeout) || parsedTimeout < 5 || parsedTimeout > 600) {
          setFormError('Timeout must be between 5 and 600 seconds');
          return;
        }
        timeoutSeconds = parsedTimeout;
      }

      const issuedBy = issuedByInput.trim();
      const simulatorToken = simulatorTokenInput.trim();

      issueCommandMutation.reset();

      const commandPayload: IssueDeviceCommandPayload['command'] = {
        name: trimmedName,
      };

      if (parsedPayload) {
        commandPayload.payload = parsedPayload;
      }

      const payload: IssueDeviceCommandPayload = {
        command: commandPayload,
        qos: qosValue,
        timeoutSeconds,
        issuedBy: issuedBy || undefined,
        simulatorSessionToken: simulatorToken || undefined,
      };

      try {
        const result = await issueCommandMutation.mutateAsync(payload);
        setLastIssued(result);
        setFormError(null);
        setPayloadInput('{}');
      } catch (error) {
        if (error instanceof Error && error.message) {
          setFormError(error.message);
        } else {
          setFormError('Failed to issue ondemand command. Try again shortly.');
        }
      }
    },
    [
      commandName,
      payloadInput,
      qosInput,
      timeoutInput,
      issuedByInput,
      simulatorTokenInput,
      issueCommandMutation,
    ],
  );

  return (
    <div className="space-y-6">
      <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
              Command Composer
            </h3>
            <p className="text-xs text-slate-500">
              Targeting IMEI {device.imei ?? '—'} · Device UUID {device.uuid}
            </p>
          </div>
          {lastIssued && (
            <div className="rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700">
              Command queued ({lastIssued.msgid}). Awaiting acknowledgement from topic{' '}
              <span className="font-mono text-[11px]">{lastIssued.topic}</span>
              {lastIssued.simulatorSessionId && (
                <span> · Simulator session {lastIssued.simulatorSessionId}</span>
              )}
            </div>
          )}
        </div>
        <form className="mt-4 space-y-4" onSubmit={handleSubmitCommand}>
          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <label
                className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
                htmlFor="commandName"
              >
                Command Name
              </label>
              <input
                id="commandName"
                name="commandName"
                value={commandName}
                onChange={(event) => setCommandName(event.target.value)}
                placeholder="Enter ondemand command key"
                className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                autoComplete="off"
                required
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label
                  className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
                  htmlFor="commandQos"
                >
                  QoS (0-2)
                </label>
                <input
                  id="commandQos"
                  name="commandQos"
                  value={qosInput}
                  onChange={(event) => setQosInput(event.target.value)}
                  placeholder="Default"
                  className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                  inputMode="numeric"
                  pattern="[0-2]?"
                />
              </div>
              <div>
                <label
                  className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
                  htmlFor="commandTimeout"
                >
                  Timeout (seconds)
                </label>
                <input
                  id="commandTimeout"
                  name="commandTimeout"
                  value={timeoutInput}
                  onChange={(event) => setTimeoutInput(event.target.value)}
                  placeholder="30"
                  className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                  inputMode="numeric"
                  pattern="[0-9]*"
                />
              </div>
            </div>
            <div>
              <label
                className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
                htmlFor="commandIssuedBy"
              >
                Issued By (optional)
              </label>
              <input
                id="commandIssuedBy"
                name="commandIssuedBy"
                value={issuedByInput}
                onChange={(event) => setIssuedByInput(event.target.value)}
                placeholder="Operator ID"
                className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                autoComplete="off"
              />
            </div>
            <div>
              <label
                className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
                htmlFor="commandSimulatorToken"
              >
                Simulator Session Token
              </label>
              <input
                id="commandSimulatorToken"
                name="commandSimulatorToken"
                value={simulatorTokenInput}
                onChange={(event) => setSimulatorTokenInput(event.target.value)}
                placeholder="Optional (sim testing)"
                className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                autoComplete="off"
              />
            </div>
          </div>
          <div>
            <label
              className="block text-xs font-semibold uppercase tracking-wide text-slate-500"
              htmlFor="commandPayload"
            >
              Payload (JSON)
            </label>
            <textarea
              id="commandPayload"
              name="commandPayload"
              value={payloadInput}
              onChange={(event) => setPayloadInput(event.target.value)}
              rows={6}
              className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 font-mono text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="{}"
            />
            <p className="mt-1 text-xs text-slate-500">
              Provide a JSON object matching the ondemand protocol expectations. Leave empty for
              commands that do not require payload data.
            </p>
          </div>
          {(formError || issueCommandMutation.error) && (
            <p className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {formError ?? issueCommandMutation.error?.message ?? 'Unable to issue command.'}
            </p>
          )}
          <div className="flex flex-wrap items-center gap-3">
            <button
              type="submit"
              className="inline-flex items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={issueCommandMutation.isPending}
            >
              {issueCommandMutation.isPending ? 'Sending…' : 'Send Command'}
            </button>
            <button
              type="button"
              onClick={() => {
                setPayloadInput('{}');
                setCommandName('');
                setQosInput('');
                setTimeoutInput('30');
                setIssuedByInput('');
                setSimulatorTokenInput('');
                setFormError(null);
                setLastIssued(null);
                issueCommandMutation.reset();
              }}
              className="inline-flex items-center justify-center rounded border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-600 transition-colors hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-1"
            >
              Clear Form
            </button>
          </div>
        </form>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
              Command History
            </h3>
            <p className="text-xs text-slate-500">
              Showing {commandHistoryRecords.length} entr
              {commandHistoryRecords.length === 1 ? 'y' : 'ies'} (newest first)
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {COMMAND_STATUS_FILTERS.map((filter) => (
              <button
                key={filter.id}
                type="button"
                onClick={() => onCommandStatusFilterChange(filter.id)}
                className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-wide transition-colors ${
                  commandStatusFilter === filter.id
                    ? 'bg-emerald-600 text-white'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                }`}
              >
                {filter.label}
              </button>
            ))}
            <button
              type="button"
              onClick={onRefreshHistory}
              className="rounded-full border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 transition-colors hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={isHistoryRefreshing}
            >
              {isHistoryRefreshing ? 'Refreshing…' : 'Refresh'}
            </button>
          </div>
        </div>

        {historyError && (
          <p className="mt-3 rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {historyError}
          </p>
        )}

        {isHistoryLoading && commandHistoryRecords.length === 0 && !historyError && (
          <p className="mt-4 text-sm text-slate-500">Loading command history…</p>
        )}

        {!isHistoryLoading && commandHistoryRecords.length === 0 && !historyError && (
          <p className="mt-4 text-sm text-slate-500">
            No commands issued yet for this device. Use the composer above to queue an ondemand
            command.
          </p>
        )}

        <div className="mt-4 space-y-4">
          {commandHistoryRecords.map((record) => (
            <CommandHistoryItem key={record.msgid} record={record} />
          ))}
        </div>

        {(hasNextPage || isFetchingNextPage) && (
          <div className="mt-4 flex justify-end">
            <button
              type="button"
              onClick={() => {
                if (!hasNextPage || isFetchingNextPage) {
                  return;
                }
                onLoadMore();
              }}
              className="inline-flex items-center justify-center rounded border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-600 transition-colors hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!hasNextPage || isFetchingNextPage}
            >
              {isFetchingNextPage ? 'Loading more…' : 'Load older commands'}
            </button>
          </div>
        )}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
        <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
          Raw Telemetry Buffer
        </h3>
        <div className="mt-3">
          <RawViewTab records={telemetryRecords} enableDiffTooling={enableRawDiff} />
        </div>
      </section>
    </div>
  );
}

type CommandHistoryItemProps = {
  record: DeviceCommandHistoryRecord;
};

function CommandHistoryItem({ record }: CommandHistoryItemProps) {
  const requestedAtLabel = formatDateTimeShort(record.requestedAt);
  const acknowledgedAtLabel = record.acknowledgedAt
    ? formatDateTimeShort(record.acknowledgedAt)
    : null;

  let durationLabel: string | null = null;
  if (record.acknowledgedAt) {
    const requested = new Date(record.requestedAt).getTime();
    const acknowledged = new Date(record.acknowledgedAt).getTime();
    if (Number.isFinite(requested) && Number.isFinite(acknowledged) && acknowledged >= requested) {
      durationLabel = formatDuration(acknowledged - requested, { fallback: '--' });
    }
  }

  const expectedTimeoutAtLabel = record.expectedTimeoutAt
    ? formatDateTimeShort(record.expectedTimeoutAt)
    : null;
  const timedOutAtLabel = record.metadata.timedOutAt
    ? formatDateTimeShort(record.metadata.timedOutAt)
    : null;
  const hasTimedOut = record.metadata.timedOutAt !== null;

  return (
    <article className="rounded-lg border border-slate-200 bg-slate-50 p-4 shadow-sm">
      <header className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h4 className="text-sm font-semibold text-slate-800">{record.command.name}</h4>
          <p className="text-xs text-slate-500">
            Issued {requestedAtLabel} · MsgID {record.msgid}
            {record.metadata.issuedBy ? ` · By ${record.metadata.issuedBy}` : ''}
          </p>
        </div>
        <CommandStatusBadge status={record.status} />
      </header>

      <dl className="mt-3 grid gap-3 text-xs text-slate-600 sm:grid-cols-3">
        <div>
          <dt className="font-semibold uppercase tracking-wide text-slate-500">Acknowledged</dt>
          <dd className="mt-1">
            {acknowledgedAtLabel ?? (record.status === 'pending' ? 'Awaiting response' : '—')}
          </dd>
        </div>
        <div>
          <dt className="font-semibold uppercase tracking-wide text-slate-500">Duration</dt>
          <dd className="mt-1">
            {durationLabel ?? (record.status === 'pending' ? 'Pending' : '—')}
          </dd>
        </div>
        <div>
          <dt className="font-semibold uppercase tracking-wide text-slate-500">Publish Topic</dt>
          <dd className="mt-1 font-mono text-[11px] text-slate-700">
            {record.metadata.publishTopic ?? '—'}
          </dd>
        </div>
        <div>
          <dt className="font-semibold uppercase tracking-wide text-slate-500">Timeout</dt>
          <dd className="mt-1">
            {hasTimedOut
              ? timedOutAtLabel
                ? `Timed out @ ${timedOutAtLabel}`
                : 'Timed out'
              : (expectedTimeoutAtLabel ?? '—')}
          </dd>
        </div>
        <div>
          <dt className="font-semibold uppercase tracking-wide text-slate-500">Protocol</dt>
          <dd className="mt-1">
            {record.metadata.protocolVersion ?? '—'}
            {record.metadata.serverVendorId ? ` · Vendor ${record.metadata.serverVendorId}` : ''}
          </dd>
        </div>
        <div>
          <dt className="font-semibold uppercase tracking-wide text-slate-500">
            Simulator Session
          </dt>
          <dd className="mt-1">
            {record.metadata.simulatorSessionId ? record.metadata.simulatorSessionId : '—'}
          </dd>
        </div>
      </dl>

      {record.metadata.timeoutReason && (
        <p className="mt-2 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
          Timeout reason: {record.metadata.timeoutReason}
        </p>
      )}

      <div className="mt-4 grid gap-4 md:grid-cols-2">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Command Payload
          </p>
          <JsonPreview data={record.command.payload} placeholder="// no payload provided" />
        </div>
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Response Payload
          </p>
          <JsonPreview
            data={record.response}
            placeholder={
              record.status === 'pending' ? '// awaiting response' : '// no response payload'
            }
          />
        </div>
      </div>

      {record.events.length > 0 && (
        <div className="mt-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Timeline</p>
          <ul className="mt-2 space-y-2">
            {record.events.map((event) => (
              <li
                key={event.id}
                className="rounded border border-slate-200 bg-white px-3 py-2 text-xs text-slate-600 shadow-sm"
              >
                <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                  <span className="font-semibold text-slate-700">{event.type}</span>
                  <span className="font-mono text-[11px] text-slate-500">
                    {formatDateTimeShort(event.createdAt)}
                  </span>
                </div>
                <span
                  className={`mt-1 inline-flex w-fit items-center rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${
                    event.severity === 'error'
                      ? 'bg-rose-100 text-rose-700'
                      : event.severity === 'warn'
                        ? 'bg-amber-100 text-amber-700'
                        : 'bg-slate-200 text-slate-600'
                  }`}
                >
                  {event.severity}
                </span>
                <pre className="mt-2 max-h-40 overflow-y-auto rounded bg-slate-900/90 p-2 text-[11px] text-emerald-100">
                  {JSON.stringify(event.payload ?? {}, null, 2)}
                </pre>
              </li>
            ))}
          </ul>
        </div>
      )}
    </article>
  );
}

type CommandStatusBadgeProps = {
  status: DeviceCommandStatus;
};

function CommandStatusBadge({ status }: CommandStatusBadgeProps) {
  const styleMap: Record<DeviceCommandStatus, string> = {
    pending: 'bg-amber-100 text-amber-700 border border-amber-200',
    acknowledged: 'bg-emerald-100 text-emerald-700 border border-emerald-200',
    failed: 'bg-rose-100 text-rose-700 border border-rose-200',
  };
  const labelMap: Record<DeviceCommandStatus, string> = {
    pending: 'Pending',
    acknowledged: 'Acknowledged',
    failed: 'Failed',
  };

  return (
    <span
      className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-wide ${styleMap[status]}`}
    >
      {labelMap[status]}
    </span>
  );
}

type JsonPreviewProps = {
  data: Record<string, unknown> | null | undefined;
  placeholder: string;
};

function JsonPreview({ data, placeholder }: JsonPreviewProps) {
  const hasContent = data && Object.keys(data).length > 0;
  return (
    <pre className="mt-2 max-h-48 overflow-y-auto rounded bg-slate-900/90 p-3 text-xs text-emerald-100">
      {hasContent ? JSON.stringify(data, null, 2) : placeholder}
    </pre>
  );
}

type AnnotatedPayload = {
  fields: Array<{
    key: string;
    value: unknown;
    metadata: ParameterMetadata | null;
  }>;
  unknownKeys: string[];
  metadataMap: Map<string, ParameterMetadata>;
};

type PayloadDiff = {
  added: Array<{ key: string; value: unknown }>;
  removed: Array<{ key: string; value: unknown }>;
  changed: Array<{ key: string; from: unknown; to: unknown }>;
};

function RawViewTab({ records, enableDiffTooling }: RawViewTabProps) {
  const [selectedSuffix, setSelectedSuffix] = useState<string>('all');
  const [selectedRecordId, setSelectedRecordId] = useState<string | null>(null);
  const [comparisonRecordId, setComparisonRecordId] = useState<string | null>(null);

  const suffixes = useMemo(() => {
    const unique = new Set<string>();
    for (const record of records) {
      if (record.topicSuffix) {
        unique.add(record.topicSuffix);
      }
    }
    return Array.from(unique).sort();
  }, [records]);

  useEffect(() => {
    if (selectedSuffix === 'all') {
      return;
    }

    if (!suffixes.includes(selectedSuffix)) {
      scheduleStateUpdate(() => {
        setSelectedSuffix('all');
      });
    }
  }, [selectedSuffix, suffixes]);

  const filteredRecords = useMemo(() => {
    if (selectedSuffix === 'all') {
      return records;
    }

    return records.filter((record) => record.topicSuffix === selectedSuffix);
  }, [records, selectedSuffix]);

  useEffect(() => {
    if (filteredRecords.length === 0) {
      scheduleStateUpdate(() => {
        setSelectedRecordId(null);
        if (comparisonRecordId !== null) {
          setComparisonRecordId(null);
        }
      });
      return;
    }

    const selectedExists = selectedRecordId
      ? filteredRecords.some((record) => record.telemetryId === selectedRecordId)
      : false;

    if (!selectedExists) {
      const [primary, secondary] = filteredRecords;
      scheduleStateUpdate(() => {
        setSelectedRecordId(primary.telemetryId);
        if (enableDiffTooling) {
          setComparisonRecordId(secondary ? secondary.telemetryId : null);
        }
      });
      return;
    }

    if (enableDiffTooling) {
      const comparisonExists = comparisonRecordId
        ? filteredRecords.some((record) => record.telemetryId === comparisonRecordId)
        : false;
      if (!comparisonExists) {
        const fallback = filteredRecords.find((record) => record.telemetryId !== selectedRecordId);
        scheduleStateUpdate(() => {
          setComparisonRecordId(fallback ? fallback.telemetryId : null);
        });
      }
    } else if (comparisonRecordId !== null) {
      scheduleStateUpdate(() => {
        setComparisonRecordId(null);
      });
    }
  }, [filteredRecords, selectedRecordId, enableDiffTooling, comparisonRecordId]);

  const annotatedMap = useMemo(() => {
    const entries: Array<[string, AnnotatedPayload]> = [];

    for (const record of filteredRecords) {
      const payload = record.payload ?? {};
      const metadataMap = buildParameterMetadataMap(record.topicSuffix);
      const fields = Object.keys(payload)
        .sort((a, b) => a.localeCompare(b))
        .map((key) => {
          const normalizedKey = key.toUpperCase();
          const metadata = metadataMap.get(normalizedKey) ?? null;
          return {
            key,
            value: payload[key],
            metadata,
          };
        });

      const unknownKeys = fields
        .filter((field) => field.metadata === null)
        .map((field) => field.key);
      entries.push([record.telemetryId, { fields, unknownKeys, metadataMap }]);
    }

    return new Map(entries);
  }, [filteredRecords]);

  const selectedRecord = useMemo(() => {
    if (!selectedRecordId) {
      return null;
    }

    return filteredRecords.find((record) => record.telemetryId === selectedRecordId) ?? null;
  }, [filteredRecords, selectedRecordId]);

  useEffect(() => {
    if (!enableDiffTooling) {
      return;
    }
    if (!selectedRecordId) {
      return;
    }
    if (comparisonRecordId === selectedRecordId) {
      const fallback = filteredRecords.find((record) => record.telemetryId !== selectedRecordId);
      scheduleStateUpdate(() => {
        setComparisonRecordId(fallback ? fallback.telemetryId : null);
      });
    }
  }, [enableDiffTooling, comparisonRecordId, selectedRecordId, filteredRecords]);

  const selectedPayload = selectedRecord
    ? (annotatedMap.get(selectedRecord.telemetryId) ?? null)
    : null;
  const unknownKeys = selectedPayload?.unknownKeys ?? [];
  const comparisonRecord =
    enableDiffTooling && comparisonRecordId
      ? (filteredRecords.find((record) => record.telemetryId === comparisonRecordId) ?? null)
      : null;

  const comparisonOptions = useMemo(() => {
    if (!enableDiffTooling) {
      return [];
    }

    return filteredRecords
      .filter((record) => record.telemetryId !== selectedRecordId)
      .slice(0, 100);
  }, [enableDiffTooling, filteredRecords, selectedRecordId]);

  const payloadDiff = useMemo(() => {
    if (!enableDiffTooling || !selectedRecord || !comparisonRecord) {
      return null;
    }

    const baseline = (comparisonRecord.payload ?? {}) as Record<string, unknown>;
    const next = (selectedRecord.payload ?? {}) as Record<string, unknown>;
    return computePayloadDiff(baseline, next);
  }, [enableDiffTooling, selectedRecord, comparisonRecord]);

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-600">
        Streaming buffer contains {records.length} packets. Oldest entries roll off automatically at{' '}
        {LIVE_BUFFER_LIMIT} records to avoid runaway growth.
      </p>

      <div className="flex flex-wrap items-center gap-2">
        <FilterChip
          label="All topics"
          isActive={selectedSuffix === 'all'}
          onClick={() => setSelectedSuffix('all')}
        />
        {suffixes.map((suffix) => (
          <FilterChip
            key={suffix}
            label={suffix}
            isActive={selectedSuffix === suffix}
            onClick={() => setSelectedSuffix(suffix)}
          />
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
          <header className="border-b border-slate-200 bg-slate-50 px-4 py-3">
            <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              Recent packets
            </h4>
            <p className="text-xs text-slate-500">Newest first · click to inspect details</p>
          </header>
          <div className="max-h-[420px] overflow-y-auto">
            {filteredRecords.length === 0 ? (
              <p className="px-4 py-3 text-sm text-slate-500">No packets for this filter.</p>
            ) : (
              <ul className="divide-y divide-slate-200">
                {filteredRecords.slice(0, 80).map((record) => {
                  const payloadSummary = annotatedMap.get(record.telemetryId);
                  const unknownCount = payloadSummary?.unknownKeys.length ?? 0;
                  const isActive = selectedRecordId === record.telemetryId;
                  return (
                    <li key={record.telemetryId}>
                      <button
                        type="button"
                        onClick={() => setSelectedRecordId(record.telemetryId)}
                        className={`flex w-full flex-col items-start gap-2 px-4 py-3 text-left transition-colors ${
                          isActive ? 'bg-emerald-50' : 'hover:bg-slate-50'
                        }`}
                      >
                        <div className="flex w-full flex-col gap-1 text-xs text-slate-600 sm:flex-row sm:items-center sm:justify-between">
                          <span className="font-semibold text-slate-700">
                            {formatDateTimeShort(record.receivedAt)}
                          </span>
                          <span className="font-mono text-[11px] text-slate-500">
                            {record.topic}
                          </span>
                        </div>
                        <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-wide">
                          <span className="rounded-full bg-slate-200 px-2 py-0.5 text-slate-600">
                            {record.topicSuffix || 'unknown'}
                          </span>
                          {record.metadata.msgid && (
                            <span className="rounded-full bg-slate-100 px-2 py-0.5 font-mono text-slate-500">
                              msgid {record.metadata.msgid}
                            </span>
                          )}
                          {unknownCount > 0 && (
                            <span className="rounded-full bg-amber-100 px-2 py-0.5 text-amber-700">
                              {unknownCount} unknown
                            </span>
                          )}
                        </div>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </section>

        <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
          <header className="flex flex-col gap-2 border-b border-slate-200 bg-slate-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                Packet detail
              </h4>
              {selectedRecord ? (
                <p className="text-xs text-slate-500">
                  Received {formatDateTimeShort(selectedRecord.receivedAt)} · Transport{' '}
                  {selectedRecord.metadata.transport}
                </p>
              ) : (
                <p className="text-xs text-slate-500">Select a packet to inspect payload values.</p>
              )}
            </div>
            {selectedRecord && (
              <span className="font-mono text-[11px] text-slate-500">
                Telemetry ID {selectedRecord.telemetryId}
              </span>
            )}
          </header>
          <div className="max-h-[500px] space-y-4 overflow-y-auto p-4">
            {!selectedRecord && <p className="text-sm text-slate-500">No packet selected.</p>}

            {selectedRecord && selectedPayload && (
              <>
                {unknownKeys.length > 0 && (
                  <div className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                    {unknownKeys.length} field{unknownKeys.length === 1 ? '' : 's'} not documented
                    in <span className="font-semibold"> JSON_PARAMETERS.md</span>:{' '}
                    {unknownKeys.join(', ')}. Review and update the spec if these fields are valid
                    per device protocol.
                  </div>
                )}

                {enableDiffTooling && (
                  <section className="rounded border border-slate-200 bg-slate-100 p-3 text-xs text-slate-600">
                    <header className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="font-semibold uppercase tracking-wide text-slate-500">
                          Payload diff helper
                        </p>
                        <p className="text-[11px] text-slate-500">
                          Compare this packet against another message in the buffer.
                        </p>
                      </div>
                      <div className="flex items-center gap-2">
                        <label
                          className="text-[11px] uppercase tracking-wide text-slate-500"
                          htmlFor="payloadDiffComparison"
                        >
                          Baseline packet
                        </label>
                        <select
                          id="payloadDiffComparison"
                          value={comparisonRecordId ?? ''}
                          onChange={(event) => setComparisonRecordId(event.target.value || null)}
                          className="rounded border border-slate-300 px-2 py-1 text-xs text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                        >
                          <option value="">Latest previous</option>
                          {comparisonOptions.map((option) => (
                            <option key={option.telemetryId} value={option.telemetryId}>
                              {formatDateTimeShort(option.receivedAt)} ·{' '}
                              {option.topicSuffix ?? 'unknown'}
                            </option>
                          ))}
                        </select>
                      </div>
                    </header>
                    {comparisonOptions.length === 0 && (
                      <p className="mt-2 text-[11px] text-slate-500">
                        Waiting for additional packets to compare against.
                      </p>
                    )}
                    {payloadDiff && hasDiffEntries(payloadDiff) && (
                      <div className="mt-3 space-y-2">
                        {payloadDiff.added.length > 0 && (
                          <DiffList
                            title="Added fields"
                            tone="positive"
                            items={payloadDiff.added.map((entry) => ({
                              key: entry.key,
                              label: entry.key,
                              detail: formatDiffValue(entry.value),
                            }))}
                          />
                        )}
                        {payloadDiff.changed.length > 0 && (
                          <DiffList
                            title="Changed"
                            tone="neutral"
                            items={payloadDiff.changed.map((entry) => ({
                              key: entry.key,
                              label: entry.key,
                              detail: `${formatDiffValue(entry.from)} → ${formatDiffValue(entry.to)}`,
                            }))}
                          />
                        )}
                        {payloadDiff.removed.length > 0 && (
                          <DiffList
                            title="Removed"
                            tone="negative"
                            items={payloadDiff.removed.map((entry) => ({
                              key: entry.key,
                              label: entry.key,
                              detail: formatDiffValue(entry.value),
                            }))}
                          />
                        )}
                      </div>
                    )}
                    {payloadDiff && !hasDiffEntries(payloadDiff) && (
                      <p className="mt-2 text-[11px] text-slate-500">
                        No field-level differences detected versus the selected baseline message.
                      </p>
                    )}
                  </section>
                )}

                <div className="overflow-hidden rounded-lg border border-slate-200">
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-100 text-xs uppercase tracking-wide text-slate-500">
                      <tr>
                        <th scope="col" className="px-3 py-2 text-left font-semibold">
                          Parameter
                        </th>
                        <th scope="col" className="px-3 py-2 text-left font-semibold">
                          Value
                        </th>
                        <th scope="col" className="px-3 py-2 text-left font-semibold">
                          Description
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200 bg-white">
                      {selectedPayload.fields.map((field) => {
                        const description = field.metadata?.description ?? 'Unknown parameter';
                        const unit = field.metadata?.unit ? ` (${field.metadata.unit})` : '';
                        const isUnknown = field.metadata === null;
                        return (
                          <tr key={field.key} className={isUnknown ? 'bg-amber-50' : undefined}>
                            <td
                              className="px-3 py-2 text-sm font-medium text-slate-700"
                              title={description}
                            >
                              <span className="font-mono text-[12px]">{field.key}</span>
                            </td>
                            <td className="px-3 py-2 text-sm text-slate-800">
                              {formatTelemetryValue(field.value)}
                              {field.metadata?.unit ? (
                                <span className="text-slate-500"> {field.metadata.unit}</span>
                              ) : null}
                            </td>
                            <td className="px-3 py-2 text-xs text-slate-500">
                              {description}
                              {unit}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>

                <details className="rounded border border-slate-200 bg-slate-900/90 text-emerald-100">
                  <summary className="cursor-pointer px-4 py-2 text-xs font-semibold uppercase tracking-wide text-white/80">
                    Raw JSON payload
                  </summary>
                  <pre className="max-h-[320px] overflow-y-auto px-4 py-3 text-xs">
                    {JSON.stringify(selectedRecord.payload ?? {}, null, 2)}
                  </pre>
                </details>
              </>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

type FilterChipProps = {
  label: string;
  isActive: boolean;
  onClick: () => void;
};

function FilterChip({ label, isActive, onClick }: FilterChipProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-wide transition-colors ${
        isActive ? 'bg-emerald-600 text-white' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
      }`}
    >
      {label}
    </button>
  );
}

type DiffListProps = {
  title: string;
  tone: 'positive' | 'neutral' | 'negative';
  items: Array<{ key: string; label: string; detail: string }>;
};

function DiffList({ title, tone, items }: DiffListProps) {
  const toneStyles: Record<DiffListProps['tone'], string> = {
    positive: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    neutral: 'border-slate-200 bg-white text-slate-600',
    negative: 'border-rose-200 bg-rose-50 text-rose-700',
  };

  return (
    <div className={`rounded border px-3 py-2 text-xs ${toneStyles[tone]}`}>
      <p className="text-[11px] font-semibold uppercase tracking-wide">{title}</p>
      <ul className="mt-2 space-y-1">
        {items.map((item) => (
          <li key={item.key} className="flex flex-col gap-0.5">
            <span className="font-mono text-[11px] font-semibold text-slate-700">{item.label}</span>
            <span className="text-[11px] text-slate-600">{item.detail}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function buildParameterMetadataMap(
  topicSuffix: string | undefined,
): Map<string, ParameterMetadata> {
  const entries = new Map<string, ParameterMetadata>();

  const collect = (source: Record<string, ParameterMetadata> | undefined) => {
    if (!source) {
      return;
    }
    for (const [key, metadata] of Object.entries(source)) {
      entries.set(key.toUpperCase(), metadata);
    }
  };

  collect(COMMON_PARAMETER_METADATA);
  collect(PARAMETER_METADATA_BY_SUFFIX[topicSuffix as TopicSuffix]);

  return entries;
}

function hasDiffEntries(diff: PayloadDiff): boolean {
  return diff.added.length > 0 || diff.removed.length > 0 || diff.changed.length > 0;
}

function formatDiffValue(value: unknown): string {
  if (value === null || value === undefined) {
    return String(value);
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  if (typeof value === 'string') {
    return value.length > 48 ? `${value.slice(0, 45)}…` : value;
  }
  try {
    const json = JSON.stringify(value);
    return json.length > 48 ? `${json.slice(0, 45)}…` : json;
  } catch {
    return '[unserializable]';
  }
}

function computePayloadDiff(
  baseline: Record<string, unknown>,
  next: Record<string, unknown>,
): PayloadDiff {
  const diff: PayloadDiff = {
    added: [],
    removed: [],
    changed: [],
  };

  const baselineKeys = new Set(Object.keys(baseline));
  const nextKeys = new Set(Object.keys(next));

  for (const key of nextKeys) {
    if (!baselineKeys.has(key)) {
      diff.added.push({ key, value: next[key] });
    }
  }

  for (const key of baselineKeys) {
    if (!nextKeys.has(key)) {
      diff.removed.push({ key, value: baseline[key] });
      continue;
    }

    const before = baseline[key];
    const after = next[key];
    if (!valuesEqual(before, after)) {
      diff.changed.push({ key, from: before, to: after });
    }
  }

  return diff;
}

function valuesEqual(a: unknown, b: unknown): boolean {
  if (a === b) {
    return true;
  }
  if (typeof a !== typeof b) {
    return false;
  }
  if (typeof a === 'object' && a !== null && b !== null) {
    try {
      return JSON.stringify(a) === JSON.stringify(b);
    } catch {
      return false;
    }
  }
  return false;
}

function formatTelemetryValue(value: unknown): string {
  if (value === null) {
    return 'null';
  }
  if (value === undefined) {
    return '—';
  }
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) {
      return String(value);
    }
    return value.toLocaleString();
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  if (typeof value === 'string') {
    return value.length > 80 ? `${value.slice(0, 77)}…` : value;
  }
  if (Array.isArray(value) || typeof value === 'object') {
    const json = JSON.stringify(value);
    return json.length > 80 ? `${json.slice(0, 77)}…` : json;
  }
  return String(value);
}

type ThresholdDrawerProps = {
  open: boolean;
  onClose: () => void;
  gaugeConfigs: DashboardGaugeConfig[];
  thresholds: ThresholdView | null;
  onSave: (payload: TelemetryThresholdUpsertPayload) => Promise<void>;
  onDelete: (payload?: TelemetryThresholdDeletePayload) => Promise<void>;
  isLoading: boolean;
  isSaving: boolean;
  isDeleting: boolean;
  loadError: string | null;
  mutationError: string | null;
  canEdit: boolean;
};

type ThresholdDraft = {
  min?: string;
  max?: string;
  warnLow?: string;
  warnHigh?: string;
  alertLow?: string;
  alertHigh?: string;
  target?: string;
};

function ThresholdDrawer({
  open,
  onClose,
  gaugeConfigs,
  thresholds,
  onSave,
  onDelete,
  isLoading,
  isSaving,
  isDeleting,
  loadError,
  mutationError,
  canEdit,
}: ThresholdDrawerProps) {
  const headingId = useId();
  const initialDraft = useMemo(() => {
    if (!thresholds) {
      return {};
    }

    const nextDraft: Record<string, ThresholdDraft> = {};
    thresholds.overrideEntries.forEach((entry, parameter) => {
      nextDraft[parameter] = {
        min: entry.min !== undefined && entry.min !== null ? String(entry.min) : undefined,
        max: entry.max !== undefined && entry.max !== null ? String(entry.max) : undefined,
        warnLow:
          entry.warnLow !== undefined && entry.warnLow !== null ? String(entry.warnLow) : undefined,
        warnHigh:
          entry.warnHigh !== undefined && entry.warnHigh !== null
            ? String(entry.warnHigh)
            : undefined,
        alertLow:
          entry.alertLow !== undefined && entry.alertLow !== null
            ? String(entry.alertLow)
            : undefined,
        alertHigh:
          entry.alertHigh !== undefined && entry.alertHigh !== null
            ? String(entry.alertHigh)
            : undefined,
        target:
          entry.target !== undefined && entry.target !== null ? String(entry.target) : undefined,
      };
    });

    return nextDraft;
  }, [thresholds]);

  const initialReason = useMemo(
    () => thresholds?.overrideMeta?.reason ?? '',
    [thresholds?.overrideMeta?.reason],
  );

  const [draft, setDraft] = useState<Record<string, ThresholdDraft>>(() => initialDraft);
  const [reasonInput, setReasonInput] = useState(() => initialReason);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const isReadOnly = !canEdit;
  const readOnlyReason = useMemo(() => {
    if (!isReadOnly) {
      return null;
    }
    if (loadError) {
      return 'Editing disabled because threshold data failed to load from the backend.';
    }
    return 'Editing disabled by feature flag. Update environment variables to enable overrides.';
  }, [isReadOnly, loadError]);

  if (!open) {
    return null;
  }

  const trimmedReason = reasonInput.trim();
  const hasOverride = (thresholds?.overrideEntries.size ?? 0) > 0;
  const canSubmit = canEdit && (Object.keys(draft).length > 0 || trimmedReason.length > 0);

  const handleFieldChange = (parameter: string, field: keyof ThresholdDraft, value: string) => {
    if (isReadOnly) {
      return;
    }
    setStatusMessage(null);
    setDraft((previous) => ({
      ...previous,
      [parameter]: {
        ...(previous[parameter] ?? {}),
        [field]: value,
      },
    }));
  };

  const handleClearParameter = (parameter: string) => {
    if (isReadOnly) {
      return;
    }
    setStatusMessage(null);
    setDraft((previous) => {
      if (!previous[parameter]) {
        return previous;
      }

      const next = { ...previous };
      delete next[parameter];
      return next;
    });
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setStatusMessage(null);

    if (isReadOnly) {
      onClose();
      return;
    }

    const payload = buildThresholdPayloadFromDraft(draft, gaugeConfigs, trimmedReason);
    if (payload.thresholds.length === 0 && !payload.reason) {
      setStatusMessage('Add at least one override value or provide a reason before saving.');
      return;
    }

    try {
      await onSave(payload);
      setStatusMessage('Threshold overrides saved.');
    } catch (error) {
      console.error('Failed to save threshold overrides', error);
    }
  };

  const handleRemoveOverride = async () => {
    if (isReadOnly) {
      return;
    }
    setStatusMessage(null);
    try {
      const payload: TelemetryThresholdDeletePayload = { scope: 'override' };
      if (trimmedReason) {
        payload.reason = trimmedReason;
      }
      await onDelete(payload);
      setStatusMessage('Threshold overrides removed.');
    } catch (error) {
      console.error('Failed to remove threshold overrides', error);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex">
      <button
        type="button"
        className="h-full flex-1 bg-slate-900/40"
        aria-label="Close thresholds"
        onClick={onClose}
      />
      <aside
        role="dialog"
        aria-modal="true"
        aria-labelledby={headingId}
        className="relative size-full max-w-xl bg-white shadow-2xl"
      >
        <form onSubmit={handleSubmit} className="flex h-full flex-col">
          <header className="flex items-start justify-between border-b border-slate-200 px-6 py-4">
            <div>
              <h2 id={headingId} className="text-base font-semibold text-slate-900">
                Device thresholds
              </h2>
              <p className="mt-1 text-xs text-slate-500">
                Adjust per-device ranges. Leave a field blank to fall back to the installation
                template threshold.
              </p>
              {thresholds?.installation && (
                <p className="mt-2 text-xs text-slate-400">
                  Installation template{' '}
                  {thresholds.installation.updatedAt
                    ? `updated ${formatDateTimeShort(thresholds.installation.updatedAt)}`
                    : 'last update unknown'}
                  {thresholds.installation.updatedBy
                    ? ` by ${
                        thresholds.installation.updatedBy.displayName ??
                        thresholds.installation.updatedBy.id
                      }`
                    : ''}
                </p>
              )}
              {thresholds?.overrideMeta && (
                <p className="mt-2 text-xs text-slate-400">
                  Last updated{' '}
                  {thresholds.overrideMeta.updatedAt
                    ? formatDateTimeShort(thresholds.overrideMeta.updatedAt)
                    : 'unknown'}
                  {thresholds.overrideMeta.updatedBy
                    ? ` by ${
                        thresholds.overrideMeta.updatedBy.displayName ??
                        thresholds.overrideMeta.updatedBy.id
                      }`
                    : ''}
                </p>
              )}
            </div>
            <button
              type="button"
              onClick={onClose}
              className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 transition-colors hover:bg-slate-100"
            >
              Close
            </button>
          </header>
          <div className="h-full flex-1 overflow-y-auto px-6 py-5">
            {isReadOnly && (
              <div className="mb-4 rounded border border-slate-200 bg-slate-100 px-3 py-2 text-xs text-slate-600">
                <p>
                  Threshold overrides are currently locked. Values shown below reflect the protocol
                  defaults and any previously saved overrides for this device.
                </p>
                {readOnlyReason && <p className="mt-2 text-slate-500">{readOnlyReason}</p>}
              </div>
            )}
            {isLoading && <p className="text-sm text-slate-500">Loading thresholds…</p>}
            {loadError && !isLoading && (
              <p className="rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                {loadError}
              </p>
            )}
            {!isLoading && !loadError && thresholds && (
              <div className="space-y-4">
                {gaugeConfigs.map((config) => {
                  const parameter = config.payloadKey;
                  const draftEntry = draft[parameter] ?? {};
                  const hasDraftForParameter = Boolean(
                    draft[parameter] &&
                      Object.values(draft[parameter] as ThresholdDraft).some(
                        (value) => value !== undefined,
                      ),
                  );
                  const defaults = thresholds.installation?.entries.get(parameter) ?? null;
                  const effective = thresholds.effective.get(parameter) ?? null;

                  const step = config.decimalPlaces && config.decimalPlaces > 0 ? 'any' : '1';

                  return (
                    <section
                      key={config.id}
                      className="rounded border border-slate-200 bg-slate-50 p-4 shadow-sm"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <h3 className="text-sm font-semibold text-slate-800">{config.label}</h3>
                          <p className="text-xs text-slate-500">Parameter {parameter}</p>
                        </div>
                        <button
                          type="button"
                          onClick={() => handleClearParameter(parameter)}
                          disabled={isReadOnly || !hasDraftForParameter}
                          className="text-xs font-semibold uppercase tracking-wide text-emerald-600 hover:underline disabled:cursor-not-allowed disabled:text-slate-400"
                        >
                          Reset override
                        </button>
                      </div>
                      <div className="mt-3 grid gap-3 sm:grid-cols-2">
                        <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Min
                          <input
                            type="number"
                            step={step}
                            value={draftEntry.min ?? ''}
                            placeholder={
                              defaults?.min !== undefined && defaults?.min !== null
                                ? String(defaults.min)
                                : ''
                            }
                            onChange={(event) =>
                              handleFieldChange(parameter, 'min', event.target.value)
                            }
                            disabled={isReadOnly}
                            className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                          />
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Max
                          <input
                            type="number"
                            step={step}
                            value={draftEntry.max ?? ''}
                            placeholder={
                              defaults?.max !== undefined && defaults?.max !== null
                                ? String(defaults.max)
                                : ''
                            }
                            onChange={(event) =>
                              handleFieldChange(parameter, 'max', event.target.value)
                            }
                            disabled={isReadOnly}
                            className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                          />
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Warn low
                          <input
                            type="number"
                            step={step}
                            value={draftEntry.warnLow ?? ''}
                            placeholder={
                              defaults?.warnLow !== undefined && defaults?.warnLow !== null
                                ? String(defaults.warnLow)
                                : ''
                            }
                            onChange={(event) =>
                              handleFieldChange(parameter, 'warnLow', event.target.value)
                            }
                            disabled={isReadOnly}
                            className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                          />
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Warn high
                          <input
                            type="number"
                            step={step}
                            value={draftEntry.warnHigh ?? ''}
                            placeholder={
                              defaults?.warnHigh !== undefined && defaults?.warnHigh !== null
                                ? String(defaults.warnHigh)
                                : ''
                            }
                            onChange={(event) =>
                              handleFieldChange(parameter, 'warnHigh', event.target.value)
                            }
                            disabled={isReadOnly}
                            className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                          />
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Alert low
                          <input
                            type="number"
                            step={step}
                            value={draftEntry.alertLow ?? ''}
                            placeholder={
                              defaults?.alertLow !== undefined && defaults?.alertLow !== null
                                ? String(defaults.alertLow)
                                : ''
                            }
                            onChange={(event) =>
                              handleFieldChange(parameter, 'alertLow', event.target.value)
                            }
                            disabled={isReadOnly}
                            className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                          />
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Alert high
                          <input
                            type="number"
                            step={step}
                            value={draftEntry.alertHigh ?? ''}
                            placeholder={
                              defaults?.alertHigh !== undefined && defaults?.alertHigh !== null
                                ? String(defaults.alertHigh)
                                : ''
                            }
                            onChange={(event) =>
                              handleFieldChange(parameter, 'alertHigh', event.target.value)
                            }
                            disabled={isReadOnly}
                            className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                          />
                        </label>
                        <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                          Target
                          <input
                            type="number"
                            step={step}
                            value={draftEntry.target ?? ''}
                            placeholder={
                              defaults?.target !== undefined && defaults?.target !== null
                                ? String(defaults.target)
                                : ''
                            }
                            onChange={(event) =>
                              handleFieldChange(parameter, 'target', event.target.value)
                            }
                            disabled={isReadOnly}
                            className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                          />
                        </label>
                      </div>
                      <div className="mt-3 flex flex-col gap-1 text-xs text-slate-500 sm:flex-row sm:items-center sm:justify-between">
                        <span>Effective: {formatThresholdSummary(effective)}</span>
                        <span>Installation: {formatThresholdSummary(defaults)}</span>
                      </div>
                    </section>
                  );
                })}
              </div>
            )}
            {!isLoading && !loadError && !thresholds && (
              <p className="text-sm text-slate-500">
                Threshold metadata is unavailable for this device.
              </p>
            )}
            <div className="mt-6">
              <label className="flex flex-col gap-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                Change reason (optional)
                <textarea
                  rows={3}
                  value={reasonInput}
                  onChange={(event) => {
                    setReasonInput(event.target.value);
                    setStatusMessage(null);
                  }}
                  disabled={isReadOnly}
                  className="rounded border border-slate-300 px-2 py-1 text-sm text-slate-700 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                  placeholder="Document why thresholds were updated"
                />
              </label>
            </div>
            {mutationError && (
              <p className="mt-4 rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                {mutationError}
              </p>
            )}
            {statusMessage && !mutationError && (
              <p className="mt-4 rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {statusMessage}
              </p>
            )}
          </div>
          <footer className="flex flex-col gap-3 border-t border-slate-200 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
            <button
              type="button"
              onClick={onClose}
              className="rounded border border-slate-300 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-slate-600 transition-colors hover:bg-slate-100"
            >
              {isReadOnly ? 'Close' : 'Cancel'}
            </button>
            {isReadOnly ? (
              <p className="text-xs text-slate-500">{readOnlyReason}</p>
            ) : (
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                {hasOverride && (
                  <button
                    type="button"
                    onClick={handleRemoveOverride}
                    disabled={isDeleting}
                    className="rounded border border-rose-200 px-3 py-2 text-xs font-semibold uppercase tracking-wide text-rose-600 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {isDeleting ? 'Removing…' : 'Remove override'}
                  </button>
                )}
                <button
                  type="submit"
                  disabled={!canSubmit || isSaving}
                  className="rounded bg-emerald-600 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-white transition-colors hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isSaving ? 'Saving…' : 'Save changes'}
                </button>
              </div>
            )}
          </footer>
        </form>
      </aside>
    </div>
  );
}

function collectNumericValues(records: TelemetryRecord[], key: string, limit = 200): number[] {
  const values: number[] = [];
  const slice = records.slice(0, limit);
  for (const record of slice) {
    const parsed = parseTelemetryNumeric(record.payload?.[key]);
    if (parsed !== null && parsed !== undefined && Number.isFinite(parsed)) {
      values.push(parsed);
    }
  }
  return values;
}

function computeAnalyticsCallouts(
  eventsBySuffix: TelemetryBySuffix,
  heartbeatAgeMinutes: number | null,
): AnalyticsCallout[] {
  const dataLikeRecords = eventsBySuffix.data;

  const peakPower = safeMax(collectNumericValues(dataLikeRecords, 'POPKW1'));
  const averageFlow = safeAverage(collectNumericValues(dataLikeRecords, 'PFLWRT1'));
  const runtimeValues = collectNumericValues(dataLikeRecords, 'PTOTHR1');
  const latestRuntime = runtimeValues[0];
  const earliestRuntime = runtimeValues[runtimeValues.length - 1];
  const runtimeDelta =
    runtimeValues.length >= 2 && latestRuntime !== undefined && earliestRuntime !== undefined
      ? Math.max(0, latestRuntime - earliestRuntime)
      : null;

  const voltageValues = collectNumericValues(dataLikeRecords, 'PDC1V1');
  const voltageRange =
    voltageValues.length >= 2
      ? Math.max(...voltageValues) - Math.min(...voltageValues)
      : voltageValues.length === 1
        ? 0
        : null;

  const callouts: AnalyticsCallout[] = [];

  if (peakPower !== null) {
    callouts.push({
      id: 'peak-power',
      label: 'Peak Pump Power',
      value: formatNumber(peakPower, 'kW', 2),
      description: 'Highest reading across the current telemetry buffer.',
      emphasis: peakPower > 8 ? 'negative' : undefined,
    });
  }

  if (averageFlow !== null) {
    callouts.push({
      id: 'average-flow-rate',
      label: 'Avg Flow Rate',
      value: `${averageFlow.toFixed(1)} L/min`,
      description: 'Mean of recent flow telemetry samples.',
    });
  }

  if (runtimeDelta !== null) {
    callouts.push({
      id: 'runtime-delta',
      label: 'Runtime Gain',
      value: `${runtimeDelta.toFixed(2)} h`,
      description: 'Increase in total run-hours during the observed window.',
      emphasis: runtimeDelta > 0 ? 'positive' : undefined,
    });
  }

  if (voltageRange !== null) {
    callouts.push({
      id: 'voltage-variance',
      label: 'DC Voltage Swing',
      value: `${voltageRange.toFixed(0)} V`,
      description: 'Spread between min/max array voltage samples.',
      emphasis: voltageRange > 300 ? 'negative' : undefined,
    });
  }

  if (heartbeatAgeMinutes !== null) {
    callouts.push({
      id: 'heartbeat-staleness',
      label: 'Heartbeat Staleness',
      value: `${heartbeatAgeMinutes.toFixed(1)} min`,
      description: 'Minutes since last heartbeat arrived from field device.',
      emphasis: heartbeatAgeMinutes > 5 ? 'negative' : undefined,
    });
  }

  return callouts;
}

function safeAverage(values: number[]): number | null {
  if (values.length === 0) {
    return null;
  }
  const total = values.reduce((sum, value) => sum + value, 0);
  return total / values.length;
}

function safeMax(values: number[]): number | null {
  if (values.length === 0) {
    return null;
  }
  return Math.max(...values);
}

export function buildThresholdPayloadFromDraft(
  draft: Record<string, ThresholdDraft>,
  gaugeConfigs: DashboardGaugeConfig[],
  reason: string,
): TelemetryThresholdUpsertPayload {
  const thresholds: TelemetryThresholdUpsertPayload['thresholds'] = [];
  const configByParameter = new Map(gaugeConfigs.map((config) => [config.payloadKey, config]));

  for (const [parameter, entry] of Object.entries(draft)) {
    const min = parseNumberInput(entry.min);
    const max = parseNumberInput(entry.max);
    const warnLow = parseNumberInput(entry.warnLow);
    const warnHigh = parseNumberInput(entry.warnHigh);
    const alertLow = parseNumberInput(entry.alertLow);
    const alertHigh = parseNumberInput(entry.alertHigh);
    const target = parseNumberInput(entry.target);

    if (
      min === undefined &&
      max === undefined &&
      warnLow === undefined &&
      warnHigh === undefined &&
      alertLow === undefined &&
      alertHigh === undefined &&
      target === undefined
    ) {
      continue;
    }

    const config = configByParameter.get(parameter);
    const payload: TelemetryThresholdUpsertPayload['thresholds'][number] = { parameter };

    if (min !== undefined) payload.min = min;
    if (max !== undefined) payload.max = max;
    if (warnLow !== undefined) payload.warnLow = warnLow;
    if (warnHigh !== undefined) payload.warnHigh = warnHigh;
    if (alertLow !== undefined) payload.alertLow = alertLow;
    if (alertHigh !== undefined) payload.alertHigh = alertHigh;
    if (target !== undefined) payload.target = target;

    if (config?.unit) {
      payload.unit = config.unit;
    }
    if (typeof config?.decimalPlaces === 'number') {
      payload.decimalPlaces = config.decimalPlaces;
    }

    thresholds.push(payload);
  }

  const trimmedReason = reason.trim();

  return {
    scope: 'override' as const,
    thresholds,
    ...(trimmedReason ? { reason: trimmedReason } : {}),
  };
}

function parseNumberInput(value?: string): number | null | undefined {
  if (value === undefined) {
    return undefined;
  }

  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }

  const parsed = Number(trimmed);
  if (Number.isNaN(parsed)) {
    return undefined;
  }

  return parsed;
}

export function formatThresholdSummary(
  entry?: TelemetryThresholdEntry | TelemetryThresholdConfig | null,
): string {
  if (!entry) {
    return '—';
  }

  const fractionDigits = typeof entry.decimalPlaces === 'number' ? entry.decimalPlaces : 1;
  const unit = entry.unit ?? undefined;

  const parts: string[] = [];

  if (entry.min !== undefined && entry.min !== null) {
    parts.push(`min ${formatNumber(entry.min, unit ?? undefined, fractionDigits)}`);
  }

  if (entry.max !== undefined && entry.max !== null) {
    parts.push(`max ${formatNumber(entry.max, unit ?? undefined, fractionDigits)}`);
  }

  if (entry.warnLow !== undefined && entry.warnLow !== null) {
    parts.push(`warn low ${formatNumber(entry.warnLow, unit ?? undefined, fractionDigits)}`);
  }

  if (entry.warnHigh !== undefined && entry.warnHigh !== null) {
    parts.push(`warn high ${formatNumber(entry.warnHigh, unit ?? undefined, fractionDigits)}`);
  }

  if (entry.alertLow !== undefined && entry.alertLow !== null) {
    parts.push(`alert low ${formatNumber(entry.alertLow, unit ?? undefined, fractionDigits)}`);
  }

  if (entry.alertHigh !== undefined && entry.alertHigh !== null) {
    parts.push(`alert high ${formatNumber(entry.alertHigh, unit ?? undefined, fractionDigits)}`);
  }

  if (entry.target !== undefined && entry.target !== null) {
    parts.push(`target ${formatNumber(entry.target, unit ?? undefined, fractionDigits)}`);
  }

  if (parts.length === 0) {
    return '—';
  }

  return parts.join(', ');
}

function recordToEntryMap(
  record: Record<string, TelemetryThresholdConfig> | null | undefined,
  source: TelemetryThresholdEntry['source'],
): Map<string, TelemetryThresholdEntry> {
  if (!record) {
    return new Map();
  }

  const entries: Array<[string, TelemetryThresholdEntry]> = [];

  for (const [parameter, config] of Object.entries(record)) {
    if (!parameter) {
      continue;
    }

    entries.push([
      parameter,
      {
        parameter,
        min: config.min ?? null,
        max: config.max ?? null,
        warnLow: config.warnLow ?? null,
        warnHigh: config.warnHigh ?? null,
        alertLow: config.alertLow ?? null,
        alertHigh: config.alertHigh ?? null,
        target: config.target ?? null,
        unit: config.unit ?? null,
        decimalPlaces: config.decimalPlaces ?? null,
        source,
      },
    ]);
  }

  return new Map(entries);
}
