import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type ApiError = Error & {
  status?: number;
  details?: unknown;
};

function buildApiError(response: Response, body: unknown, fallbackMessage: string): ApiError {
  const candidate =
    typeof body === 'object' && body !== null ? (body as Record<string, unknown>) : undefined;

  const messageCandidate = candidate?.message;
  const nestedError = candidate && typeof candidate.error === 'object' ? candidate.error : null;
  const nestedMessage =
    nestedError && nestedError !== null && 'message' in nestedError
      ? (nestedError.message as string | undefined)
      : undefined;

  const resolvedMessage =
    (typeof messageCandidate === 'string' && messageCandidate.trim().length > 0
      ? messageCandidate
      : undefined) ??
    (typeof nestedMessage === 'string' && nestedMessage.trim().length > 0
      ? nestedMessage
      : undefined) ??
    fallbackMessage;

  const error = new Error(resolvedMessage) as ApiError;
  error.status = response.status;

  if (candidate && 'details' in candidate && candidate.details !== undefined) {
    error.details = candidate.details;
  } else if (
    nestedError &&
    typeof nestedError === 'object' &&
    nestedError !== null &&
    'details' in nestedError &&
    (nestedError as Record<string, unknown>).details !== undefined
  ) {
    error.details = (nestedError as Record<string, unknown>).details;
  }

  return error;
}

export type InstallationStatus = 'active' | 'inactive' | 'decommissioned';
export type AssignmentStatus = 'active' | 'removed';
export type BeneficiaryAccountStatus = 'pending' | 'invited' | 'active' | 'disabled';

export type GeoLocation = {
  latitude: number;
  longitude: number;
  accuracyMeters: number | null;
  source: string | null;
  capturedAt: string | null;
} | null;

export type GeoLocationInput = {
  latitude?: number | null;
  longitude?: number | null;
  accuracyMeters?: number | null;
  precisionMeters?: number | null;
  source?: string | null;
  capturedAt?: string | null;
} | null;

export type BeneficiaryContact = {
  type: 'phone' | 'email';
  value: string;
  isPrimary: boolean;
  proxyBeneficiaryUuid: string | null;
};

export type BeneficiaryContactInput = {
  type: 'phone' | 'email';
  value: string;
  isPrimary?: boolean;
  proxyBeneficiaryUuid?: string | null;
};

export type BeneficiaryLocation = {
  state: string | null;
  district: string | null;
  tehsil: string | null;
  village: string | null;
  geoLocation: GeoLocation;
} | null;

export type BeneficiaryLocationInput = {
  state?: string | null;
  district?: string | null;
  tehsil?: string | null;
  village?: string | null;
  geoLocation?: GeoLocationInput;
} | null;

export type Installation = {
  id: string;
  uuid: string;
  deviceId: string;
  deviceUuid: string;
  imei: string;
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  serverVendorId: string;
  solarPumpVendorId: string;
  protocolVersionId: string;
  vfdDriveModelId: string | null;
  status: InstallationStatus;
  metadata: Record<string, unknown> | null;
  notes: string | null;
  geoLocation: GeoLocation;
  activatedAt: string | null;
  decommissionedAt: string | null;
  createdAt: string;
  updatedAt: string;
  beneficiaryCount: number;
};

export type Beneficiary = {
  id: string;
  uuid: string;
  name: string;
  email: string | null;
  phoneNumber: string | null;
  address: string | null;
  contacts: BeneficiaryContact[];
  location: BeneficiaryLocation;
  metadata: Record<string, unknown> | null;
  accountStatus: BeneficiaryAccountStatus;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
};

export type InstallationAssignment = {
  id: string;
  installationId: string;
  installationUuid: string;
  beneficiaryId: string;
  beneficiaryUuid: string;
  role: 'owner' | 'secondary';
  assignmentStatus: AssignmentStatus;
  createdAt: string;
  updatedAt: string;
  removedAt: string | null;
  beneficiary: Beneficiary;
};

export type ListInstallationsParams = {
  stateId?: string;
  stateAuthorityId?: string;
  projectId?: string;
  deviceUuid?: string;
  status?: InstallationStatus;
  search?: string;
};

export type ListBeneficiariesParams = {
  search?: string;
  accountStatus?: BeneficiaryAccountStatus;
  installationUuid?: string;
  includeSoftDeleted?: boolean;
  limit?: number;
};

export type CreateBeneficiaryPayload = {
  name: string;
  email?: string | null;
  phoneNumber?: string | null;
  address?: string | null;
  contacts?: BeneficiaryContactInput[];
  location?: BeneficiaryLocationInput;
  metadata?: Record<string, unknown> | null;
  accountStatus?: BeneficiaryAccountStatus;
};

export type UpdateBeneficiaryPayload = {
  name?: string;
  email?: string | null;
  phoneNumber?: string | null;
  address?: string | null;
  contacts?: BeneficiaryContactInput[];
  location?: BeneficiaryLocationInput | null;
  metadata?: Record<string, unknown> | null;
  accountStatus?: BeneficiaryAccountStatus;
  deleted?: boolean;
};

export async function fetchInstallations(
  params?: ListInstallationsParams,
): Promise<Installation[]> {
  const searchParams = new URLSearchParams();

  if (params?.stateId) {
    searchParams.set('stateId', params.stateId);
  }
  if (params?.stateAuthorityId) {
    searchParams.set('stateAuthorityId', params.stateAuthorityId);
  }
  if (params?.projectId) {
    searchParams.set('projectId', params.projectId);
  }
  if (params?.deviceUuid) {
    searchParams.set('deviceUuid', params.deviceUuid);
  }
  if (params?.status) {
    searchParams.set('status', params.status);
  }
  if (params?.search) {
    searchParams.set('search', params.search);
  }

  const query = searchParams.toString();
  const url = query ? `${API_BASE_URL}/installations?${query}` : `${API_BASE_URL}/installations`;

  const response = await apiFetch(url);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load installations';
    throw new Error(message);
  }

  return (responseBody?.installations ?? []) as Installation[];
}

export async function fetchInstallation(installationUuid: string): Promise<Installation> {
  const response = await apiFetch(`${API_BASE_URL}/installations/${installationUuid}`);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load installation';
    throw new Error(message);
  }

  return responseBody.installation as Installation;
}

export async function fetchInstallationAssignments(
  installationUuid: string,
  includeRemoved: boolean = false,
): Promise<InstallationAssignment[]> {
  const searchParams = new URLSearchParams();
  if (includeRemoved) {
    searchParams.set('includeRemoved', String(includeRemoved));
  }

  const query = searchParams.toString();
  const url = query
    ? `${API_BASE_URL}/installations/${installationUuid}/beneficiaries?${query}`
    : `${API_BASE_URL}/installations/${installationUuid}/beneficiaries`;

  const response = await apiFetch(url);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load installation beneficiaries';
    throw new Error(message);
  }

  return (responseBody?.assignments ?? []) as InstallationAssignment[];
}

export async function assignBeneficiaryToInstallation(
  installationUuid: string,
  payload: { beneficiaryUuid: string; role?: 'owner' | 'secondary' },
): Promise<InstallationAssignment> {
  const response = await apiFetch(
    `${API_BASE_URL}/installations/${installationUuid}/beneficiaries`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    },
  );

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to assign beneficiary';
    throw new Error(message);
  }

  return responseBody.assignment as InstallationAssignment;
}

export async function removeBeneficiaryAssignment(
  installationUuid: string,
  beneficiaryUuid: string,
): Promise<InstallationAssignment> {
  const response = await apiFetch(
    `${API_BASE_URL}/installations/${installationUuid}/beneficiaries/${beneficiaryUuid}`,
    {
      method: 'DELETE',
    },
  );

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to remove beneficiary assignment';
    throw new Error(message);
  }

  return responseBody.assignment as InstallationAssignment;
}

export async function fetchBeneficiaries(params?: ListBeneficiariesParams): Promise<Beneficiary[]> {
  const searchParams = new URLSearchParams();

  if (params?.search) {
    searchParams.set('search', params.search);
  }
  if (params?.accountStatus) {
    searchParams.set('accountStatus', params.accountStatus);
  }
  if (params?.installationUuid) {
    searchParams.set('installationUuid', params.installationUuid);
  }
  if (params?.includeSoftDeleted !== undefined) {
    searchParams.set('includeSoftDeleted', String(params.includeSoftDeleted));
  }
  if (params?.limit !== undefined) {
    searchParams.set('limit', String(params.limit));
  }

  const query = searchParams.toString();
  const url = query ? `${API_BASE_URL}/beneficiaries?${query}` : `${API_BASE_URL}/beneficiaries`;

  const response = await apiFetch(url);
  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    const message =
      (responseBody && (responseBody.message ?? responseBody.error?.message)) ||
      'Unable to load beneficiaries';
    throw new Error(message);
  }

  return (responseBody?.beneficiaries ?? []) as Beneficiary[];
}

export async function createBeneficiary(payload: CreateBeneficiaryPayload): Promise<Beneficiary> {
  const response = await apiFetch(`${API_BASE_URL}/beneficiaries`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    throw buildApiError(response, responseBody, 'Unable to create beneficiary');
  }

  return responseBody.beneficiary as Beneficiary;
}

export async function updateBeneficiary(
  beneficiaryUuid: string,
  payload: UpdateBeneficiaryPayload,
): Promise<Beneficiary> {
  const response = await apiFetch(`${API_BASE_URL}/beneficiaries/${beneficiaryUuid}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const responseBody = await readJsonBody<any>(response);

  if (!response.ok) {
    throw buildApiError(response, responseBody, 'Unable to update beneficiary');
  }

  return responseBody.beneficiary as Beneficiary;
}
