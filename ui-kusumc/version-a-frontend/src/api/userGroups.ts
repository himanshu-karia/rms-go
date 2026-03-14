import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';
import type { UserSummary } from './users';

export type GroupMemberSummary = {
  user: UserSummary;
  managedBindings: Array<{ roleId: string; bindingId: string }>;
  addedBy: string | null;
  createdAt: string;
  updatedAt: string;
};

export type UserGroupSummary = {
  id: string;
  name: string;
  description?: string | null;
  scope: {
    stateId?: string;
    authorityId?: string;
    projectId?: string;
  };
  defaultRoleIds: string[];
  metadata: Record<string, unknown> | null;
  createdAt: string;
  updatedAt: string;
};

export type ListUserGroupsParams = {
  stateId?: string;
  authorityId?: string;
  projectId?: string;
};

export type ListUserGroupsResponse = {
  groups: UserGroupSummary[];
  nextCursor: string | null;
};

export type CreateUserGroupPayload = {
  name: string;
  description?: string | null;
  scope: {
    stateId?: string;
    authorityId?: string;
    projectId?: string;
  };
  defaultRoleIds: string[];
  metadata?: Record<string, unknown> | null;
};

export type UpdateUserGroupPayload = {
  name?: string;
  description?: string | null;
  defaultRoleIds?: string[];
  metadata?: Record<string, unknown> | null;
};

export type AddGroupMemberPayload = {
  userId: string;
};

function buildGroupsUrl(path = '', search?: Record<string, string | undefined>) {
  const url = new URL(`${API_BASE_URL}/user-groups${path}`);
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

async function parseResponse<T>(response: Response, fallbackMessage: string): Promise<T> {
  const body = await readJsonBody<any>(response);
  if (!response.ok) {
    const message = (body as { message?: string } | null)?.message ?? fallbackMessage;
    throw new Error(message);
  }
  return body as T;
}

async function ensureEmptyResponse(response: Response, fallbackMessage: string): Promise<void> {
  if (response.ok) {
    return;
  }
  const body = await readJsonBody<any>(response);
  const message = (body as { message?: string } | null)?.message ?? fallbackMessage;
  throw new Error(message);
}

export async function fetchUserGroups(
  params: ListUserGroupsParams = {},
): Promise<ListUserGroupsResponse> {
  const url = buildGroupsUrl('', {
    stateId: params.stateId,
    authorityId: params.authorityId,
    projectId: params.projectId,
  });
  const response = await apiFetch(url);
  return parseResponse<ListUserGroupsResponse>(response, 'Unable to load user groups');
}

export async function createUserGroup(payload: CreateUserGroupPayload): Promise<UserGroupSummary> {
  const response = await apiFetch(buildGroupsUrl(), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<{ group: UserGroupSummary }>(
    response,
    'Unable to create user group',
  );
  return body.group;
}

export async function updateUserGroup(
  groupId: string,
  payload: UpdateUserGroupPayload,
): Promise<UserGroupSummary> {
  const response = await apiFetch(buildGroupsUrl(`/${groupId}`), {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<{ group: UserGroupSummary }>(
    response,
    'Unable to update user group',
  );
  return body.group;
}

export async function deleteUserGroup(groupId: string): Promise<void> {
  const response = await apiFetch(buildGroupsUrl(`/${groupId}`), {
    method: 'DELETE',
  });
  await ensureEmptyResponse(response, 'Unable to delete user group');
}

export async function fetchGroupMembers(groupId: string): Promise<GroupMemberSummary[]> {
  const response = await apiFetch(buildGroupsUrl(`/${groupId}/members`));
  const body = await parseResponse<{ members: GroupMemberSummary[] }>(
    response,
    'Unable to load group members',
  );
  return body.members;
}

export async function addGroupMember(
  groupId: string,
  payload: AddGroupMemberPayload,
): Promise<GroupMemberSummary> {
  const response = await apiFetch(buildGroupsUrl(`/${groupId}/members`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const body = await parseResponse<{ membership: GroupMemberSummary }>(
    response,
    'Unable to add user to group',
  );
  return body.membership;
}

export async function removeGroupMember(
  groupId: string,
  userId: string,
): Promise<GroupMemberSummary> {
  const response = await apiFetch(buildGroupsUrl(`/${groupId}/members/${userId}`), {
    method: 'DELETE',
  });
  const body = await parseResponse<{ membership: GroupMemberSummary }>(
    response,
    'Unable to remove user from group',
  );
  return body.membership;
}
