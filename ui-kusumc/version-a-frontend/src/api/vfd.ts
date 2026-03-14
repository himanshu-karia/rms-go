import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type Rs485Config = {
  baudRate: number;
  dataBits: number;
  stopBits: number;
  parity: string;
  flowControl: string;
  metadata?: Record<string, unknown> | null;
};

export type RealtimeParameterDefinition = {
  parameterName: string;
  address: number;
  multiplier: number;
  unit: string;
  metadata?: Record<string, unknown>;
};

export type FaultDefinition = {
  faultCode: string;
  address: number;
  faultName: string;
  description: string;
  severity?: string | null;
  metadata?: Record<string, unknown>;
};

export type CommandDefinition = {
  commandName: string;
  address: number;
  functionCode?: string | number | null;
  description?: string | null;
  payloadTemplate?: Record<string, unknown> | null;
  minValue?: number | null;
  maxValue?: number | null;
  metadata?: Record<string, unknown> | null;
};

export type VfdAssignment = {
  id: string;
  protocolVersionId: string;
  protocolVersionVersion: string;
  protocolVersionName: string | null;
  serverVendorId: string;
  serverVendorName: string | null;
  vfdModelId: string;
  assignedAt: string;
  assignedBy: string | null;
  metadata: Record<string, unknown> | null;
  revokedAt: string | null;
  revokedBy: string | null;
  revocationReason: string | null;
};

export type VfdModel = {
  id: string;
  manufacturerId: string;
  manufacturer: string;
  manufacturerName: string;
  model: string;
  version: string;
  rs485: Rs485Config;
  realtimeParameters: RealtimeParameterDefinition[];
  faultMap: FaultDefinition[];
  commandDictionary: CommandDefinition[];
  metadata: Record<string, unknown> | null;
  assignments: VfdAssignment[];
  createdAt: string;
  updatedAt: string;
};

type ApiVfdModel = {
  id: string;
  manufacturerId: string;
  manufacturerName: string;
  model: string;
  version: string;
  rs485: Omit<Rs485Config, 'metadata'> & { metadata?: Record<string, unknown> | null };
  realtimeParameters: RealtimeParameterDefinition[];
  faultMap: FaultDefinition[];
  commandDictionary?: CommandDefinition[];
  metadata: Record<string, unknown> | null;
  assignments?: VfdAssignment[];
  createdAt: string;
  updatedAt: string;
};

type ApiError = Error & {
  status?: number;
  details?: unknown;
};

function normalizeVfdModel(model: ApiVfdModel): VfdModel {
  return {
    id: model.id,
    manufacturerId: model.manufacturerId,
    manufacturer: model.manufacturerName,
    manufacturerName: model.manufacturerName,
    model: model.model,
    version: model.version,
    rs485: {
      ...model.rs485,
      metadata: model.rs485.metadata ?? null,
    },
    realtimeParameters: model.realtimeParameters,
    faultMap: model.faultMap,
    commandDictionary: model.commandDictionary ?? [],
    metadata: model.metadata,
    assignments: model.assignments ?? [],
    createdAt: model.createdAt,
    updatedAt: model.updatedAt,
  };
}

export async function fetchVfdModels(protocolVersionId?: string): Promise<VfdModel[]> {
  const query = protocolVersionId
    ? `?protocolVersionId=${encodeURIComponent(protocolVersionId)}`
    : '';
  const response = await apiFetch(`${API_BASE_URL}/vfd-models${query}`);
  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to load VFD models';
    const error = new Error(message) as ApiError;
    error.status = response.status;
    if (body?.issues) {
      error.details = body.issues;
    }
    throw error;
  }

  const models = Array.isArray(body.models) ? (body.models as ApiVfdModel[]) : [];
  return models.map(normalizeVfdModel);
}

export type CreateVfdModelPayload = {
  manufacturerId: string;
  model: string;
  version: string;
  rs485: Rs485Config;
  realtimeParameters?: RealtimeParameterDefinition[];
  faultMap?: FaultDefinition[];
  metadata?: Record<string, unknown> | null;
  protocolVersionId?: string;
  assignmentMetadata?: Record<string, unknown>;
  commandDictionary?: CommandDefinition[];
};

export async function createVfdModel(payload: CreateVfdModelPayload): Promise<VfdModel> {
  const response = await apiFetch(`${API_BASE_URL}/vfd-models`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to create VFD model';
    const error = new Error(message) as ApiError;
    error.status = response.status;
    if (body?.issues) {
      error.details = body.issues;
    }
    throw error;
  }

  return normalizeVfdModel(body as ApiVfdModel);
}

export type ImportVfdModelsResult = {
  created: number;
  errors: Array<{ row: number; message: string }>;
  models: VfdModel[];
};

export async function importVfdModelsCsv(payload: {
  csv: string;
  projectId: string;
  manufacturerId?: string;
  protocolVersionId?: string;
}): Promise<{ models: VfdModel[]; count: number }> {
  const response = await apiFetch(`${API_BASE_URL}/vfd-models/import`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to import VFD models';
    const error = new Error(message) as ApiError;
    error.status = response.status;
    if (body?.issues) {
      error.details = body.issues;
    }
    throw error;
  }

  const created = Array.isArray(body.models) ? (body.models as ApiVfdModel[]).map(normalizeVfdModel) : [];
  return { models: created, count: typeof body.count === 'number' ? body.count : created.length };
}

export function buildVfdModelsExportUrl(projectId: string): string {
  const trimmed = projectId.trim();
  if (!trimmed) {
    throw new Error('projectId is required');
  }
  const query = new URLSearchParams({ projectId: trimmed });
  return `${API_BASE_URL}/vfd-models/export.csv?${query.toString()}`;
}

export type VfdCommandImportJob = Record<string, unknown>;

export async function listVfdCommandImportJobs(params: {
  projectId: string;
  status?: string;
  limit?: number;
}): Promise<{ jobs: VfdCommandImportJob[]; count: number }> {
  const projectId = params.projectId.trim();
  if (!projectId) throw new Error('projectId is required');

  const query = new URLSearchParams({ projectId });
  if (params.status) query.set('status', params.status);
  if (typeof params.limit === 'number') query.set('limit', String(params.limit));

  const response = await apiFetch(`${API_BASE_URL}/vfd-models/command-dictionaries/import/jobs?${query.toString()}`);
  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to load import jobs';
    throw new Error(message);
  }

  return body as { jobs: VfdCommandImportJob[]; count: number };
}

export async function importVfdCommandDictionary(payload: {
  projectId: string;
  modelId: string;
  mergeStrategy: string;
  csv?: string;
  json?: string;
}): Promise<{ model: VfdModel }> {
  const response = await apiFetch(`${API_BASE_URL}/vfd-models/command-dictionaries/import`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = body?.message ?? 'Unable to import command dictionary';
    throw new Error(message);
  }

  const model = body.model ? normalizeVfdModel(body.model as ApiVfdModel) : null;
  if (!model) {
    throw new Error('Missing model in response');
  }
  return { model };
}
