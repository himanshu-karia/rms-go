import { ChangeEvent, FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  AdminProject,
  AdminState,
  AdminStateAuthority,
  createAdminProject,
  deleteAdminProject,
  fetchAdminProjects,
  fetchAdminStateAuthorities,
  fetchAdminStates,
  updateAdminProject,
} from '../../../api/admin';
import { useAuth } from '../../../auth';
import { IconActionButton } from '../../../components/IconActionButton';
import { useSessionStorage } from '../../../hooks/useSessionStorage';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';
import { getMetadataPreview, parseMetadata, stringifyMetadata } from '../utils/metadata';
import { AdminKpiGrid, type AdminKpiItem } from '../components/AdminKpiGrid';
import { deriveStatus } from '../utils/status';

type ProjectFormState = {
  authorityId: string;
  name: string;
  metadata: string;
};

const emptyForm: ProjectFormState = {
  authorityId: '',
  name: '',
  metadata: '',
};

export function ProjectsSection() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const canManageHierarchy = hasCapability('hierarchy:manage') || hasCapability('admin:all');
  const canDeleteHierarchy = hasCapability('hierarchy:manage') || hasCapability('admin:all');

  const [stateFilter, setStateFilter] = useSessionStorage('admin.projects.stateFilter', '');
  const [authorityFilter, setAuthorityFilter] = useSessionStorage(
    'admin.projects.authorityFilter',
    '',
  );

  const [form, setForm] = useState<ProjectFormState>({
    ...emptyForm,
    authorityId: authorityFilter,
  });
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const statesQuery = useQuery<AdminState[], Error>({
    queryKey: ['admin', 'states'],
    queryFn: fetchAdminStates,
  });

  const authoritiesQuery = useQuery<AdminStateAuthority[], Error>({
    queryKey: ['admin', 'projects', 'state-authorities'],
    queryFn: () => fetchAdminStateAuthorities(),
  });

  const stateOptions = useMemo(() => statesQuery.data ?? [], [statesQuery.data]);

  const authorityOptions = useMemo(() => {
    const authorities = authoritiesQuery.data ?? [];
    if (!stateFilter) {
      return authorities;
    }
    return authorities.filter((authority) => authority.stateId === stateFilter);
  }, [authoritiesQuery.data, stateFilter]);

  const allAuthorities = useMemo(() => authoritiesQuery.data ?? [], [authoritiesQuery.data]);

  const resolvedAuthorityFilter = useMemo(() => {
    if (!authorityFilter) {
      return '';
    }
    return authorityOptions.some((authority) => authority.id === authorityFilter)
      ? authorityFilter
      : '';
  }, [authorityFilter, authorityOptions]);

  const projectsQuery = useQuery<AdminProject[], Error>({
    queryKey: ['admin', 'projects', resolvedAuthorityFilter || 'all'],
    queryFn: () =>
      fetchAdminProjects(
        resolvedAuthorityFilter ? { stateAuthorityId: resolvedAuthorityFilter } : undefined,
      ),
  });

  const projectList = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);

  const projectMetrics = useMemo(() => {
    if (!projectList.length) {
      return { total: 0, active: 0, inactive: 0 };
    }

    let active = 0;
    let inactive = 0;

    for (const project of projectList) {
      const status = deriveStatus(project.metadata);
      if (status === 'active') {
        active += 1;
      } else if (status === 'inactive') {
        inactive += 1;
      }
    }

    return { total: projectList.length, active, inactive };
  }, [projectList]);

  const projectKpis: AdminKpiItem[] = useMemo(
    () => [
      { id: 'total-projects', label: 'Total Projects', value: projectMetrics.total },
      { id: 'active-projects', label: 'Active Projects', value: projectMetrics.active },
      { id: 'inactive-projects', label: 'Inactive Projects', value: projectMetrics.inactive },
    ],
    [projectMetrics],
  );

  const projectKpiLoading = projectsQuery.isLoading;

  const resolvedFormAuthorityId = useMemo(() => {
    if (!form.authorityId) {
      return '';
    }
    return authorityOptions.some((authority) => authority.id === form.authorityId)
      ? form.authorityId
      : '';
  }, [form.authorityId, authorityOptions]);

  const createMutation = useMutation<
    AdminProject,
    Error,
    { authorityId: string; name: string; metadata?: Record<string, unknown> | null }
  >({
    mutationFn: createAdminProject,
    onSuccess: (project) => {
      setStatus({ type: 'success', message: `Project "${project.name}" created.` });
      queryClient.invalidateQueries({ queryKey: ['admin', 'projects'] });
      const authority = allAuthorities.find((item) => item.id === project.authorityId);
      if (authority) {
        setStateFilter(authority.stateId);
      }
      setAuthorityFilter(project.authorityId);
      resetForm(project.authorityId);
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create project.' });
    },
  });

  const updateMutation = useMutation<
    AdminProject,
    Error,
    { projectId: string; payload: { name?: string; metadata?: Record<string, unknown> | null } }
  >({
    mutationFn: ({ projectId, payload }) => updateAdminProject(projectId, payload),
    onSuccess: (project) => {
      setStatus({ type: 'success', message: `Project "${project.name}" updated.` });
      queryClient.invalidateQueries({ queryKey: ['admin', 'projects'] });
      const authority = allAuthorities.find((item) => item.id === project.authorityId);
      if (authority) {
        setStateFilter(authority.stateId);
      }
      setAuthorityFilter(project.authorityId);
      resetForm(project.authorityId);
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to update project.' });
    },
  });

  const deleteMutation = useMutation<void, Error, { projectId: string }>({
    mutationFn: ({ projectId }) => deleteAdminProject(projectId),
  });

  function resetForm(authorityId: string) {
    setForm({ ...emptyForm, authorityId });
    setEditingId(null);
    setFormError(null);
  }

  function handleStateFilterChange(event: ChangeEvent<HTMLSelectElement>) {
    const value = event.target.value;
    setStateFilter(value);
    setAuthorityFilter('');
    setForm((current) => ({ ...current, authorityId: '' }));
    setEditingId(null);
  }

  function handleAuthorityFilterChange(event: ChangeEvent<HTMLSelectElement>) {
    const value = event.target.value;
    setAuthorityFilter(value);
    setForm((current) => ({ ...current, authorityId: value }));
    setEditingId(null);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);
    setStatus(null);

    if (!canManageHierarchy) {
      setFormError('Creating or editing projects requires hierarchy:manage or admin:all capability.');
      return;
    }

    if (!form.name.trim()) {
      setFormError('Project name is required.');
      return;
    }

    if (!resolvedFormAuthorityId) {
      setFormError('Select an authority before saving.');
      return;
    }

    const { metadata, error } = parseMetadata(form.metadata);
    if (error) {
      setFormError(error);
      return;
    }

    const payload = {
      name: form.name.trim(),
      metadata,
    };

    if (editingId) {
      updateMutation.mutate({ projectId: editingId, payload });
      return;
    }

    createMutation.mutate({
      authorityId: resolvedFormAuthorityId,
      name: payload.name,
      metadata,
    });
  }

  function handleEdit(project: AdminProject) {
    if (!canManageHierarchy) {
      setStatus({
        type: 'error',
        message:
          'Creating or editing projects requires the hierarchy:manage capability or super-admin access.',
      });
      return;
    }

    const authority = allAuthorities.find((item) => item.id === project.authorityId);
    if (authority) {
      setStateFilter(authority.stateId);
    }
    setAuthorityFilter(project.authorityId);
    setEditingId(project.id);
    setStatus(null);
    setFormError(null);
    setForm({
      authorityId: project.authorityId,
      name: project.name,
      metadata: stringifyMetadata(project.metadata),
    });
  }

  function handleDelete(project: AdminProject) {
    if (!canDeleteHierarchy) {
      setStatus({
        type: 'error',
        message:
          'Deleting projects requires the hierarchy:manage capability or super-admin access.',
      });
      return;
    }

    if (!window.confirm(`Delete project "${project.name}"?`)) {
      return;
    }

    setDeletingId(project.id);
    setStatus(null);
    deleteMutation.mutate(
      { projectId: project.id },
      {
        onSuccess: () => {
          setStatus({ type: 'success', message: `Project "${project.name}" deleted.` });
          queryClient.invalidateQueries({ queryKey: ['admin', 'projects'] });
          if (editingId === project.id) {
            resetForm('');
          }
        },
        onError: (error) => {
          setStatus({ type: 'error', message: error.message ?? 'Unable to delete project.' });
        },
        onSettled: () => {
          setDeletingId(null);
        },
      },
    );
  }

  const isSaving = createMutation.isPending || updateMutation.isPending;

  const hierarchyWriteDisabledMessage = canManageHierarchy
    ? null
    : 'Read-only mode: hierarchy write actions require hierarchy:manage or admin:all capability.';

  const hierarchyDeleteDisabledMessage = canDeleteHierarchy
    ? null
    : 'Hierarchy deletion requires the hierarchy:manage capability or super-admin access.';

  return (
    <section
      aria-labelledby="admin-projects-heading"
      className="rounded border border-slate-200 bg-white p-6 shadow-sm"
    >
      <header className="mb-4">
        <h2 id="admin-projects-heading" className="text-xl font-semibold text-slate-900">
          Projects
        </h2>
        <p className="text-sm text-slate-600">
          Projects sit under an authority and inherit the state context for provisioning.
        </p>
      </header>

      <div className="mb-5">
        <AdminKpiGrid items={projectKpis} isLoading={projectKpiLoading} />
      </div>

      <AdminStatusBanner status={status} />

      {hierarchyWriteDisabledMessage && (
        <div className="mb-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
          {hierarchyWriteDisabledMessage}
        </div>
      )}

      {hierarchyDeleteDisabledMessage && (
        <div className="mb-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
          {hierarchyDeleteDisabledMessage}
        </div>
      )}

      <div className="mb-4 grid gap-3 md:grid-cols-2 md:items-end">
        <label className="flex-1 text-sm">
          <span className="mb-1 block font-medium text-slate-700">Filter by state</span>
          <select
            className="w-full rounded border border-slate-300 px-3 py-2"
            value={stateFilter}
            onChange={handleStateFilterChange}
          >
            <option value="">All states</option>
            {stateOptions.map((state) => (
              <option key={state.id} value={state.id}>
                {state.name}
              </option>
            ))}
          </select>
        </label>

        <label className="flex-1 text-sm">
          <span className="mb-1 block font-medium text-slate-700">Filter by authority</span>
          <select
            className="w-full rounded border border-slate-300 px-3 py-2"
            value={resolvedAuthorityFilter}
            onChange={handleAuthorityFilterChange}
          >
            <option value="">All authorities</option>
            {authorityOptions.map((authority) => (
              <option key={authority.id} value={authority.id}>
                {authority.name}
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
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              setForm((current) => ({ ...current, name: event.target.value }))
            }
            placeholder="PM_KUSUM_SolarPump_RMS"
          />
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Authority</span>
          <select
            className="rounded border border-slate-300 px-3 py-2"
            value={resolvedFormAuthorityId}
            onChange={(event: ChangeEvent<HTMLSelectElement>) =>
              setForm((current) => ({ ...current, authorityId: event.target.value }))
            }
          >
            <option value="">Select authority…</option>
            {authorityOptions.map((authority) => (
              <option key={authority.id} value={authority.id}>
                {authority.name}
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
            onChange={(event: ChangeEvent<HTMLTextAreaElement>) =>
              setForm((current) => ({ ...current, metadata: event.target.value }))
            }
            placeholder='{"priority": "phase1"}'
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
                : 'Update Project'
              : isSaving
                ? 'Saving…'
                : 'Add Project'}
          </button>
          {editingId && (
            <button
              type="button"
              className="rounded border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100"
              onClick={() => resetForm(resolvedAuthorityFilter)}
              disabled={isSaving}
            >
              Cancel
            </button>
          )}
        </div>
      </form>

      {projectsQuery.error && (
        <p className="mb-4 text-sm text-red-600">{projectsQuery.error.message}</p>
      )}

      <ProjectTable
        projects={projectList}
        authorities={allAuthorities}
        isLoading={projectsQuery.isLoading}
        editingId={editingId}
        deletingId={deletingId}
        canManageHierarchy={canManageHierarchy}
        canDelete={canDeleteHierarchy}
        onEdit={handleEdit}
        onDelete={handleDelete}
      />
    </section>
  );
}

type ProjectTableProps = {
  projects: AdminProject[];
  authorities: AdminStateAuthority[];
  isLoading: boolean;
  editingId: string | null;
  deletingId: string | null;
  canManageHierarchy: boolean;
  canDelete: boolean;
  onEdit: (project: AdminProject) => void;
  onDelete: (project: AdminProject) => void;
};

function ProjectTable({
  projects,
  authorities,
  isLoading,
  editingId,
  deletingId,
  canManageHierarchy,
  canDelete,
  onEdit,
  onDelete,
}: ProjectTableProps) {
  if (isLoading) {
    return <p className="text-sm text-slate-600">Loading projects…</p>;
  }

  if (!projects.length) {
    return <p className="text-sm text-slate-600">No projects registered yet.</p>;
  }

  const authorityLookup = new Map(authorities.map((authority) => [authority.id, authority.name]));

  return (
    <div className="overflow-hidden rounded border border-slate-200">
      <table className="w-full table-fixed divide-y divide-slate-200 text-left text-sm">
        <thead className="bg-slate-50">
          <tr>
            <th className="px-4 py-2 font-semibold text-slate-700">Name</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Authority</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Metadata</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 bg-white">
          {projects.map((project) => {
            const isEditing = editingId === project.id;
            return (
              <tr key={project.id} className={isEditing ? 'bg-emerald-50/40' : undefined}>
                <td className="px-4 py-2">{project.name}</td>
                <td className="px-4 py-2">{authorityLookup.get(project.authorityId) ?? '—'}</td>
                <td className="px-4 py-2 text-xs text-slate-600">
                  {getMetadataPreview(project.metadata)}
                </td>
                <td className="px-4 py-2">
                  <div className="flex gap-2">
                    <IconActionButton
                      label="Edit"
                      onClick={() => onEdit(project)}
                      disabled={!canManageHierarchy}
                      title={
                        canManageHierarchy
                          ? `Edit ${project.name}`
                          : 'Hierarchy edits require hierarchy:manage or admin:all capability.'
                      }
                    />
                    <IconActionButton
                      label="Delete"
                      onClick={() => onDelete(project)}
                      variant="danger"
                      disabled={!canDelete}
                      loading={deletingId === project.id}
                      title={
                        canDelete
                          ? `Delete ${project.name}`
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
