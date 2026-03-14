import { FormEvent, useEffect, useMemo, useState } from 'react';
import {
  UseMutationResult,
  UseQueryResult,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import {
  AdminVendor,
  VendorCollectionKey,
  createAdminVendor,
  deleteAdminVendor,
  fetchAdminVendors,
  updateAdminVendor,
} from '../../../api/admin';
import { useAuth } from '../../../auth';
import { IconActionButton } from '../../../components/IconActionButton';
import { AdminStatusBanner, type AdminStatusMessage } from '../components/AdminStatusBanner';
import { AdminKpiGrid, type AdminKpiItem } from '../components/AdminKpiGrid';
import { isActiveVendor } from '../utils/status';
import { getMetadataPreview, parseMetadata, stringifyMetadata } from '../utils/metadata';

const vendorLabels: Record<VendorCollectionKey, string> = {
  server: 'Server Vendors',
  solarPump: 'Solar Pump Vendors',
  vfdManufacturer: 'VFD Drive Manufacturers',
  rmsManufacturer: 'RMS Manufacturers',
};

const vendorKeys: VendorCollectionKey[] = [
  'server',
  'solarPump',
  'vfdManufacturer',
  'rmsManufacturer',
];

const vendorHeadingIds: Record<VendorCollectionKey, string> = {
  server: 'admin-server-vendors-heading',
  solarPump: 'admin-pump-vendors-heading',
  vfdManufacturer: 'admin-drive-manufacturers-heading',
  rmsManufacturer: 'admin-rms-manufacturers-heading',
};

type VendorFormState = {
  name: string;
  metadata: string;
};

const emptyVendorForm: VendorFormState = {
  name: '',
  metadata: '',
};

const defaultVendorForms: Record<VendorCollectionKey, VendorFormState> = {
  server: { ...emptyVendorForm },
  solarPump: { ...emptyVendorForm },
  vfdManufacturer: { ...emptyVendorForm },
  rmsManufacturer: { ...emptyVendorForm },
};

const defaultVendorErrors: Record<VendorCollectionKey, string | null> = {
  server: null,
  solarPump: null,
  vfdManufacturer: null,
  rmsManufacturer: null,
};

const defaultVendorEditing: Record<VendorCollectionKey, string | null> = {
  server: null,
  solarPump: null,
  vfdManufacturer: null,
  rmsManufacturer: null,
};

const defaultVendorStatus: Record<VendorCollectionKey, AdminStatusMessage | null> = {
  server: null,
  solarPump: null,
  vfdManufacturer: null,
  rmsManufacturer: null,
};

const defaultVendorDeleting: Record<VendorCollectionKey, string | null> = {
  server: null,
  solarPump: null,
  vfdManufacturer: null,
  rmsManufacturer: null,
};

type VendorsSectionProps = {
  initialCollection?: VendorCollectionKey;
  collections?: VendorCollectionKey[];
};

export function VendorsSection({ initialCollection, collections }: VendorsSectionProps = {}) {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const canManageVendors = hasCapability('vendors:manage') || hasCapability('admin:all');
  const canDeleteVendors = hasCapability('vendors:manage') || hasCapability('admin:all');

  const visibleCollections = collections && collections.length > 0 ? collections : vendorKeys;

  const [forms, setForms] = useState(defaultVendorForms);
  const [errors, setErrors] = useState(defaultVendorErrors);
  const [editing, setEditing] = useState(defaultVendorEditing);
  const [status, setStatus] = useState(defaultVendorStatus);
  const [deleting, setDeleting] = useState(defaultVendorDeleting);

  const serverVendorsQuery = useQuery<AdminVendor[], Error>({
    queryKey: ['admin', 'vendors', 'server'],
    queryFn: () => fetchAdminVendors('server'),
  });
  const solarPumpVendorsQuery = useQuery<AdminVendor[], Error>({
    queryKey: ['admin', 'vendors', 'solar-pump'],
    queryFn: () => fetchAdminVendors('solarPump'),
  });
  const vfdVendorsQuery = useQuery<AdminVendor[], Error>({
    queryKey: ['admin', 'vendors', 'vfd-manufacturer'],
    queryFn: () => fetchAdminVendors('vfdManufacturer'),
  });
  const rmsVendorsQuery = useQuery<AdminVendor[], Error>({
    queryKey: ['admin', 'vendors', 'rms-manufacturer'],
    queryFn: () => fetchAdminVendors('rmsManufacturer'),
  });

  const vendorQueries: Record<VendorCollectionKey, UseQueryResult<AdminVendor[], Error>> = {
    server: serverVendorsQuery,
    solarPump: solarPumpVendorsQuery,
    vfdManufacturer: vfdVendorsQuery,
    rmsManufacturer: rmsVendorsQuery,
  };

  const vendorCreateMutations: Record<
    VendorCollectionKey,
    UseMutationResult<
      AdminVendor,
      Error,
      {
        collection: VendorCollectionKey;
        payload: { name: string; metadata?: Record<string, unknown> | null };
      }
    >
  > = {
    server: useMutation({
      mutationFn: ({ collection, payload }) => createAdminVendor(collection, payload),
      onSuccess: (vendor) => handleMutationSuccess('server', vendor, 'created'),
      onError: (error) => handleMutationError('server', error, 'create'),
    }),
    solarPump: useMutation({
      mutationFn: ({ collection, payload }) => createAdminVendor(collection, payload),
      onSuccess: (vendor) => handleMutationSuccess('solarPump', vendor, 'created'),
      onError: (error) => handleMutationError('solarPump', error, 'create'),
    }),
    vfdManufacturer: useMutation({
      mutationFn: ({ collection, payload }) => createAdminVendor(collection, payload),
      onSuccess: (vendor) => handleMutationSuccess('vfdManufacturer', vendor, 'created'),
      onError: (error) => handleMutationError('vfdManufacturer', error, 'create'),
    }),
    rmsManufacturer: useMutation({
      mutationFn: ({ collection, payload }) => createAdminVendor(collection, payload),
      onSuccess: (vendor) => handleMutationSuccess('rmsManufacturer', vendor, 'created'),
      onError: (error) => handleMutationError('rmsManufacturer', error, 'create'),
    }),
  };

  const vendorUpdateMutations: Record<
    VendorCollectionKey,
    UseMutationResult<
      AdminVendor,
      Error,
      {
        collection: VendorCollectionKey;
        entityId: string;
        payload: { name?: string; metadata?: Record<string, unknown> | null };
      }
    >
  > = {
    server: useMutation({
      mutationFn: ({ collection, entityId, payload }) =>
        updateAdminVendor(collection, entityId, payload),
      onSuccess: (vendor) => handleMutationSuccess('server', vendor, 'updated'),
      onError: (error) => handleMutationError('server', error, 'update'),
    }),
    solarPump: useMutation({
      mutationFn: ({ collection, entityId, payload }) =>
        updateAdminVendor(collection, entityId, payload),
      onSuccess: (vendor) => handleMutationSuccess('solarPump', vendor, 'updated'),
      onError: (error) => handleMutationError('solarPump', error, 'update'),
    }),
    vfdManufacturer: useMutation({
      mutationFn: ({ collection, entityId, payload }) =>
        updateAdminVendor(collection, entityId, payload),
      onSuccess: (vendor) => handleMutationSuccess('vfdManufacturer', vendor, 'updated'),
      onError: (error) => handleMutationError('vfdManufacturer', error, 'update'),
    }),
    rmsManufacturer: useMutation({
      mutationFn: ({ collection, entityId, payload }) =>
        updateAdminVendor(collection, entityId, payload),
      onSuccess: (vendor) => handleMutationSuccess('rmsManufacturer', vendor, 'updated'),
      onError: (error) => handleMutationError('rmsManufacturer', error, 'update'),
    }),
  };

  const vendorDeleteMutations: Record<
    VendorCollectionKey,
    UseMutationResult<void, Error, { collection: VendorCollectionKey; entityId: string }>
  > = {
    server: useMutation({
      mutationFn: ({ collection, entityId }) => deleteAdminVendor(collection, entityId),
    }),
    solarPump: useMutation({
      mutationFn: ({ collection, entityId }) => deleteAdminVendor(collection, entityId),
    }),
    vfdManufacturer: useMutation({
      mutationFn: ({ collection, entityId }) => deleteAdminVendor(collection, entityId),
    }),
    rmsManufacturer: useMutation({
      mutationFn: ({ collection, entityId }) => deleteAdminVendor(collection, entityId),
    }),
  };

  const vendorDeleteDisabledMessage = canDeleteVendors
    ? null
    : 'Deleting vendor records requires the vendors:manage capability or super-admin access.';

  const vendorWriteDisabledMessage = canManageVendors
    ? null
    : 'Read-only mode: vendor write actions require vendors:manage or admin:all capability.';

  const vendorLists = useMemo(() => {
    const lists: Record<VendorCollectionKey, AdminVendor[]> = {
      server: serverVendorsQuery.data ?? [],
      solarPump: solarPumpVendorsQuery.data ?? [],
      vfdManufacturer: vfdVendorsQuery.data ?? [],
      rmsManufacturer: rmsVendorsQuery.data ?? [],
    };
    return lists;
  }, [
    serverVendorsQuery.data,
    solarPumpVendorsQuery.data,
    vfdVendorsQuery.data,
    rmsVendorsQuery.data,
  ]);

  const vendorMetrics = useMemo(() => {
    const metrics: Record<
      VendorCollectionKey,
      { total: number; active: number; missingMetadata: number }
    > = {
      server: { total: 0, active: 0, missingMetadata: 0 },
      solarPump: { total: 0, active: 0, missingMetadata: 0 },
      vfdManufacturer: { total: 0, active: 0, missingMetadata: 0 },
      rmsManufacturer: { total: 0, active: 0, missingMetadata: 0 },
    };

    for (const collection of vendorKeys) {
      const list = vendorLists[collection];
      if (!list?.length) {
        continue;
      }

      let activeCount = 0;
      let missingMetadata = 0;

      for (const vendor of list) {
        if (isActiveVendor(vendor)) {
          activeCount += 1;
        }

        const metadata = vendor.metadata;
        if (!metadata || (typeof metadata === 'object' && !Object.keys(metadata).length)) {
          missingMetadata += 1;
        }
      }

      metrics[collection] = {
        total: list.length,
        active: activeCount,
        missingMetadata,
      };
    }

    return metrics;
  }, [vendorLists]);

  const vendorKpiItems = useMemo(() => {
    const items: Record<VendorCollectionKey, AdminKpiItem[]> = {
      server: [],
      solarPump: [],
      vfdManufacturer: [],
      rmsManufacturer: [],
    };

    for (const collection of vendorKeys) {
      const metrics = vendorMetrics[collection];
      items[collection] = [
        {
          id: `${collection}-total-vendors`,
          label: 'Total Vendors',
          value: metrics.total,
        },
        {
          id: `${collection}-active-vendors`,
          label: 'Active Vendors',
          value: metrics.active,
        },
        {
          id: `${collection}-missing-metadata`,
          label: 'Missing Metadata',
          value: metrics.missingMetadata,
        },
      ];
    }

    return items;
  }, [vendorMetrics]);

  useEffect(() => {
    if (!initialCollection) {
      return;
    }
    if (!visibleCollections.includes(initialCollection)) {
      return;
    }
    const headingId = vendorHeadingIds[initialCollection];
    if (!headingId) {
      return;
    }
    const node = document.getElementById(headingId);
    if (node) {
      node.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [initialCollection, visibleCollections]);

  function isVendorSaving(collection: VendorCollectionKey) {
    return (
      vendorCreateMutations[collection].isPending || vendorUpdateMutations[collection].isPending
    );
  }

  function resetVendorForm(collection: VendorCollectionKey) {
    setForms((current) => ({ ...current, [collection]: { ...emptyVendorForm } }));
    setErrors((current) => ({ ...current, [collection]: null }));
    setEditing((current) => ({ ...current, [collection]: null }));
  }

  function handleMutationSuccess(
    collection: VendorCollectionKey,
    vendor: AdminVendor,
    verb: 'created' | 'updated',
  ) {
    setStatus((current) => ({
      ...current,
      [collection]: { type: 'success', message: `${vendor.name} ${verb}.` },
    }));
    resetVendorForm(collection);
    queryClient.invalidateQueries({ queryKey: ['admin', 'vendors'] });
  }

  function handleMutationError(
    collection: VendorCollectionKey,
    error: Error,
    action: 'create' | 'update',
  ) {
    const message = error.message ?? `Unable to ${action} vendor.`;
    setStatus((current) => ({
      ...current,
      [collection]: { type: 'error', message },
    }));
  }

  function handleVendorSubmit(collection: VendorCollectionKey) {
    return (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setStatus((current) => ({ ...current, [collection]: null }));
      setErrors((current) => ({ ...current, [collection]: null }));

      if (!canManageVendors) {
        setErrors((current) => ({
          ...current,
          [collection]: 'Creating or editing vendors requires vendors:manage or admin:all capability.',
        }));
        return;
      }

      const form = forms[collection];
      if (!form.name.trim()) {
        setErrors((current) => ({ ...current, [collection]: 'Vendor name is required.' }));
        return;
      }

      const { metadata, error } = parseMetadata(form.metadata);
      if (error) {
        setErrors((current) => ({ ...current, [collection]: error }));
        return;
      }

      if (editing[collection]) {
        vendorUpdateMutations[collection].mutate({
          collection,
          entityId: editing[collection] as string,
          payload: {
            name: form.name.trim(),
            metadata,
          },
        });
        return;
      }

      vendorCreateMutations[collection].mutate({
        collection,
        payload: {
          name: form.name.trim(),
          metadata,
        },
      });
    };
  }

  function handleVendorEdit(collection: VendorCollectionKey, vendor: AdminVendor) {
    if (!canManageVendors) {
      setStatus((current) => ({
        ...current,
        [collection]: {
          type: 'error',
          message:
            'Creating or editing vendors requires the vendors:manage capability or super-admin access.',
        },
      }));
      return;
    }

    setEditing((current) => ({ ...current, [collection]: vendor.id }));
    setForms((current) => ({
      ...current,
      [collection]: {
        name: vendor.name,
        metadata: stringifyMetadata(vendor.metadata),
      },
    }));
    setStatus((current) => ({ ...current, [collection]: null }));
    setErrors((current) => ({ ...current, [collection]: null }));
  }

  function handleVendorDelete(collection: VendorCollectionKey, vendor: AdminVendor) {
    if (!canDeleteVendors) {
      setStatus((current) => ({
        ...current,
        [collection]: {
          type: 'error',
          message:
            'Deleting vendor records requires the vendors:manage capability or super-admin access.',
        },
      }));
      return;
    }

    if (!window.confirm(`Delete vendor "${vendor.name}"?`)) {
      return;
    }

    setDeleting((current) => ({ ...current, [collection]: vendor.id }));
    setStatus((current) => ({ ...current, [collection]: null }));

    vendorDeleteMutations[collection].mutate(
      { collection, entityId: vendor.id },
      {
        onSuccess: () => {
          setStatus((current) => ({
            ...current,
            [collection]: {
              type: 'success',
              message: `${vendor.name} deleted.`,
            },
          }));
          queryClient.invalidateQueries({ queryKey: ['admin', 'vendors'] });
          if (editing[collection] === vendor.id) {
            resetVendorForm(collection);
          }
        },
        onError: (error) => {
          setStatus((current) => ({
            ...current,
            [collection]: {
              type: 'error',
              message: error.message ?? 'Unable to delete vendor.',
            },
          }));
        },
        onSettled: () => {
          setDeleting((current) => ({ ...current, [collection]: null }));
        },
      },
    );
  }

  return (
    <section
      aria-labelledby="admin-vendors-heading"
      className="rounded border border-slate-200 bg-white p-6 shadow-sm"
    >
      <header className="mb-4">
        <h2 id="admin-vendors-heading" className="text-xl font-semibold text-slate-900">
          Vendors
        </h2>
        <p className="text-sm text-slate-600">
          Maintain vendor registries so protocol selection stays driven by metadata and avoids
          branch logic.
        </p>
      </header>

      <div className="grid gap-6 md:grid-cols-2">
        {visibleCollections.map((collection) => {
          const query = vendorQueries[collection];
          const vendorList = vendorLists[collection];
          const form = forms[collection];
          const error = errors[collection];
          const editId = editing[collection];
          const deletingId = deleting[collection];
          const saving = isVendorSaving(collection);
          const headingId = vendorHeadingIds[collection];

          return (
            <section
              key={collection}
              aria-labelledby={headingId}
              className="rounded border border-slate-200 p-4"
            >
              <h3 id={headingId} className="text-lg font-semibold text-slate-900">
                {vendorLabels[collection]}
              </h3>

              <div className="mt-3">
                <AdminKpiGrid items={vendorKpiItems[collection]} isLoading={query.isLoading} />
              </div>

              <AdminStatusBanner status={status[collection]} />

              {vendorWriteDisabledMessage && (
                <div className="mt-3 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
                  {vendorWriteDisabledMessage}
                </div>
              )}

              {vendorDeleteDisabledMessage && (
                <div className="mt-3 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
                  {vendorDeleteDisabledMessage}
                </div>
              )}

              <form className="mt-3 space-y-3" onSubmit={handleVendorSubmit(collection)}>
                <label className="grid gap-1 text-sm">
                  <span className="font-medium text-slate-700">Name</span>
                  <input
                    className="rounded border border-slate-300 px-3 py-2"
                    value={form.name}
                    onChange={(event) =>
                      setForms((current) => ({
                        ...current,
                        [collection]: { ...current[collection], name: event.target.value },
                      }))
                    }
                    placeholder="Vendor name"
                    disabled={!canManageVendors || saving}
                  />
                </label>

                <label className="grid gap-1 text-sm">
                  <span className="font-medium text-slate-700">Metadata JSON (optional)</span>
                  <textarea
                    className="rounded border border-slate-300 px-3 py-2"
                    rows={3}
                    value={form.metadata}
                    onChange={(event) =>
                      setForms((current) => ({
                        ...current,
                        [collection]: { ...current[collection], metadata: event.target.value },
                      }))
                    }
                    placeholder='{"supportEmail": "ops@example.com"}'
                    disabled={!canManageVendors || saving}
                  />
                </label>

                {error && <p className="text-sm text-red-600">{error}</p>}

                <div className="flex gap-2">
                  <button
                    type="submit"
                    className="rounded bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-slate-700 disabled:cursor-not-allowed disabled:bg-slate-400"
                    disabled={saving || !canManageVendors}
                  >
                    {editId
                      ? saving
                        ? 'Updating…'
                        : 'Update Vendor'
                      : saving
                        ? 'Saving…'
                        : 'Add Vendor'}
                  </button>
                  {editId && (
                    <button
                      type="button"
                      className="rounded border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100"
                      onClick={() => resetVendorForm(collection)}
                      disabled={saving}
                    >
                      Cancel
                    </button>
                  )}
                </div>
              </form>

              {query.error && <p className="mt-3 text-sm text-red-600">{query.error.message}</p>}

              <VendorList
                vendors={vendorList}
                isLoading={query.isLoading}
                editingId={editId}
                deletingId={deletingId}
                canManage={canManageVendors}
                canDelete={canDeleteVendors}
                onEdit={(vendor) => handleVendorEdit(collection, vendor)}
                onDelete={(vendor) => handleVendorDelete(collection, vendor)}
                deleteDisabledReason={vendorDeleteDisabledMessage ?? undefined}
                editDisabledReason={vendorWriteDisabledMessage ?? undefined}
              />
            </section>
          );
        })}
      </div>
    </section>
  );
}

type VendorListProps = {
  vendors: AdminVendor[];
  isLoading: boolean;
  editingId: string | null;
  deletingId: string | null;
  canManage: boolean;
  canDelete: boolean;
  editDisabledReason?: string;
  deleteDisabledReason?: string;
  onEdit: (vendor: AdminVendor) => void;
  onDelete: (vendor: AdminVendor) => void;
};

function VendorList({
  vendors,
  isLoading,
  editingId,
  deletingId,
  canManage,
  canDelete,
  editDisabledReason,
  deleteDisabledReason,
  onEdit,
  onDelete,
}: VendorListProps) {
  if (isLoading) {
    return <p className="mt-3 text-sm text-slate-600">Loading…</p>;
  }

  if (!vendors.length) {
    return <p className="mt-3 text-sm text-slate-600">No vendors registered yet.</p>;
  }

  return (
    <ul className="mt-3 space-y-2 text-sm">
      {vendors.map((vendor) => {
        const isEditing = editingId === vendor.id;
        return (
          <li
            key={vendor.id}
            className={`rounded border px-3 py-2 ${
              isEditing ? 'border-emerald-300 bg-emerald-50/60' : 'border-slate-200'
            }`}
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="font-medium text-slate-900">{vendor.name}</p>
                <p className="text-xs text-slate-600">{getMetadataPreview(vendor.metadata)}</p>
              </div>
              <div className="flex gap-2">
                <IconActionButton
                  label="Edit"
                  onClick={() => onEdit(vendor)}
                  disabled={!canManage}
                  title={canManage ? `Edit ${vendor.name}` : (editDisabledReason ?? 'Insufficient privileges')}
                />
                <IconActionButton
                  label="Delete"
                  onClick={() => onDelete(vendor)}
                  variant="danger"
                  disabled={!canDelete}
                  loading={deletingId === vendor.id}
                  title={
                    canDelete
                      ? `Delete ${vendor.name}`
                      : (deleteDisabledReason ?? 'Insufficient privileges')
                  }
                />
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}
