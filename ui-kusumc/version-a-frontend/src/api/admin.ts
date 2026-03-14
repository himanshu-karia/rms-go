import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

const DEFAULT_URL_BASE = 'http://localhost';

export type AdminState = {
  id: string;
  name: string;
  isoCode: string | null;
  metadata: Record<string, unknown> | null;
  createdAt: string;
  updatedAt: string;
};

export type AdminStateAuthority = {
  id: string;
  stateId: string;
  name: string;
  metadata: Record<string, unknown> | null;
  createdAt: string;
  updatedAt: string;
};

function pickAuthority(body: unknown): AdminStateAuthority {
  if (!body || typeof body !== 'object') {
    throw new Error('Unable to parse state authority response');
  }

  const record = body as Record<string, unknown>;
  const authority = (record.stateAuthority ?? record.state_authority ?? record.authority) as
    | Record<string, unknown>
    | undefined;

  if (!authority || typeof authority !== 'object') {
    throw new Error('State authority payload is missing in response');
  }

  const stateId = (authority.stateId ?? authority.state_id) as string | undefined;
  const createdAt = (authority.createdAt ?? authority.created_at) as string | undefined;
  const updatedAt =
    (authority.updatedAt ?? authority.updated_at ?? authority.createdAt ?? authority.created_at) as
      | string
      | undefined;

  return {
    id: String(authority.id ?? ''),
    stateId: stateId ?? '',
    name: String(authority.name ?? ''),
    metadata: (authority.metadata ?? authority.contactInfo ?? authority.contact_info ?? null) as
      | Record<string, unknown>
      | null,
    createdAt: createdAt ?? '',
    updatedAt: updatedAt ?? '',
  };
}

export type AdminProject = {
  id: string;
  authorityId: string;
  name: string;
  metadata: Record<string, unknown> | null;
  createdAt: string;
  updatedAt: string;
};

export type AdminProtocolVersion = {
  id: string;
  stateId: string;
  authorityId: string;
  projectId: string;
  serverVendorId: string;
  serverVendorName: string | null;
  version: string;
  name: string | null;
  metadata: Record<string, unknown> | null;
  createdAt: string;
  updatedAt: string;
};

export type AdminVendor = {
  id: string;
  name: string;
  metadata: Record<string, unknown> | null;
  createdAt: string;
  updatedAt: string;
};

export type VendorCollectionKey = 'server' | 'solarPump' | 'vfdManufacturer' | 'rmsManufacturer';

const vendorPaths: Record<VendorCollectionKey, string> = {
  server: 'server-vendors',
  solarPump: 'solar-pump-vendors',
  vfdManufacturer: 'vfd-drive-manufacturers',
  rmsManufacturer: 'rms-manufacturers',
};

function buildUrl(path: string, search?: Record<string, string | undefined>) {
  const raw = `${API_BASE_URL}/admin/${path}`;
  const base = typeof window !== 'undefined' && window.location?.origin ? window.location.origin : DEFAULT_URL_BASE;
  const url = new URL(raw, base);
  if (search) {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(search)) {
      if (value) {
        params.set(key, value);
      }
    }
    url.search = params.toString();
  }
  return url.toString();
}

async function parseResponse<T>(response: Response, fallbackMessage: string): Promise<T> {
  const body = await readJsonBody<unknown>(response);
  if (!response.ok) {
    const message = (body as { message?: string } | null)?.message ?? fallbackMessage;
    throw new Error(message);
  }
  return body as T;
}

async function parseEmptyResponse(response: Response, fallbackMessage: string): Promise<void> {
  if (response.ok) {
    return;
  }

  const body = await readJsonBody<unknown>(response);
  const message = (body as { message?: string } | null)?.message ?? fallbackMessage;
  throw new Error(message);
}

function coerceCollection<T>(
  body: unknown,
  keys: string[],
): T[] {
  if (Array.isArray(body)) {
    return body as T[];
  }

  if (body && typeof body === 'object') {
    const record = body as Record<string, unknown>;
    for (const key of keys) {
      const value = record[key];
      if (Array.isArray(value)) {
        return value as T[];
      }
    }
  }

  return [];
}

function pickEntity<T>(body: unknown, keys: string[], errorMessage: string): T {
  if (!body || typeof body !== 'object') {
    throw new Error(errorMessage);
  }

  const record = body as Record<string, unknown>;
  for (const key of keys) {
    const value = record[key];
    if (value && typeof value === 'object') {
      return value as T;
    }
  }

  throw new Error(errorMessage);
}

export async function fetchAdminStates(): Promise<AdminState[]> {
  const response = await apiFetch(buildUrl('states'));
  const body = await parseResponse<unknown>(response, 'Unable to load states');
  return coerceCollection<AdminState>(body, ['states']);
}

export async function createAdminState(payload: {
  name: string;
  isoCode?: string | null;
  metadata?: Record<string, unknown> | null;
}): Promise<AdminState> {
  const response = await apiFetch(buildUrl('states'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<unknown>(response, 'Unable to create state');
  return pickEntity<AdminState>(body, ['state'], 'State payload is missing in response');
}

export async function updateAdminState(
  stateId: string,
  payload: {
    name?: string;
    isoCode?: string | null;
    metadata?: Record<string, unknown> | null;
  },
): Promise<AdminState> {
  const response = await apiFetch(buildUrl(`states/${stateId}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<unknown>(response, 'Unable to update state');
  return pickEntity<AdminState>(body, ['state'], 'State payload is missing in response');
}

export async function deleteAdminState(stateId: string): Promise<void> {
  const response = await apiFetch(buildUrl(`states/${stateId}`), {
    method: 'DELETE',
  });
  await parseEmptyResponse(response, 'Unable to delete state');
}

export async function fetchAdminStateAuthorities(
  params: {
    stateId?: string;
  } = {},
): Promise<AdminStateAuthority[]> {
  const response = await apiFetch(buildUrl('state-authorities', { stateId: params.stateId }));
  const body = await parseResponse<unknown>(
    response,
    'Unable to load state authorities',
  );
  return coerceCollection<AdminStateAuthority>(body, ['stateAuthorities', 'state_authorities']);
}

export async function createAdminStateAuthority(payload: {
  stateId: string;
  name: string;
  metadata?: Record<string, unknown> | null;
}): Promise<AdminStateAuthority> {
  const response = await apiFetch(buildUrl('state-authorities'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      state_id: payload.stateId,
      stateId: payload.stateId,
      name: payload.name,
      metadata: payload.metadata,
      contact_info: payload.metadata,
    }),
  });
  const body = await parseResponse<unknown>(
    response,
    'Unable to create state authority',
  );
  return pickAuthority(body);
}

export async function updateAdminStateAuthority(
  stateAuthorityId: string,
  payload: {
    name?: string;
    metadata?: Record<string, unknown> | null;
  },
): Promise<AdminStateAuthority> {
  const response = await apiFetch(buildUrl(`state-authorities/${stateAuthorityId}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      ...payload,
      contact_info: payload.metadata,
    }),
  });
  const body = await parseResponse<unknown>(
    response,
    'Unable to update state authority',
  );
  return pickAuthority(body);
}

export async function deleteAdminStateAuthority(stateAuthorityId: string): Promise<void> {
  const response = await apiFetch(buildUrl(`state-authorities/${stateAuthorityId}`), {
    method: 'DELETE',
  });
  await parseEmptyResponse(response, 'Unable to delete state authority');
}

export async function fetchAdminProjects(
  params: {
    stateAuthorityId?: string;
  } = {},
): Promise<AdminProject[]> {
  const response = await apiFetch(
    buildUrl('projects', { stateAuthorityId: params.stateAuthorityId }),
  );
  const body = await parseResponse<unknown>(
    response,
    'Unable to load projects',
  );
  return coerceCollection<AdminProject>(body, ['projects']);
}

export async function fetchAdminProtocolVersions(params: {
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  serverVendorId?: string;
}): Promise<AdminProtocolVersion[]> {
  const response = await apiFetch(
    buildUrl('protocol-versions', {
      stateId: params.stateId,
      stateAuthorityId: params.stateAuthorityId,
      projectId: params.projectId,
      serverVendorId: params.serverVendorId,
    }),
  );

  const body = await parseResponse<unknown>(
    response,
    'Unable to load protocol versions',
  );

  return coerceCollection<AdminProtocolVersion>(body, ['protocolVersions', 'protocol_versions']);
}

export async function createAdminProtocolVersion(payload: {
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  serverVendorId: string;
  version: string;
  name?: string | null;
  metadata?: Record<string, unknown> | null;
}): Promise<AdminProtocolVersion> {
  const response = await apiFetch(buildUrl('protocol-versions'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const body = await parseResponse<unknown>(
    response,
    'Unable to create protocol version',
  );

  return pickEntity<AdminProtocolVersion>(
    body,
    ['protocolVersion', 'protocol_version'],
    'Protocol version payload is missing in response',
  );
}

export async function updateAdminProtocolVersion(
  protocolVersionId: string,
  payload: {
    serverVendorId?: string;
    version?: string;
    name?: string | null;
    metadata?: Record<string, unknown> | null;
  },
): Promise<AdminProtocolVersion> {
  const response = await apiFetch(buildUrl(`protocol-versions/${protocolVersionId}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const body = await parseResponse<unknown>(
    response,
    'Unable to update protocol version',
  );

  return pickEntity<AdminProtocolVersion>(
    body,
    ['protocolVersion', 'protocol_version'],
    'Protocol version payload is missing in response',
  );
}

export async function createAdminProject(payload: {
  authorityId: string;
  name: string;
  metadata?: Record<string, unknown> | null;
}): Promise<AdminProject> {
  const response = await apiFetch(buildUrl('projects'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<unknown>(response, 'Unable to create project');
  return pickEntity<AdminProject>(body, ['project'], 'Project payload is missing in response');
}

export async function updateAdminProject(
  projectId: string,
  payload: {
    name?: string;
    metadata?: Record<string, unknown> | null;
  },
): Promise<AdminProject> {
  const response = await apiFetch(buildUrl(`projects/${projectId}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<unknown>(response, 'Unable to update project');
  return pickEntity<AdminProject>(body, ['project'], 'Project payload is missing in response');
}

export async function deleteAdminProject(projectId: string): Promise<void> {
  const response = await apiFetch(buildUrl(`projects/${projectId}`), {
    method: 'DELETE',
  });
  await parseEmptyResponse(response, 'Unable to delete project');
}

export async function fetchAdminVendors(collection: VendorCollectionKey): Promise<AdminVendor[]> {
  const response = await apiFetch(buildUrl(vendorPaths[collection]));
  const body = await parseResponse<unknown>(response, 'Unable to load vendors');
  return coerceCollection<AdminVendor>(body, ['vendors']);
}

export async function createAdminVendor(
  collection: VendorCollectionKey,
  payload: { name: string; metadata?: Record<string, unknown> | null },
): Promise<AdminVendor> {
  const response = await apiFetch(buildUrl(vendorPaths[collection]), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<{ vendor: AdminVendor }>(response, 'Unable to create vendor');
  return body.vendor;
}

export async function updateAdminVendor(
  collection: VendorCollectionKey,
  entityId: string,
  payload: { name?: string; metadata?: Record<string, unknown> | null },
): Promise<AdminVendor> {
  const response = await apiFetch(buildUrl(`${vendorPaths[collection]}/${entityId}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<{ vendor: AdminVendor }>(response, 'Unable to update vendor');
  return body.vendor;
}

export async function deleteAdminVendor(
  collection: VendorCollectionKey,
  entityId: string,
): Promise<void> {
  const response = await apiFetch(buildUrl(`${vendorPaths[collection]}/${entityId}`), {
    method: 'DELETE',
  });
  await parseEmptyResponse(response, 'Unable to delete vendor');
}
