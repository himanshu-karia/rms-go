import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type CredentialEndpoint = {
  protocol: 'mqtt' | 'mqtts';
  host: string;
  port: number;
  url?: string;
};

export type GovernmentCredentialPayload = {
  clientId: string;
  username: string;
  password: string;
  endpoints: CredentialEndpoint[];
  topics?: {
    publish?: string[];
    subscribe?: string[];
  };
  metadata?: Record<string, unknown>;
  issuedBy?: string;
};

export type CsvImportError = {
  row: number;
  message: string;
  payload?: Record<string, string> | null;
};

export type ImportJobType = 'device' | 'government_credentials' | 'installation_beneficiaries';
export type ImportJobStatus = 'pending' | 'completed';

export type ImportJob = {
  id: string;
  type: ImportJobType;
  status: ImportJobStatus;
  processed: number;
  succeeded: number;
  failed: number;
  errorCount: number;
  errors: CsvImportError[];
  issuedBy: string | null;
  stateId: string | null;
  stateAuthorityId: string | null;
  projectId: string | null;
  createdAt: string;
  completedAt: string | null;
  metadata: Record<string, unknown> | null;
};

export type RegisterDevicePayload = {
  imei: string;
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  serverVendorId: string;
  protocolVersionId: string;
  solarPumpVendorId: string;
  issuedBy?: string;
  governmentCredentials?: GovernmentCredentialPayload;
};

export type DeviceCredentialEndpoint = {
  protocol: 'mqtt' | 'mqtts';
  host: string;
  port: number;
  url: string;
};

export type DeviceCredentialBundle = {
  clientId: string;
  username: string;
  password: string;
  endpoints: DeviceCredentialEndpoint[];
  topics: {
    publish: string[];
    subscribe: string[];
  };
  mqttAccess: {
    applied: boolean | null;
  };
};

export type MqttProvisioningStatus = 'pending' | 'in_progress' | 'applied' | 'failed';

export type MqttProvisioningErrorInfo = {
  message: string;
  status?: number;
  endpoint?: string;
};

export type MqttProvisioningInfo = {
  status: MqttProvisioningStatus;
  jobId: string;
  attemptCount: number;
  maxAttempts: number;
  baseRetryDelayMs: number;
  lastAttemptAt: string | null;
  nextAttemptAt: string;
  lastError: MqttProvisioningErrorInfo | null;
};

export type GovernmentCredentialBundle = {
  clientId: string;
  username: string;
  password: string;
  endpoints: DeviceCredentialEndpoint[];
  topics: {
    publish: string[];
    subscribe: string[];
  };
  metadata?: Record<string, unknown>;
};

export type CredentialLifecycleState = 'pending' | 'active' | 'revoked' | 'expired';

export type CredentialLifecycleHistoryEntry = {
  state: CredentialLifecycleState;
  occurredAt: string;
  reason: string | null;
  actorId: string | null;
  source: 'system' | 'user';
};

export type LifecycleTransitionRecord = {
  type: 'local' | 'government';
  from: string;
  to: CredentialLifecycleState;
  count: number;
};

export type TelemetryThresholdConfig = {
  min?: number | null;
  max?: number | null;
  warnLow?: number | null;
  warnHigh?: number | null;
  alertLow?: number | null;
  alertHigh?: number | null;
  target?: number | null;
  unit?: string | null;
  decimalPlaces?: number | null;
};

export type DeviceThresholdSummary = {
  effective: Record<string, TelemetryThresholdConfig>;
  installation: {
    thresholds: Record<string, TelemetryThresholdConfig>;
    templateId: string | null;
    updatedAt: string | null;
    updatedBy: string | null;
    metadata: Record<string, unknown> | null;
  } | null;
  override: {
    thresholds: Record<string, TelemetryThresholdConfig>;
    reason: string | null;
    updatedAt: string | null;
    updatedBy: string | null;
  } | null;
};

export type ProtocolSelectorSummary = {
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  serverVendorId: string;
  protocolVersionId: string;
  version: string;
};

export type DeviceCredentialHistoryItem = {
  type: 'local' | 'government';
  clientId: string;
  username: string;
  password: string;
  endpoints: DeviceCredentialEndpoint[];
  topics: {
    publish: string[];
    subscribe: string[];
  };
  validFrom: string;
  validTo: string | null;
  rotationReason: string | null;
  mqttAccessApplied: boolean | null;
  issuedBy: string | null;
  mqttAccess: {
    applied: boolean | null;
    jobId: string | null;
    lastAttemptAt: string | null;
    lastSuccessAt: string | null;
    lastFailureAt: string | null;
    operations: unknown[];
    error: unknown | null;
  } | null;
  lifecycle: CredentialLifecycleState | null;
  lifecycleHistory: CredentialLifecycleHistoryEntry[];
  originImportJobId: string | null;
  protocolSelector: ProtocolSelectorSummary | null;
};

export type ActiveCredentialSummary = {
  type: 'local' | 'government';
  clientId: string;
  username: string;
  password: string;
  endpoints: DeviceCredentialEndpoint[];
  topics: {
    publish: string[];
    subscribe: string[];
  };
  validFrom: string;
  issuedBy: string | null;
  lifecycle: CredentialLifecycleState | null;
  originImportJobId: string | null;
  protocolSelector: ProtocolSelectorSummary | null;
  mqttAccess?: {
    applied: boolean | null;
  } | null;
};

export type RegisterDeviceResponse = {
  device: {
    id: string;
    imei: string;
  };
  credentials: DeviceCredentialBundle | null;
  mqttProvisioning: MqttProvisioningInfo;
  governmentCredentials: GovernmentCredentialBundle | null;
};

export type RotateDeviceCredentialsPayload = {
  reason?: string;
  issuedBy?: string;
};

export type RotateDeviceCredentialsResponse = RegisterDeviceResponse;

export type RevokeDeviceCredentialsPayload = {
  type: 'local' | 'government';
  issuedBy?: string;
  reason?: string;
};

export type RevokeDeviceCredentialsResponse = {
  device: {
    id: string;
    imei: string;
  };
  revokedCount: number;
  lifecycleTransitions: LifecycleTransitionRecord[];
};

export type ImportDevicesCsvResult = {
  jobId: string;
  processed: number;
  enrolled: number;
  failed: number;
  errors: CsvImportError[];
  stateId: string | null;
  stateAuthorityId: string | null;
  projectId: string | null;
};

export type ImportGovernmentCredentialsCsvResult = {
  jobId: string;
  processed: number;
  updated: number;
  failed: number;
  errors: CsvImportError[];
  stateId: string | null;
  stateAuthorityId: string | null;
  projectId: string | null;
};

export type DeviceImportRetrySummary = {
  jobId: string;
  sourceJobId: string;
  rows: number[];
  processed: number;
  enrolled: number;
  failed: number;
  errors: CsvImportError[];
  stateId: string | null;
  stateAuthorityId: string | null;
  projectId: string | null;
};

export type DeviceStatusResponse = {
  device: {
    uuid: string;
    imei: string;
    status: string | null;
    configurationStatus: string | null;
    lastTelemetryAt: string | null;
    lastHeartbeatAt: string | null;
    connectivityStatus: 'unknown' | 'online' | 'offline';
    connectivityUpdatedAt: string | null;
    offlineThresholdMs: number;
    offlineNotificationChannelCount: number;
    originImportJobId: string | null;
    protocolVersion: {
      id: string;
      version: string;
      name: string | null;
      metadata: Record<string, unknown> | null;
    } | null;
  };
  telemetry: Array<{
    topic: string;
    topicSuffix: string;
    payload: Record<string, unknown>;
    receivedAt: string;
    ingestedAt: string;
    metadata: {
      qos: number | null;
      msgid: string | null;
      transport: string;
    };
    telemetryId?: string;
  }>;
  recentEvents: Array<{
    type: string;
    severity: 'info' | 'warn' | 'error';
    payload: Record<string, unknown>;
    createdAt: string;
  }>;
  credentialsHistory: DeviceCredentialHistoryItem[];
  activeCredentials: {
    local: ActiveCredentialSummary | null;
    government: ActiveCredentialSummary | null;
  };
  mqttProvisioning: MqttProvisioningInfo | null;
  thresholds: DeviceThresholdSummary;
};

export type DeviceListItem = {
  uuid: string;
  imei: string;
  status: string | null;
  configurationStatus: string | null;
  connectivityStatus: 'unknown' | 'online' | 'offline';
  connectivityUpdatedAt: string | null;
  lastTelemetryAt: string | null;
  lastHeartbeatAt: string | null;
  offlineThresholdMs: number;
  offlineNotificationChannelCount: number;
  protocolVersion: {
    id: string;
    version: string;
    name: string | null;
  } | null;
};

export type DeviceListResponse = {
  devices: DeviceListItem[];
  pagination: {
    total: number;
    limit: number;
    includeInactive: boolean;
    status: string | null;
  };
};

export type DeviceLookupResponse = {
  device: {
    uuid: string;
    imei: string;
    status: string | null;
    configurationStatus: string | null;
    connectivityStatus: 'unknown' | 'online' | 'offline';
    connectivityUpdatedAt: string | null;
    lastTelemetryAt: string | null;
    lastHeartbeatAt: string | null;
    offlineThresholdMs: number;
    offlineNotificationChannelCount: number;
    protocolVersion: {
      id: string;
      version: string;
      name: string | null;
    } | null;
  };
};

export type RetryMqttProvisioningResponse = {
  device: {
    id: string;
    imei: string;
  };
  mqttProvisioning: MqttProvisioningInfo;
  attemptsReset: boolean;
};

export type ResyncDeviceMqttProvisioningResponse = {
  device: {
    id: string;
    imei: string;
  };
  credentialHistoryId: string;
  previousJobId: string;
  resyncCount: number;
  mqttProvisioning: MqttProvisioningInfo;
  scope: {
    stateId: string | null;
    authorityId: string | null;
    projectId: string | null;
  };
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
  status: 'pending';
  topic: string;
  device: {
    uuid: string;
    imei: string;
  };
  simulatorSessionId: string | null;
};

export type AcknowledgeDeviceCommandPayload = {
  msgid: string;
  status: 'acknowledged' | 'failed';
  payload?: Record<string, unknown>;
  receivedAt?: string;
};

export type AcknowledgeDeviceCommandResponse = {
  msgid: string;
  status: 'acknowledged' | 'failed';
  acknowledgedAt: string;
};

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
  status: 'pending' | 'acknowledged' | 'failed';
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

export type FetchDeviceCommandHistoryResponse = {
  device: {
    uuid: string;
    imei: string;
  };
  commands: DeviceCommandHistoryRecord[];
  nextCursor: string | null;
};

export type FetchDeviceCommandHistoryParams = {
  limit?: number;
  cursor?: string | null;
  statuses?: Array<'pending' | 'acknowledged' | 'failed'>;
};

export async function registerDevice(
  payload: RegisterDevicePayload,
): Promise<RegisterDeviceResponse> {
  const response = await apiFetch(`${API_BASE_URL}/devices`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to register device';
    throw new Error(message);
  }

  return responseBody as RegisterDeviceResponse;
}

export async function fetchDeviceStatus(deviceUuid: string): Promise<DeviceStatusResponse> {
  const response = await apiFetch(`${API_BASE_URL}/devices/${deviceUuid}/status`);

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load device status';
    throw new Error(message);
  }

  return responseBody as DeviceStatusResponse;
}

export type DeviceSummary = {
  id: string;
  uuid?: string;
  imei: string;
  projectId: string;
  name: string;
  status: string;
  model_id?: string;
  metadata: Record<string, unknown> | null;
  last_seen?: string;
  lastTelemetryAt?: string;
  lastHeartbeatAt?: string;
  connectivity_status?: string;
  connectivityStatus?: string;
  connectivity_updated_at?: string;
  connectivityUpdatedAt?: string;
  shadow?: Record<string, unknown>;
  configurationStatus?: unknown;
  protocolVersion?: unknown;
  offlineThresholdMs?: unknown;
  offlineNotificationChannelCount?: unknown;
};

export async function fetchDevice(idOrUuid: string): Promise<DeviceSummary> {
  const trimmed = idOrUuid.trim();
  if (!trimmed) {
    throw new Error('Device identifier is required');
  }

  const response = await apiFetch(`${API_BASE_URL}/devices/${encodeURIComponent(trimmed)}`);
  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? body?.error ?? 'Unable to load device';
    throw new Error(message);
  }

  return body as DeviceSummary;
}

export async function updateDevice(idOrUuid: string, payload: Record<string, unknown>): Promise<DeviceSummary> {
  const trimmed = idOrUuid.trim();
  if (!trimmed) {
    throw new Error('Device identifier is required');
  }

  const response = await apiFetch(`${API_BASE_URL}/devices/${encodeURIComponent(trimmed)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload ?? {}),
  });

  const body = await readJsonBody<any>(response);
  if (!response.ok || !body) {
    const message = body?.message ?? body?.error ?? 'Unable to update device';
    throw new Error(message);
  }

  return body as DeviceSummary;
}

export async function deleteDevice(idOrUuid: string): Promise<void> {
  const trimmed = idOrUuid.trim();
  if (!trimmed) {
    throw new Error('Device identifier is required');
  }

  const response = await apiFetch(`${API_BASE_URL}/devices/${encodeURIComponent(trimmed)}`, {
    method: 'DELETE',
  });

  if (!response.ok) {
    const body = await readJsonBody<any>(response);
    const message = body?.message ?? body?.error ?? 'Unable to delete device';
    throw new Error(message);
  }
}

export async function fetchDeviceCredentialHistory(deviceUuid: string): Promise<{ items: DeviceCredentialHistoryItem[] }> {
  const trimmedDeviceUuid = deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to load credential history');
  }

  const response = await apiFetch(
    `${API_BASE_URL}/devices/${encodeURIComponent(trimmedDeviceUuid)}/credentials/history`,
  );
  const body = await readJsonBody<any>(response);
  if (!response.ok || !body) {
    const message = body?.message ?? body?.error?.message ?? 'Unable to load credential history';
    throw new Error(message);
  }
  return body as { items: DeviceCredentialHistoryItem[] };
}

export async function issueCredentialDownloadToken(deviceUuid: string, payload?: { expiresInSeconds?: number }): Promise<{ token: string; expiresAt: string }> {
  const trimmedDeviceUuid = deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to issue download token');
  }

  const response = await apiFetch(
    `${API_BASE_URL}/devices/${encodeURIComponent(trimmedDeviceUuid)}/credentials/download-token`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload ?? {}),
    },
  );

  const body = await readJsonBody<any>(response);
  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to issue credential download token';
    throw new Error(message);
  }
  return body as { token: string; expiresAt: string };
}

export async function fetchDeviceList(params?: {
  limit?: number;
  includeInactive?: boolean;
  status?: 'active' | 'inactive';
}): Promise<DeviceListResponse> {
  const searchParams = new URLSearchParams();

  if (params?.limit) {
    searchParams.set('limit', String(params.limit));
  }
  if (params?.includeInactive) {
    searchParams.set('includeInactive', String(params.includeInactive));
  }
  if (params?.status) {
    searchParams.set('status', params.status);
  }

  const query = searchParams.toString();
  const url = query ? `${API_BASE_URL}/devices?${query}` : `${API_BASE_URL}/devices`;

  const response = await apiFetch(url);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load devices';
    throw new Error(message);
  }

  return responseBody as DeviceListResponse;
}

export async function lookupDevice(params: {
  deviceUuid?: string;
  imei?: string;
}): Promise<DeviceLookupResponse> {
  const query = new URLSearchParams();
  if (params.deviceUuid) {
    query.set('deviceUuid', params.deviceUuid);
  }
  if (params.imei) {
    query.set('imei', params.imei);
  }

  if (!query.toString()) {
    throw new Error('Provide deviceUuid or IMEI to lookup a device');
  }

  const response = await apiFetch(`${API_BASE_URL}/devices/lookup?${query.toString()}`);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to lookup device';
    throw new Error(message);
  }

  return responseBody as DeviceLookupResponse;
}

export async function rotateDeviceCredentials(
  deviceUuid: string,
  payload: RotateDeviceCredentialsPayload,
): Promise<RotateDeviceCredentialsResponse> {
  const response = await apiFetch(`${API_BASE_URL}/devices/${deviceUuid}/credentials/rotate`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload ?? {}),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to rotate device credentials';
    throw new Error(message);
  }

  return responseBody as RotateDeviceCredentialsResponse;
}

export async function revokeDeviceCredentials(
  deviceUuid: string,
  payload: RevokeDeviceCredentialsPayload,
): Promise<RevokeDeviceCredentialsResponse> {
  const trimmedDeviceUuid = deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to revoke credentials');
  }

  const response = await apiFetch(
    `${API_BASE_URL}/devices/${encodeURIComponent(trimmedDeviceUuid)}/credentials/revoke`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload ?? {}),
    },
  );

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to revoke credentials';
    throw new Error(message);
  }

  return responseBody as RevokeDeviceCredentialsResponse;
}

export async function fetchImportJobs(params?: {
  cursor?: string | null;
  limit?: number;
  types?: ImportJobType[];
  statuses?: ImportJobStatus[];
  stateId?: string | null;
  stateAuthorityId?: string | null;
  projectId?: string | null;
  jobId?: string | null;
}): Promise<{ jobs: ImportJob[]; nextCursor: string | null }> {
  const searchParams = new URLSearchParams();

  if (params?.limit) {
    searchParams.set('limit', String(params.limit));
  }

  if (params?.cursor) {
    searchParams.set('cursor', params.cursor);
  }

  if (params?.types?.length) {
    for (const type of params.types) {
      searchParams.append('type', type);
    }
  }

  if (params?.statuses?.length) {
    for (const status of params.statuses) {
      searchParams.append('status', status);
    }
  }

  if (params?.stateId) {
    searchParams.set('stateId', params.stateId);
  }

  if (params?.stateAuthorityId) {
    searchParams.set('stateAuthorityId', params.stateAuthorityId);
  }

  if (params?.projectId) {
    searchParams.set('projectId', params.projectId);
  }

  if (params?.jobId) {
    searchParams.set('jobId', params.jobId);
  }

  const query = searchParams.toString();
  const url = query
    ? `${API_BASE_URL}/devices/import/jobs?${query}`
    : `${API_BASE_URL}/devices/import/jobs`;

  const response = await apiFetch(url);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load import jobs';
    throw new Error(message);
  }

  return responseBody as { jobs: ImportJob[]; nextCursor: string | null };
}

export async function fetchImportJob(jobId: string): Promise<ImportJob> {
  const trimmed = jobId.trim();
  if (!trimmed.length) {
    throw new Error('A job identifier is required to load details');
  }

  const response = await apiFetch(
    `${API_BASE_URL}/devices/import/jobs/${encodeURIComponent(trimmed)}`,
  );
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load import job details';
    throw new Error(message);
  }

  return responseBody as ImportJob;
}

type ApiErrorPayload = {
  message?: string | null;
  error?: {
    message?: string | null;
  } | null;
};
function extractApiErrorMessage(payload: unknown): string | null {
  if (!payload || typeof payload !== 'object') {
    return null;
  }

  const data = payload as ApiErrorPayload;
  const directMessage = typeof data.message === 'string' ? data.message.trim() : '';
  if (directMessage.length) {
    return directMessage;
  }
  const nestedMessage = typeof data.error?.message === 'string' ? data.error.message.trim() : '';
  return nestedMessage.length ? nestedMessage : null;
}

export async function downloadImportJobErrorsCsv(
  jobId: string,
): Promise<{ blob: Blob; filename: string }> {
  const trimmed = jobId.trim();
  if (!trimmed) {
    throw new Error('A job identifier is required to download errors');
  }

  const response = await apiFetch(
    `${API_BASE_URL}/devices/import/jobs/${encodeURIComponent(trimmed)}/errors.csv`,
    {
      headers: {
        Accept: 'text/csv',
      },
    },
  );

  if (!response.ok) {
    const contentType = response.headers.get('content-type') ?? '';
    let message = 'Unable to download import job errors';

    if (contentType.includes('application/json')) {
      const body = await readJsonBody<unknown>(response);
      const extracted = extractApiErrorMessage(body);
      if (extracted) {
        message = extracted;
      }
    } else {
      const text = await response.text().catch(() => null);
      if (typeof text === 'string' && text.trim().length) {
        message = text.trim();
      }
    }

    throw new Error(message);
  }

  const blob = await response.blob();
  const disposition = response.headers.get('content-disposition') ?? '';
  let filename = `import-job-${trimmed}-errors.csv`;

  const encodedMatch = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (encodedMatch && encodedMatch[1]) {
    filename = decodeURIComponent(encodedMatch[1]);
  } else {
    const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
    if (plainMatch && plainMatch[1]) {
      filename = plainMatch[1];
    }
  }

  return { blob, filename };
}

export async function upsertDeviceGovernmentCredentials(
  deviceUuid: string,
  payload: GovernmentCredentialPayload,
): Promise<{ device: { id: string; imei: string }; credentials: GovernmentCredentialBundle }> {
  const response = await apiFetch(`${API_BASE_URL}/devices/${deviceUuid}/government-credentials`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to save government credentials';
    throw new Error(message);
  }

  const { device, credentials } = responseBody as {
    device: { id: string; imei: string };
    credentials: GovernmentCredentialBundle;
  };

  return { device, credentials };
}

export async function importDevicesCsv(payload: {
  csv: string;
  issuedBy?: string;
}): Promise<ImportDevicesCsvResult> {
  const response = await apiFetch(`${API_BASE_URL}/devices/import`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to import devices';
    throw new Error(message);
  }

  return responseBody as ImportDevicesCsvResult;
}

export async function importGovernmentCredentialsCsv(payload: {
  csv: string;
  issuedBy?: string;
}): Promise<ImportGovernmentCredentialsCsvResult> {
  const response = await apiFetch(`${API_BASE_URL}/devices/government-credentials/import`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to import government credentials';
    throw new Error(message);
  }

  return responseBody as ImportGovernmentCredentialsCsvResult;
}

export async function retryDeviceMqttProvisioning(
  deviceUuid: string,
): Promise<RetryMqttProvisioningResponse> {
  const response = await apiFetch(`${API_BASE_URL}/devices/${deviceUuid}/mqtt-provisioning/retry`, {
    method: 'POST',
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to retry MQTT provisioning';
    throw new Error(message);
  }

  return responseBody as RetryMqttProvisioningResponse;
}

export async function resyncDeviceMqttProvisioning(payload: {
  deviceUuid: string;
  reason?: string;
}): Promise<ResyncDeviceMqttProvisioningResponse> {
  const trimmedDeviceUuid = payload.deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to resync provisioning');
  }

  const body: Record<string, unknown> = { deviceUuid: trimmedDeviceUuid };
  if (payload.reason && payload.reason.trim().length > 0) {
    body.reason = payload.reason.trim();
  }

  const response = await apiFetch(`${API_BASE_URL}/broker/sync`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to queue broker resync';
    throw new Error(message);
  }

  return responseBody as ResyncDeviceMqttProvisioningResponse;
}

export async function issueDeviceCommand(
  deviceUuid: string,
  payload: IssueDeviceCommandPayload,
): Promise<IssueDeviceCommandResponse> {
  const trimmedDeviceUuid = deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to issue a command');
  }

  const response = await apiFetch(`${API_BASE_URL}/devices/${trimmedDeviceUuid}/commands`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to issue device command';
    throw new Error(message);
  }

  return responseBody as IssueDeviceCommandResponse;
}

type SimpleCommandResponse = {
  msgid: string;
  status: 'pending';
  topic: string;
  device: { uuid: string; imei: string };
};

async function issueSimpleDeviceCommand(
  deviceUuid: string,
  path: string,
  payload?: Record<string, unknown>,
): Promise<SimpleCommandResponse> {
  const trimmedDeviceUuid = deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to issue a command');
  }

  const response = await apiFetch(`${API_BASE_URL}/devices/${trimmedDeviceUuid}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload ?? {}),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to issue device command';
    throw new Error(message);
  }

  return responseBody as SimpleCommandResponse;
}

export function setVfdConfig(deviceUuid: string, payload: Record<string, unknown>) {
  return issueSimpleDeviceCommand(deviceUuid, '/commands/set-vfd', { payload });
}

export function getVfdConfig(deviceUuid: string, payload?: Record<string, unknown>) {
  return issueSimpleDeviceCommand(deviceUuid, '/commands/get-vfd', payload ? { payload } : {});
}

export function setBeneficiary(deviceUuid: string, payload: Record<string, unknown>) {
  return issueSimpleDeviceCommand(deviceUuid, '/commands/set-beneficiary', { payload });
}

export function getBeneficiary(deviceUuid: string, payload?: Record<string, unknown>) {
  return issueSimpleDeviceCommand(deviceUuid, '/commands/get-beneficiary', payload ? { payload } : {});
}

export function setInstallation(deviceUuid: string, payload: Record<string, unknown>) {
  return issueSimpleDeviceCommand(deviceUuid, '/commands/set-installation', { payload });
}

export function getInstallation(deviceUuid: string, payload?: Record<string, unknown>) {
  return issueSimpleDeviceCommand(deviceUuid, '/commands/get-installation', payload ? { payload } : {});
}

export async function acknowledgeDeviceCommand(
  deviceUuid: string,
  payload: AcknowledgeDeviceCommandPayload,
): Promise<AcknowledgeDeviceCommandResponse> {
  const trimmedDeviceUuid = deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to acknowledge a command');
  }

  const response = await apiFetch(`${API_BASE_URL}/devices/${trimmedDeviceUuid}/commands/ack`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to acknowledge device command';
    throw new Error(message);
  }

  return responseBody as AcknowledgeDeviceCommandResponse;
}

export async function fetchDeviceCommandHistory(
  deviceUuid: string,
  params?: FetchDeviceCommandHistoryParams,
): Promise<FetchDeviceCommandHistoryResponse> {
  const trimmedDeviceUuid = deviceUuid.trim();
  if (!trimmedDeviceUuid) {
    throw new Error('Device identifier is required to load command history');
  }

  const searchParams = new URLSearchParams();

  if (typeof params?.limit === 'number') {
    searchParams.set('limit', String(params.limit));
  }

  if (params?.cursor) {
    searchParams.set('cursor', params.cursor);
  }

  if (params?.statuses?.length) {
    for (const status of params.statuses) {
      searchParams.append('status', status);
    }
  }

  const query = searchParams.toString();
  const url = query
    ? `${API_BASE_URL}/devices/${trimmedDeviceUuid}/commands/history?${query}`
    : `${API_BASE_URL}/devices/${trimmedDeviceUuid}/commands/history`;

  const response = await apiFetch(url);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to load device command history';
    throw new Error(message);
  }

  return responseBody as FetchDeviceCommandHistoryResponse;
}

export async function retryDeviceImportJobRows(
  jobId: string,
  payload: { rows: number[]; issuedBy?: string },
): Promise<DeviceImportRetrySummary> {
  const trimmedJobId = jobId.trim();
  if (!trimmedJobId) {
    throw new Error('A job identifier is required to retry rows');
  }

  const response = await apiFetch(
    `${API_BASE_URL}/devices/import/jobs/${encodeURIComponent(trimmedJobId)}/retry`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    },
  );

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok || !responseBody) {
    const message = responseBody?.message ?? 'Unable to retry import rows';
    throw new Error(message);
  }

  return responseBody as DeviceImportRetrySummary;
}
