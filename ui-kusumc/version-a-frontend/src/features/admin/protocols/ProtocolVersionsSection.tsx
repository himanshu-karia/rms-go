import { ChangeEvent, FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  AdminProject,
  AdminProtocolVersion,
  AdminState,
  AdminStateAuthority,
  AdminVendor,
  createAdminProtocolVersion,
  fetchAdminProjects,
  fetchAdminProtocolVersions,
  fetchAdminStateAuthorities,
  fetchAdminStates,
  fetchAdminVendors,
  updateAdminProtocolVersion,
} from '../../../api/admin';
import { useAuth } from '../../../auth';
import { IconActionButton } from '../../../components/IconActionButton';
import { useSessionStorage } from '../../../hooks/useSessionStorage';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';
import { getMetadataPreview, parseMetadata, stringifyMetadata } from '../utils/metadata';
import { AdminKpiGrid, type AdminKpiItem } from '../components/AdminKpiGrid';

type ProtocolFilterState = {
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  serverVendorId: string;
};

type ProtocolFormState = {
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  serverVendorId: string;
  version: string;
  name: string;
  metadata: string;
};

const emptyFilters: ProtocolFilterState = {
  stateId: '',
  stateAuthorityId: '',
  projectId: '',
  serverVendorId: '',
};

const emptyForm: ProtocolFormState = {
  stateId: '',
  stateAuthorityId: '',
  projectId: '',
  serverVendorId: '',
  version: '',
  name: '',
  metadata: '',
};

function hasValidationWarnings(metadata: Record<string, unknown> | null | undefined): boolean {
  if (!metadata || typeof metadata !== 'object') {
    return false;
  }

  const warnings = (metadata as Record<string, unknown>).validationWarnings;
  if (Array.isArray(warnings)) {
    return warnings.length > 0;
  }
  if (typeof warnings === 'number') {
    return warnings > 0;
  }
  if (typeof warnings === 'string') {
    return warnings.trim().length > 0;
  }
  return false;
}

export function ProtocolVersionsSection() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const canManageHierarchy = hasCapability('admin:all') || hasCapability('hierarchy:manage');

  const [filters, setFilters] = useSessionStorage<ProtocolFilterState>(
    'admin.protocols.filters',
    emptyFilters,
  );
  const [form, setForm] = useState<ProtocolFormState>({ ...emptyForm, ...filters });
  const [status, setStatus] = useState<AdminStatusMessage | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);

  const statesQuery = useQuery<AdminState[], Error>({
    queryKey: ['admin', 'states'],
    queryFn: fetchAdminStates,
  });

  const authoritiesQuery = useQuery<AdminStateAuthority[], Error>({
    queryKey: ['admin', 'protocols', 'state-authorities', filters.stateId || 'all'],
    queryFn: () =>
      fetchAdminStateAuthorities(filters.stateId ? { stateId: filters.stateId } : undefined),
    enabled: Boolean(filters.stateId),
  });

  const projectsQuery = useQuery<AdminProject[], Error>({
    queryKey: ['admin', 'protocols', 'projects', filters.stateAuthorityId || 'all'],
    queryFn: () =>
      fetchAdminProjects(
        filters.stateAuthorityId ? { stateAuthorityId: filters.stateAuthorityId } : undefined,
      ),
    enabled: Boolean(filters.stateAuthorityId),
  });

  const serverVendorsQuery = useQuery<AdminVendor[], Error>({
    queryKey: ['admin', 'vendors', 'server'],
    queryFn: () => fetchAdminVendors('server'),
  });

  const protocolVersionsQuery = useQuery<AdminProtocolVersion[], Error>({
    queryKey: [
      'admin',
      'protocol-versions',
      filters.stateId || 'none',
      filters.stateAuthorityId || 'none',
      filters.projectId || 'none',
      filters.serverVendorId || 'all',
    ],
    queryFn: () =>
      fetchAdminProtocolVersions({
        stateId: filters.stateId,
        stateAuthorityId: filters.stateAuthorityId,
        projectId: filters.projectId,
        serverVendorId: filters.serverVendorId || undefined,
      }),
    enabled: Boolean(filters.stateId && filters.stateAuthorityId && filters.projectId),
  });

  const stateOptions = useMemo(() => statesQuery.data ?? [], [statesQuery.data]);
  const authorityOptions = useMemo(() => authoritiesQuery.data ?? [], [authoritiesQuery.data]);
  const projectOptions = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);
  const serverVendorOptions = useMemo(
    () => serverVendorsQuery.data ?? [],
    [serverVendorsQuery.data],
  );

  const protocolList = useMemo(
    () => protocolVersionsQuery.data ?? [],
    [protocolVersionsQuery.data],
  );

  const filtersReady = Boolean(filters.stateId && filters.stateAuthorityId && filters.projectId);

  const protocolMetrics = useMemo(() => {
    if (!protocolList.length) {
      return { total: 0, distinctVendors: 0, warnings: 0 };
    }

    const vendorSet = new Set<string>();
    let warnings = 0;

    for (const protocol of protocolList) {
      vendorSet.add(protocol.serverVendorId);
      if (hasValidationWarnings(protocol.metadata)) {
        warnings += 1;
      }
    }

    return {
      total: protocolList.length,
      distinctVendors: vendorSet.size,
      warnings,
    };
  }, [protocolList]);

  const protocolKpis: AdminKpiItem[] = useMemo(() => {
    if (!filtersReady) {
      return [
        {
          id: 'total-protocols',
          label: 'Total Protocol Versions',
          value: null,
          description: 'Select state, authority, and project to view protocol metrics.',
        },
        {
          id: 'distinct-vendors',
          label: 'Unique Server Vendors',
          value: null,
        },
        {
          id: 'protocol-warnings',
          label: 'Versions With Warnings',
          value: null,
        },
      ];
    }

    return [
      {
        id: 'total-protocols',
        label: 'Total Protocol Versions',
        value: protocolMetrics.total,
      },
      {
        id: 'distinct-vendors',
        label: 'Unique Server Vendors',
        value: protocolMetrics.distinctVendors,
      },
      {
        id: 'protocol-warnings',
        label: 'Versions With Warnings',
        value: protocolMetrics.warnings,
      },
    ];
  }, [filtersReady, protocolMetrics]);

  const kpiLoading = filtersReady && protocolVersionsQuery.isLoading;

  const createMutation = useMutation<
    AdminProtocolVersion,
    Error,
    {
      stateId: string;
      stateAuthorityId: string;
      projectId: string;
      serverVendorId: string;
      version: string;
      name?: string | null;
      metadata?: Record<string, unknown> | null;
    }
  >({
    mutationFn: createAdminProtocolVersion,
    onSuccess: (protocol) => {
      setStatus({
        type: 'success',
        message: `Protocol version ${protocol.version} created.`,
      });
      queryClient.invalidateQueries({ queryKey: ['admin', 'protocol-versions'] });
      const nextFilters: ProtocolFilterState = {
        stateId: protocol.stateId,
        stateAuthorityId: protocol.authorityId,
        projectId: protocol.projectId,
        serverVendorId: filters.serverVendorId,
      };
      setFilters(nextFilters);
      resetForm(nextFilters);
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to create protocol version.' });
    },
  });

  const updateMutation = useMutation<
    AdminProtocolVersion,
    Error,
    {
      protocolVersionId: string;
      payload: {
        serverVendorId?: string;
        version?: string;
        name?: string | null;
        metadata?: Record<string, unknown> | null;
      };
    }
  >({
    mutationFn: ({ protocolVersionId, payload }) =>
      updateAdminProtocolVersion(protocolVersionId, payload),
    onSuccess: (protocol) => {
      setStatus({
        type: 'success',
        message: `Protocol version ${protocol.version} updated.`,
      });
      queryClient.invalidateQueries({ queryKey: ['admin', 'protocol-versions'] });
      resetForm();
    },
    onError: (error) => {
      setStatus({ type: 'error', message: error.message ?? 'Unable to update protocol version.' });
    },
  });

  function computeNextFilters(
    previous: ProtocolFilterState,
    field: keyof ProtocolFilterState,
    value: string,
  ): ProtocolFilterState {
    switch (field) {
      case 'stateId':
        return {
          stateId: value,
          stateAuthorityId: '',
          projectId: '',
          serverVendorId: '',
        };
      case 'stateAuthorityId':
        return {
          stateId: previous.stateId,
          stateAuthorityId: value,
          projectId: '',
          serverVendorId: '',
        };
      case 'projectId':
        return {
          stateId: previous.stateId,
          stateAuthorityId: previous.stateAuthorityId,
          projectId: value,
          serverVendorId: '',
        };
      case 'serverVendorId':
      default:
        return {
          ...previous,
          serverVendorId: value,
        };
    }
  }

  function resetForm(nextFilters?: ProtocolFilterState) {
    const base = nextFilters ?? filters;
    setEditingId(null);
    setForm({
      stateId: base.stateId,
      stateAuthorityId: base.stateAuthorityId,
      projectId: base.projectId,
      serverVendorId: base.serverVendorId,
      version: '',
      name: '',
      metadata: '',
    });
    setFormError(null);
  }

  const handleFilterSelect =
    (field: keyof ProtocolFilterState) => (event: ChangeEvent<HTMLSelectElement>) => {
      const value = event.target.value;
      setStatus(null);
      setFormError(null);
      if (field !== 'serverVendorId') {
        setEditingId(null);
      }

      let nextFilters = filters;
      setFilters((previous) => {
        nextFilters = computeNextFilters(previous, field, value);
        return nextFilters;
      });

      if (field === 'serverVendorId') {
        setForm((current) => ({ ...current, serverVendorId: value }));
        return;
      }

      const target = nextFilters;
      setForm((current) => {
        if (editingId) {
          return current;
        }
        return {
          stateId: target.stateId,
          stateAuthorityId: target.stateAuthorityId,
          projectId: target.projectId,
          serverVendorId: '',
          version: '',
          name: '',
          metadata: '',
        };
      });
    };

  function handleEdit(protocol: AdminProtocolVersion) {
    if (!canManageHierarchy) {
      setStatus({
        type: 'error',
        message:
          'Creating or editing protocol versions requires hierarchy:manage or admin:all capability.',
      });
      return;
    }

    setEditingId(protocol.id);
    setStatus(null);
    setFormError(null);
    setFilters((previous) => ({
      ...previous,
      stateId: protocol.stateId,
      stateAuthorityId: protocol.authorityId,
      projectId: protocol.projectId,
    }));
    setForm({
      stateId: protocol.stateId,
      stateAuthorityId: protocol.authorityId,
      projectId: protocol.projectId,
      serverVendorId: protocol.serverVendorId,
      version: protocol.version,
      name: protocol.name ?? '',
      metadata: stringifyMetadata(protocol.metadata),
    });
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus(null);
    setFormError(null);

    if (!canManageHierarchy) {
      setFormError(
        'Creating or editing protocol versions requires hierarchy:manage or admin:all capability.',
      );
      return;
    }

    if (!form.stateId || !form.stateAuthorityId || !form.projectId) {
      setFormError('Select state, authority, and project for the protocol version');
      return;
    }

    if (!form.serverVendorId) {
      setFormError('Select a server vendor for the protocol version');
      return;
    }

    const version = form.version.trim();
    if (!version) {
      setFormError('Protocol version is required');
      return;
    }

    const { metadata, error } = parseMetadata(form.metadata);
    if (error) {
      setFormError(error);
      return;
    }

    const name = form.name.trim();
    const payloadName = name ? name : null;

    if (editingId) {
      updateMutation.mutate({
        protocolVersionId: editingId,
        payload: {
          serverVendorId: form.serverVendorId,
          version,
          name: payloadName,
          metadata,
        },
      });
      return;
    }

    createMutation.mutate({
      stateId: form.stateId,
      stateAuthorityId: form.stateAuthorityId,
      projectId: form.projectId,
      serverVendorId: form.serverVendorId,
      version,
      name: payloadName,
      metadata,
    });
  }

  function handleVendorChange(event: ChangeEvent<HTMLSelectElement>) {
    const value = event.target.value;
    setForm((current) => ({ ...current, serverVendorId: value }));
  }

  const handleInputChange =
    (field: 'version' | 'name') => (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.target.value;
      setForm((current) => ({ ...current, [field]: value }));
    };

  function handleMetadataChange(event: ChangeEvent<HTMLTextAreaElement>) {
    const value = event.target.value;
    setForm((current) => ({ ...current, metadata: value }));
  }

  function handleCancel() {
    resetForm();
    setStatus(null);
  }

  const selectedState = stateOptions.find((state) => state.id === filters.stateId);
  const selectedAuthority = authorityOptions.find(
    (authority) => authority.id === filters.stateAuthorityId,
  );
  const selectedProject = projectOptions.find((project) => project.id === filters.projectId);

  const isSaving = createMutation.isPending || updateMutation.isPending;

  return (
    <section
      aria-labelledby="admin-protocol-versions-heading"
      className="rounded border border-slate-200 bg-white p-6 shadow-sm"
    >
      <header className="mb-4">
        <h2 id="admin-protocol-versions-heading" className="text-xl font-semibold text-slate-900">
          Protocol Versions
        </h2>
        <p className="text-sm text-slate-600">
          Configure protocol metadata keyed by state, authority, project, and server vendor so
          provisioning and telemetry flows resolve the correct defaults.
        </p>
      </header>

      <div className="mb-5">
        <AdminKpiGrid items={protocolKpis} isLoading={kpiLoading} />
      </div>

      <AdminStatusBanner status={status} />

      {!canManageHierarchy && (
        <div className="mb-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
          Read-only mode: hierarchy write actions require hierarchy:manage or admin:all
          capability.
        </div>
      )}

      <div className="mb-4 grid gap-3 md:grid-cols-4">
        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">State</span>
          <select
            className="rounded border border-slate-300 px-3 py-2"
            value={filters.stateId}
            onChange={handleFilterSelect('stateId')}
          >
            <option value="">Select state…</option>
            {stateOptions.map((state) => (
              <option key={state.id} value={state.id}>
                {state.name}
              </option>
            ))}
          </select>
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">State Authority</span>
          <select
            className="rounded border border-slate-300 px-3 py-2"
            value={filters.stateAuthorityId}
            onChange={handleFilterSelect('stateAuthorityId')}
            disabled={!filters.stateId || authoritiesQuery.isLoading}
          >
            <option value="">Select authority…</option>
            {authorityOptions.map((authority) => (
              <option key={authority.id} value={authority.id}>
                {authority.name}
              </option>
            ))}
          </select>
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Project</span>
          <select
            className="rounded border border-slate-300 px-3 py-2"
            value={filters.projectId}
            onChange={handleFilterSelect('projectId')}
            disabled={!filters.stateAuthorityId || projectsQuery.isLoading}
          >
            <option value="">Select project…</option>
            {projectOptions.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Filter by Server Vendor</span>
          <select
            className="rounded border border-slate-300 px-3 py-2"
            value={filters.serverVendorId}
            onChange={handleFilterSelect('serverVendorId')}
          >
            <option value="">All server vendors</option>
            {serverVendorOptions.map((vendor) => (
              <option key={vendor.id} value={vendor.id}>
                {vendor.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      <form className="mb-4 grid gap-3 md:grid-cols-2" onSubmit={handleSubmit}>
        <div className="rounded border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600 md:col-span-2">
          <span className="font-semibold text-slate-700">Target hierarchy:</span>{' '}
          {selectedState?.name ?? '—'} / {selectedAuthority?.name ?? '—'} /{' '}
          {selectedProject?.name ?? '—'}
        </div>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Server Vendor</span>
          <select
            className="rounded border border-slate-300 px-3 py-2"
            value={form.serverVendorId}
            onChange={handleVendorChange}
            disabled={!filtersReady || isSaving || !canManageHierarchy}
          >
            <option value="">Select server vendor…</option>
            {serverVendorOptions.map((vendor) => (
              <option key={vendor.id} value={vendor.id}>
                {vendor.name}
              </option>
            ))}
          </select>
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Protocol Version</span>
          <input
            className="rounded border border-slate-300 px-3 py-2"
            value={form.version}
            onChange={handleInputChange('version')}
            placeholder="1.0.0"
            disabled={!filtersReady || isSaving || !canManageHierarchy}
          />
        </label>

        <label className="grid gap-1 text-sm">
          <span className="font-medium text-slate-700">Display Name (optional)</span>
          <input
            className="rounded border border-slate-300 px-3 py-2"
            value={form.name}
            onChange={handleInputChange('name')}
            placeholder="Primary MQTT feed"
            disabled={!filtersReady || isSaving || !canManageHierarchy}
          />
        </label>

        <label className="grid gap-1 text-sm md:col-span-2">
          <span className="font-medium text-slate-700">Metadata JSON (optional)</span>
          <textarea
            className="rounded border border-slate-300 px-3 py-2"
            rows={3}
            value={form.metadata}
            onChange={handleMetadataChange}
            placeholder='{"governmentCredentials":{"endpointDefaults":[{"protocol":"mqtt","host":"broker.gov.in","port":1886}]}}'
            disabled={!filtersReady || isSaving || !canManageHierarchy}
          />
        </label>

        {formError && <p className="text-sm text-red-600 md:col-span-2">{formError}</p>}

        <div className="flex gap-2 md:col-span-2">
          <button
            type="submit"
            className="rounded bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-slate-700 disabled:cursor-not-allowed disabled:bg-slate-400"
            disabled={!filtersReady || isSaving || !canManageHierarchy}
          >
            {editingId
              ? isSaving
                ? 'Updating…'
                : 'Update Protocol Version'
              : isSaving
                ? 'Saving…'
                : 'Add Protocol Version'}
          </button>
          {editingId && (
            <button
              type="button"
              className="rounded border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100"
              onClick={handleCancel}
              disabled={isSaving}
            >
              Cancel
            </button>
          )}
        </div>
      </form>

      {protocolVersionsQuery.error && (
        <p className="mb-4 text-sm text-red-600">{protocolVersionsQuery.error.message}</p>
      )}

      <ProtocolVersionTable
        protocols={protocolList}
        isLoading={protocolVersionsQuery.isLoading}
        filtersReady={filtersReady}
        canManageHierarchy={canManageHierarchy}
        onEdit={handleEdit}
      />
    </section>
  );
}

type ProtocolVersionTableProps = {
  protocols: AdminProtocolVersion[];
  isLoading: boolean;
  filtersReady: boolean;
  canManageHierarchy: boolean;
  onEdit: (protocol: AdminProtocolVersion) => void;
};

function ProtocolVersionTable({
  protocols,
  isLoading,
  filtersReady,
  canManageHierarchy,
  onEdit,
}: ProtocolVersionTableProps) {
  if (!filtersReady) {
    return (
      <p className="text-sm text-slate-600">
        Select a state, authority, and project to manage protocol versions.
      </p>
    );
  }

  if (isLoading) {
    return <p className="text-sm text-slate-600">Loading protocol versions…</p>;
  }

  if (!protocols.length) {
    return (
      <p className="text-sm text-slate-600">No protocol versions captured for this project yet.</p>
    );
  }

  return (
    <div className="overflow-hidden rounded border border-slate-200">
      <table className="w-full table-fixed divide-y divide-slate-200 text-left text-sm">
        <thead className="bg-slate-50">
          <tr>
            <th className="px-4 py-2 font-semibold text-slate-700">Version</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Name</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Server Vendor</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Metadata</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Updated</th>
            <th className="px-4 py-2 font-semibold text-slate-700">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 bg-white">
          {protocols.map((protocol) => (
            <tr key={protocol.id}>
              <td className="px-4 py-2">{protocol.version}</td>
              <td className="px-4 py-2">{protocol.name ?? '—'}</td>
              <td className="px-4 py-2">{protocol.serverVendorName ?? '—'}</td>
              <td className="px-4 py-2 text-xs text-slate-600">
                {getMetadataPreview(protocol.metadata)}
              </td>
              <td className="px-4 py-2 text-xs text-slate-500">
                {new Date(protocol.updatedAt).toLocaleString()}
              </td>
              <td className="px-4 py-2">
                <div className="flex gap-2">
                  <IconActionButton
                    label="Edit"
                    onClick={() => onEdit(protocol)}
                    disabled={!canManageHierarchy}
                    title={
                      canManageHierarchy
                        ? `Edit protocol ${protocol.version}`
                        : 'Hierarchy edits require hierarchy:manage or admin:all capability.'
                    }
                  />
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
