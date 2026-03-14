import { ChangeEvent, FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  AdminState,
  AdminStateAuthority,
  AdminProject,
  createAdminState,
  deleteAdminState,
  fetchAdminStates,
  fetchAdminStateAuthorities,
  fetchAdminProjects,
  updateAdminState,
} from '../../../api/admin';
import { useAuth } from '../../../auth';
import { IconActionButton } from '../../../components/IconActionButton';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';
import { getMetadataPreview, parseMetadata, stringifyMetadata } from '../utils/metadata';
import { AdminKpiGrid, type AdminKpiItem } from '../components/AdminKpiGrid';
import { deriveStatus, isActiveProject } from '../utils/status';

type StateFormState = {
  name: string;
  isoCode: string;
  metadata: string;
};

const emptyForm: StateFormState = {
  name: '',
  isoCode: '',
  metadata: '',
};

export function StatesSection() {
  const headingId = 'admin-states-heading';
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const canManageHierarchy = hasCapability('hierarchy:manage') || hasCapability('admin:all');
  const canDeleteHierarchy = hasCapability('hierarchy:manage') || hasCapability('admin:all');

  const [form, setForm] = useState<StateFormState>(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const statesQuery = useQuery<AdminState[], Error>({
    queryKey: ['admin', 'states'],
    queryFn: fetchAdminStates,
  });

  const authoritiesQuery = useQuery<AdminStateAuthority[], Error>({
    queryKey: ['admin', 'state-authorities', 'all-for-states'],
    queryFn: () => fetchAdminStateAuthorities(),
  });

  const projectsQuery = useQuery<AdminProject[], Error>({
    queryKey: ['admin', 'projects', 'all-for-states'],
    queryFn: () => fetchAdminProjects(),
  });

  const stateList = useMemo(() => statesQuery.data ?? [], [statesQuery.data]);
  const authorityList = useMemo(() => authoritiesQuery.data ?? [], [authoritiesQuery.data]);
  const projectList = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);

  const stateMetrics = useMemo(() => {
    if (!stateList.length) {
      return {
        total: 0,
        active: 0,
        inactive: 0,
        statesWithProjects: 0,
      };
    }

    const authoritiesByState = new Map<string, AdminStateAuthority[]>();
    for (const authority of authorityList) {
      const bucket = authoritiesByState.get(authority.stateId) ?? [];
      bucket.push(authority);
      authoritiesByState.set(authority.stateId, bucket);
    }

    const projectsByAuthority = new Map<string, AdminProject[]>();
    for (const project of projectList) {
      const bucket = projectsByAuthority.get(project.authorityId) ?? [];
      bucket.push(project);
      projectsByAuthority.set(project.authorityId, bucket);
    }

    let activeStates = 0;
    let inactiveStates = 0;
    let statesWithProjects = 0;

    for (const state of stateList) {
      const authorities = authoritiesByState.get(state.id) ?? [];
      const projects: AdminProject[] = [];
      for (const authority of authorities) {
        const list = projectsByAuthority.get(authority.id);
        if (list?.length) {
          projects.push(...list);
        }
      }

      const hasProjects = projects.length > 0;
      const hasActiveProject = projects.some((project) => isActiveProject(project));

      if (hasProjects) {
        statesWithProjects += 1;
      }

      if (hasActiveProject) {
        activeStates += 1;
        continue;
      }

      const stateStatus = deriveStatus(state.metadata);
      if (stateStatus === 'active') {
        activeStates += 1;
      } else if (stateStatus === 'inactive' || hasProjects) {
        inactiveStates += 1;
      }
    }

    return {
      total: stateList.length,
      active: activeStates,
      inactive: inactiveStates,
      statesWithProjects,
    };
  }, [stateList, authorityList, projectList]);

  const stateKpis: AdminKpiItem[] = useMemo(
    () => [
      {
        id: 'total-states',
        label: 'Total States',
        value: stateMetrics.total,
      },
      {
        id: 'active-states',
        label: 'Active States',
        value: stateMetrics.active,
        description:
          stateMetrics.active === 0 && stateMetrics.statesWithProjects === 0
            ? 'No active projects detected for any state.'
            : undefined,
      },
      {
        id: 'inactive-states',
        label: 'Inactive States',
        value: stateMetrics.inactive,
        description:
          stateMetrics.statesWithProjects < stateMetrics.total
            ? `${stateMetrics.total - stateMetrics.statesWithProjects} states have no projects yet.`
            : undefined,
      },
    ],
    [stateMetrics],
  );

  const kpiLoading = statesQuery.isLoading || authoritiesQuery.isLoading || projectsQuery.isLoading;

  const createMutation = useMutation<
    AdminState,
    Error,
    { name: string; isoCode?: string | null; metadata?: Record<string, unknown> | null }
  >({
    mutationFn: createAdminState,
    onSuccess: (created) => {
      setStatus({ type: 'success', message: `State "${created.name}" created.` });
      resetForm();
      queryClient.invalidateQueries({ queryKey: ['admin', 'states'] });
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create state.' });
    },
  });

  const updateMutation = useMutation<
    AdminState,
    Error,
    {
      stateId: string;
      payload: {
        name?: string;
        isoCode?: string | null;
        metadata?: Record<string, unknown> | null;
      };
    }
  >({
    mutationFn: ({ stateId, payload }) => updateAdminState(stateId, payload),
    onSuccess: (updated) => {
      setStatus({ type: 'success', message: `State "${updated.name}" updated.` });
      resetForm();
      queryClient.invalidateQueries({ queryKey: ['admin', 'states'] });
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to update state.' });
    },
  });

  const deleteMutation = useMutation<void, Error, { stateId: string }>({
    mutationFn: ({ stateId }) => deleteAdminState(stateId),
  });

  function resetForm() {
    setForm(emptyForm);
    setEditingId(null);
    setFormError(null);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!canManageHierarchy) {
      setFormError('Creating or editing states requires hierarchy:manage or admin:all capability.');
      return;
    }

    if (!form.name.trim()) {
      setFormError('Name is required.');
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
        stateId: editingId,
        payload: {
          name: form.name.trim(),
          isoCode: form.isoCode.trim() ? form.isoCode.trim() : null,
          metadata,
        },
      });
    } else {
      createMutation.mutate({
        name: form.name.trim(),
        isoCode: form.isoCode.trim() ? form.isoCode.trim() : null,
        metadata,
      });
    }
  }

  function handleEdit(state: AdminState) {
    setEditingId(state.id);
    setForm({
      name: state.name,
      isoCode: state.isoCode ?? '',
      metadata: stringifyMetadata(state.metadata),
    });
  }

  function handleDelete(state: AdminState) {
    if (!canDeleteHierarchy) {
      setStatus({
        type: 'error',
        message: 'Deleting states requires the hierarchy:manage capability or super-admin access.',
      });
      return;
    }
    if (!window.confirm(`Delete state "${state.name}"? This cannot be undone.`)) {
      return;
    }
    setDeletingId(state.id);
    deleteMutation.mutate(
      { stateId: state.id },
      {
        onSuccess: () => {
          setStatus({ type: 'success', message: `State "${state.name}" deleted.` });
          if (editingId === state.id) {
            resetForm();
          }
          queryClient.invalidateQueries({ queryKey: ['admin', 'states'] });
        },
        onError: (error) => {
          setStatus({ type: 'error', message: error.message ?? 'Unable to delete state.' });
        },
        onSettled: () => {
          setDeletingId(null);
        },
      },
    );
  }

  const isSaving = createMutation.isPending || updateMutation.isPending;

  return (
    <section
      aria-labelledby={headingId}
      className="rounded border border-slate-200 bg-white p-6 shadow-sm"
    >
      <header className="mb-4">
        <h2 id={headingId} className="text-xl font-semibold text-slate-900">
          States
        </h2>
        <p className="text-sm text-slate-600">Manage state definitions for PM KUSUM projects.</p>
      </header>

      <div className="mb-5">
        <AdminKpiGrid items={stateKpis} isLoading={kpiLoading} />
      </div>

      <AdminStatusBanner status={status} />

      {!canManageHierarchy && (
        <div className="mb-4 rounded border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          Read-only mode: hierarchy write actions require hierarchy:manage or admin:all capability.
        </div>
      )}

      <form className="mb-4 grid gap-3 md:grid-cols-2" onSubmit={handleSubmit}>
        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Name</span>
          <input
            className="rounded border border-slate-300 px-3 py-2"
            value={form.name}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setForm((current) => ({ ...current, name: event.target.value }))
            }
            placeholder="Maharashtra"
          />
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">ISO Code</span>
          <input
            className="rounded border border-slate-300 px-3 py-2"
            value={form.isoCode}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setForm((current) => ({ ...current, isoCode: event.target.value }))
            }
            placeholder="MH"
          />
        </label>

        <label className="grid gap-1 text-sm md:col-span-2">
          <span className="font-medium text-slate-700">Metadata JSON (optional)</span>
          <textarea
            className="rounded border border-slate-300 px-3 py-2"
            rows={3}
            value={form.metadata}
            onChange={(event: ChangeEvent<HTMLTextAreaElement>) =>
              setForm((current) => ({ ...current, metadata: event.target.value }))
            }
            placeholder='{"loadShedding": "phase1"}'
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
                : 'Update State'
              : isSaving
                ? 'Saving…'
                : 'Add State'}
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

      {statesQuery.error && (
        <p className="mb-4 text-sm text-red-600">{statesQuery.error.message}</p>
      )}

      <StateTable
        states={stateList}
        isLoading={statesQuery.isLoading}
        editingId={editingId}
        deletingId={deletingId}
        canDelete={canDeleteHierarchy}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />
    </section>
  );
}

type StateTableProps = {
  states: AdminState[];
  isLoading: boolean;
  editingId: string | null;
  deletingId: string | null;
  canDelete: boolean;
  onEdit: (state: AdminState) => void;
  onDelete: (state: AdminState) => void;
};

function StateTable({
  states,
  isLoading,
  editingId,
  deletingId,
  canDelete,
  onEdit,
  onDelete,
}: StateTableProps) {
  if (isLoading) {
    return <p className="text-sm text-slate-600">Loading states…</p>;
  }

  if (!states.length) {
    return <p className="text-sm text-slate-600">No states registered yet.</p>;
  }

  return (
    <div className="overflow-hidden rounded border border-slate-200">
      <table className="w-full table-fixed divide-y divide-slate-200 text-left text-sm">
        <thead className="bg-slate-50">
          <tr>
            <th className="px-4 py-2 font-semibold text-slate-700">Name</th>
            <th className="px-4 py-2 font-semibold text-slate-700">ISO Code</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Metadata</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 bg-white">
          {states.map((state) => {
            const isEditing = editingId === state.id;
            return (
              <tr key={state.id} className={isEditing ? 'bg-emerald-50/40' : undefined}>
                <td className="px-4 py-2">{state.name}</td>
                <td className="px-4 py-2">{state.isoCode || '—'}</td>
                <td className="px-4 py-2 text-xs text-slate-600">
                  {getMetadataPreview(state.metadata)}
                </td>
                <td className="px-4 py-2">
                  <div className="flex gap-2">
                    <IconActionButton
                      label="Edit"
                      onClick={() => onEdit(state)}
                      disabled={!canManageHierarchy}
                      title={`Edit ${state.name}`}
                    />
                    <IconActionButton
                      label="Delete"
                      onClick={() => onDelete(state)}
                      variant="danger"
                      disabled={!canDelete}
                      loading={deletingId === state.id}
                      title={
                        canDelete
                          ? `Delete ${state.name}`
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
