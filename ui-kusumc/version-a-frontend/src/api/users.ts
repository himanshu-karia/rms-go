import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';
import type { CapabilityKey } from './capabilities';

export type RoleScopeSummary = {
  stateId?: string;
  authorityId?: string;
  projectId?: string;
} | null;

export type UserRoleSummary = {
  bindingId: string;
  roleId: string;
  roleKey: string;
  roleName: string;
  capabilities: CapabilityKey[];
  scope: RoleScopeSummary;
};

export type UserSummary = {
  id: string;
  username: string;
  displayName: string;
  status: 'active' | 'disabled';
  mustRotatePassword: boolean;
  metadata: Record<string, unknown> | null;
  createdAt: string;
  updatedAt: string;
  roles: UserRoleSummary[];
};

export type UserRoleDefinition = {
  id: string;
  key: string;
  name: string;
  description?: string | null;
  capabilities: CapabilityKey[];
};

export type ListUsersParams = {
  stateId?: string;
  authorityId?: string;
  projectId?: string;
  groupId?: string;
  roleKey?: string;
  status?: 'active' | 'disabled';
  query?: string;
  cursor?: string | null;
  limit?: number;
};

export type ListUsersResponse = {
  users: UserSummary[];
  nextCursor: string | null;
};

export type CreateUserPayload = {
  username: string;
  password: string;
  email?: string | null;
  phone?: string | null;
  displayName?: string | null;
  status?: 'active' | 'disabled';
  mustRotatePassword?: boolean;
  role?: string | null;
  metadata?: Record<string, unknown> | null;
};

export type UpdateUserPayload = {
  displayName?: string | null;
  status?: 'active' | 'disabled';
  mustRotatePassword?: boolean;
  metadata?: Record<string, unknown> | null;
};

export type AssignUserRolePayload = {
  roleKey: string;
  roleType?: string | null;
  scope?: RoleScopeSummary;
};

function buildUsersUrl(path = '', search?: Record<string, string | undefined>) {
  const url = new URL(`${API_BASE_URL}/users${path}`);
  if (search) {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(search)) {
      if (value) {
        params.set(key, value);
      }
    }
    if ([...params.keys()].length) {
      url.search = params.toString();
    }
  }
  return url.toString();
}

function buildAdminUsersUrl(path = '') {
  return new URL(`${API_BASE_URL}/admin/users${path}`).toString();
}

async function parseResponse<T>(response: Response, fallbackMessage: string): Promise<T> {
  const body = await readJsonBody<any>(response);
  if (!response.ok) {
    const message = (body as { message?: string } | null)?.message ?? fallbackMessage;
    throw new Error(message);
  }
  return body as T;
}

export async function fetchUserRoles(): Promise<UserRoleDefinition[]> {
  const response = await apiFetch(buildUsersUrl('/roles'));
  const body = await parseResponse<{ roles: UserRoleDefinition[] }>(
    response,
    'Unable to load roles',
  );
  return body.roles;
}

export async function listUsers(params: ListUsersParams = {}): Promise<ListUsersResponse> {
  const search: Record<string, string | undefined> = {
    stateId: params.stateId,
    authorityId: params.authorityId,
    projectId: params.projectId,
    groupId: params.groupId,
    roleKey: params.roleKey,
    status: params.status,
    query: params.query,
    cursor: params.cursor ?? undefined,
    limit: params.limit ? String(params.limit) : undefined,
  };

  const response = await apiFetch(buildUsersUrl('', search));
  return parseResponse<ListUsersResponse>(response, 'Unable to load users');
}

export async function createUser(payload: CreateUserPayload): Promise<UserSummary> {
  const response = await apiFetch(buildUsersUrl(''), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const body = await parseResponse<{ user: UserSummary }>(response, 'Unable to create user');
  return body.user;
}

export async function updateUser(args: {
  id: string;
  payload: UpdateUserPayload;
}): Promise<UserSummary> {
  const response = await apiFetch(buildUsersUrl(`/${encodeURIComponent(args.id)}`), {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(args.payload),
  });

  const body = await parseResponse<{ user: UserSummary }>(response, 'Unable to update user');
  return body.user;
}

export async function resetUserPassword(args: {
  id: string;
  password: string;
  requirePasswordChange?: boolean;
}): Promise<{ ok: true }> {
  const response = await apiFetch(buildUsersUrl(`/${encodeURIComponent(args.id)}/password`), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      password: args.password,
      requirePasswordChange: args.requirePasswordChange ?? true,
    }),
  });

  return parseResponse<{ ok: true }>(response, 'Unable to reset password');
}

export async function assignUserRole(args: {
  id: string;
  roleKey: string;
  roleType?: string | null;
  scope?: RoleScopeSummary;
}): Promise<{ ok: true }> {
  const payload: AssignUserRolePayload = {
    roleKey: args.roleKey,
    roleType: args.roleType ?? null,
    scope: args.scope ?? null,
  };

  const response = await apiFetch(buildUsersUrl(`/${encodeURIComponent(args.id)}/roles`), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  return parseResponse<{ ok: true }>(response, 'Unable to assign role');
}

export async function removeUserRole(args: {
  id: string;
  bindingId: string;
}): Promise<{ ok: true }> {
  const response = await apiFetch(
    buildUsersUrl(`/${encodeURIComponent(args.id)}/roles/${encodeURIComponent(args.bindingId)}`),
    {
      method: 'DELETE',
    },
  );

  return parseResponse<{ ok: true }>(response, 'Unable to remove role');
}

export async function deleteUser(args: { id: string }): Promise<{ ok: true }> {
  const response = await apiFetch(buildAdminUsersUrl(`/${encodeURIComponent(args.id)}`), {
    method: 'DELETE',
  });

  return parseResponse<{ ok: true }>(response, 'Unable to delete user');
}
