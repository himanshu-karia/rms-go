import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';
import type { FaultDefinition, RealtimeParameterDefinition, Rs485Config } from './vfd';

export type DeviceConfigurationPayload = {
  imei: string;
  modelId: string;
  manufacturer: string;
  model: string;
  version: string;
  rs485: Rs485Config;
  realtimeParameters: RealtimeParameterDefinition[];
  faultMap: FaultDefinition[];
  metadata: Record<string, unknown> | null;
};

export type QueueDeviceConfigurationPayload = {
  vfdModelId: string;
  overrides?: {
    rs485?: Partial<Rs485Config>;
    realtimeParameters?: RealtimeParameterDefinition[];
    faultMap?: FaultDefinition[];
    metadata?: Record<string, unknown>;
  };
  transport?: 'mqtt' | 'https';
  issuedBy?: string;
  qos?: number;
};

export type QueueDeviceConfigurationResponse = {
  status: 'pending';
  transport: 'mqtt' | 'https';
  msgid: string | null;
  configuration: DeviceConfigurationPayload;
};

export async function queueDeviceConfiguration(
  deviceUuid: string,
  payload: QueueDeviceConfigurationPayload,
): Promise<QueueDeviceConfigurationResponse> {
  const response = await apiFetch(
    `${API_BASE_URL}/devices/${encodeURIComponent(deviceUuid)}/configuration`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    },
  );

  const body = await readJsonBody<unknown>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to queue configuration';
    throw new Error(message);
  }

  return body as QueueDeviceConfigurationResponse;
}

export type DeviceConfigurationRecord = {
  id: string;
  status: 'pending' | 'acknowledged' | 'failed';
  transport: 'mqtt' | 'https';
  msgid: string | null;
  requestedAt: string;
  acknowledgedAt: string | null;
  configuration: DeviceConfigurationPayload;
  acknowledgementPayload: Record<string, unknown> | null;
};

export async function fetchPendingDeviceConfiguration(
  deviceUuid: string,
): Promise<DeviceConfigurationRecord | null> {
  const response = await apiFetch(
    `${API_BASE_URL}/devices/${encodeURIComponent(deviceUuid)}/configuration/pending`,
  );

  if (response.status === 204) {
    return null;
  }

  const body = await readJsonBody<unknown>(response);

  if (!response.ok) {
    const message = body?.message ?? 'Unable to load pending configuration';
    throw new Error(message);
  }

  return (body ?? null) as DeviceConfigurationRecord | null;
}

export async function acknowledgeDeviceConfiguration(
  deviceUuid: string,
  payload: {
    status: 'acknowledged' | 'failed';
    msgid?: string;
    receivedAt?: string;
    acknowledgementPayload?: Record<string, unknown>;
  },
): Promise<DeviceConfigurationRecord> {
  const response = await apiFetch(
    `${API_BASE_URL}/devices/${encodeURIComponent(deviceUuid)}/configuration/ack`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        status: payload.status,
        msgid: payload.msgid,
        receivedAt: payload.receivedAt,
        payload: payload.acknowledgementPayload,
      }),
    },
  );

  const body = await readJsonBody<unknown>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to acknowledge configuration';
    throw new Error(message);
  }

  return body as DeviceConfigurationRecord;
}

export type ImportDeviceConfigurationsResult = {
  processed: number;
  queued: number;
  errors: Array<{ row: number; message: string }>;
};

export async function importDeviceConfigurationsCsv(payload: {
  csv: string;
  transport?: 'mqtt' | 'https';
  issuedBy?: string;
}): Promise<ImportDeviceConfigurationsResult> {
  const response = await apiFetch(`${API_BASE_URL}/devices/configuration/import`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<unknown>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to import device configurations';
    throw new Error(message);
  }

  return body as ImportDeviceConfigurationsResult;
}
