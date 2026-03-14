import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  registerDevice,
  fetchDeviceStatus,
  type RegisterDevicePayload,
  type RegisterDeviceResponse,
  type DeviceStatusResponse,
  type DeviceCredentialEndpoint,
  type CredentialLifecycleState,
} from '../api/devices';
import {
  fetchAuthorities,
  fetchProjects,
  fetchStates,
  type AuthorityOption,
  type ProjectOption,
  type ProtocolVersionOption,
  type StateOption,
} from '../api/lookups';
import { usePollingGate } from '../session';
import { StatusBadge } from '../components/StatusBadge';
import { downloadJsonFile } from '../utils/download';

function formatOptionalTimestamp(value: string | null | undefined) {
  return value ? new Date(value).toLocaleString() : '—';
}

function formatDurationMs(value: number | null | undefined) {
  if (!value || value <= 0) {
    return '—';
  }

  const hours = value / 3600000;
  if (hours < 24) {
    return `${hours.toFixed(hours % 1 === 0 ? 0 : 1)} hr`;
  }

  const days = hours / 24;
  return `${days.toFixed(days % 1 === 0 ? 0 : 1)} day${days >= 2 ? 's' : ''}`;
}

function SpinnerIcon({ className = 'text-blue-600' }: { className?: string }) {
  return (
    <svg
      aria-hidden="true"
      className={`size-4 animate-spin ${className}`}
      viewBox="0 0 20 20"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
    >
      <path d="M10 3a7 7 0 1 1-4.95 2.05" />
    </svg>
  );
}

function SuccessIcon({ className = 'text-emerald-600' }: { className?: string }) {
  return (
    <svg
      aria-hidden="true"
      className={`size-5 ${className}`}
      viewBox="0 0 20 20"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M17 9a7 7 0 1 1-14 0 7 7 0 0 1 14 0Z" />
      <path d="m7.5 10.5 1.8 1.8 3.2-3.6" />
    </svg>
  );
}

type BaseFormFields = {
  imei: string;
  solarPumpVendorId: string;
  issuedBy: string;
};

type EnrollmentSelections = {
  stateId: string;
  stateAuthorityId: string;
  projectId: string;
  protocolVersionId: string;
};

type SubmissionContext = {
  state?: StateOption;
  authority?: AuthorityOption;
  project?: ProjectOption;
  protocol?: ProtocolVersionOption;
};

const initialFormFields: BaseFormFields = {
  imei: '',
  solarPumpVendorId: '',
  issuedBy: '',
};

const initialSelections: EnrollmentSelections = {
  stateId: '',
  stateAuthorityId: '',
  projectId: '',
  protocolVersionId: '',
};

type GovernmentFormFields = {
  enabled: boolean;
  protocol: 'mqtt' | 'mqtts';
  host: string;
  port: string;
  clientId: string;
  username: string;
  password: string;
};

const initialGovernmentForm: GovernmentFormFields = {
  enabled: false,
  protocol: 'mqtts',
  host: '',
  port: '',
  clientId: '',
  username: '',
  password: '',
};

type ResolvedCredentialBundle = {
  source: 'status' | 'initial';
  clientId: string;
  username: string;
  password: string;
  endpoints: DeviceCredentialEndpoint[];
  topics: {
    publish: string[];
    subscribe: string[];
  };
  issuedBy: string | null;
  validFrom: string | null;
  lifecycle: CredentialLifecycleState | null;
  originImportJobId: string | null;
  mqttAccessApplied: boolean | null;
};

function formatProvisioningLabel(status: RegisterDeviceResponse['mqttProvisioning']['status']) {
  switch (status) {
    case 'pending':
      return 'Pending';
    case 'in_progress':
      return 'In Progress';
    case 'applied':
      return 'Applied';
    case 'failed':
      return 'Failed';
    default:
      return status;
  }
}

function formatEnrollmentError(error: unknown): string {
  const raw = error instanceof Error ? error.message : 'Unable to enroll device.';
  const lower = raw.toLowerCase();

  if (lower.includes('state_id') || lower.includes('authority') || lower.includes('project_id')) {
    return `Enrollment failed: ${raw} Suggestion: verify selected State, Authority, Project, and Protocol still exist and match current admin hierarchy.`;
  }

  if (lower.includes('protocol') || lower.includes('server_vendor')) {
    return `Enrollment failed: ${raw} Suggestion: check protocol version and server vendor configuration for the selected project.`;
  }

  if (lower.includes('imei') || lower.includes('device')) {
    return `Enrollment failed: ${raw} Suggestion: ensure IMEI is valid and not already registered for a conflicting project.`;
  }

  return `Enrollment failed: ${raw}`;
}

export function DeviceEnrollmentPage() {
  const queryClient = useQueryClient();
  const [formFields, setFormFields] = useState<BaseFormFields>(initialFormFields);
  const [selections, setSelections] = useState<EnrollmentSelections>(initialSelections);
  const [result, setResult] = useState<RegisterDeviceResponse | null>(null);
  const [submissionContext, setSubmissionContext] = useState<SubmissionContext | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [governmentForm, setGovernmentForm] = useState<GovernmentFormFields>(initialGovernmentForm);
  const [enrolledDeviceUuid, setEnrolledDeviceUuid] = useState<string | null>(null);

  const statesQuery = useQuery({
    queryKey: ['states'],
    queryFn: fetchStates,
    staleTime: 5 * 60 * 1000,
  });
  const authoritiesQuery = useQuery({
    queryKey: ['authorities', selections.stateId],
    queryFn: () => fetchAuthorities(selections.stateId),
    enabled: Boolean(selections.stateId),
    staleTime: 5 * 60 * 1000,
  });
  const projectsQuery = useQuery({
    queryKey: ['projects', selections.stateId, selections.stateAuthorityId],
    queryFn: () =>
      fetchProjects({
        stateId: selections.stateId,
        stateAuthorityId: selections.stateAuthorityId,
      }),
    enabled: Boolean(selections.stateId && selections.stateAuthorityId),
    staleTime: 5 * 60 * 1000,
  });

  const mutation = useMutation({ mutationFn: registerDevice });

  const states = useMemo<StateOption[]>(() => statesQuery.data ?? [], [statesQuery.data]);
  const authorities = useMemo<AuthorityOption[]>(
    () => authoritiesQuery.data ?? [],
    [authoritiesQuery.data],
  );
  const projects = useMemo<ProjectOption[]>(() => projectsQuery.data ?? [], [projectsQuery.data]);

  const selectedState = useMemo(
    () => states.find((state) => state.id === selections.stateId),
    [states, selections.stateId],
  );

  const selectedAuthority = useMemo(
    () => authorities.find((authority) => authority.id === selections.stateAuthorityId),
    [authorities, selections.stateAuthorityId],
  );

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selections.projectId),
    [projects, selections.projectId],
  );

  const protocolOptions = useMemo<ProtocolVersionOption[]>(
    () => selectedProject?.protocolVersions ?? [],
    [selectedProject],
  );

  const selectedProtocol = useMemo(
    () => protocolOptions.find((protocol) => protocol.id === selections.protocolVersionId),
    [protocolOptions, selections.protocolVersionId],
  );

  const noProtocolsForProject = Boolean(
    selections.projectId && selectedProject && protocolOptions.length === 0,
  );

  const governmentDefaults = selectedProtocol?.governmentCredentialDefaults ?? null;

  const governmentDefaultEndpointSummary = useMemo(() => {
    if (!governmentDefaults || governmentDefaults.endpoints.length === 0) {
      return null;
    }

    return governmentDefaults.endpoints
      .map((endpoint) => `${endpoint.protocol}://${endpoint.host}:${endpoint.port}`)
      .join(', ');
  }, [governmentDefaults]);

  useEffect(() => {
    if (!governmentForm.enabled) {
      return;
    }

    if (!governmentDefaults || governmentDefaults.endpoints.length === 0) {
      setGovernmentForm((prev) => {
        if (!prev.host && !prev.port) {
          return prev;
        }
        return {
          ...prev,
          host: '',
          port: '',
        };
      });
      return;
    }

    const endpoint = governmentDefaults.endpoints[0];
    setGovernmentForm((prev) => {
      const host = endpoint.host;
      const port = endpoint.port ? String(endpoint.port) : '';
      if (prev.host === host && prev.port === port && prev.protocol === endpoint.protocol) {
        return prev;
      }

      return {
        ...prev,
        protocol: endpoint.protocol,
        host,
        port,
      };
    });
  }, [governmentDefaults, governmentForm.enabled]);

  const handleInputChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = event.target;
    setFormFields((prev) => ({
      ...prev,
      [name]: value,
    }));
    setFormError(null);
  };

  const handleStateSelect = (value: string) => {
    setSelections({
      stateId: value,
      stateAuthorityId: '',
      projectId: '',
      protocolVersionId: '',
    });
    setFormError(null);
    setResult(null);
    setSubmissionContext(null);
  };

  const handleAuthoritySelect = (value: string) => {
    setSelections((prev) => ({
      stateId: prev.stateId,
      stateAuthorityId: value,
      projectId: '',
      protocolVersionId: '',
    }));
    setFormError(null);
    setResult(null);
    setSubmissionContext(null);
  };

  const handleProjectSelect = (value: string) => {
    setSelections((prev) => ({
      ...prev,
      projectId: value,
      protocolVersionId: '',
    }));
    setFormError(null);
    setResult(null);
    setSubmissionContext(null);
  };

  const handleProtocolSelect = (value: string) => {
    setSelections((prev) => ({
      ...prev,
      protocolVersionId: value,
    }));
    setFormError(null);
    setResult(null);
    setSubmissionContext(null);
  };

  const handleGovernmentToggle = (event: React.ChangeEvent<HTMLInputElement>) => {
    const enabled = event.target.checked;
    if (enabled) {
      const endpoint = governmentDefaults?.endpoints[0];
      setGovernmentForm((prev) => ({
        ...prev,
        enabled: true,
        protocol: endpoint?.protocol ?? prev.protocol,
        host: endpoint?.host ?? '',
        port: endpoint?.port ? String(endpoint.port) : '',
      }));
    } else {
      setGovernmentForm(initialGovernmentForm);
    }
    setFormError(null);
  };

  const handleGovernmentFieldChange = (
    event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    const nextValue = name === 'protocol' ? (value as GovernmentFormFields['protocol']) : value;
    setGovernmentForm((prev) => ({
      ...prev,
      [name as keyof GovernmentFormFields]: nextValue,
    }));
    setFormError(null);
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!selectedProtocol) {
      setFormError('Select a protocol version before enrolling the device.');
      return;
    }

    const trimmedIssuedBy = formFields.issuedBy.trim();
    const payload: RegisterDevicePayload = {
      imei: formFields.imei.trim(),
      solarPumpVendorId: formFields.solarPumpVendorId.trim(),
      stateId: selections.stateId,
      stateAuthorityId: selections.stateAuthorityId,
      projectId: selections.projectId,
      protocolVersionId: selections.protocolVersionId,
      serverVendorId: selectedProtocol.serverVendorId,
      issuedBy: trimmedIssuedBy ? trimmedIssuedBy : undefined,
    };

    const missingField = Object.entries({
      imei: payload.imei,
      state: payload.stateId,
      authority: payload.stateAuthorityId,
      project: payload.projectId,
      protocol: payload.protocolVersionId,
      solarPumpVendor: payload.solarPumpVendorId,
    }).find(([, value]) => !value);

    if (missingField) {
      setFormError('Complete all required fields before enrolling the device.');
      return;
    }

    if (governmentForm.enabled) {
      const requiredGovFields: Array<[string, string]> = [
        ['Government host', governmentForm.host.trim()],
        ['Government port', governmentForm.port.trim()],
        ['Government client ID', governmentForm.clientId.trim()],
        ['Government username', governmentForm.username.trim()],
        ['Government password', governmentForm.password],
      ];

      const missingGov = requiredGovFields.find(([, value]) => !value);
      if (missingGov) {
        setFormError(`${missingGov[0]} is required when providing government credentials.`);
        return;
      }

      const portValue = Number(governmentForm.port.trim());
      if (!Number.isFinite(portValue) || portValue <= 0) {
        setFormError('Government port must be a positive number.');
        return;
      }

      payload.governmentCredentials = {
        clientId: governmentForm.clientId.trim(),
        username: governmentForm.username.trim(),
        password: governmentForm.password,
        endpoints: [
          {
            protocol: governmentForm.protocol,
            host: governmentForm.host.trim(),
            port: portValue,
            url: `${governmentForm.protocol}://${governmentForm.host.trim()}:${portValue}`,
          },
        ],
        topics: {
          publish: governmentDefaults?.topics.publish ?? [],
          subscribe: governmentDefaults?.topics.subscribe ?? [],
        },
        metadata: {
          source: 'ui-manual-entry',
          defaultsApplied: Boolean(governmentDefaults),
        },
      };
    }

    const contextSnapshot: SubmissionContext = {
      state: selectedState,
      authority: selectedAuthority,
      project: selectedProject,
      protocol: selectedProtocol,
    };

    setResult(null);
    setSubmissionContext(null);
    setFormError(null);
    setEnrolledDeviceUuid(null);
    queryClient.removeQueries({ queryKey: ['device-status'] });

    mutation.mutate(payload, {
      onSuccess: (data) => {
        setResult(data);
        setSubmissionContext(contextSnapshot);
        setEnrolledDeviceUuid(data.device.id);
        queryClient.invalidateQueries({ queryKey: ['device-status', data.device.id] });
      },
      onError: (error) => {
        setFormError(formatEnrollmentError(error));
      },
    });
  };

  const handleReset = () => {
    setFormFields(initialFormFields);
    setSelections(initialSelections);
    setResult(null);
    setSubmissionContext(null);
    setFormError(null);
    setGovernmentForm(initialGovernmentForm);
    setEnrolledDeviceUuid(null);
    queryClient.removeQueries({ queryKey: ['device-status'] });
    mutation.reset();
  };

  const canSubmit = Boolean(
    formFields.imei.trim() &&
      formFields.solarPumpVendorId.trim() &&
      selections.stateId &&
      selections.stateAuthorityId &&
      selections.projectId &&
      selections.protocolVersionId &&
      !noProtocolsForProject,
  );

  const provisioningInfo = result?.mqttProvisioning ?? null;
  const initialLocalCredentials = result?.credentials ?? null;
  const statusPollingGate = usePollingGate('device-enrollment-status', {
    isActive: Boolean(enrolledDeviceUuid),
  });
  const deviceStatusQuery = useQuery<DeviceStatusResponse, Error>({
    queryKey: ['device-status', enrolledDeviceUuid],
    queryFn: () => fetchDeviceStatus(enrolledDeviceUuid!),
    enabled: Boolean(enrolledDeviceUuid) && statusPollingGate.enabled,
    refetchInterval: (query) => {
      if (!enrolledDeviceUuid || !statusPollingGate.enabled) {
        return false;
      }

      const status = query.state.data?.mqttProvisioning?.status ?? provisioningInfo?.status;
      return status === 'pending' || status === 'in_progress' ? 3_000 : false;
    },
    refetchOnWindowFocus: false,
  });
  const deviceStatus = enrolledDeviceUuid ? (deviceStatusQuery.data ?? null) : null;
  const activeLocalCredentials = deviceStatus?.activeCredentials.local ?? null;
  const resolvedLocalCredentials: ResolvedCredentialBundle | null = useMemo(() => {
    if (activeLocalCredentials) {
      const appliedFromStatus = activeLocalCredentials.mqttAccess?.applied ?? null;
      return {
        source: 'status',
        clientId: activeLocalCredentials.clientId,
        username: activeLocalCredentials.username,
        password: activeLocalCredentials.password,
        endpoints: activeLocalCredentials.endpoints,
        topics: activeLocalCredentials.topics,
        issuedBy: activeLocalCredentials.issuedBy ?? null,
        validFrom: activeLocalCredentials.validFrom ?? null,
        lifecycle: activeLocalCredentials.lifecycle ?? null,
        originImportJobId: activeLocalCredentials.originImportJobId ?? null,
        mqttAccessApplied: appliedFromStatus,
      } satisfies ResolvedCredentialBundle;
    }

    if (initialLocalCredentials) {
      const appliedFromRegistration = initialLocalCredentials.mqttAccess?.applied ?? null;
      return {
        source: 'initial',
        clientId: initialLocalCredentials.clientId,
        username: initialLocalCredentials.username,
        password: initialLocalCredentials.password,
        endpoints: initialLocalCredentials.endpoints,
        topics: initialLocalCredentials.topics,
        issuedBy: null,
        validFrom: null,
        lifecycle: null,
        originImportJobId: null,
        mqttAccessApplied: appliedFromRegistration,
      } satisfies ResolvedCredentialBundle;
    }

    return null;
  }, [activeLocalCredentials, initialLocalCredentials]);
  const liveProvisioning = deviceStatus?.mqttProvisioning ?? provisioningInfo;
  const isProvisioningPending =
    liveProvisioning?.status === 'pending' || liveProvisioning?.status === 'in_progress';

  const handleDownloadCredentials = () => {
    if (!resolvedLocalCredentials || !enrolledDeviceUuid) {
      return;
    }

    downloadJsonFile(`device-${enrolledDeviceUuid}-credentials.json`, {
      deviceUuid: enrolledDeviceUuid,
      clientId: resolvedLocalCredentials.clientId,
      username: resolvedLocalCredentials.username,
      password: resolvedLocalCredentials.password,
      endpoints: resolvedLocalCredentials.endpoints,
      topics: resolvedLocalCredentials.topics,
      issuedBy: resolvedLocalCredentials.issuedBy,
      validFrom: resolvedLocalCredentials.validFrom,
      lifecycle: resolvedLocalCredentials.lifecycle,
      originImportJobId: resolvedLocalCredentials.originImportJobId,
      generatedAt: new Date().toISOString(),
    });
  };

  const canDownloadCredentials = Boolean(resolvedLocalCredentials && enrolledDeviceUuid);

  const provisioningStatusAlert = (() => {
    if (!enrolledDeviceUuid || !liveProvisioning) {
      return null;
    }

    if (liveProvisioning.status === 'failed') {
      return (
        <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          <p className="font-semibold">Broker synchronization failed.</p>
          {liveProvisioning.lastError?.message && (
            <p className="mt-1 text-xs text-red-600">
              {liveProvisioning.lastError.message}
              {liveProvisioning.lastError.status && (
                <span className="ml-1">(HTTP {liveProvisioning.lastError.status})</span>
              )}
              {liveProvisioning.lastError.endpoint && (
                <span className="ml-1 text-slate-500">{liveProvisioning.lastError.endpoint}</span>
              )}
            </p>
          )}
          <p className="mt-2 text-xs">
            Retry from the provisioning tools once the broker issue is resolved. The device will
            keep using any previously issued credentials.
          </p>
        </div>
      );
    }

    const downloadButton = canDownloadCredentials ? (
      <button
        type="button"
        onClick={handleDownloadCredentials}
        className="inline-flex items-center justify-center rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
      >
        Download credentials JSON
      </button>
    ) : null;

    if (liveProvisioning.status === 'applied') {
      return (
        <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex items-start gap-3 md:items-center">
              <SuccessIcon />
              <div>
                <p className="font-semibold">Provisioning complete.</p>
                <p className="mt-1">
                  {resolvedLocalCredentials
                    ? resolvedLocalCredentials.source === 'status'
                      ? 'Credentials now reflect the broker-confirmed bundle.'
                      : 'Initial credentials remain active; broker metadata will reconcile on the next refresh.'
                    : 'Credentials will appear below after the next status refresh.'}
                </p>
              </div>
            </div>
            {downloadButton}
          </div>
        </div>
      );
    }

    if (isProvisioningPending) {
      return (
        <div className="rounded-md border border-blue-200 bg-blue-50 p-4 text-sm text-blue-700">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex items-start gap-3 md:items-center">
              <SpinnerIcon />
              <div>
                <p className="font-semibold">Provisioning in progress…</p>
                <p className="mt-1 text-xs text-blue-600">
                  We’re syncing with EMQX. This may take up to 30 seconds.
                </p>
              </div>
            </div>
            {downloadButton}
          </div>
          {deviceStatusQuery.isFetching && (
            <p className="mt-2 text-xs text-blue-500">Refreshing status…</p>
          )}
          {resolvedLocalCredentials && (
            <p className="mt-2 text-xs text-blue-500">
              Initial credentials are ready below if installers need to proceed while the broker
              catches up.
            </p>
          )}
          {liveProvisioning.nextAttemptAt && (
            <p className="mt-2 text-xs text-blue-500">
              Next attempt {formatOptionalTimestamp(liveProvisioning.nextAttemptAt)}.
            </p>
          )}
        </div>
      );
    }

    return (
      <div className="rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <p className="font-semibold">Broker synchronization pending.</p>
            <p className="mt-1 text-slate-600">
              Provisioning metadata has been queued; we’ll refresh the status shortly.
            </p>
            {resolvedLocalCredentials && (
              <p className="mt-2 text-xs text-slate-500">
                Initial credentials are available below while we wait for EMQX to confirm the
                bundle.
              </p>
            )}
          </div>
          {downloadButton}
        </div>
      </div>
    );
  })();

  return (
    <div className="space-y-6">
      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold">Enroll Device</h2>
        <p className="text-sm text-slate-600">
          Select the project hierarchy to pull the correct MQTT metadata, then issue credentials for
          the installer.
        </p>
        <form className="mt-6 grid gap-4 md:grid-cols-2" onSubmit={handleSubmit}>
          <InputField
            label="IMEI"
            name="imei"
            value={formFields.imei}
            onChange={handleInputChange}
            required
          />
          <SelectField
            label="State"
            value={selections.stateId}
            onChange={handleStateSelect}
            options={states.map((state) => ({
              value: state.id,
              label: state.name,
              description: state.isoCode ?? undefined,
            }))}
            placeholder={statesQuery.isLoading ? 'Loading states…' : 'Select state'}
            disabled={statesQuery.isLoading}
            required
          />
          <SelectField
            label="State Authority"
            value={selections.stateAuthorityId}
            onChange={handleAuthoritySelect}
            options={authorities.map((authority) => ({
              value: authority.id,
              label: authority.name,
              description:
                authority.projectCount > 0
                  ? `${authority.projectCount} project${authority.projectCount > 1 ? 's' : ''}`
                  : 'No projects',
            }))}
            placeholder={
              selections.stateId
                ? authoritiesQuery.isLoading
                  ? 'Loading authorities…'
                  : authorities.length
                    ? 'Select authority'
                    : 'No authorities available'
                : 'Select state first'
            }
            disabled={!selections.stateId || authoritiesQuery.isLoading}
            required
          />
          <SelectField
            label="Project"
            value={selections.projectId}
            onChange={handleProjectSelect}
            options={projects.map((project) => ({
              value: project.id,
              label: project.name,
              description: `${project.protocolVersions.length} protocol${
                project.protocolVersions.length === 1 ? '' : 's'
              }`,
            }))}
            placeholder={
              selections.stateAuthorityId
                ? projectsQuery.isLoading
                  ? 'Loading projects…'
                  : projects.length
                    ? 'Select project'
                    : 'No projects available'
                : 'Select authority first'
            }
            disabled={!selections.stateAuthorityId || projectsQuery.isLoading}
            required
          />
          <SelectField
            label="Protocol Version"
            value={selections.protocolVersionId}
            onChange={handleProtocolSelect}
            options={protocolOptions.map((protocol) => ({
              value: protocol.id,
              label: protocol.version,
              description: protocol.serverVendorName ?? `Vendor ID ${protocol.serverVendorId}`,
            }))}
            placeholder={
              selections.projectId
                ? protocolOptions.length
                  ? 'Select protocol version'
                  : 'No protocols available'
                : 'Select project first'
            }
            disabled={!selections.projectId || protocolOptions.length === 0}
            required
            error={
              noProtocolsForProject
                ? 'No protocol versions configured for this project. Update the metadata before provisioning.'
                : undefined
            }
          />
          <InputField
            label="Solar Pump Vendor ID"
            name="solarPumpVendorId"
            value={formFields.solarPumpVendorId}
            onChange={handleInputChange}
            placeholder="24-char hex ObjectId"
            required
          />
          <InputField
            label="Issued By (optional)"
            name="issuedBy"
            value={formFields.issuedBy}
            onChange={handleInputChange}
            placeholder="Operator user ID"
          />
          <div className="rounded-md border border-slate-200 bg-slate-50 p-4 md:col-span-2">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <label className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <input
                  type="checkbox"
                  className="size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                  checked={governmentForm.enabled}
                  onChange={handleGovernmentToggle}
                />
                Provide government server credentials
              </label>
              {governmentDefaults && (
                <span className="text-xs text-slate-500">
                  Prefill endpoints: {governmentDefaultEndpointSummary ?? 'n/a'}
                </span>
              )}
            </div>
            {!governmentForm.enabled && governmentDefaults && (
              <p className="mt-2 text-xs text-slate-500">
                Toggle on to auto-fill these values; adjust per device if required.
              </p>
            )}
            {governmentForm.enabled && (
              <div className="mt-4 grid gap-4 md:grid-cols-3">
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Protocol</span>
                  <select
                    name="protocol"
                    value={governmentForm.protocol}
                    onChange={handleGovernmentFieldChange}
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  >
                    <option value="mqtt">mqtt</option>
                    <option value="mqtts">mqtts</option>
                  </select>
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Host</span>
                  <input
                    type="text"
                    name="host"
                    value={governmentForm.host}
                    onChange={handleGovernmentFieldChange}
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    placeholder="gov.example"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Port</span>
                  <input
                    type="text"
                    name="port"
                    value={governmentForm.port}
                    onChange={handleGovernmentFieldChange}
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    placeholder="8883"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-3">
                  <span>Client ID</span>
                  <input
                    type="text"
                    name="clientId"
                    value={governmentForm.clientId}
                    onChange={handleGovernmentFieldChange}
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    autoComplete="off"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Username</span>
                  <input
                    type="text"
                    name="username"
                    value={governmentForm.username}
                    onChange={handleGovernmentFieldChange}
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    autoComplete="off"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Password</span>
                  <input
                    type="text"
                    name="password"
                    value={governmentForm.password}
                    onChange={handleGovernmentFieldChange}
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    autoComplete="off"
                    required
                  />
                </label>
              </div>
            )}
          </div>
          <div className="flex items-center gap-3 pt-2 md:col-span-2">
            <button
              type="submit"
              className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
              disabled={!canSubmit || mutation.isPending}
            >
              {mutation.isPending ? 'Provisioning…' : 'Enroll Device'}
            </button>
            <button
              type="button"
              className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              onClick={handleReset}
            >
              Reset
            </button>
          </div>
        </form>
        {formError && (
          <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
            {formError}
          </div>
        )}
        {mutation.isError && (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {(mutation.error as Error).message}
          </div>
        )}
      </section>

      {result && liveProvisioning && (
        <section className="rounded-lg border border-emerald-200 bg-white p-6 shadow-sm">
          <h3 className="text-base font-semibold text-emerald-700">Enrollment Result</h3>
          <p className="mt-2 text-sm text-slate-600">
            MQTT credentials are issued asynchronously. Track the provisioning job below and
            download secrets once status becomes <strong>Applied</strong>.
          </p>
          {submissionContext && (
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              {submissionContext.state && (
                <Detail label="State" value={submissionContext.state.name} />
              )}
              {submissionContext.authority && (
                <Detail label="State Authority" value={submissionContext.authority.name} />
              )}
              {submissionContext.project && (
                <Detail label="Project" value={submissionContext.project.name} />
              )}
              {submissionContext.protocol && (
                <Detail
                  label="Protocol Version"
                  value={`${submissionContext.protocol.version} — ${
                    submissionContext.protocol.serverVendorName ??
                    `Vendor ID ${submissionContext.protocol.serverVendorId}`
                  }`}
                />
              )}
            </div>
          )}
          <div className="mt-6 grid gap-4 md:grid-cols-2">
            <Detail label="Device UUID" value={result.device.id} />
            <Detail label="IMEI" value={result.device.imei} />
            <Detail label="Job ID" value={liveProvisioning.jobId} />
            <div>
              <span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">
                Status
              </span>
              <StatusBadge
                status={liveProvisioning.status}
                label={formatProvisioningLabel(liveProvisioning.status)}
              />
            </div>
          </div>
          <div className="mt-6 grid gap-4 md:grid-cols-3">
            <Detail
              label="Attempts"
              value={`${liveProvisioning.attemptCount} / ${liveProvisioning.maxAttempts}`}
            />
            <Detail
              label="Base Retry"
              value={`${Math.round(liveProvisioning.baseRetryDelayMs / 1000)} sec`}
            />
            <Detail
              label="Next Attempt"
              value={formatOptionalTimestamp(liveProvisioning.nextAttemptAt)}
            />
            <Detail
              label="Last Attempt"
              value={formatOptionalTimestamp(liveProvisioning.lastAttemptAt)}
            />
            {liveProvisioning.lastError && (
              <div className="md:col-span-3">
                <span className="block text-xs font-semibold uppercase tracking-wide text-red-600">
                  Last Error
                </span>
                <p className="mt-1 text-sm text-red-600">
                  {liveProvisioning.lastError.message}
                  {liveProvisioning.lastError.status && (
                    <span className="ml-2 text-xs">(HTTP {liveProvisioning.lastError.status})</span>
                  )}
                  {liveProvisioning.lastError.endpoint && (
                    <span className="ml-2 text-xs text-slate-500">
                      {liveProvisioning.lastError.endpoint}
                    </span>
                  )}
                </p>
              </div>
            )}
          </div>
          <div className="mt-6 space-y-6">
            {provisioningStatusAlert}
            {resolvedLocalCredentials ? (
              <div className="space-y-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <Detail label="Client ID" value={resolvedLocalCredentials.clientId} />
                  <Detail label="Username" value={resolvedLocalCredentials.username} />
                  <Detail label="Password" value={resolvedLocalCredentials.password} secret />
                </div>
                <div>
                  <h4 className="text-sm font-semibold text-slate-700">Endpoints</h4>
                  <ul className="mt-2 space-y-2 text-sm text-slate-700">
                    {resolvedLocalCredentials.endpoints.map((endpoint) => (
                      <li
                        key={`${endpoint.protocol}-${endpoint.host}-${endpoint.port}`}
                        className="rounded border border-slate-200 p-3"
                      >
                        <span className="font-medium uppercase">{endpoint.protocol}</span>
                        <span className="ml-2">
                          {endpoint.url ?? `${endpoint.host}:${endpoint.port}`}
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  <TopicList
                    title="Publish Topics"
                    topics={resolvedLocalCredentials.topics.publish}
                  />
                  <TopicList
                    title="Subscribe Topics"
                    topics={resolvedLocalCredentials.topics.subscribe}
                  />
                </div>
                {(resolvedLocalCredentials.mqttAccessApplied !== null ||
                  resolvedLocalCredentials.lifecycle ||
                  resolvedLocalCredentials.validFrom ||
                  resolvedLocalCredentials.issuedBy ||
                  resolvedLocalCredentials.originImportJobId) && (
                  <div className="rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600">
                    <p className="font-semibold text-slate-700">Credential Metadata</p>
                    <ul className="mt-2 space-y-1">
                      {resolvedLocalCredentials.mqttAccessApplied !== null && (
                        <li>
                          Broker access applied:{' '}
                          {resolvedLocalCredentials.mqttAccessApplied ? 'Yes' : 'No'}
                        </li>
                      )}
                      {resolvedLocalCredentials.lifecycle && (
                        <li>Lifecycle: {resolvedLocalCredentials.lifecycle}</li>
                      )}
                      {resolvedLocalCredentials.validFrom && (
                        <li>
                          Valid from {formatOptionalTimestamp(resolvedLocalCredentials.validFrom)}
                        </li>
                      )}
                      {resolvedLocalCredentials.issuedBy && (
                        <li>Issued by {resolvedLocalCredentials.issuedBy}</li>
                      )}
                      {resolvedLocalCredentials.originImportJobId && (
                        <li>Origin job: {resolvedLocalCredentials.originImportJobId}</li>
                      )}
                    </ul>
                  </div>
                )}
              </div>
            ) : (
              <div className="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
                Broker synchronization has not issued credentials yet. We keep polling every few
                seconds and will surface the bundle here once available.
              </div>
            )}
          </div>
          {enrolledDeviceUuid && (
            <div className="mt-6 rounded-md border border-slate-200 bg-slate-50 p-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h4 className="text-sm font-semibold text-slate-800">Device Status</h4>
                <span className="text-xs text-slate-500">UUID {enrolledDeviceUuid}</span>
              </div>
              {deviceStatusQuery.isLoading && (
                <p className="mt-2 text-xs text-slate-500">Syncing status…</p>
              )}
              {deviceStatusQuery.isError && (
                <p className="mt-2 text-xs text-red-600">
                  {deviceStatusQuery.error?.message || 'Unable to load device status'}
                </p>
              )}
              {deviceStatus && (
                <div className="mt-3 grid gap-3 md:grid-cols-4">
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      IMEI
                    </span>
                    {deviceStatus.device.imei}
                  </div>
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      Device
                    </span>
                    <StatusBadge status={deviceStatus.device.status} />
                  </div>
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      Connectivity
                    </span>
                    <div className="flex items-center gap-2">
                      <StatusBadge status={deviceStatus.device.connectivityStatus} />
                      <span className="text-[11px] text-slate-500">
                        {formatOptionalTimestamp(deviceStatus.device.connectivityUpdatedAt)}
                      </span>
                    </div>
                  </div>
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      Offline Threshold
                    </span>
                    {formatDurationMs(deviceStatus.device.offlineThresholdMs)}
                  </div>
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      Last Telemetry
                    </span>
                    {formatOptionalTimestamp(deviceStatus.device.lastTelemetryAt)}
                  </div>
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      Last Heartbeat
                    </span>
                    {formatOptionalTimestamp(deviceStatus.device.lastHeartbeatAt)}
                  </div>
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      Notifications
                    </span>
                    {deviceStatus.device.offlineNotificationChannelCount}
                  </div>
                  <div>
                    <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                      Protocol
                    </span>
                    {deviceStatus.device.protocolVersion
                      ? `v${deviceStatus.device.protocolVersion.version}`
                      : '—'}
                  </div>
                </div>
              )}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

type InputFieldProps = {
  label: string;
  name: keyof BaseFormFields;
  value: string;
  onChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
  placeholder?: string;
  required?: boolean;
};

function InputField({ label, name, value, onChange, placeholder, required }: InputFieldProps) {
  return (
    <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
      <span>{label}</span>
      <input
        className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
        name={name}
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        required={required}
        autoComplete="off"
      />
    </label>
  );
}

type SelectFieldProps = {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<{ value: string; label: string; description?: string }>;
  placeholder: string;
  disabled?: boolean;
  required?: boolean;
  error?: string;
};

function SelectField({
  label,
  value,
  onChange,
  options,
  placeholder,
  disabled,
  required,
  error,
}: SelectFieldProps) {
  return (
    <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
      <span>{label}</span>
      <select
        className={`rounded-md border px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500 ${
          disabled ? 'bg-slate-100 text-slate-500' : 'bg-white'
        }`}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        required={required}
      >
        <option value="" disabled>
          {placeholder}
        </option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
            {option.description ? ` — ${option.description}` : ''}
          </option>
        ))}
      </select>
      {error && <span className="text-xs text-amber-700">{error}</span>}
    </label>
  );
}

type DetailProps = {
  label: string;
  value: string;
  secret?: boolean;
};

function Detail({ label, value, secret }: DetailProps) {
  return (
    <div className="rounded border border-slate-200 p-3">
      <p className="text-xs uppercase tracking-wide text-slate-500">{label}</p>
      <p className={`mt-1 text-sm ${secret ? 'font-semibold text-emerald-700' : 'text-slate-800'}`}>
        {value}
      </p>
    </div>
  );
}

type TopicListProps = {
  title: string;
  topics: string[];
};

function TopicList({ title, topics }: TopicListProps) {
  return (
    <div>
      <h4 className="text-sm font-semibold text-slate-700">{title}</h4>
      <ul className="mt-2 space-y-2 text-sm text-slate-700">
        {topics.map((topic) => (
          <li key={topic} className="rounded border border-slate-200 p-2">
            {topic}
          </li>
        ))}
      </ul>
    </div>
  );
}
