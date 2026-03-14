import { FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  AdminState,
  AdminStateAuthority,
  AdminProject,
  createAdminStateAuthority,
  deleteAdminStateAuthority,
  fetchAdminStateAuthorities,
  fetchAdminStates,
  fetchAdminProjects,
  updateAdminStateAuthority,
} from '../../../api/admin';
import { useAuth } from '../../../auth';
import { IconActionButton } from '../../../components/IconActionButton';
import { useSessionStorage } from '../../../hooks/useSessionStorage';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';
import { getMetadataPreview, parseMetadata, stringifyMetadata } from '../utils/metadata';
import { AdminKpiGrid, type AdminKpiItem } from '../components/AdminKpiGrid';
import { deriveStatus, isActiveProject } from '../utils/status';

type AuthorityFormState = {
  stateId: string;
  name: string;
  metadata: string;
};

const emptyForm: AuthorityFormState = {
  stateId: '',
  name: '',
  metadata: '',
};

export function AuthoritiesSection() {
  const headingId = 'admin-state-authorities-heading';
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const canManageHierarchy = hasCapability('admin:all') || hasCapability('hierarchy:manage');
  const canDeleteHierarchy = hasCapability('admin:all') || hasCapability('hierarchy:manage');

  const [selectedStateId, setSelectedStateId] = useSessionStorage(
    'admin.authorities.stateFilter',
    '',
  );
  const [form, setForm] = useState<AuthorityFormState>({ ...emptyForm, stateId: selectedStateId });
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const statesQuery = useQuery<AdminState[], Error>({
    queryKey: ['admin', 'states'],
    queryFn: fetchAdminStates,
  });

  const authoritiesQuery = useQuery<AdminStateAuthority[], Error>({
    queryKey: ['admin', 'state-authorities', selectedStateId || 'all'],
    queryFn: () =>
      fetchAdminStateAuthorities(selectedStateId ? { stateId: selectedStateId } : undefined),
  });

  const projectsQuery = useQuery<AdminProject[], Error>({
    queryKey: ['admin', 'projects', 'all-for-authorities'],
    queryFn: () => fetchAdminProjects(),
  });

  const stateOptions = useMemo(() => statesQuery.data ?? [], [statesQuery.data]);
  const authorityList = useMemo(() => authoritiesQuery.data ?? [], [authoritiesQuery.data]);
  const allProjects = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);

  const authorityMetrics = useMemo(() => {
    if (!authorityList.length) {
      return { total: 0, active: 0, inactive: 0 };
    }

    const projectsByAuthority = new Map<string, AdminProject[]>();
    for (const project of allProjects) {
      const bucket = projectsByAuthority.get(project.authorityId) ?? [];
      bucket.push(project);
      projectsByAuthority.set(project.authorityId, bucket);
    }

    let active = 0;
    let inactive = 0;

    for (const authority of authorityList) {
      const projects = projectsByAuthority.get(authority.id) ?? [];
      const hasActiveProject = projects.some((project) => isActiveProject(project));
      if (hasActiveProject) {
        active += 1;
        continue;
      }

      const status = deriveStatus(authority.metadata);
      if (status === 'active') {
        active += 1;
      } else if (status === 'inactive' || projects.length > 0) {
        inactive += 1;
      }
    }

    return { total: authorityList.length, active, inactive };
  }, [authorityList, allProjects]);

  const authorityKpis: AdminKpiItem[] = useMemo(
    () => [
      { id: 'total-authorities', label: 'Total Authorities', value: authorityMetrics.total },
      { id: 'active-authorities', label: 'Active Authorities', value: authorityMetrics.active },
      {
        id: 'inactive-authorities',
        label: 'Inactive Authorities',
        value: authorityMetrics.inactive,
      },
    ],
    [authorityMetrics],
  );

  const kpiLoading = statesQuery.isLoading || authoritiesQuery.isLoading || projectsQuery.isLoading;

  const createMutation = useMutation<
    AdminStateAuthority,
    Error,
    { stateId: string; name: string; metadata?: Record<string, unknown> | null }
  >({
    mutationFn: createAdminStateAuthority,
    onSuccess: (authority) => {
      setStatus({ type: 'success', message: `Authority "${authority.name}" created.` });
      queryClient.invalidateQueries({ queryKey: ['admin', 'state-authorities'] });
      resetForm(authority.stateId);
      setSelectedStateId(authority.stateId);
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create authority.' });
    },
  });

  const updateMutation = useMutation<
    AdminStateAuthority,
    Error,
    {
      stateAuthorityId: string;
      payload: { name?: string; metadata?: Record<string, unknown> | null };
    }
  >({
    mutationFn: ({ stateAuthorityId, payload }) =>
      updateAdminStateAuthority(stateAuthorityId, payload),
    onSuccess: (authority) => {
      setStatus({ type: 'success', message: `Authority "${authority.name}" updated.` });
      queryClient.invalidateQueries({ queryKey: ['admin', 'state-authorities'] });
      resetForm(authority.stateId);
      setSelectedStateId(authority.stateId);
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to update authority.' });
    },
  });

  const deleteMutation = useMutation<void, Error, { stateAuthorityId: string }>({
    mutationFn: ({ stateAuthorityId }) => deleteAdminStateAuthority(stateAuthorityId),
  });

  const isSaving = createMutation.isPending || updateMutation.isPending;

  function resetForm(stateId: string) {
    setForm({ ...emptyForm, stateId });
    setEditingId(null);
    setFormError(null);
  }

  function handleFilterChange(stateId: string) {
    setSelectedStateId(stateId);
    resetForm(stateId);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canManageHierarchy) {
      setFormError('Creating or editing authorities requires hierarchy:manage or admin:all capability.');
      return;
    }
    if (!form.name.trim()) {
      setFormError('Authority name is required.');
      return;
    }
    if (!form.stateId) {
      setFormError('Select a state before saving.');
      return;
    }

    const { metadata, error } = parseMetadata(form.metadata);
    if (error) {
      setFormError(error);
      return;
    }

    setFormError(null);

    if (editingId) {
      updateMutation.mutate({
        stateAuthorityId: editingId,
        payload: {
          name: form.name.trim(),
          metadata,
        },
      });
    } else {
      createMutation.mutate({
        stateId: form.stateId,
        name: form.name.trim(),
        metadata,
      });
    }
  }

  function handleEdit(authority: AdminStateAuthority) {
    setEditingId(authority.id);
    setForm({
      stateId: authority.stateId,
      name: authority.name,
      metadata: stringifyMetadata(authority.metadata),
    });
    setSelectedStateId(authority.stateId);
  }

  function handleDelete(authority: AdminStateAuthority) {
    if (!canDeleteHierarchy) {
      setStatus({
        type: 'error',
        message:
          'Deleting authorities requires the hierarchy:manage capability or super-admin access.',
      });
      return;
    }
    if (!window.confirm(`Delete authority "${authority.name}"?`)) {
      return;
    }
    setDeletingId(authority.id);
    deleteMutation.mutate(
      { stateAuthorityId: authority.id },
      {
        onSuccess: () => {
          setStatus({ type: 'success', message: `Authority "${authority.name}" deleted.` });
          if (editingId === authority.id) {
            resetForm(authority.stateId);
          }
          queryClient.invalidateQueries({ queryKey: ['admin', 'state-authorities'] });
        },
        onError: (error) => {
          setStatus({ type: 'error', message: error.message ?? 'Unable to delete authority.' });
        },
        onSettled: () => {
          setDeletingId(null);
        },
      },
    );
  }

  return (
    <section
      aria-labelledby={headingId}
      className="rounded border border-slate-200 bg-white p-6 shadow-sm"
    >
      <header className="mb-4">
        <h2 id={headingId} className="text-xl font-semibold text-slate-900">
          State Authorities
        </h2>
        <p className="text-sm text-slate-600">
          Each authority is scoped to a state and controls project onboarding.
        </p>
      </header>

      <div className="mb-5">
        <AdminKpiGrid items={authorityKpis} isLoading={kpiLoading} />
      </div>

      <AdminStatusBanner status={status} />

      {!canManageHierarchy && (
        <div className="mb-4 rounded border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          Read-only mode: hierarchy write actions require hierarchy:manage or admin:all capability.
        </div>
      )}

      <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-end">
        <label className="flex-1 text-sm">
          <span className="mb-1 block font-medium text-slate-700">Filter by state</span>
          <select
            className="w-full rounded border border-slate-300 px-3 py-2"
            value={selectedStateId}
            onChange={(event) => handleFilterChange(event.target.value)}
          >
            <option value="">All states</option>
            {stateOptions.map((state) => (
              <option key={state.id} value={state.id}>
                {state.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      <form className="mb-4 grid gap-3 md:grid-cols-2" onSubmit={handleSubmit}>
        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Name</span>
          <input
            className="rounded border border-slate-300 px-3 py-2"
            value={form.name}
            onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
            placeholder="MSEDCL"
          />
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">State</span>
          <select
            className="rounded border border-slate-300 px-3 py-2"
            value={form.stateId}
            onChange={(event) =>
              setForm((current) => ({ ...current, stateId: event.target.value }))
            }
          >
            <option value="">Select state…</option>
            {stateOptions.map((state) => (
              <option key={state.id} value={state.id}>
                {state.name}
              </option>
            ))}
          </select>
        </label>

        <label className="grid gap-1 text-sm md:col-span-2">
          <span className="font-medium text-slate-700">Metadata JSON (optional)</span>
          <textarea
            className="rounded border border-slate-300 px-3 py-2"
            rows={3}
            value={form.metadata}
            onChange={(event) =>
              setForm((current) => ({ ...current, metadata: event.target.value }))
            }
            placeholder='{"division": "Pune"}'
          />
        </label>

        {formError && <p className="text-sm text-red-600">{formError}</p>}

        <div className="flex gap-2 md:col-span-2">
          <button
            type="submit"
            className="rounded bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-slate-700 disabled:cursor-not-allowed disabled:bg-slate-400"
            disabled={isSaving || !canManageHierarchy}
          >
            {editingId
              ? isSaving
                ? 'Updating…'
                : 'Update Authority'
              : isSaving
                ? 'Saving…'
                : 'Add Authority'}
          </button>
          {editingId && (
            <button
              type="button"
              className="rounded border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100"
              onClick={() => resetForm(selectedStateId)}
              disabled={isSaving}
            >
              Cancel
            </button>
          )}
        </div>
      </form>

      {authoritiesQuery.error && (
        <p className="mb-4 text-sm text-red-600">{authoritiesQuery.error.message}</p>
      )}

      <AuthorityTable
        authorities={authorityList}
        states={stateOptions}
        isLoading={authoritiesQuery.isLoading}
        deletingId={deletingId}
        editingId={editingId}
        canDelete={canDeleteHierarchy}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />
    </section>
  );
}

type AuthorityTableProps = {
  authorities: AdminStateAuthority[];
  states: AdminState[];
  isLoading: boolean;
  editingId: string | null;
  deletingId: string | null;
  canDelete: boolean;
  onEdit: (authority: AdminStateAuthority) => void;
  onDelete: (authority: AdminStateAuthority) => void;
};

function AuthorityTable({
  authorities,
  states,
  isLoading,
  editingId,
  deletingId,
  canDelete,
  onEdit,
  onDelete,
}: AuthorityTableProps) {
  if (isLoading) {
    return <p className="text-sm text-slate-600">Loading authorities…</p>;
  }

  if (!authorities.length) {
    return <p className="text-sm text-slate-600">No authorities registered yet.</p>;
  }

  const stateLookup = new Map(states.map((state) => [state.id, state.name]));

  return (
    <div className="overflow-hidden rounded border border-slate-200">
      <table className="w-full table-fixed divide-y divide-slate-200 text-left text-sm">
        <thead className="bg-slate-50">
          <tr>
            <th className="px-4 py-2 font-semibold text-slate-700">Name</th>
            <th className="px-4 py-2 font-semibold text-slate-700">State</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Metadata</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 bg-white">
          {authorities.map((authority) => {
            const isEditing = editingId === authority.id;
            return (
              <tr key={authority.id} className={isEditing ? 'bg-emerald-50/40' : undefined}>
                <td className="px-4 py-2">{authority.name}</td>
                <td className="px-4 py-2">{stateLookup.get(authority.stateId) ?? '—'}</td>
                <td className="px-4 py-2 text-xs text-slate-600">
                  {getMetadataPreview(authority.metadata)}
                </td>
                <td className="px-4 py-2">
                  <div className="flex gap-2">
                    <IconActionButton
                      label="Edit"
                      onClick={() => onEdit(authority)}
                      disabled={!canManageHierarchy}
                      title={`Edit ${authority.name}`}
                    />
                    <IconActionButton
                      label="Delete"
                      onClick={() => onDelete(authority)}
                      variant="danger"
                      disabled={!canDelete}
                      loading={deletingId === authority.id}
                      title={
                        canDelete
                          ? `Delete ${authority.name}`
                          : 'Hierarchy deletion requires the hierarchy:manage capability or super-admin access.'
                      }
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
