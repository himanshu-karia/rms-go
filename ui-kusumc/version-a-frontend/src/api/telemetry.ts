import { API_BASE_URL } from './config';
import { apiFetch, camelizeKeysDeep, readJsonBody } from './http';

export type TelemetryRecord = {
  telemetryId: string;
  deviceUuid: string;
  imei: string;
  topic: string;
  topicSuffix: string;
  payload: Record<string, unknown>;
  receivedAt: string;
  ingestedAt: string;
  source?: 'live' | 'history';
  metadata: {
    qos: number | null;
    msgid: string | null;
    transport: string;
  };
};

export type TelemetryThresholdEntry = {
  parameter: string;
  min?: number | null;
  max?: number | null;
  warnLow?: number | null;
  warnHigh?: number | null;
  alertLow?: number | null;
  alertHigh?: number | null;
  target?: number | null;
  unit?: string | null;
  decimalPlaces?: number | null;
  source: 'installation' | 'override' | 'effective';
};

export type TelemetryThresholdResponse = {
  deviceUuid: string;
  thresholds: {
    effective: TelemetryThresholdEntry[];
    installation: {
      entries: TelemetryThresholdEntry[];
      templateId: string | null;
      updatedAt: string | null;
      updatedBy: { id: string; displayName: string | null } | null;
      metadata: Record<string, unknown> | null;
    } | null;
    override: {
      entries: TelemetryThresholdEntry[];
      reason: string | null;
      updatedAt: string | null;
      updatedBy: { id: string; displayName: string | null } | null;
    } | null;
  };
};

export type TelemetryThresholdUpsertPayload = {
  scope?: 'installation' | 'override';
  templateId?: string;
  thresholds: Array<{
    parameter: string;
    min?: number | null;
    max?: number | null;
    warnLow?: number | null;
    warnHigh?: number | null;
    alertLow?: number | null;
    alertHigh?: number | null;
    target?: number | null;
    unit?: string | null;
    decimalPlaces?: number | null;
  }>;
  reason?: string;
  metadata?: Record<string, unknown>;
};

export type TelemetryThresholdDeletePayload = {
  scope?: 'installation' | 'override';
  reason?: string;
};

export type TelemetryHistoryResponse = {
  deviceUuid: string;
  imei?: string;
  count: number;
  records: TelemetryRecord[];
};

export type TelemetryLiveTicketResponse = {
  token: string;
  expiresAt: string;
};

export async function fetchTelemetryHistory(params: {
  deviceUuid?: string;
  imei?: string;
  topicSuffix?: string;
  from?: string;
  to?: string;
  limit?: number;
}): Promise<TelemetryHistoryResponse> {
  const query = new URLSearchParams();
  if (params.topicSuffix) query.set('topicSuffix', params.topicSuffix);
  if (params.from) query.set('from', params.from);
  if (params.to) query.set('to', params.to);
  if (typeof params.limit === 'number') query.set('limit', params.limit.toString());

  const queryString = query.toString();

  let basePath: string | null = null;
  if (params.deviceUuid) {
    basePath = `${API_BASE_URL}/telemetry/devices/${encodeURIComponent(params.deviceUuid)}/history`;
  } else if (params.imei) {
    basePath = `${API_BASE_URL}/telemetry/devices/imei/${encodeURIComponent(params.imei)}/history`;
  }

  if (!basePath) {
    throw new Error('Provide deviceUuid or IMEI to fetch telemetry history');
  }

  const url = queryString ? `${basePath}?${queryString}` : basePath;

  const response = await apiFetch(url);

  const body = await readJsonBody<TelemetryHistoryResponse>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to load telemetry history';
    throw new Error(message);
  }

  return body as TelemetryHistoryResponse;
}

export async function requestTelemetryLiveTicket(
  deviceUuid: string,
): Promise<TelemetryLiveTicketResponse> {
  const response = await apiFetch(
    `${API_BASE_URL}/telemetry/devices/${encodeURIComponent(deviceUuid)}/live-token`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({}),
    },
  );

  const body = await readJsonBody<TelemetryLiveTicketResponse>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to request live telemetry token';
    throw new Error(message);
  }

  if (typeof body.token !== 'string') {
    throw new Error('Live telemetry token missing from response');
  }

  return body as TelemetryLiveTicketResponse;
}

export async function fetchTelemetryThresholds(
  deviceUuid: string,
): Promise<TelemetryThresholdResponse> {
  const response = await apiFetch(`${API_BASE_URL}/telemetry/thresholds/${deviceUuid}`);
  const body = await readJsonBody<TelemetryThresholdResponse>(response);
  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to load telemetry thresholds';
    throw new Error(message);
  }

  return body as TelemetryThresholdResponse;
}

export async function upsertTelemetryThresholds(
  deviceUuid: string,
  payload: TelemetryThresholdUpsertPayload,
): Promise<TelemetryThresholdResponse> {
  const response = await apiFetch(`${API_BASE_URL}/telemetry/thresholds/${deviceUuid}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<TelemetryThresholdResponse>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to save telemetry thresholds';
    throw new Error(message);
  }

  return body as TelemetryThresholdResponse;
}

export async function deleteTelemetryThresholds(
  deviceUuid: string,
  payload?: TelemetryThresholdDeletePayload,
): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/telemetry/thresholds/${deviceUuid}`, {
    method: 'DELETE',
    headers: payload ? { 'Content-Type': 'application/json' } : undefined,
    body: payload ? JSON.stringify(payload) : undefined,
  });

  if (!response.ok) {
    const body = await readJsonBody<{ message?: string }>(response);
    const message = body?.message ?? 'Unable to delete telemetry thresholds';
    throw new Error(message);
  }
}

export type TelemetryEventListener = (event: TelemetryRecord) => void;

export type TelemetryStreamOptions = {
  topicSuffix?: string;
  historyHours?: number;
  historyLimit?: number;
  includeHistory?: boolean;
  since?: string;
  onError?: (error: Error) => void;
  onTicketIssued?: (ticket: TelemetryLiveTicketResponse) => void;
  /** When true (default), automatically attempts to reconnect with a fresh ticket. */
  autoReconnect?: boolean;
  /** Maximum number of consecutive reconnect attempts before surfacing an error (default 5). */
  maxReconnectAttempts?: number;
  /** Initial delay (in ms) between reconnect attempts (default 2000ms). */
  reconnectInitialDelayMs?: number;
  /** Upper bound (in ms) for the reconnect backoff delay (default 15000ms). */
  reconnectMaxDelayMs?: number;
};

export function subscribeToTelemetryStream(
  deviceUuid: string,
  listener: TelemetryEventListener,
  options: TelemetryStreamOptions = {},
) {
  let source: EventSource | null = null;
  let closed = false;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let reconnectAttempts = 0;

  const autoReconnect = options.autoReconnect !== false;
  const maxReconnectAttempts = options.maxReconnectAttempts ?? 5;
  const reconnectInitialDelayMs = options.reconnectInitialDelayMs ?? 2_000;
  const reconnectMaxDelayMs = options.reconnectMaxDelayMs ?? 15_000;

  function handleTelemetryEvent(event: MessageEvent<string>) {
    try {
      const parsed = JSON.parse(event.data) as unknown;
      const data = camelizeKeysDeep<TelemetryRecord>(parsed);
      listener(data);
    } catch (error) {
      console.error('Failed to parse telemetry event', error);
    }
  }

  function handleHeartbeatEvent() {
    // Keepalive-only; no-op.
  }

  function handleOpen() {
    reconnectAttempts = 0;
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function cleanupSource() {
    if (source) {
      source.removeEventListener('telemetry', handleTelemetryEvent);
      source.removeEventListener('heartbeat', handleHeartbeatEvent);
      source.removeEventListener('error', handleStreamError);
      source.removeEventListener('open', handleOpen);
      source.close();
      source = null;
    }
  }

  function notifyFatalError(error?: unknown) {
    if (typeof options.onError === 'function') {
      const normalized =
        error instanceof Error
          ? error
          : new Error('Live telemetry stream disconnected. Try again shortly.');
      options.onError(normalized);
    }
  }

  function scheduleReconnect(error?: unknown) {
    if (!autoReconnect || closed) {
      notifyFatalError(error);
      cleanup();
      return;
    }

    if (reconnectAttempts >= maxReconnectAttempts) {
      notifyFatalError(error);
      cleanup();
      return;
    }

    const backoffMs = Math.min(
      reconnectInitialDelayMs * Math.pow(2, reconnectAttempts),
      reconnectMaxDelayMs,
    );

    reconnectAttempts += 1;
    clearReconnectTimer();
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      startStream();
    }, backoffMs);
  }

  function handleStreamError(event: Event) {
    console.warn('Telemetry stream encountered an error', event);
    cleanupSource();
    scheduleReconnect(event instanceof ErrorEvent ? event.error : undefined);
  }

  function buildStreamUrl(liveTicket: string) {
    const url = new URL(`${API_BASE_URL}/telemetry/devices/${encodeURIComponent(deviceUuid)}/live`);
    url.searchParams.set('token', liveTicket);
    return url;
  }

  function startStream() {
    requestTelemetryLiveTicket(deviceUuid)
      .then(({ token, expiresAt }) => {
        if (closed) {
          return;
        }

        if (typeof options.onTicketIssued === 'function') {
          options.onTicketIssued({ token, expiresAt });
        }

        const url = buildStreamUrl(token);
        cleanupSource();

        source = new EventSource(url.toString());
        source.addEventListener('telemetry', handleTelemetryEvent);
        source.addEventListener('heartbeat', handleHeartbeatEvent);
        source.addEventListener('error', handleStreamError);
        source.addEventListener('open', handleOpen);
      })
      .catch((error) => {
        scheduleReconnect(error instanceof Error ? error : new Error(String(error)));
      });
  }

  startStream();

  function cleanup() {
    closed = true;
    clearReconnectTimer();
    cleanupSource();
  }

  return cleanup;
}
