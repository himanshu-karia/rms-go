import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';

import {
  downloadImportJobErrorsCsv,
  fetchImportJob,
  fetchImportJobs,
  retryDeviceImportJobRows,
  type ImportJob,
  type ImportJobStatus,
  type ImportJobType,
  type DeviceImportRetrySummary,
} from '../api/devices';
import { ImportJobLink } from '../components/ImportJobLink';
import { usePollingGate } from '../session';

const jobTypeLabels: Record<ImportJobType, string> = {
  device: 'Device enrollment',
  government_credentials: 'Government credential',
  installation_beneficiaries: 'Installation beneficiary linkage',
};

const jobStatusLabels: Record<ImportJobStatus, string> = {
  pending: 'Pending',
  completed: 'Completed',
};

const successLabelByType: Record<ImportJobType, string> = {
  device: 'Enrolled',
  government_credentials: 'Updated',
  installation_beneficiaries: 'Linked',
};

const typeOptions: Array<{ value: ImportJobType; label: string }> = [
  { value: 'device', label: jobTypeLabels.device },
  { value: 'government_credentials', label: jobTypeLabels.government_credentials },
  {
    value: 'installation_beneficiaries',
    label: jobTypeLabels.installation_beneficiaries,
  },
];

const statusOptions: Array<{ value: ImportJobStatus; label: string }> = [
  { value: 'pending', label: jobStatusLabels.pending },
  { value: 'completed', label: jobStatusLabels.completed },
];

type FilterState = {
  types: ImportJobType[];
  statuses: ImportJobStatus[];
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  limit: number;
};

function createInitialFilters(): FilterState {
  return {
    types: [],
    statuses: [],
    stateId: '',
    stateAuthorityId: '',
    projectId: '',
    limit: 25,
  };
}

function formatDateTime(value: string | null) {
  return value ? new Date(value).toLocaleString() : '—';
}

function hasFiltersApplied(filters: FilterState, jobId: string) {
  return (
    filters.types.length > 0 ||
    filters.statuses.length > 0 ||
    filters.stateId.trim().length > 0 ||
    filters.stateAuthorityId.trim().length > 0 ||
    filters.projectId.trim().length > 0 ||
    filters.limit !== 25 ||
    jobId.trim().length > 0
  );
}

function formatScope(job: ImportJob) {
  const segments: string[] = [];

  if (job.stateId) {
    segments.push(`State ${job.stateId}`);
  }

  if (job.stateAuthorityId) {
    segments.push(`Authority ${job.stateAuthorityId}`);
  }

  if (job.projectId) {
    segments.push(`Project ${job.projectId}`);
  }

  return segments.length ? segments.join(' · ') : '—';
}

function formatUpdatedTime(timestamp: number | null | undefined) {
  if (!timestamp) {
    return null;
  }

  try {
    return new Date(timestamp).toLocaleTimeString();
  } catch {
    return null;
  }
}

type InstallationBeneficiaryMetadataSummary = {
  created: number;
  updated: number;
  reactivated: number;
};

function parseInstallationBeneficiaryMetadata(
  metadata: Record<string, unknown> | null | undefined,
): InstallationBeneficiaryMetadataSummary | null {
  if (!metadata || typeof metadata !== 'object') {
    return null;
  }

  const parseCount = (value: unknown): number | null => {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
    if (typeof value === 'string') {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : null;
    }
    return null;
  };

  const record = metadata as Record<string, unknown>;
  const created = parseCount(record['createdBeneficiaries']);
  const updated = parseCount(record['updatedBeneficiaries']);
  const reactivated = parseCount(record['reactivatedAssignments']);

  if (created === null && updated === null && reactivated === null) {
    return null;
  }

  return {
    created: created ?? 0,
    updated: updated ?? 0,
    reactivated: reactivated ?? 0,
  };
}

export function DeviceImportJobsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [filters, setFilters] = useState<FilterState>(createInitialFilters);
  const jobIdParamRaw = searchParams.get('jobId') ?? '';
  const jobIdParam = jobIdParamRaw.trim();
  const [selectedJobId, setSelectedJobId] = useState<string | null>(() =>
    jobIdParam.length ? jobIdParam : null,
  );
  const [dismissedJobId, setDismissedJobId] = useState<string | null>(null);

  const syncJobIdSearchParam = (value: string) => {
    const next = new URLSearchParams(searchParams);
    const trimmed = value.trim();

    if (trimmed.length) {
      next.set('jobId', trimmed);
    } else {
      next.delete('jobId');
    }

    setSearchParams(next, { replace: true });
  };

  const handleRetrySuccess = () => {
    setDismissedJobId(null);
    query.refetch();
  };

  const query = useInfiniteQuery({
    queryKey: ['devices', 'import-jobs', filters, jobIdParam],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam }) => {
      const trimmedStateId = filters.stateId.trim();
      const trimmedAuthorityId = filters.stateAuthorityId.trim();
      const trimmedProjectId = filters.projectId.trim();

      return fetchImportJobs({
        cursor: pageParam,
        limit: filters.limit,
        types: filters.types.length ? filters.types : undefined,
        statuses: filters.statuses.length ? filters.statuses : undefined,
        stateId: trimmedStateId.length ? trimmedStateId : null,
        stateAuthorityId: trimmedAuthorityId.length ? trimmedAuthorityId : null,
        projectId: trimmedProjectId.length ? trimmedProjectId : null,
        jobId: jobIdParam.length ? jobIdParam : null,
      });
    },
    getNextPageParam: (lastPage) => lastPage.nextCursor,
  });

  const jobs = useMemo(() => {
    if (!query.data) {
      return [] as ImportJob[];
    }

    return query.data.pages.flatMap((page) => page.jobs);
  }, [query.data]);

  const isLoading = query.isLoading;
  const loadError = query.error instanceof Error ? query.error : null;
  const highlightJobId = jobIdParam.length ? jobIdParam : null;
  const highlightDismissed = highlightJobId && highlightJobId === dismissedJobId;
  const activeHighlightJobId = highlightDismissed ? null : highlightJobId;
  const effectiveSelectedJobId = selectedJobId ?? activeHighlightJobId;

  const selectedJobFromList = useMemo(() => {
    if (!effectiveSelectedJobId) {
      return undefined;
    }

    return jobs.find((job) => job.id === effectiveSelectedJobId);
  }, [jobs, effectiveSelectedJobId]);

  const shouldFetchSelectedJob = Boolean(effectiveSelectedJobId && !selectedJobFromList);

  const { data: fetchedSelectedJob } = useQuery({
    queryKey: ['devices', 'import-job', 'selected', effectiveSelectedJobId],
    queryFn: () => fetchImportJob(effectiveSelectedJobId as string),
    enabled: shouldFetchSelectedJob,
    staleTime: 30_000,
    gcTime: 5 * 60_000,
  });

  const selectedJob = useMemo(() => {
    if (!effectiveSelectedJobId) {
      return undefined;
    }

    return selectedJobFromList ?? fetchedSelectedJob ?? undefined;
  }, [effectiveSelectedJobId, fetchedSelectedJob, selectedJobFromList]);

  const handleViewDetails = (jobId: string) => {
    setSelectedJobId(jobId);
  };

  const handleCloseDetails = () => {
    if (highlightJobId) {
      setDismissedJobId(highlightJobId);
    }
    setSelectedJobId(null);
  };

  const handleTypeToggle = (type: ImportJobType) => {
    setFilters((prev) => {
      const exists = prev.types.includes(type);
      return {
        ...prev,
        types: exists ? prev.types.filter((value) => value !== type) : [...prev.types, type],
      };
    });
  };

  const handleStatusToggle = (status: ImportJobStatus) => {
    setFilters((prev) => {
      const exists = prev.statuses.includes(status);
      return {
        ...prev,
        statuses: exists
          ? prev.statuses.filter((value) => value !== status)
          : [...prev.statuses, status],
      };
    });
  };

  const handleScopeChange = (
    field: 'stateId' | 'stateAuthorityId' | 'projectId',
    value: string,
  ) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleLimitChange = (value: number) => {
    setFilters((prev) => ({
      ...prev,
      limit: value,
    }));
  };

  const handleJobIdChange = (value: string) => {
    const trimmed = value.trim();
    if (trimmed.length) {
      setSelectedJobId(trimmed);
    } else {
      setSelectedJobId(null);
    }
    setDismissedJobId(null);
    syncJobIdSearchParam(value);
  };

  const handleResetFilters = () => {
    setFilters(createInitialFilters());
    setSelectedJobId(null);
    setDismissedJobId(null);
    syncJobIdSearchParam('');
  };

  const successLabel = (job: ImportJob) => successLabelByType[job.type];

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold text-slate-900">Import Job History</h1>
        <p className="text-sm text-slate-600">
          Review recent CSV import activity, filter by scope, and trace job identifiers referenced
          on the device provisioning screens.
        </p>
      </header>

      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="grid gap-4 md:grid-cols-2">
            <label className="flex flex-col gap-2 text-sm" htmlFor="filter-job-id">
              <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                Job ID (direct lookup)
              </span>
              <input
                id="filter-job-id"
                type="text"
                value={jobIdParamRaw}
                onChange={(event) => handleJobIdChange(event.target.value)}
                placeholder="Paste an import job identifier"
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              />
              <span className="text-xs text-slate-500">
                When provided, results focus on the matching job and ignore pagination.
              </span>
            </label>
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                Job types
              </p>
              <div className="mt-2 flex flex-col gap-2 text-sm">
                {typeOptions.map((option) => (
                  <label className="flex items-center gap-2" key={option.value}>
                    <input
                      type="checkbox"
                      className="size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                      checked={filters.types.includes(option.value)}
                      onChange={() => handleTypeToggle(option.value)}
                    />
                    <span>{option.label}</span>
                  </label>
                ))}
              </div>
            </div>

            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Status</p>
              <div className="mt-2 flex flex-col gap-2 text-sm">
                {statusOptions.map((option) => (
                  <label className="flex items-center gap-2" key={option.value}>
                    <input
                      type="checkbox"
                      className="size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                      checked={filters.statuses.includes(option.value)}
                      onChange={() => handleStatusToggle(option.value)}
                    />
                    <span>{option.label}</span>
                  </label>
                ))}
              </div>
            </div>

            <label className="flex flex-col gap-2 text-sm" htmlFor="filter-state-id">
              <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                State ID
              </span>
              <input
                id="filter-state-id"
                type="text"
                value={filters.stateId}
                onChange={(event) => handleScopeChange('stateId', event.target.value)}
                placeholder="e.g. maharashtra"
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              />
            </label>

            <label className="flex flex-col gap-2 text-sm" htmlFor="filter-authority-id">
              <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                Authority ID
              </span>
              <input
                id="filter-authority-id"
                type="text"
                value={filters.stateAuthorityId}
                onChange={(event) => handleScopeChange('stateAuthorityId', event.target.value)}
                placeholder="e.g. msecdl"
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              />
            </label>

            <label className="flex flex-col gap-2 text-sm" htmlFor="filter-project-id">
              <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                Project ID
              </span>
              <input
                id="filter-project-id"
                type="text"
                value={filters.projectId}
                onChange={(event) => handleScopeChange('projectId', event.target.value)}
                placeholder="e.g. pm_kusum_solarpump_rms"
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              />
            </label>

            <label className="flex flex-col gap-2 text-sm" htmlFor="filter-limit">
              <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                Page size
              </span>
              <select
                id="filter-limit"
                value={filters.limit}
                onChange={(event) => handleLimitChange(Number(event.target.value))}
                className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              >
                {[10, 25, 50].map((option) => (
                  <option key={option} value={option}>
                    {option} records
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="flex gap-3 text-sm">
            <button
              type="button"
              onClick={() => query.refetch()}
              className="inline-flex items-center justify-center rounded bg-emerald-600 px-4 py-2 font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2"
            >
              Refresh
            </button>
            <button
              type="button"
              onClick={handleResetFilters}
              disabled={!hasFiltersApplied(filters, jobIdParam)}
              className="inline-flex items-center justify-center rounded border border-slate-300 px-4 py-2 font-medium text-slate-600 transition hover:border-slate-400 hover:text-slate-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Reset
            </button>
          </div>
        </div>
      </section>

      {query.isError && loadError ? (
        <div className="rounded border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
          Failed to load import jobs: {loadError.message}
        </div>
      ) : null}

      {highlightJobId ? (
        <div className="rounded border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
          Focusing on import job{' '}
          <code className="rounded bg-emerald-200 px-1 py-0.5 text-[0.65rem] text-emerald-900">
            {highlightJobId}
          </code>
          .
        </div>
      ) : null}

      {selectedJob ? (
        <ImportJobDetail
          jobId={selectedJob.id}
          initialJob={selectedJob}
          onClose={handleCloseDetails}
          successLabel={successLabel(selectedJob)}
          onRetrySuccess={handleRetrySuccess}
        />
      ) : null}

      <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3 text-left">Job ID</th>
                <th className="px-4 py-3 text-left">Type</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-left">Processed</th>
                <th className="px-4 py-3 text-left">Succeeded</th>
                <th className="px-4 py-3 text-left">Failed</th>
                <th className="px-4 py-3 text-left">Scope</th>
                <th className="px-4 py-3 text-left">Issued by</th>
                <th className="px-4 py-3 text-left">Created</th>
                <th className="px-4 py-3 text-left">Completed</th>
                <th className="px-4 py-3 text-left">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {jobs.map((job) => {
                const isHighlighted =
                  activeHighlightJobId === job.id || effectiveSelectedJobId === job.id;
                const beneficiarySummary =
                  job.type === 'installation_beneficiaries'
                    ? parseInstallationBeneficiaryMetadata(job.metadata)
                    : null;
                return (
                  <tr
                    key={job.id}
                    className={`${isHighlighted ? 'bg-emerald-50' : ''} hover:bg-slate-50`}
                  >
                    <td className="whitespace-nowrap px-4 py-3">
                      <code className="rounded bg-slate-200 px-1 py-0.5 text-xs text-slate-800">
                        {job.id}
                      </code>
                      {isHighlighted ? (
                        <span className="mt-1 block text-[11px] font-medium uppercase tracking-wide text-emerald-600">
                          Linked job
                        </span>
                      ) : null}
                      {job.errorCount > 0 ? (
                        <details className="mt-2 text-xs text-rose-600">
                          <summary className="cursor-pointer text-rose-700">
                            {job.errorCount} row issue{job.errorCount === 1 ? '' : 's'}
                          </summary>
                          <ul className="mt-1 space-y-1">
                            {job.errors.slice(0, 5).map((error) => (
                              <li key={`${job.id}-${error.row}`}>
                                Row {error.row}: {error.message}
                              </li>
                            ))}
                            {job.errorCount > job.errors.length ? (
                              <li className="italic text-slate-500">Additional errors truncated</li>
                            ) : null}
                          </ul>
                        </details>
                      ) : null}
                    </td>
                    <td className="px-4 py-3 text-slate-700">{jobTypeLabels[job.type]}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${
                          job.status === 'completed'
                            ? 'bg-emerald-100 text-emerald-800'
                            : 'bg-amber-100 text-amber-800'
                        }`}
                      >
                        {jobStatusLabels[job.status]}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-700">{job.processed}</td>
                    <td className="px-4 py-3 text-slate-700">
                      {job.succeeded}
                      <span className="ml-1 text-xs text-slate-500">{successLabel(job)}</span>
                      {beneficiarySummary ? (
                        <span className="mt-1 block text-[11px] text-slate-500">
                          {`Created ${beneficiarySummary.created.toLocaleString()} · Updated ${beneficiarySummary.updated.toLocaleString()} · Reactivated ${beneficiarySummary.reactivated.toLocaleString()}`}
                        </span>
                      ) : null}
                    </td>
                    <td className="px-4 py-3 text-slate-700">{job.failed}</td>
                    <td className="px-4 py-3 text-slate-700">{formatScope(job)}</td>
                    <td className="px-4 py-3 text-slate-700">{job.issuedBy ?? '—'}</td>
                    <td className="px-4 py-3 text-slate-700">{formatDateTime(job.createdAt)}</td>
                    <td className="px-4 py-3 text-slate-700">{formatDateTime(job.completedAt)}</td>
                    <td className="px-4 py-3">
                      <button
                        type="button"
                        onClick={() => handleViewDetails(job.id)}
                        className="inline-flex items-center justify-center rounded border border-slate-300 px-3 py-1 text-xs font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
                      >
                        View details
                      </button>
                    </td>
                  </tr>
                );
              })}
              {jobs.length === 0 && !isLoading ? (
                <tr>
                  <td colSpan={11} className="px-4 py-6 text-center text-sm text-slate-500">
                    {highlightJobId
                      ? 'No import job was found for the provided identifier or you do not have access to it.'
                      : 'No import jobs match the selected filters.'}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        {isLoading ? (
          <div className="border-t border-slate-200 px-4 py-6 text-center text-sm text-slate-500">
            Loading import jobs…
          </div>
        ) : null}

        <div className="flex items-center justify-between border-t border-slate-200 p-4 text-sm text-slate-600">
          <span>
            Showing {jobs.length} job{jobs.length === 1 ? '' : 's'}
            {query.hasNextPage ? ' (more available)' : ''}
          </span>
          <button
            type="button"
            onClick={() => query.fetchNextPage()}
            disabled={!query.hasNextPage || query.isFetchingNextPage || Boolean(highlightJobId)}
            className="inline-flex items-center justify-center rounded border border-slate-300 px-4 py-2 font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {query.isFetchingNextPage ? 'Loading more…' : 'Load more'}
          </button>
        </div>
      </section>
    </div>
  );
}

type ImportJobDetailProps = {
  jobId: string;
  initialJob: ImportJob;
  onClose: () => void;
  successLabel: string;
  onRetrySuccess: (summary: DeviceImportRetrySummary) => void;
};

function ImportJobDetail({
  jobId,
  initialJob,
  onClose,
  successLabel,
  onRetrySuccess,
}: ImportJobDetailProps) {
  const [isDownloading, setIsDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const [retrySummary, setRetrySummary] = useState<DeviceImportRetrySummary | null>(null);
  const refetchRef = useRef<(() => Promise<unknown>) | undefined>(undefined);
  const pollingGate = usePollingGate(`device-import-job:${jobId}`);

  const {
    data: job,
    isFetching: isRefreshing,
    isError: isLiveError,
    error: liveError,
    dataUpdatedAt,
    refetch,
  } = useQuery({
    queryKey: ['devices', 'import-job', jobId],
    queryFn: () => fetchImportJob(jobId),
    initialData: initialJob,
    placeholderData: (previousData) => previousData ?? initialJob,
    enabled: pollingGate.enabled,
    refetchInterval: pollingGate.enabled ? 15_000 : false,
    refetchOnWindowFocus: false,
  });

  useEffect(() => {
    refetchRef.current = refetch;
  }, [refetch]);

  useEffect(() => {
    setDownloadError(null);
    setIsDownloading(false);
    setRetrySummary(null);
    const refetchFn = refetchRef.current;
    if (refetchFn) {
      void refetchFn();
    }
  }, [jobId]);

  const lastUpdatedDisplay = formatUpdatedTime(dataUpdatedAt);
  const liveStatusClass = isLiveError ? 'text-rose-600' : 'text-slate-500';
  const liveStatusText = isLiveError
    ? `Refresh failed: ${
        liveError instanceof Error ? liveError.message : 'Unable to refresh live preview'
      }`
    : isRefreshing
      ? 'Refreshing…'
      : lastUpdatedDisplay
        ? `Updated ${lastUpdatedDisplay}`
        : 'Updated just now';

  const beneficiarySummary =
    job.type === 'installation_beneficiaries'
      ? parseInstallationBeneficiaryMetadata(job.metadata)
      : null;

  const retryPanelKey = useMemo(() => {
    if (job.type !== 'device') {
      return job.id;
    }

    const rowsSignature = job.errors
      .map((error) => error.row)
      .sort((a, b) => a - b)
      .join(',');

    return `${job.id}:${rowsSignature}`;
  }, [job.errors, job.id, job.type]);

  const handleDownloadErrors = async () => {
    if (isDownloading) {
      return;
    }

    setIsDownloading(true);
    setDownloadError(null);

    try {
      const { blob, filename } = await downloadImportJobErrorsCsv(job.id);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = filename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to download row issues';
      setDownloadError(message);
    } finally {
      setIsDownloading(false);
    }
  };

  const handleCloseClick = () => {
    setDownloadError(null);
    onClose();
  };

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-slate-900">Import job details</h2>
          <p className="mt-1 text-sm text-slate-600">
            Review scope, row counts, and recorded issues for job{' '}
            <code className="rounded bg-slate-200 px-1 py-0.5 text-[11px] text-slate-700">
              {job.id}
            </code>
            .
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {job.errorCount > 0 ? (
            <button
              type="button"
              onClick={handleDownloadErrors}
              disabled={isDownloading}
              className="inline-flex items-center justify-center rounded border border-emerald-300 bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-700 transition hover:border-emerald-400 hover:text-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isDownloading ? 'Downloading…' : 'Download row issues CSV'}
            </button>
          ) : null}
          <button
            type="button"
            onClick={handleCloseClick}
            className="inline-flex items-center justify-center rounded border border-slate-300 px-3 py-1 text-xs font-medium text-slate-700 transition hover:border-slate-400 hover:text-slate-900"
          >
            Close details
          </button>
        </div>
      </div>

      {downloadError ? (
        <p className="mt-3 text-xs text-rose-600" role="alert">
          {downloadError}
        </p>
      ) : null}

      <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        <Detail label="Job ID" value={<ImportJobLink jobId={job.id} />} monospace />
        <Detail label="Type" value={jobTypeLabels[job.type]} />
        <Detail label="Status" value={<JobStatusBadge status={job.status} />} />
        <Detail label="Issued by" value={job.issuedBy ?? '—'} />
        <Detail label="State ID" value={job.stateId ?? '—'} monospace />
        <Detail label="Authority ID" value={job.stateAuthorityId ?? '—'} monospace />
        <Detail label="Project ID" value={job.projectId ?? '—'} monospace />
        <Detail label="Processed rows" value={job.processed.toLocaleString()} />
        <Detail
          label="Succeeded rows"
          value={
            <span>
              {job.succeeded.toLocaleString()}
              <span className="ml-1 text-xs uppercase tracking-wide text-slate-500">
                {successLabel}
              </span>
            </span>
          }
        />
        <Detail label="Failed rows" value={job.failed.toLocaleString()} />
        <Detail
          label="Row issues recorded (snapshot)"
          value={initialJob.errorCount.toLocaleString()}
        />
        <Detail
          label="Live row issues (preview)"
          value={
            <span className="flex flex-col">
              <span>{job.errorCount.toLocaleString()}</span>
              <span className={`text-xs ${liveStatusClass}`}>{liveStatusText}</span>
            </span>
          }
        />
        <Detail label="Created" value={formatDateTime(job.createdAt)} />
        <Detail label="Completed" value={formatDateTime(job.completedAt)} />
      </div>

      {job.type === 'installation_beneficiaries' ? (
        <div className="mt-6 space-y-3">
          <h3 className="text-sm font-semibold text-slate-900">Beneficiary impact</h3>
          {beneficiarySummary ? (
            <div className="grid gap-3 md:grid-cols-3">
              <Detail
                label="Created beneficiaries"
                value={beneficiarySummary.created.toLocaleString()}
              />
              <Detail
                label="Updated beneficiaries"
                value={beneficiarySummary.updated.toLocaleString()}
              />
              <Detail
                label="Reactivated assignments"
                value={beneficiarySummary.reactivated.toLocaleString()}
              />
            </div>
          ) : (
            <p className="rounded border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
              No beneficiary metadata was recorded for this job.
            </p>
          )}
        </div>
      ) : null}

      <div className="mt-6 space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-slate-900">Row-level issues</h3>
          {job.errorCount > job.errors.length ? (
            <span className="text-[11px] uppercase tracking-wide text-slate-500">
              Showing first {job.errors.length} of {job.errorCount}
            </span>
          ) : null}
        </div>

        {job.errorCount === 0 ? (
          <p className="rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            No row-level issues were captured for this import.
          </p>
        ) : (
          <ul className="space-y-2 text-sm">
            {job.errors.map((error) => (
              <li
                key={`${job.id}-${error.row}-${error.message}`}
                className="rounded border border-rose-200 bg-rose-50 px-3 py-2 text-rose-700"
              >
                <span className="font-semibold">Row {error.row}:</span> {error.message}
              </li>
            ))}
          </ul>
        )}
      </div>

      {job.type === 'device' && job.errorCount > 0 ? (
        <RetryFailedRowsPanel
          key={retryPanelKey}
          job={job}
          summary={retrySummary}
          onSummaryChange={setRetrySummary}
          onRetrySuccess={onRetrySuccess}
        />
      ) : null}
    </section>
  );
}

type DetailProps = {
  label: string;
  value: ReactNode;
  monospace?: boolean;
};

function Detail({ label, value, monospace = false }: DetailProps) {
  return (
    <div className="flex flex-col gap-1 rounded border border-slate-200 bg-slate-50 px-3 py-2">
      <span className="text-[10px] font-semibold uppercase tracking-wide text-slate-500">
        {label}
      </span>
      <span className={`text-sm ${monospace ? 'font-mono text-slate-800' : 'text-slate-800'}`}>
        {value}
      </span>
    </div>
  );
}

function JobStatusBadge({ status }: { status: ImportJobStatus }) {
  const className =
    status === 'completed' ? 'bg-emerald-100 text-emerald-800' : 'bg-amber-100 text-amber-800';

  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${className}`}>
      {jobStatusLabels[status]}
    </span>
  );
}

type RetryFailedRowsPanelProps = {
  job: ImportJob;
  summary: DeviceImportRetrySummary | null;
  onSummaryChange: (summary: DeviceImportRetrySummary | null) => void;
  onRetrySuccess: (summary: DeviceImportRetrySummary) => void;
};

function RetryFailedRowsPanel({
  job,
  summary,
  onSummaryChange,
  onRetrySuccess,
}: RetryFailedRowsPanelProps) {
  const queryClient = useQueryClient();
  const defaultRows = useMemo(() => {
    const unique = new Set<number>();
    for (const error of job.errors) {
      unique.add(error.row);
    }
    return Array.from(unique).sort((a, b) => a - b);
  }, [job.errors]);

  const [selectedRows, setSelectedRows] = useState<number[]>(() => defaultRows);
  const [additionalRows, setAdditionalRows] = useState('');
  const [issuedBy, setIssuedBy] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  const retryMutation = useMutation({
    mutationFn: ({ rows, issuedBy }: { rows: number[]; issuedBy?: string }) =>
      retryDeviceImportJobRows(job.id, { rows, issuedBy }),
    onSuccess: (summary) => {
      onSummaryChange(summary);
      setFormError(null);
      setSelectedRows(summary.errors.map((error) => error.row));
      setAdditionalRows('');
      setIssuedBy('');
      void queryClient.invalidateQueries({ queryKey: ['devices', 'import-jobs'] });
      void queryClient.invalidateQueries({ queryKey: ['devices', 'import-job', job.id] });
      onRetrySuccess(summary);
    },
    onError: (error: unknown) => {
      const message = error instanceof Error ? error.message : 'Failed to retry import rows';
      setFormError(message);
    },
  });

  const toggleRow = (row: number) => {
    setSelectedRows((prev) => {
      if (prev.includes(row)) {
        return prev.filter((value) => value !== row);
      }
      return [...prev, row].sort((a, b) => a - b);
    });
  };

  const handleSelectAll = () => {
    setSelectedRows(defaultRows);
  };

  const handleClearAll = () => {
    setSelectedRows([]);
    onSummaryChange(null);
  };

  const parseRowNumbers = () => {
    const manualTokens = additionalRows
      .split(/[\s,]+/)
      .map((token) => token.trim())
      .filter((token) => token.length);

    const manualRows = manualTokens
      .map((token) => Number(token))
      .filter((value) => Number.isFinite(value) && value > 0);

    const combined = new Set<number>([...selectedRows, ...manualRows]);
    return Array.from(combined).sort((a, b) => a - b);
  };

  const handleRetry = () => {
    if (retryMutation.isPending) {
      return;
    }

    const rows = parseRowNumbers();
    if (!rows.length) {
      setFormError('Select or enter at least one row to retry');
      return;
    }

    onSummaryChange(null);

    const trimmedIssuedBy = issuedBy.trim();
    setFormError(null);
    retryMutation.mutate({
      rows,
      issuedBy: trimmedIssuedBy.length ? trimmedIssuedBy : undefined,
    });
  };

  return (
    <section className="mt-8 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-slate-900">Retry failed rows</h3>
        <div className="flex gap-2 text-xs">
          <button
            type="button"
            onClick={handleSelectAll}
            className="rounded border border-slate-300 px-2 py-1 text-slate-600 transition hover:border-slate-400 hover:text-slate-700"
          >
            Select all
          </button>
          <button
            type="button"
            onClick={handleClearAll}
            className="rounded border border-slate-300 px-2 py-1 text-slate-600 transition hover:border-slate-400 hover:text-slate-700"
          >
            Clear
          </button>
        </div>
      </div>

      <p className="text-sm text-slate-600">
        Choose the rows to replay. Archived payloads will be reused when available. If some rows are
        not listed below (e.g. beyond the preview limit), enter them manually as a comma or
        whitespace separated list.
      </p>

      <div className="grid gap-2 md:grid-cols-2">
        {job.errors.map((error) => {
          const checked = selectedRows.includes(error.row);
          return (
            <label
              key={`${job.id}-retry-${error.row}`}
              className="flex items-start gap-2 rounded border border-slate-200 bg-slate-50 px-3 py-2"
            >
              <input
                type="checkbox"
                className="mt-1 size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                checked={checked}
                onChange={() => toggleRow(error.row)}
              />
              <span className="text-sm text-slate-700">
                <span className="font-semibold">Row {error.row}:</span> {error.message}
              </span>
            </label>
          );
        })}
      </div>

      <label className="flex flex-col gap-2 text-sm" htmlFor="retry-additional-rows">
        <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
          Additional row numbers
        </span>
        <input
          id="retry-additional-rows"
          type="text"
          value={additionalRows}
          onChange={(event) => setAdditionalRows(event.target.value)}
          placeholder="e.g. 17 18 29"
          className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
        />
        <span className="text-xs text-slate-500">
          Optional. Separate values with commas or whitespace.
        </span>
      </label>

      <label className="flex flex-col gap-2 text-sm" htmlFor="retry-issued-by">
        <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
          Issued by (optional)
        </span>
        <input
          id="retry-issued-by"
          type="text"
          value={issuedBy}
          onChange={(event) => setIssuedBy(event.target.value)}
          placeholder="MongoDB ObjectId of the operator"
          className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
        />
      </label>

      {formError ? (
        <p className="text-sm text-rose-600" role="alert">
          {formError}
        </p>
      ) : null}

      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={handleRetry}
          disabled={retryMutation.isPending}
          className="inline-flex items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {retryMutation.isPending ? 'Queuing retry…' : 'Create retry job'}
        </button>
        <span className="text-xs text-slate-500">
          Rows retried: {summary ? summary.errors.length : parseRowNumbers().length}
        </span>
      </div>

      {summary ? (
        <div className="rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
          <p>
            Retry job <ImportJobLink jobId={summary.jobId} /> created. Processed {summary.processed}{' '}
            row{summary.processed === 1 ? '' : 's'}, enrolled {summary.enrolled}, failed{' '}
            {summary.failed}.
          </p>
          {summary.errors.length ? (
            <p className="mt-1 text-xs text-emerald-900">
              {summary.errors.length} row{summary.errors.length === 1 ? '' : 's'} still need
              attention in the new job.
            </p>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
