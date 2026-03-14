import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type StateOption = {
  id: string;
  name: string;
  isoCode: string | null;
  authorityCount: number;
};

export type AuthorityOption = {
  id: string;
  name: string;
  stateId: string;
  projectCount: number;
};

export type ProtocolVersionOption = {
  id: string;
  version: string;
  serverVendorId: string;
  serverVendorName: string | null;
  governmentCredentialDefaults?: GovernmentCredentialDefaults;
};

export type ProjectOption = {
  id: string;
  name: string;
  authorityId: string;
  protocolVersions: ProtocolVersionOption[];
};

export type GovernmentCredentialDefaults = {
  endpoints: Array<{
    protocol: 'mqtt' | 'mqtts';
    host: string;
    port: number;
    url: string;
  }>;
  topics: {
    publish: string[];
    subscribe: string[];
  };
};

export async function fetchStates(): Promise<StateOption[]> {
  const response = await apiFetch(`${API_BASE_URL}/lookup/states`);
  if (!response.ok) {
    throw new Error('Unable to load states');
  }

  const body = await readJsonBody<{ states: StateOption[] }>(response);
  return body?.states ?? [];
}

export async function fetchAuthorities(stateId: string): Promise<AuthorityOption[]> {
  const response = await apiFetch(
    `${API_BASE_URL}/lookup/authorities?stateId=${encodeURIComponent(stateId)}`,
  );
  if (!response.ok) {
    const body = await readJsonBody<{ message?: string }>(response);
    const message = body?.message ?? 'Unable to load authorities';
    throw new Error(message);
  }

  const body = await readJsonBody<{ authorities: AuthorityOption[] }>(response);
  return body?.authorities ?? [];
}

export async function fetchProjects(params: {
  stateId: string;
  stateAuthorityId: string;
}): Promise<ProjectOption[]> {
  const query = new URLSearchParams({
    stateId: params.stateId,
    stateAuthorityId: params.stateAuthorityId,
  });
  const response = await apiFetch(`${API_BASE_URL}/lookup/projects?${query.toString()}`);
  if (!response.ok) {
    const body = await readJsonBody<{ message?: string }>(response);
    const message = body?.message ?? 'Unable to load projects';
    throw new Error(message);
  }

  const body = await readJsonBody<{ projects: ProjectOption[] }>(response);
  return body?.projects ?? [];
}
