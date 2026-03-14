import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  assignUserRole,
  createUser,
  deleteUser,
  fetchUserRoles,
  listUsers,
  removeUserRole,
  resetUserPassword,
  updateUser,
  type ListUsersResponse,
  type UserRoleDefinition,
  type UserSummary,
} from '../../../api/users';
import { useAuth } from '../../../auth';
import { useActiveProject } from '../../../activeProject';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';

type FiltersState = {
  query: string;
  status: '' | 'active' | 'disabled';
  roleKey: string;
  projectId: string;
};

type CreateUserFormState = {
  username: string;
  password: string;
  email: string;
  phone: string;
  displayName: string;
  status: 'active' | 'disabled';
  mustRotatePassword: boolean;
  roleKey: string;
};

type EditUserFormState = {
  id: string;
  displayName: string;
  status: 'active' | 'disabled';
  mustRotatePassword: boolean;
};

type PasswordFormState = {
  id: string;
  password: string;
  requirePasswordChange: boolean;
};

type RoleAssignFormState = {
  id: string;
  roleKey: string;
  roleType: string;
  scopeProjectId: string;
  scopeAuthorityId: string;
  scopeStateId: string;
};

function coerceUserId(user: UserSummary): string {
  return String(user.id);
}

export function UsersSection() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const { activeProjectId } = useActiveProject();

  const canManage = hasCapability('users:manage') || hasCapability('admin:all');

  const [filtersInput, setFiltersInput] = useState<FiltersState>(() => ({
    query: '',
    status: '',
    roleKey: '',
    projectId: activeProjectId ?? '',
  }));
  const [filtersActive, setFiltersActive] = useState<FiltersState>(() => ({
    query: '',
    status: '',
    roleKey: '',
    projectId: activeProjectId ?? '',
  }));

  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const [createForm, setCreateForm] = useState<CreateUserFormState>(() => ({
    username: '',
    password: '',
    email: '',
    phone: '',
    displayName: '',
    status: 'active',
    mustRotatePassword: true,
    roleKey: '',
  }));
  const [showCreateForm, setShowCreateForm] = useState(false);

  const [editForm, setEditForm] = useState<EditUserFormState | null>(null);
  const [passwordForm, setPasswordForm] = useState<PasswordFormState | null>(null);
  const [roleForm, setRoleForm] = useState<RoleAssignFormState | null>(null);

  const rolesQuery = useQuery<UserRoleDefinition[], Error>({
    queryKey: ['admin', 'users', 'roles'],
    queryFn: fetchUserRoles,
    enabled: canManage,
    refetchOnWindowFocus: false,
  });

  const usersQuery = useQuery<ListUsersResponse, Error>({
    queryKey: ['admin', 'users', filtersActive],
    queryFn: () =>
      listUsers({
        query: filtersActive.query.trim() || undefined,
        status: filtersActive.status || undefined,
        roleKey: filtersActive.roleKey.trim() || undefined,
        projectId: filtersActive.projectId.trim() || undefined,
        limit: 50,
      }),
    enabled: canManage,
    refetchOnWindowFocus: false,
  });

  const users = useMemo(() => usersQuery.data?.users ?? [], [usersQuery.data?.users]);
  const roles = useMemo(() => rolesQuery.data ?? [], [rolesQuery.data]);

  const createMutation = useMutation({
    mutationFn: createUser,
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'User created.' });
      setFormError(null);
      setCreateForm((prev) => ({ ...prev, username: '', password: '' }));
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create user.' });
    },
  });

  const updateMutation = useMutation({
    mutationFn: updateUser,
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'User updated.' });
      setEditForm(null);
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to update user.' });
    },
  });

  const passwordMutation = useMutation({
    mutationFn: resetUserPassword,
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Password reset.' });
      setPasswordForm(null);
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to reset password.' });
    },
  });

  const assignRoleMutation = useMutation({
    mutationFn: assignUserRole,
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Role assigned.' });
      setRoleForm(null);
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to assign role.' });
    },
  });

  const removeRoleMutation = useMutation({
    mutationFn: removeUserRole,
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'Role removed.' });
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to remove role.' });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: async () => {
      setStatus({ type: 'success', message: 'User deleted.' });
      await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
    },
    onError: (error: Error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to delete user.' });
    },
  });

  function applyFilters(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    setFormError(null);
    setFiltersActive(filtersInput);
  }

  function startEdit(user: UserSummary) {
    setStatus(null);
    setFormError(null);
    setPasswordForm(null);
    setRoleForm(null);
    setEditForm({
      id: coerceUserId(user),
      displayName: user.displayName ?? '',
      status: user.status ?? 'active',
      mustRotatePassword: Boolean(user.mustRotatePassword),
    });
  }

  function startPasswordReset(user: UserSummary) {
    setStatus(null);
    setFormError(null);
    setEditForm(null);
    setRoleForm(null);
    setPasswordForm({
      id: coerceUserId(user),
      password: '',
      requirePasswordChange: true,
    });
  }

  function startAssignRole(user: UserSummary) {
    setStatus(null);
    setFormError(null);
    setEditForm(null);
    setPasswordForm(null);
    setRoleForm({
      id: coerceUserId(user),
      roleKey: '',
      roleType: '',
      scopeProjectId: filtersActive.projectId || activeProjectId || '',
      scopeAuthorityId: '',
      scopeStateId: '',
    });
  }

  function submitCreate(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    setFormError(null);

    if (!canManage) {
      setFormError('Requires users:manage capability.');
      return;
    }

    if (!createForm.username.trim() || !createForm.password.trim()) {
      setFormError('Username and password are required.');
      return;
    }

    const email = createForm.email.trim();
    const phone = createForm.phone.trim();
    if (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setFormError('Enter a valid email address.');
      return;
    }
    if (phone && !/^\d{10}$/.test(phone)) {
      setFormError('Phone must be exactly 10 digits.');
      return;
    }

    createMutation.mutate({
      username: createForm.username.trim(),
      password: createForm.password,
      email: email || null,
      phone: phone || null,
      displayName: createForm.displayName.trim() ? createForm.displayName.trim() : null,
      status: createForm.status,
      mustRotatePassword: createForm.mustRotatePassword,
      role: createForm.roleKey.trim() ? createForm.roleKey.trim() : null,
      metadata: phone ? { phone } : {},
    });
  }

  function submitEdit(event: FormEvent) {
    event.preventDefault();
    if (!editForm) return;

    updateMutation.mutate({
      id: editForm.id,
      payload: {
        displayName: editForm.displayName.trim() ? editForm.displayName.trim() : null,
        status: editForm.status,
        mustRotatePassword: editForm.mustRotatePassword,
      },
    });
  }

  function submitPassword(event: FormEvent) {
    event.preventDefault();
    if (!passwordForm) return;

    if (!passwordForm.password.trim()) {
      setFormError('New password is required.');
      return;
    }

    passwordMutation.mutate({
      id: passwordForm.id,
      password: passwordForm.password,
      requirePasswordChange: passwordForm.requirePasswordChange,
    });
  }

  function submitRole(event: FormEvent) {
    event.preventDefault();
    if (!roleForm) return;

    if (!roleForm.roleKey.trim()) {
      setFormError('Role is required.');
      return;
    }

    const scope: Record<string, string> = {};
    if (roleForm.scopeStateId.trim()) scope.stateId = roleForm.scopeStateId.trim();
    if (roleForm.scopeAuthorityId.trim()) scope.authorityId = roleForm.scopeAuthorityId.trim();
    if (roleForm.scopeProjectId.trim()) scope.projectId = roleForm.scopeProjectId.trim();

    assignRoleMutation.mutate({
      id: roleForm.id,
      roleKey: roleForm.roleKey.trim(),
      roleType: roleForm.roleType.trim() || null,
      scope,
    });
  }

  const busy =
    usersQuery.isFetching ||
    createMutation.isPending ||
    updateMutation.isPending ||
    passwordMutation.isPending ||
    assignRoleMutation.isPending ||
    removeRoleMutation.isPending ||
    deleteMutation.isPending;

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <h2 className="text-xl font-semibold text-slate-900">Users</h2>
        <p className="text-sm text-slate-600">Manage user accounts, roles, and password resets.</p>
      </header>

      <AdminStatusBanner status={status} />

      <form onSubmit={applyFilters} className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="grid gap-4 md:grid-cols-4">
          <label className="flex flex-col gap-2 text-sm md:col-span-2">
            <span className="font-medium text-slate-800">Search</span>
            <input
              value={filtersInput.query}
              onChange={(e) => setFiltersInput((p) => ({ ...p, query: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="username or name"
              disabled={!canManage}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Status</span>
            <select
              value={filtersInput.status}
              onChange={(e) => setFiltersInput((p) => ({ ...p, status: e.target.value as FiltersState['status'] }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage}
            >
              <option value="">All</option>
              <option value="active">Active</option>
              <option value="disabled">Disabled</option>
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Role</span>
            <select
              value={filtersInput.roleKey}
              onChange={(e) => setFiltersInput((p) => ({ ...p, roleKey: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage}
            >
              <option value="">All</option>
              {roles.map((r) => (
                <option key={r.key} value={r.key}>
                  {r.name}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-2 text-sm md:col-span-3">
            <span className="font-medium text-slate-800">Project ID (optional)</span>
            <input
              value={filtersInput.projectId}
              onChange={(e) => setFiltersInput((p) => ({ ...p, projectId: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="scope filter"
              disabled={!canManage}
            />
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canManage || usersQuery.isFetching}
            >
              {usersQuery.isFetching ? 'Loading…' : 'Apply'}
            </button>
          </div>
        </div>
      </form>

      {formError ? (
        <p className="text-sm text-rose-600" role="alert">
          {formError}
        </p>
      ) : null}

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Create user</h3>
          <button
            type="button"
            onClick={() => {
              setFormError(null);
              setShowCreateForm((prev) => !prev);
            }}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-700 hover:bg-slate-100"
            disabled={!canManage || busy}
          >
            {showCreateForm ? 'Hide' : 'New user'}
          </button>
        </div>

        {showCreateForm ? (
        <form onSubmit={submitCreate} className="mt-4 grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Username</span>
            <input
              value={createForm.username}
              onChange={(e) => setCreateForm((p) => ({ ...p, username: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Phone (10 digits, optional)</span>
            <input
              value={createForm.phone}
              onChange={(e) => setCreateForm((p) => ({ ...p, phone: e.target.value.replace(/\D/g, '').slice(0, 10) }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || busy}
              placeholder="9876543210"
              inputMode="numeric"
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Password</span>
            <input
              type="password"
              value={createForm.password}
              onChange={(e) => setCreateForm((p) => ({ ...p, password: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Display name</span>
            <input
              value={createForm.displayName}
              onChange={(e) => setCreateForm((p) => ({ ...p, displayName: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Email (optional)</span>
            <input
              value={createForm.email}
              onChange={(e) => setCreateForm((p) => ({ ...p, email: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || busy}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Status</span>
            <select
              value={createForm.status}
              onChange={(e) => setCreateForm((p) => ({ ...p, status: e.target.value as CreateUserFormState['status'] }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || busy}
            >
              <option value="active">Active</option>
              <option value="disabled">Disabled</option>
            </select>
          </label>

          <label className="flex items-center gap-2 text-sm pt-7">
            <input
              type="checkbox"
              checked={createForm.mustRotatePassword}
              onChange={(e) => setCreateForm((p) => ({ ...p, mustRotatePassword: e.target.checked }))}
              className="size-4 rounded border-slate-300"
              disabled={!canManage || busy}
            />
            <span className="text-slate-700">Require password change</span>
          </label>

          <label className="flex flex-col gap-2 text-sm md:col-span-2">
            <span className="font-medium text-slate-800">Initial role (optional)</span>
            <select
              value={createForm.roleKey}
              onChange={(e) => setCreateForm((p) => ({ ...p, roleKey: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canManage || busy}
            >
              <option value="">None</option>
              {roles.map((r) => (
                <option key={r.key} value={r.key}>
                  {r.name}
                </option>
              ))}
            </select>
          </label>

          <div className="flex items-end">
            <button
              type="submit"
              className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canManage || busy}
            >
              {createMutation.isPending ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
        ) : null}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-lg font-semibold text-slate-900">Users</h3>
          <button
            type="button"
            onClick={() => usersQuery.refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
            disabled={!usersQuery.isFetched || usersQuery.isFetching}
          >
            {usersQuery.isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {usersQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {usersQuery.error instanceof Error ? usersQuery.error.message : 'Unable to load users'}
          </p>
        ) : null}

        <p className="mt-2 text-xs text-slate-500">Loaded: {users.length} user(s).</p>

        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-100 text-slate-700">
              <tr>
                <th className="px-3 py-2 text-left text-xs font-semibold">Username</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Display</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Status</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Must rotate</th>
                <th className="px-3 py-2 text-left text-xs font-semibold">Roles</th>
                <th className="px-3 py-2 text-right text-xs font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {users.map((u) => (
                <tr key={u.id}>
                  <td className="px-3 py-2 font-mono text-[0.8rem] text-slate-700">{u.username}</td>
                  <td className="px-3 py-2">{u.displayName}</td>
                  <td className="px-3 py-2">{u.status}</td>
                  <td className="px-3 py-2">{u.mustRotatePassword ? 'Yes' : 'No'}</td>
                  <td className="px-3 py-2 text-xs text-slate-700">
                    {(u.roles ?? []).map((r) => r.roleKey).join(', ') || '—'}
                  </td>
                  <td className="px-3 py-2 text-right">
                    <div className="flex justify-end gap-2">
                      <button
                        type="button"
                        onClick={() => startEdit(u)}
                        className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                        disabled={!canManage || busy}
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        onClick={() => startPasswordReset(u)}
                        className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                        disabled={!canManage || busy}
                      >
                        Password
                      </button>
                      <button
                        type="button"
                        onClick={() => startAssignRole(u)}
                        className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                        disabled={!canManage || busy}
                      >
                        Roles
                      </button>
                      <button
                        type="button"
                        onClick={() => deleteMutation.mutate({ id: u.id })}
                        className="rounded border border-rose-200 bg-rose-50 px-3 py-1 text-xs font-semibold text-rose-700 hover:bg-rose-100 disabled:opacity-60"
                        disabled={!canManage || busy}
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {!users.length ? (
                <tr>
                  <td className="px-3 py-3 text-sm text-slate-500" colSpan={6}>
                    No users found.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        {editForm ? (
          <form onSubmit={submitEdit} className="mt-6 rounded border border-slate-200 bg-slate-50 p-4">
            <div className="flex items-center justify-between gap-3">
              <p className="font-semibold text-slate-900">Edit user</p>
              <button
                type="button"
                onClick={() => setEditForm(null)}
                className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100"
              >
                Close
              </button>
            </div>

            <div className="mt-4 grid gap-4 md:grid-cols-3">
              <label className="flex flex-col gap-2 text-sm md:col-span-2">
                <span className="font-medium text-slate-800">Display name</span>
                <input
                  value={editForm.displayName}
                  onChange={(e) => setEditForm((p) => (p ? { ...p, displayName: e.target.value } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm"
                />
              </label>

              <label className="flex flex-col gap-2 text-sm">
                <span className="font-medium text-slate-800">Status</span>
                <select
                  value={editForm.status}
                  onChange={(e) => setEditForm((p) => (p ? { ...p, status: e.target.value as EditUserFormState['status'] } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm"
                >
                  <option value="active">Active</option>
                  <option value="disabled">Disabled</option>
                </select>
              </label>

              <label className="flex items-center gap-2 text-sm md:col-span-2">
                <input
                  type="checkbox"
                  checked={editForm.mustRotatePassword}
                  onChange={(e) => setEditForm((p) => (p ? { ...p, mustRotatePassword: e.target.checked } : p))}
                  className="size-4 rounded border-slate-300"
                />
                <span className="text-slate-700">Require password change</span>
              </label>

              <div className="flex items-end">
                <button
                  type="submit"
                  className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white"
                  disabled={updateMutation.isPending}
                >
                  {updateMutation.isPending ? 'Saving…' : 'Save'}
                </button>
              </div>
            </div>
          </form>
        ) : null}

        {passwordForm ? (
          <form onSubmit={submitPassword} className="mt-6 rounded border border-slate-200 bg-slate-50 p-4">
            <div className="flex items-center justify-between gap-3">
              <p className="font-semibold text-slate-900">Reset password</p>
              <button
                type="button"
                onClick={() => setPasswordForm(null)}
                className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100"
              >
                Close
              </button>
            </div>

            <div className="mt-4 grid gap-4 md:grid-cols-3">
              <label className="flex flex-col gap-2 text-sm md:col-span-2">
                <span className="font-medium text-slate-800">New password</span>
                <input
                  type="password"
                  value={passwordForm.password}
                  onChange={(e) => setPasswordForm((p) => (p ? { ...p, password: e.target.value } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm"
                />
              </label>

              <label className="flex items-center gap-2 text-sm pt-7">
                <input
                  type="checkbox"
                  checked={passwordForm.requirePasswordChange}
                  onChange={(e) => setPasswordForm((p) => (p ? { ...p, requirePasswordChange: e.target.checked } : p))}
                  className="size-4 rounded border-slate-300"
                />
                <span className="text-slate-700">Require password change</span>
              </label>

              <div className="flex items-end">
                <button
                  type="submit"
                  className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white"
                  disabled={passwordMutation.isPending}
                >
                  {passwordMutation.isPending ? 'Saving…' : 'Reset'}
                </button>
              </div>
            </div>
          </form>
        ) : null}

        {roleForm ? (
          <form onSubmit={submitRole} className="mt-6 rounded border border-slate-200 bg-slate-50 p-4">
            <div className="flex items-center justify-between gap-3">
              <p className="font-semibold text-slate-900">Assign role</p>
              <button
                type="button"
                onClick={() => setRoleForm(null)}
                className="rounded border border-slate-300 bg-white px-3 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100"
              >
                Close
              </button>
            </div>

            <div className="mt-4 grid gap-4 md:grid-cols-4">
              <label className="flex flex-col gap-2 text-sm md:col-span-2">
                <span className="font-medium text-slate-800">Role</span>
                <select
                  value={roleForm.roleKey}
                  onChange={(e) => setRoleForm((p) => (p ? { ...p, roleKey: e.target.value } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700"
                >
                  <option value="">Select a role</option>
                  {roles.map((r) => (
                    <option key={r.key} value={r.key}>
                      {r.name}
                    </option>
                  ))}
                </select>
              </label>

              <label className="flex flex-col gap-2 text-sm">
                <span className="font-medium text-slate-800">Role type (optional)</span>
                <input
                  value={roleForm.roleType}
                  onChange={(e) => setRoleForm((p) => (p ? { ...p, roleType: e.target.value } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700"
                  placeholder=""
                />
              </label>

              <label className="flex flex-col gap-2 text-sm">
                <span className="font-medium text-slate-800">Scope projectId</span>
                <input
                  value={roleForm.scopeProjectId}
                  onChange={(e) => setRoleForm((p) => (p ? { ...p, scopeProjectId: e.target.value } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700"
                  placeholder="optional"
                />
              </label>

              <label className="flex flex-col gap-2 text-sm">
                <span className="font-medium text-slate-800">Scope authorityId</span>
                <input
                  value={roleForm.scopeAuthorityId}
                  onChange={(e) => setRoleForm((p) => (p ? { ...p, scopeAuthorityId: e.target.value } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700"
                  placeholder="optional"
                />
              </label>

              <label className="flex flex-col gap-2 text-sm">
                <span className="font-medium text-slate-800">Scope stateId</span>
                <input
                  value={roleForm.scopeStateId}
                  onChange={(e) => setRoleForm((p) => (p ? { ...p, scopeStateId: e.target.value } : p))}
                  className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700"
                  placeholder="optional"
                />
              </label>

              <div className="flex items-end">
                <button
                  type="submit"
                  className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white"
                  disabled={assignRoleMutation.isPending}
                >
                  {assignRoleMutation.isPending ? 'Assigning…' : 'Assign'}
                </button>
              </div>
            </div>

            <div className="mt-4">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Existing roles</p>
              <div className="mt-2 space-y-2">
                {(() => {
                  const currentRoles = users.find((u) => u.id === roleForm.id)?.roles ?? [];
                  if (!currentRoles.length) {
                    return <p className="text-xs text-slate-500">No roles.</p>;
                  }

                  return currentRoles.map((r) => (
                    <div
                      key={r.bindingId}
                      className="flex items-center justify-between rounded border border-slate-200 bg-white px-3 py-2"
                    >
                      <div className="text-sm">
                        <p className="font-medium text-slate-800">{r.roleName}</p>
                        <p className="text-xs text-slate-500">{r.roleKey}</p>
                      </div>
                      <button
                        type="button"
                        onClick={() => removeRoleMutation.mutate({ id: roleForm.id, bindingId: r.bindingId })}
                        className="rounded border border-rose-200 bg-rose-50 px-3 py-1 text-xs font-semibold text-rose-700 hover:bg-rose-100"
                        disabled={removeRoleMutation.isPending}
                      >
                        Remove
                      </button>
                    </div>
                  ));
                })()}
              </div>
            </div>
          </form>
        ) : null}
      </section>
    </section>
  );
}
