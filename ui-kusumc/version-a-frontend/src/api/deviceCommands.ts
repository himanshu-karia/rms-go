import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type DeviceCommandStatus = 'pending' | 'acknowledged' | 'failed';

export type DeviceCommandHistoryEvent = {
  id: string;
  type: string;
  severity: 'info' | 'warn' | 'error';
  payload: Record<string, unknown>;
  createdAt: string;
};

export type DeviceCommandHistoryRecord = {
  msgid: string;
  command: {
    name: string;
    payload: Record<string, unknown>;
  };
  status: DeviceCommandStatus;
  requestedAt: string;
  acknowledgedAt: string | null;
  timeoutSeconds: number | null;
  expectedTimeoutAt: string | null;
  response: Record<string, unknown> | null;
  metadata: {
    issuedBy: string | null;
    publishTopic: string | null;
    protocolVersion: string | null;
    serverVendorId: string | null;
    simulatorSessionId: string | null;
    timedOutAt: string | null;
    timeoutReason: string | null;
  };
  events: DeviceCommandHistoryEvent[];
};

export type DeviceCommandHistoryResponse = {
  device: {
    uuid: string;
    imei: string;
  };
  commands: DeviceCommandHistoryRecord[];
  nextCursor: string | null;
};

export type FetchDeviceCommandHistoryParams = {
  deviceUuid: string;
  limit?: number;
  cursor?: string | null;
  statuses?: DeviceCommandStatus[];
};

export type IssueDeviceCommandPayload = {
  command: {
    name: string;
    payload?: Record<string, unknown>;
  };
  qos?: number;
  timeoutSeconds?: number;
  issuedBy?: string;
  simulatorSessionToken?: string;
};

export type IssueDeviceCommandResponse = {
  msgid: string;
  status: DeviceCommandStatus;
  topic: string;
  device: {
    uuid: string;
    imei: string;
  };
  simulatorSessionId: string | null;
};

export async function fetchDeviceCommandHistory(
  params: FetchDeviceCommandHistoryParams,
): Promise<DeviceCommandHistoryResponse> {
  const query = new URLSearchParams();

  if (typeof params.limit === 'number') {
    query.set('limit', params.limit.toString());
  }

  if (params.cursor) {
    query.set('cursor', params.cursor);
  }

  if (Array.isArray(params.statuses) && params.statuses.length > 0) {
    params.statuses.forEach((status) => {
      query.append('status', status);
    });
  }

  const queryString = query.toString();
  const url = `${API_BASE_URL}/devices/${encodeURIComponent(params.deviceUuid)}/commands/history${
    queryString ? `?${queryString}` : ''
  }`;

  const response = await apiFetch(url);
  const body = await readJsonBody<DeviceCommandHistoryResponse>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to load device command history';
    throw new Error(message);
  }

  return body as DeviceCommandHistoryResponse;
}

export async function issueDeviceCommand(
  deviceUuid: string,
  payload: IssueDeviceCommandPayload,
): Promise<IssueDeviceCommandResponse> {
  const response = await apiFetch(
    `${API_BASE_URL}/devices/${encodeURIComponent(deviceUuid)}/commands`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    },
  );

  const body = await readJsonBody<IssueDeviceCommandResponse>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to issue device command';
    throw new Error(message);
  }

  return body as IssueDeviceCommandResponse;
}
