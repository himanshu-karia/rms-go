import { ChangeEvent, FormEvent, useCallback, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useAuth } from '../../../auth';
import { IconActionButton } from '../../../components/IconActionButton';
import { fetchAuthorities, fetchProjects, fetchStates } from '../../../api/lookups';
import type { AuthorityOption, ProjectOption, StateOption } from '../../../api/lookups';
import {
  addGroupMember,
  createUserGroup,
  deleteUserGroup,
  fetchGroupMembers,
  fetchUserGroups,
  removeGroupMember,
  updateUserGroup,
  type AddGroupMemberPayload,
  type GroupMemberSummary,
  type ListUserGroupsParams,
  type UserGroupSummary,
} from '../../../api/userGroups';
import {
  fetchUserRoles,
  listUsers,
  type ListUsersParams,
  type UserRoleDefinition,
  type UserSummary,
} from '../../../api/users';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';
import { getMetadataPreview, parseMetadata, stringifyMetadata } from '../utils/metadata';
import { AdminKpiGrid, type AdminKpiItem } from '../components/AdminKpiGrid';
import { deriveStatus } from '../utils/status';

const emptyFormState = {
  name: '',
  description: '',
  metadata: '',
  defaultRoleIds: [] as string[],
  scopeStateId: '',
  scopeAuthorityId: '',
  scopeProjectId: '',
};

type GroupFormState = typeof emptyFormState;

type MembershipSearchState = {
  input: string;
  submitted: string;
};

const initialMembershipSearch: MembershipSearchState = {
  input: '',
  submitted: '',
};

export function UserGroupsSection() {
  const headingId = 'admin-user-groups-heading';
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();

  const hasUsersManage = hasCapability('users:manage');
  const hasAdminAll = hasCapability('admin:all');
  const canManageGroups = hasUsersManage || hasAdminAll;
  const canDeleteGroups = hasAdminAll;
  const canSearchUsers = hasUsersManage || hasAdminAll;

  const [filters, setFilters] = useState<{
    stateId: string;
    authorityId: string;
    projectId: string;
  }>({ stateId: '', authorityId: '', projectId: '' });
  const [form, setForm] = useState<GroupFormState>(emptyFormState);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [membershipStatus, setMembershipStatus] = useState<AdminStatusMessage | null>(null);
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  const [membershipSearch, setMembershipSearch] =
    useState<MembershipSearchState>(initialMembershipSearch);
  const [removingUserId, setRemovingUserId] = useState<string | null>(null);
  const [addingUserId, setAddingUserId] = useState<string | null>(null);

  const updateSelectedGroup = useCallback(
    (groupId: string | null) => {
      setSelectedGroupId(groupId);
      setMembershipSearch(() => ({ ...initialMembershipSearch }));
      setMembershipStatus(null);
    },
    [setSelectedGroupId, setMembershipSearch, setMembershipStatus],
  );

  const statesQuery = useQuery<StateOption[], Error>({
    queryKey: ['lookup', 'states'],
    queryFn: fetchStates,
  });

  const authoritiesQuery = useQuery<AuthorityOption[], Error>({
    queryKey: ['lookup', 'authorities', filters.stateId],
    queryFn: () => fetchAuthorities(filters.stateId),
    enabled: Boolean(filters.stateId),
  });

  const projectsQuery = useQuery<ProjectOption[], Error>({
    queryKey: ['lookup', 'projects', filters.stateId, filters.authorityId],
    queryFn: () =>
      fetchProjects({ stateId: filters.stateId, stateAuthorityId: filters.authorityId }),
    enabled: Boolean(filters.stateId && filters.authorityId),
  });

  const formAuthoritiesQuery = useQuery<AuthorityOption[], Error>({
    queryKey: ['lookup', 'authorities', form.scopeStateId, 'form'],
    queryFn: () => fetchAuthorities(form.scopeStateId),
    enabled: Boolean(form.scopeStateId),
  });

  const formProjectsQuery = useQuery<ProjectOption[], Error>({
    queryKey: ['lookup', 'projects', form.scopeStateId, form.scopeAuthorityId, 'form'],
    queryFn: () =>
      fetchProjects({ stateId: form.scopeStateId, stateAuthorityId: form.scopeAuthorityId }),
    enabled: Boolean(form.scopeStateId && form.scopeAuthorityId),
  });

  const rolesQuery = useQuery<UserRoleDefinition[], Error>({
    queryKey: ['admin', 'user-roles'],
    queryFn: fetchUserRoles,
  });

  const groupFilters = useMemo<ListUserGroupsParams>(
    () => ({
      stateId: filters.stateId || undefined,
      authorityId: filters.authorityId || undefined,
      projectId: filters.projectId || undefined,
    }),
    [filters],
  );

  const groupsQueryKey = useMemo(
    () => ['admin', 'user-groups', groupFilters] as const,
    [groupFilters],
  );

  const groupsQuery = useQuery({
    queryKey: groupsQueryKey,
    queryFn: () => fetchUserGroups(groupFilters),
  });

  const groups = useMemo(() => groupsQuery.data?.groups ?? [], [groupsQuery.data?.groups]);
  const selectedGroup = selectedGroupId
    ? (groups.find((group) => group.id === selectedGroupId) ?? null)
    : null;
  const activeGroupId = selectedGroup?.id ?? null;

  const groupMetrics = useMemo(() => {
    if (!groups.length) {
      return { total: 0, active: 0, inactive: 0 };
    }

    let active = 0;
    let inactive = 0;

    for (const group of groups) {
      const status = deriveStatus(group.metadata);
      if (status === 'active') {
        active += 1;
      } else if (status === 'inactive') {
        inactive += 1;
      }
    }

    return { total: groups.length, active, inactive };
  }, [groups]);

  const groupKpis: AdminKpiItem[] = useMemo(
    () => [
      { id: 'total-groups', label: 'Total User Groups', value: groupMetrics.total },
      { id: 'active-groups', label: 'Active Groups', value: groupMetrics.active },
      { id: 'inactive-groups', label: 'Inactive Groups', value: groupMetrics.inactive },
    ],
    [groupMetrics],
  );

  const kpiLoading = groupsQuery.isLoading;

  const membersQuery = useQuery<GroupMemberSummary[], Error>({
    queryKey: ['admin', 'user-groups', 'members', activeGroupId],
    queryFn: () => fetchGroupMembers(activeGroupId as string),
    enabled: Boolean(activeGroupId),
  });

  const memberSearchParams: ListUsersParams | null = useMemo(() => {
    if (!activeGroupId || !membershipSearch.submitted.trim() || !canSearchUsers) {
      return null;
    }
    return {
      query: membershipSearch.submitted.trim(),
      limit: 20,
    };
  }, [activeGroupId, membershipSearch.submitted, canSearchUsers]);

  const memberSearchQuery = useQuery({
    queryKey: ['admin', 'user-groups', 'member-search', activeGroupId, memberSearchParams],
    queryFn: () => listUsers(memberSearchParams as ListUsersParams),
    enabled: Boolean(memberSearchParams),
  });

  const memberSearchError =
    memberSearchQuery.error instanceof Error
      ? memberSearchQuery.error
      : memberSearchQuery.error
        ? new Error('Unable to search users')
        : null;

  const createGroupMutation = useMutation({
    mutationFn: createUserGroup,
    onSuccess: (group) => {
      setStatus({ type: 'success', message: `Group "${group.name}" created.` });
      resetForm();
      queryClient.invalidateQueries({ queryKey: groupsQueryKey });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message || 'Unable to create group.' });
    },
  });

  const updateGroupMutation = useMutation({
    mutationFn: ({
      groupId,
      payload,
    }: {
      groupId: string;
      payload: Parameters<typeof updateUserGroup>[1];
    }) => updateUserGroup(groupId, payload),
    onSuccess: (group) => {
      setStatus({ type: 'success', message: `Group "${group.name}" updated.` });
      resetForm();
      queryClient.invalidateQueries({ queryKey: groupsQueryKey });
      if (selectedGroupId === group.id) {
        queryClient.invalidateQueries({ queryKey: ['admin', 'user-groups', 'members', group.id] });
      }
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message || 'Unable to update group.' });
    },
  });

  const deleteGroupMutation = useMutation({
    mutationFn: ({ groupId }: { groupId: string }) => deleteUserGroup(groupId),
  });

  const addMemberMutation = useMutation({
    mutationFn: ({ groupId, payload }: { groupId: string; payload: AddGroupMemberPayload }) =>
      addGroupMember(groupId, payload),
    onMutate: ({ payload }) => {
      setAddingUserId(payload.userId);
    },
    onSuccess: (_membership, variables) => {
      setMembershipStatus({ type: 'success', message: 'User added to group.' });
      queryClient.invalidateQueries({
        queryKey: ['admin', 'user-groups', 'members', variables.groupId],
      });
    },
    onError: (error: Error) => {
      setMembershipStatus({
        type: 'error',
        message: error.message || 'Unable to add user to group.',
      });
    },
    onSettled: () => {
      setAddingUserId(null);
    },
  });

  const removeMemberMutation = useMutation({
    mutationFn: ({ groupId, userId }: { groupId: string; userId: string }) =>
      removeGroupMember(groupId, userId),
    onMutate: ({ userId }) => {
      setRemovingUserId(userId);
    },
    onSuccess: (_membership, variables) => {
      setMembershipStatus({ type: 'success', message: 'User removed from group.' });
      queryClient.invalidateQueries({
        queryKey: ['admin', 'user-groups', 'members', variables.groupId],
      });
    },
    onError: (error: Error) => {
      setMembershipStatus({
        type: 'error',
        message: error.message || 'Unable to remove user from group.',
      });
    },
    onSettled: () => {
      setRemovingUserId(null);
    },
  });

  const roleLookup = useMemo(() => {
    const map = new Map<string, UserRoleDefinition>();
    if (rolesQuery.data) {
      for (const role of rolesQuery.data) {
        map.set(role.id, role);
      }
    }
    return map;
  }, [rolesQuery.data]);

  function resetForm() {
    setForm(emptyFormState);
    setEditingId(null);
    setFormError(null);
  }

  function handleFilterChange(field: 'stateId' | 'authorityId' | 'projectId', value: string): void {
    updateSelectedGroup(null);
    setFilters((current) => {
      const next = { ...current, [field]: value };
      if (field === 'stateId') {
        next.authorityId = '';
        next.projectId = '';
      }
      if (field === 'authorityId') {
        next.projectId = '';
      }
      return next;
    });
  }

  function handleFormScopeChange(
    field: 'scopeStateId' | 'scopeAuthorityId' | 'scopeProjectId',
    value: string,
  ) {
    setForm((current) => {
      const next = { ...current, [field]: value };
      if (field === 'scopeStateId') {
        next.scopeAuthorityId = '';
        next.scopeProjectId = '';
      }
      if (field === 'scopeAuthorityId') {
        next.scopeProjectId = '';
      }
      return next;
    });
  }

  function handleFormSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canManageGroups) {
      setStatus({ type: 'error', message: 'You do not have permission to manage groups.' });
      return;
    }

    if (!form.name.trim()) {
      setFormError('Group name is required.');
      return;
    }

    const scope: { stateId?: string; authorityId?: string; projectId?: string } = {};
    if (form.scopeStateId) {
      scope.stateId = form.scopeStateId;
    }
    if (form.scopeAuthorityId) {
      scope.authorityId = form.scopeAuthorityId;
    }
    if (form.scopeProjectId) {
      scope.projectId = form.scopeProjectId;
    }

    if (!Object.keys(scope).length) {
      setFormError('Select at least one scope value (state, authority, or project).');
      return;
    }

    if (!form.defaultRoleIds.length) {
      setFormError('Select at least one default role.');
      return;
    }

    const { metadata, error } = parseMetadata(form.metadata);
    if (error) {
      setFormError(error);
      return;
    }

    setFormError(null);

    const payload = {
      name: form.name.trim(),
      description: form.description.trim() ? form.description.trim() : null,
      scope,
      defaultRoleIds: form.defaultRoleIds,
      metadata,
    };

    if (editingId) {
      updateGroupMutation.mutate({ groupId: editingId, payload });
    } else {
      createGroupMutation.mutate(payload);
    }
  }

  function handleEdit(group: UserGroupSummary) {
    setEditingId(group.id);
    setForm({
      name: group.name,
      description: group.description ?? '',
      metadata: stringifyMetadata(group.metadata),
      defaultRoleIds: [...group.defaultRoleIds],
      scopeStateId: group.scope.stateId ?? '',
      scopeAuthorityId: group.scope.authorityId ?? '',
      scopeProjectId: group.scope.projectId ?? '',
    });
  }

  function handleManageMembers(group: UserGroupSummary) {
    updateSelectedGroup(group.id);
  }

  function handleDelete(group: UserGroupSummary) {
    if (!canDeleteGroups) {
      setStatus({ type: 'error', message: 'Admin privileges are required to delete groups.' });
      return;
    }
    if (!window.confirm(`Delete group "${group.name}"? This cannot be undone.`)) {
      return;
    }
    deleteGroupMutation.mutate(
      { groupId: group.id },
      {
        onSuccess: () => {
          setStatus({ type: 'success', message: `Group "${group.name}" deleted.` });
          queryClient.invalidateQueries({ queryKey: groupsQueryKey });
          if (selectedGroupId === group.id) {
            updateSelectedGroup(null);
          }
        },
        onError: (error: Error) => {
          setStatus({ type: 'error', message: error.message || 'Unable to delete group.' });
        },
      },
    );
  }

  function handleMemberSearchSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!activeGroupId) {
      return;
    }
    setMembershipStatus(null);
    setMembershipSearch((current) => ({ ...current, submitted: current.input.trim() }));
  }

  function handleAddMember(user: UserSummary) {
    if (!activeGroupId) {
      return;
    }
    if (!canManageGroups) {
      setMembershipStatus({
        type: 'error',
        message: 'You do not have permission to modify members.',
      });
      return;
    }
    addMemberMutation.mutate({ groupId: activeGroupId, payload: { userId: user.id } });
  }

  function handleRemoveMember(member: GroupMemberSummary) {
    if (!activeGroupId) {
      return;
    }
    if (!canManageGroups) {
      setMembershipStatus({
        type: 'error',
        message: 'You do not have permission to modify members.',
      });
      return;
    }
    removeMemberMutation.mutate({ groupId: activeGroupId, userId: member.user.id });
  }

  const isSaving = createGroupMutation.isPending || updateGroupMutation.isPending;
  const isAddingMember = addMemberMutation.isPending;

  return (
    <section
      aria-labelledby={headingId}
      className="space-y-6 rounded border border-slate-200 bg-white p-6 shadow-sm"
    >
      <header className="space-y-2">
        <h2 id={headingId} className="text-xl font-semibold text-slate-900">
          User Groups
        </h2>
        <p className="text-sm text-slate-600">
          Manage user groups and membership assignments tied to hierarchy scope and default roles.
        </p>
      </header>

      <AdminKpiGrid items={groupKpis} isLoading={kpiLoading} />

      {!canManageGroups && (
        <p className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          You have read-only access. Default roles and membership changes require the users:manage
          capability.
        </p>
      )}

      <div className="grid gap-6 lg:grid-cols-[2fr,3fr]">
        <div className="space-y-6">
          <div className="rounded border border-slate-200 p-4">
            <h3 className="text-sm font-semibold text-slate-700">Filter Groups</h3>
            <div className="mt-3 grid gap-3">
              <label className="grid gap-1 text-sm">
                <span className="text-slate-600">State</span>
                <select
                  className="rounded border border-slate-300 px-3 py-2"
                  value={filters.stateId}
                  onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                    handleFilterChange('stateId', event.target.value)
                  }
                >
                  <option value="">All states</option>
                  {(statesQuery.data ?? []).map((state) => (
                    <option key={state.id} value={state.id}>
                      {state.name}
                    </option>
                  ))}
                </select>
              </label>

              <label className="grid gap-1 text-sm">
                <span className="text-slate-600">State Authority</span>
                <select
                  className="rounded border border-slate-300 px-3 py-2"
                  value={filters.authorityId}
                  onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                    handleFilterChange('authorityId', event.target.value)
                  }
                  disabled={!filters.stateId || authoritiesQuery.isLoading}
                >
                  <option value="">All authorities</option>
                  {(authoritiesQuery.data ?? []).map((authority) => (
                    <option key={authority.id} value={authority.id}>
                      {authority.name}
                    </option>
                  ))}
                </select>
              </label>

              <label className="grid gap-1 text-sm">
                <span className="text-slate-600">Project</span>
                <select
                  className="rounded border border-slate-300 px-3 py-2"
                  value={filters.projectId}
                  onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                    handleFilterChange('projectId', event.target.value)
                  }
                  disabled={!filters.authorityId || projectsQuery.isLoading}
                >
                  <option value="">All projects</option>
                  {(projectsQuery.data ?? []).map((project) => (
                    <option key={project.id} value={project.id}>
                      {project.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          </div>

          <div className="rounded border border-slate-200 p-4">
            <h3 className="text-sm font-semibold text-slate-700">
              {editingId ? 'Edit Group' : 'Create Group'}
            </h3>

            <AdminStatusBanner status={status} />

            <form className="grid gap-3" onSubmit={handleFormSubmit}>
              <label className="grid gap-1 text-sm">
                <span className="text-slate-600">Group Name</span>
                <input
                  className="rounded border border-slate-300 px-3 py-2"
                  value={form.name}
                  onChange={(event: ChangeEvent<HTMLInputElement>) =>
                    setForm((current) => ({ ...current, name: event.target.value }))
                  }
                  placeholder="State Control Room"
                  disabled={!canManageGroups}
                />
              </label>

              <label className="grid gap-1 text-sm">
                <span className="text-slate-600">Description</span>
                <input
                  className="rounded border border-slate-300 px-3 py-2"
                  value={form.description}
                  onChange={(event: ChangeEvent<HTMLInputElement>) =>
                    setForm((current) => ({ ...current, description: event.target.value }))
                  }
                  placeholder="Operators managing on-ground teams"
                  disabled={!canManageGroups}
                />
              </label>

              <div className="grid gap-2 rounded border border-slate-200 bg-slate-50 p-3">
                <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                  Scope
                </span>
                <label className="grid gap-1 text-sm">
                  <span className="text-slate-600">State</span>
                  <select
                    className="rounded border border-slate-300 px-3 py-2"
                    value={form.scopeStateId}
                    onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                      handleFormScopeChange('scopeStateId', event.target.value)
                    }
                    disabled={!canManageGroups}
                  >
                    <option value="">Select state</option>
                    {(statesQuery.data ?? []).map((state) => (
                      <option key={state.id} value={state.id}>
                        {state.name}
                      </option>
                    ))}
                  </select>
                </label>

                <label className="grid gap-1 text-sm">
                  <span className="text-slate-600">State Authority (optional)</span>
                  <select
                    className="rounded border border-slate-300 px-3 py-2"
                    value={form.scopeAuthorityId}
                    onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                      handleFormScopeChange('scopeAuthorityId', event.target.value)
                    }
                    disabled={
                      !form.scopeStateId || formAuthoritiesQuery.isLoading || !canManageGroups
                    }
                  >
                    <option value="">None</option>
                    {(formAuthoritiesQuery.data ?? []).map((authority) => (
                      <option key={authority.id} value={authority.id}>
                        {authority.name}
                      </option>
                    ))}
                  </select>
                </label>

                <label className="grid gap-1 text-sm">
                  <span className="text-slate-600">Project (optional)</span>
                  <select
                    className="rounded border border-slate-300 px-3 py-2"
                    value={form.scopeProjectId}
                    onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                      handleFormScopeChange('scopeProjectId', event.target.value)
                    }
                    disabled={
                      !form.scopeAuthorityId || formProjectsQuery.isLoading || !canManageGroups
                    }
                  >
                    <option value="">None</option>
                    {(formProjectsQuery.data ?? []).map((project) => (
                      <option key={project.id} value={project.id}>
                        {project.name}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              <label className="grid gap-1 text-sm">
                <span className="text-slate-600">Default Roles</span>
                <select
                  multiple
                  className="rounded border border-slate-300 px-3 py-2"
                  value={form.defaultRoleIds}
                  onChange={(event: ChangeEvent<HTMLSelectElement>) => {
                    const values = Array.from(event.target.selectedOptions).map(
                      (option) => option.value,
                    );
                    setForm((current) => ({ ...current, defaultRoleIds: values }));
                  }}
                  disabled={!canManageGroups || rolesQuery.isLoading}
                >
                  {(rolesQuery.data ?? []).map((role) => (
                    <option key={role.id} value={role.id}>
                      {role.name} ({role.key})
                    </option>
                  ))}
                </select>
                <span className="text-xs text-slate-500">
                  New members inherit these managed role bindings.
                </span>
              </label>

              <label className="grid gap-1 text-sm">
                <span className="text-slate-600">Metadata JSON (optional)</span>
                <textarea
                  className="rounded border border-slate-300 px-3 py-2"
                  rows={3}
                  value={form.metadata}
                  onChange={(event: ChangeEvent<HTMLTextAreaElement>) =>
                    setForm((current) => ({ ...current, metadata: event.target.value }))
                  }
                  placeholder='{"dashboard": "ops"}'
                  disabled={!canManageGroups}
                />
              </label>

              {formError && <p className="text-sm text-red-600">{formError}</p>}

              <div className="flex gap-2">
                <button
                  type="submit"
                  className="rounded bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-slate-700 disabled:cursor-not-allowed disabled:bg-slate-400"
                  disabled={isSaving || !canManageGroups}
                >
                  {editingId
                    ? isSaving
                      ? 'Updating…'
                      : 'Update Group'
                    : isSaving
                      ? 'Creating…'
                      : 'Create Group'}
                </button>
                {editingId && (
                  <button
                    type="button"
                    className="rounded border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100"
                    onClick={resetForm}
                    disabled={isSaving}
                  >
                    Cancel
                  </button>
                )}
              </div>
            </form>
          </div>
        </div>

        <div className="space-y-6">
          <GroupTable
            groups={groups}
            isLoading={groupsQuery.isLoading}
            roleLookup={roleLookup}
            selectedGroupId={selectedGroupId}
            onEdit={handleEdit}
            onManageMembers={handleManageMembers}
            onDelete={handleDelete}
            canManage={canManageGroups}
            canDelete={canDeleteGroups}
          />

          {selectedGroup && (
            <MembershipPanel
              group={selectedGroup}
              members={membersQuery.data ?? []}
              membersLoading={membersQuery.isLoading}
              memberStatus={membershipStatus}
              searchState={membershipSearch}
              setSearchState={setMembershipSearch}
              onSearchSubmit={handleMemberSearchSubmit}
              searchResults={memberSearchQuery.data?.users ?? []}
              searchError={memberSearchError}
              searchLoading={memberSearchQuery.isLoading}
              canSearchUsers={canSearchUsers}
              onAddMember={handleAddMember}
              onRemoveMember={handleRemoveMember}
              removePendingMemberId={removingUserId}
              addPending={isAddingMember}
              addingUserId={addingUserId}
              roleLookup={roleLookup}
              canManage={canManageGroups}
            />
          )}
        </div>
      </div>
    </section>
  );
}

type GroupTableProps = {
  groups: UserGroupSummary[];
  isLoading: boolean;
  roleLookup: Map<string, UserRoleDefinition>;
  selectedGroupId: string | null;
  onEdit: (group: UserGroupSummary) => void;
  onManageMembers: (group: UserGroupSummary) => void;
  onDelete: (group: UserGroupSummary) => void;
  canManage: boolean;
  canDelete: boolean;
};

function GroupTable({
  groups,
  isLoading,
  roleLookup,
  selectedGroupId,
  onEdit,
  onManageMembers,
  onDelete,
  canManage,
  canDelete,
}: GroupTableProps) {
  if (isLoading) {
    return <p className="text-sm text-slate-600">Loading groups…</p>;
  }

  if (!groups.length) {
    return <p className="text-sm text-slate-600">No user groups found for the selected scope.</p>;
  }

  return (
    <div className="overflow-hidden rounded border border-slate-200">
      <table className="w-full table-fixed divide-y divide-slate-200 text-left text-sm">
        <thead className="bg-slate-50">
          <tr>
            <th className="px-4 py-2 font-semibold text-slate-700">Name</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Scope</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Default Roles</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Metadata</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 bg-white">
          {groups.map((group) => {
            const isActive = selectedGroupId === group.id;
            const defaultRoleLabels = group.defaultRoleIds.map(
              (roleId) => roleLookup.get(roleId)?.name ?? roleId,
            );
            return (
              <tr key={group.id} className={isActive ? 'bg-emerald-50/40' : undefined}>
                <td className="px-4 py-2 font-medium text-slate-800">{group.name}</td>
                <td className="px-4 py-2 text-xs text-slate-600">{formatScope(group.scope)}</td>
                <td className="px-4 py-2 text-xs text-slate-600">
                  {defaultRoleLabels.length ? defaultRoleLabels.join(', ') : '—'}
                </td>
                <td className="px-4 py-2 text-xs text-slate-600">
                  {getMetadataPreview(group.metadata)}
                </td>
                <td className="px-4 py-2">
                  <div className="flex flex-wrap gap-2">
                    <IconActionButton
                      label="Manage"
                      onClick={() => onManageMembers(group)}
                      title={`Manage members in ${group.name}`}
                    />
                    <IconActionButton
                      title="Edit group"
                      label="Edit"
                      onClick={() => onEdit(group)}
                      disabled={!canManage}
                    />
                    <IconActionButton
                      label="Delete"
                      variant="danger"
                      onClick={() => onDelete(group)}
                      disabled={!canDelete}
                    />
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

type MembershipPanelProps = {
  group: UserGroupSummary;
  members: GroupMemberSummary[];
  membersLoading: boolean;
  memberStatus: AdminStatusMessage | null;
  searchState: MembershipSearchState;
  setSearchState: (next: MembershipSearchState) => void;
  onSearchSubmit: (event: FormEvent<HTMLFormElement>) => void;
  searchResults: UserSummary[];
  searchError: Error | null;
  searchLoading: boolean;
  canSearchUsers: boolean;
  onAddMember: (user: UserSummary) => void;
  onRemoveMember: (member: GroupMemberSummary) => void;
  removePendingMemberId: string | null;
  addPending: boolean;
  addingUserId: string | null;
  roleLookup: Map<string, UserRoleDefinition>;
  canManage: boolean;
};

function MembershipPanel({
  group,
  members,
  membersLoading,
  memberStatus,
  searchState,
  setSearchState,
  onSearchSubmit,
  searchResults,
  searchError,
  searchLoading,
  canSearchUsers,
  onAddMember,
  onRemoveMember,
  removePendingMemberId,
  addPending,
  addingUserId,
  roleLookup,
  canManage,
}: MembershipPanelProps) {
  const existingMemberIds = useMemo(
    () => new Set(members.map((member) => member.user.id)),
    [members],
  );

  return (
    <div className="rounded border border-slate-200 p-4">
      <h3 className="text-sm font-semibold text-slate-700">Membership – {group.name}</h3>
      <p className="text-xs text-slate-500">{formatScope(group.scope)}</p>

      <div className="mt-3 space-y-4">
        <AdminStatusBanner status={memberStatus} />

        <div className="space-y-2">
          <h4 className="text-sm font-semibold text-slate-700">Current Members</h4>
          {membersLoading ? (
            <p className="text-sm text-slate-600">Loading members…</p>
          ) : members.length ? (
            <div className="overflow-hidden rounded border border-slate-200">
              <table className="w-full table-fixed divide-y divide-slate-200 text-left text-sm">
                <thead className="bg-slate-50">
                  <tr>
                    <th className="px-3 py-2 font-semibold text-slate-700">User</th>
                    <th className="px-3 py-2 font-semibold text-slate-700">Managed Roles</th>
                    <th className="px-3 py-2 font-semibold text-slate-700">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 bg-white">
                  {members.map((member) => (
                    <tr key={member.user.id}>
                      <td className="px-3 py-2">
                        <div className="text-sm font-medium text-slate-800">
                          {member.user.displayName}
                        </div>
                        <div className="text-xs text-slate-500">{member.user.username}</div>
                      </td>
                      <td className="px-3 py-2 text-xs text-slate-600">
                        {member.managedBindings.length
                          ? member.managedBindings
                              .map(
                                (binding) => roleLookup.get(binding.roleId)?.name ?? binding.roleId,
                              )
                              .join(', ')
                          : '—'}
                      </td>
                      <td className="px-3 py-2">
                        <IconActionButton
                          label="Remove"
                          variant="danger"
                          onClick={() => onRemoveMember(member)}
                          loading={removePendingMemberId === member.user.id}
                          disabled={!canManage}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="text-sm text-slate-600">No members assigned yet.</p>
          )}
        </div>

        {canSearchUsers && canManage ? (
          <div className="space-y-3 rounded border border-slate-200 bg-slate-50 p-3">
            <h4 className="text-sm font-semibold text-slate-700">Add Members</h4>
            <form className="flex flex-col gap-2 md:flex-row" onSubmit={onSearchSubmit}>
              <input
                className="flex-1 rounded border border-slate-300 px-3 py-2"
                value={searchState.input}
                onChange={(event: ChangeEvent<HTMLInputElement>) =>
                  setSearchState({ input: event.target.value, submitted: searchState.submitted })
                }
                placeholder="Search by username or display name"
              />
              <button
                type="submit"
                className="rounded bg-slate-900 px-3 py-2 text-sm font-semibold text-white hover:bg-slate-700 disabled:cursor-not-allowed disabled:bg-slate-400"
                disabled={!searchState.input.trim()}
              >
                Search
              </button>
            </form>

            {searchError && <p className="text-sm text-red-600">{searchError.message}</p>}

            {searchLoading ? (
              <p className="text-sm text-slate-600">Searching users…</p>
            ) : searchResults.length ? (
              <div className="overflow-hidden rounded border border-slate-200">
                <table className="w-full table-fixed divide-y divide-slate-200 text-left text-sm">
                  <thead className="bg-slate-50">
                    <tr>
                      <th className="px-3 py-2 font-semibold text-slate-700">User</th>
                      <th className="px-3 py-2 font-semibold text-slate-700">Status</th>
                      <th className="px-3 py-2 font-semibold text-slate-700">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 bg-white">
                    {searchResults.map((user) => (
                      <tr key={user.id}>
                        <td className="px-3 py-2">
                          <div className="text-sm font-medium text-slate-800">
                            {user.displayName}
                          </div>
                          <div className="text-xs text-slate-500">{user.username}</div>
                        </td>
                        <td className="px-3 py-2 text-xs text-slate-600">{user.status}</td>
                        <td className="px-3 py-2">
                          <IconActionButton
                            label="Add"
                            onClick={() => onAddMember(user)}
                            loading={addPending && addingUserId === user.id}
                            disabled={!canManage || addPending || existingMemberIds.has(user.id)}
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : searchState.submitted ? (
              <p className="text-sm text-slate-600">
                No users matched &quot;{searchState.submitted}&quot;.
              </p>
            ) : null}
          </div>
        ) : (
          <p className="text-xs text-slate-500">
            Additional privileges are required to search users for membership. Contact an
            administrator to manage assignments.
          </p>
        )}
      </div>
    </div>
  );
}

function formatScope(scope: UserGroupSummary['scope']): string {
  const parts: string[] = [];
  if (scope.stateId) {
    parts.push(`State: ${scope.stateId}`);
  }
  if (scope.authorityId) {
    parts.push(`Authority: ${scope.authorityId}`);
  }
  if (scope.projectId) {
    parts.push(`Project: ${scope.projectId}`);
  }
  return parts.length ? parts.join(' • ') : '—';
}
