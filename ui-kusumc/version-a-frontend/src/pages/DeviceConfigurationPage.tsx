import {
  ChangeEvent,
  FormEvent,
  ReactNode,
  RefObject,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useLocation } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  acknowledgeDeviceConfiguration,
  fetchPendingDeviceConfiguration,
  importDeviceConfigurationsCsv,
  queueDeviceConfiguration,
  type DeviceConfigurationRecord,
  type ImportDeviceConfigurationsResult,
  type QueueDeviceConfigurationPayload,
  type QueueDeviceConfigurationResponse,
} from '../api/deviceConfigurations';
import {
  fetchDeviceList,
  fetchDeviceStatus,
  issueDeviceCommand,
  acknowledgeDeviceCommand,
  fetchDeviceCommandHistory,
  resyncDeviceMqttProvisioning,
  retryDeviceMqttProvisioning,
  revokeDeviceCredentials,
  rotateDeviceCredentials,
  upsertDeviceGovernmentCredentials,
  type ActiveCredentialSummary,
  type DeviceCredentialBundle,
  type DeviceCredentialHistoryItem,
  type CredentialLifecycleHistoryEntry,
  type CredentialLifecycleState,
  type DeviceListItem,
  type DeviceStatusResponse,
  type GovernmentCredentialBundle,
  type GovernmentCredentialPayload,
  type MqttProvisioningInfo,
  type ProtocolSelectorSummary,
  type IssueDeviceCommandPayload,
  type IssueDeviceCommandResponse,
  type AcknowledgeDeviceCommandPayload,
  type AcknowledgeDeviceCommandResponse,
  type FetchDeviceCommandHistoryParams,
  type FetchDeviceCommandHistoryResponse,
  type DeviceCommandHistoryRecord,
  type DeviceCommandHistoryEvent,
  type ResyncDeviceMqttProvisioningResponse,
  type RetryMqttProvisioningResponse,
  type RevokeDeviceCredentialsPayload,
  type RevokeDeviceCredentialsResponse,
  type RotateDeviceCredentialsPayload,
  type RotateDeviceCredentialsResponse,
  getBeneficiary,
  getInstallation,
  getVfdConfig,
  setBeneficiary,
  setInstallation,
  setVfdConfig,
} from '../api/devices';
import { fetchVfdModels, type CommandDefinition, type VfdModel } from '../api/vfd';
import { ImportJobLink } from '../components/ImportJobLink';
import { StatusBadge } from '../components/StatusBadge';
import { useAuth } from '../auth';
import { usePollingGate } from '../session';
import { downloadBlob } from '../utils/download';
import { createZipBlob } from '../utils/zipBuilder';
import { Link } from 'react-router-dom';

const PROVISIONING_STALLED_THRESHOLD_MS = 10 * 60 * 1000;
const DEVICE_CREDENTIAL_GUARD_MESSAGE =
  'Device credential management requires the devices:credentials capability or super-admin access.';
// Backend caps device list queries at 100 records (see deviceListQuerySchema in devices.routes.ts).
const BULK_DEVICE_LIST_LIMIT = 100;

type DeviceConfigActionKey =
  | 'queue-config'
  | 'ack-config'
  | 'csv-import'
  | 'rotate-credentials'
  | 'revoke-credentials'
  | 'resync-broker'
  | 'issue-command'
  | 'ack-command'
  | 'save-government-credentials'
  | 'retry-provisioning'
  | 'set-vfd-config'
  | 'get-vfd-config'
  | 'set-beneficiary'
  | 'get-beneficiary'
  | 'set-installation'
  | 'get-installation'
  | 'load-command-history'
  | 'load-pending-config'
  | 'bulk-rotate-credentials';

function formatDeviceConfigActionError(
  action: DeviceConfigActionKey,
  error: unknown,
  fallback: string,
): string {
  const base = error instanceof Error && error.message ? error.message : fallback;
  if (!base || /suggestion:/i.test(base)) {
    return base || fallback;
  }

  const normalized = base.toLowerCase();
  if (
    normalized.includes('forbidden') ||
    normalized.includes('unauthorized') ||
    normalized.includes('capability')
  ) {
    return `${base} Suggestion: Verify required role capabilities and retry with an authorized session.`;
  }

  const hintByAction: Record<DeviceConfigActionKey, string> = {
    'queue-config': 'Confirm device UUID, selected VFD model, and queue payload fields (QoS/overrides).',
    'ack-config': 'Ensure message identifiers match pending configuration records before acknowledging.',
    'csv-import': 'Validate CSV headers and row format, then retry import with a small sample file first.',
    'rotate-credentials': 'Check device UUID and credential lifecycle state, then retry credential rotation.',
    'revoke-credentials': 'Confirm credential type and reason, then retry revocation for the target device.',
    'resync-broker': 'Verify EMQX/broker connectivity and device ACL provisioning state before retrying.',
    'issue-command': 'Validate command name/payload JSON and ensure the target device can receive commands.',
    'ack-command': 'Use the latest msgid from command history and send acknowledgement for that exact command.',
    'save-government-credentials': 'Validate endpoint JSON/topic fields and ensure protocol metadata defaults are aligned.',
    'retry-provisioning': 'Check provisioning job status on device status panel, then retry when backend is reachable.',
    'set-vfd-config': 'Verify VFD payload schema keys and try a GET action first to compare expected shape.',
    'get-vfd-config': 'Confirm device UUID and VFD connectivity, then retry reading current VFD configuration.',
    'set-beneficiary': 'Validate beneficiary payload keys and retry after checking required identity fields.',
    'get-beneficiary': 'Confirm beneficiary data exists for this device and retry fetch.',
    'set-installation': 'Verify installation payload fields and identifiers before issuing set-installation command.',
    'get-installation': 'Confirm installation data has been provisioned for this device and retry fetch.',
    'load-command-history': 'Confirm device UUID and reduce history limit/filter scope before retrying.',
    'load-pending-config': 'Check device UUID and ensure configuration queue has pending entries.',
    'bulk-rotate-credentials': 'Review per-device provisioning state and rerun bulk action for failed devices only.',
  };

  const hint = hintByAction[action];
  return `${base} Suggestion: ${hint}`;
}

type BundleFlavor = 'local' | 'government';

type BundleCopyFeedback = Record<BundleFlavor, { message: string | null; error: string | null }>;

const defaultQueueForm = {
  deviceUuid: '',
  protocolVersionIdFilter: '',
  vfdModelId: '',
  transport: 'mqtt' as 'mqtt' | 'https',
  issuedBy: '',
  qos: '',
  overridesJson: '',
};

type PendingState = {
  deviceUuid: string;
  lastFetchedAt: Date | null;
};

const defaultAckForm = {
  deviceUuid: '',
  status: 'acknowledged' as 'acknowledged' | 'failed',
  msgid: '',
  receivedAt: '',
  payloadJson: '',
};

const defaultCommandIssueForm = {
  deviceUuid: '',
  commandName: '',
  payloadJson: '',
  qos: '',
  timeoutSeconds: '',
  issuedBy: '',
  simulatorSessionToken: '',
};

const defaultCommandAckForm = {
  deviceUuid: '',
  msgid: '',
  status: 'acknowledged' as 'acknowledged' | 'failed',
  payloadJson: '',
  receivedAt: '',
};

type ConfigKind = 'vfd' | 'beneficiary' | 'installation';

const defaultConfigForms: Record<ConfigKind, { deviceUuid: string; payloadJson: string }> = {
  vfd: { deviceUuid: '', payloadJson: '{}' },
  beneficiary: { deviceUuid: '', payloadJson: '{}' },
  installation: { deviceUuid: '', payloadJson: '{}' },
};

const defaultCommandHistoryFilters = {
  deviceUuid: '',
  limit: '25',
  statuses: {
    pending: true,
    acknowledged: true,
    failed: true,
  },
};

const defaultCsvForm = {
  csv: '',
  transport: 'mqtt' as 'mqtt' | 'https',
  issuedBy: '',
};

const defaultRotationForm = {
  deviceUuid: '',
  reason: '',
  issuedBy: '',
};

const defaultGovernmentForm = {
  deviceUuid: '',
  clientId: '',
  username: '',
  password: '',
  endpointsJson: '',
  publishTopicsRaw: '',
  subscribeTopicsRaw: '',
  metadataJson: '',
  issuedBy: '',
};

type GovernmentFormState = typeof defaultGovernmentForm;

const defaultRevokeForm = {
  deviceUuid: '',
  credentialType: 'local' as 'local' | 'government',
  issuedBy: '',
  reason: '',
};

type RevokeFormState = typeof defaultRevokeForm;

type CommandIssueFormState = typeof defaultCommandIssueForm;

type CommandAckFormState = typeof defaultCommandAckForm;

type CommandHistoryFiltersState = typeof defaultCommandHistoryFilters;

type CommandHistoryStatusKey = keyof CommandHistoryFiltersState['statuses'];

const COMMAND_HISTORY_STATUS_OPTIONS: CommandHistoryStatusKey[] = [
  'pending',
  'acknowledged',
  'failed',
];

const defaultResyncForm = {
  deviceUuid: '',
  reason: '',
};

type ResyncFormState = typeof defaultResyncForm;

type BulkCredentialStatus = 'pending' | 'in_progress' | 'success' | 'error';

type BulkCredentialResult = {
  status: BulkCredentialStatus;
  error?: string;
  credentials?: DeviceCredentialBundle | null;
  deviceImei?: string | null;
};

type BulkRotationState = {
  status: 'running' | 'completed';
  devices: DeviceListItem[];
  results: Record<string, BulkCredentialResult>;
  startedAt: Date;
  completedAt: Date | null;
  cancelled: boolean;
};

type DeviceConfigurationView = 'internal' | 'government' | 'drive';

export type DeviceConfigurationPageProps = {
  view?: DeviceConfigurationView;
};

export function DeviceConfigurationPage({ view }: DeviceConfigurationPageProps = {}) {
  const location = useLocation();
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const canManageDeviceCredentials =
    hasCapability('devices:credentials') || hasCapability('admin:all');
  const activeView = useMemo<null | DeviceConfigurationView>(() => {
    if (view) {
      return view;
    }

    const viewParam = new URLSearchParams(location.search).get('view');
    if (viewParam === 'internal' || viewParam === 'government' || viewParam === 'drive') {
      return viewParam;
    }
    return null;
  }, [view, location.search]);
  const showInternalSections = activeView === null || activeView === 'internal';
  const showGovernmentSection = activeView === null || activeView === 'government';
  const showDriveSection = activeView === null || activeView === 'drive';
  const [deviceSearch, setDeviceSearch] = useState('');
  const [selectedDeviceUuids, setSelectedDeviceUuids] = useState<Set<string>>(new Set());
  const singleSelectedDeviceUuid = useMemo(() => {
    if (selectedDeviceUuids.size === 1) {
      const [first] = Array.from(selectedDeviceUuids);
      return first;
    }
    return '';
  }, [selectedDeviceUuids]);
  const headerCheckboxRef = useRef<HTMLInputElement>(null);
  const [bulkRotationState, setBulkRotationState] = useState<BulkRotationState | null>(null);
  const bulkRotationCancelRef = useRef(false);
  const [bulkActionError, setBulkActionError] = useState<string | null>(null);
  const [queueForm, setQueueForm] = useState(defaultQueueForm);
  const [queueResult, setQueueResult] = useState<QueueDeviceConfigurationResponse | null>(null);
  const [queueError, setQueueError] = useState<string | null>(null);

  const [pendingState, setPendingState] = useState<PendingState>({
    deviceUuid: '',
    lastFetchedAt: null,
  });

  const [pendingRecord, setPendingRecord] = useState<DeviceConfigurationRecord | null>(null);
  const [pendingError, setPendingError] = useState<string | null>(null);
  const [pendingLoading, setPendingLoading] = useState(false);

  const [ackForm, setAckForm] = useState(defaultAckForm);
  const [ackResult, setAckResult] = useState<DeviceConfigurationRecord | null>(null);
  const [ackError, setAckError] = useState<string | null>(null);

  const [commandIssueForm, setCommandIssueForm] =
    useState<CommandIssueFormState>(defaultCommandIssueForm);
  const [commandIssueResult, setCommandIssueResult] = useState<IssueDeviceCommandResponse | null>(
    null,
  );
  const [commandIssueError, setCommandIssueError] = useState<string | null>(null);

  const [commandAckForm, setCommandAckForm] = useState<CommandAckFormState>(defaultCommandAckForm);
  const [commandAckResult, setCommandAckResult] = useState<AcknowledgeDeviceCommandResponse | null>(
    null,
  );
  const [commandAckError, setCommandAckError] = useState<string | null>(null);

  const [commandHistoryFilters, setCommandHistoryFilters] = useState<CommandHistoryFiltersState>(
    defaultCommandHistoryFilters,
  );
  const [commandHistoryResult, setCommandHistoryResult] =
    useState<FetchDeviceCommandHistoryResponse | null>(null);
  const [commandHistoryError, setCommandHistoryError] = useState<string | null>(null);
  const [commandHistoryLoading, setCommandHistoryLoading] = useState(false);
  const [lastCommandLoading, setLastCommandLoading] = useState(false);
  const [lastCommandError, setLastCommandError] = useState<string | null>(null);

  const [configForms, setConfigForms] = useState<Record<ConfigKind, { deviceUuid: string; payloadJson: string }>>(
    defaultConfigForms,
  );
  const [configResults, setConfigResults] = useState<Record<ConfigKind, unknown | null>>({
    vfd: null,
    beneficiary: null,
    installation: null,
  });
  const [configErrors, setConfigErrors] = useState<Record<ConfigKind, string | null>>({
    vfd: null,
    beneficiary: null,
    installation: null,
  });
  const [configLoading, setConfigLoading] = useState<Record<ConfigKind, boolean>>({
    vfd: false,
    beneficiary: false,
    installation: false,
  });

  const [csvForm, setCsvForm] = useState(defaultCsvForm);
  const [csvResult, setCsvResult] = useState<{
    processed: number;
    queued: number;
    errors: Array<{ row: number; message: string }>;
  } | null>(null);
  const [csvError, setCsvError] = useState<string | null>(null);

  const [rotationForm, setRotationForm] = useState(defaultRotationForm);
  const [rotationResult, setRotationResult] = useState<RotateDeviceCredentialsResponse | null>(
    null,
  );
  const [rotationError, setRotationError] = useState<string | null>(null);

  const [revokeForm, setRevokeForm] = useState<RevokeFormState>(defaultRevokeForm);
  const [revokeResult, setRevokeResult] = useState<RevokeDeviceCredentialsResponse | null>(null);
  const [revokeError, setRevokeError] = useState<string | null>(null);

  const [resyncForm, setResyncForm] = useState<ResyncFormState>(defaultResyncForm);
  const [resyncResult, setResyncResult] = useState<ResyncDeviceMqttProvisioningResponse | null>(
    null,
  );
  const [resyncError, setResyncError] = useState<string | null>(null);

  const [governmentForm, setGovernmentForm] = useState<GovernmentFormState>(defaultGovernmentForm);
  const [governmentResult, setGovernmentResult] = useState<GovernmentCredentialBundle | null>(null);
  const [governmentError, setGovernmentError] = useState<string | null>(null);
  const [governmentSuccess, setGovernmentSuccess] = useState<string | null>(null);
  const [bundleCopyFeedback, setBundleCopyFeedback] = useState<BundleCopyFeedback>({
    local: { message: null, error: null },
    government: { message: null, error: null },
  });
  const [provisioningRetryMessage, setProvisioningRetryMessage] = useState<string | null>(null);
  const [provisioningRetryError, setProvisioningRetryError] = useState<string | null>(null);

  const deviceListQuery = useQuery({
    queryKey: ['device-list', 'configuration'],
    queryFn: () => fetchDeviceList({ limit: BULK_DEVICE_LIST_LIMIT, includeInactive: false }),
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  });

  useEffect(() => {
    const normalizedUuid = rotationForm.deviceUuid;

    setQueueForm((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setPendingState((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setAckForm((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setCommandIssueForm((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setCommandAckForm((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setCommandHistoryFilters((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setGovernmentForm((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setRevokeForm((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setResyncForm((prev) =>
      prev.deviceUuid === normalizedUuid ? prev : { ...prev, deviceUuid: normalizedUuid },
    );

    setConfigForms((prev) => {
      const next = { ...prev };
      (Object.keys(next) as ConfigKind[]).forEach((kind) => {
        if (!next[kind].deviceUuid) {
          next[kind] = { ...next[kind], deviceUuid: normalizedUuid };
        }
      });
      return next;
    });

    setGovernmentResult(null);
    setGovernmentSuccess(null);
    setRevokeResult(null);
    setRevokeError(null);
    setResyncResult(null);
    setResyncError(null);
    setCommandIssueResult(null);
    setCommandIssueError(null);
    setCommandAckResult(null);
    setCommandAckError(null);
    setCommandHistoryResult(null);
    setCommandHistoryError(null);
  }, [rotationForm.deviceUuid]);

  const rawDeviceList = deviceListQuery.data?.devices ?? null;
  const devices = useMemo<DeviceListItem[]>(() => rawDeviceList ?? [], [rawDeviceList]);
  const deviceListError = (deviceListQuery.error ?? null) as Error | null;
  const deviceSearchTerm = deviceSearch.trim().toLowerCase();

  const filteredDevices = useMemo<DeviceListItem[]>(() => {
    if (!deviceSearchTerm) {
      return devices;
    }

    return devices.filter((device) => {
      const imei = device.imei?.toLowerCase() ?? '';
      const uuid = device.uuid.toLowerCase();
      const protocol = device.protocolVersion?.version?.toLowerCase() ?? '';
      return (
        imei.includes(deviceSearchTerm) ||
        uuid.includes(deviceSearchTerm) ||
        protocol.includes(deviceSearchTerm)
      );
    });
  }, [devices, deviceSearchTerm]);

  const totalSelected = selectedDeviceUuids.size;
  const displayedSelectedCount = useMemo(() => {
    if (!totalSelected || filteredDevices.length === 0) {
      return 0;
    }

    return filteredDevices.reduce((count, device) => {
      if (selectedDeviceUuids.has(device.uuid)) {
        return count + 1;
      }
      return count;
    }, 0);
  }, [filteredDevices, selectedDeviceUuids, totalSelected]);

  const allDisplayedSelected =
    filteredDevices.length > 0 && displayedSelectedCount === filteredDevices.length;
  const isBulkOperationRunning = bulkRotationState?.status === 'running';

  useEffect(() => {
    const checkbox = headerCheckboxRef.current;
    if (!checkbox) {
      return;
    }

    checkbox.indeterminate =
      displayedSelectedCount > 0 && displayedSelectedCount < filteredDevices.length;
  }, [displayedSelectedCount, filteredDevices.length]);

  useEffect(() => {
    setSelectedDeviceUuids((prev) => {
      if (!prev.size) {
        return prev;
      }

      const available = new Set(devices.map((device) => device.uuid));
      let changed = false;
      const next = new Set<string>();
      prev.forEach((uuid) => {
        if (available.has(uuid)) {
          next.add(uuid);
        } else {
          changed = true;
        }
      });

      return changed ? next : prev;
    });
  }, [devices]);

  const handleDeviceSearchInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    setDeviceSearch(event.target.value);
    setBulkActionError(null);
  };

  const toggleDeviceSelection = useCallback(
    (uuid: string) => {
      if (isBulkOperationRunning) {
        return;
      }

      setSelectedDeviceUuids((prev) => {
        const next = new Set(prev);
        if (next.has(uuid)) {
          next.delete(uuid);
        } else {
          next.add(uuid);
        }
        return next;
      });
      setBulkActionError(null);
    },
    [isBulkOperationRunning],
  );

  const toggleAllDisplayedSelections = useCallback(() => {
    if (isBulkOperationRunning || filteredDevices.length === 0) {
      return;
    }

    setSelectedDeviceUuids((prev) => {
      const next = new Set(prev);
      const shouldSelectAll = filteredDevices.some((device) => !next.has(device.uuid));
      if (shouldSelectAll) {
        filteredDevices.forEach((device) => next.add(device.uuid));
      } else {
        filteredDevices.forEach((device) => next.delete(device.uuid));
      }
      return next;
    });
    setBulkActionError(null);
  }, [filteredDevices, isBulkOperationRunning]);

  const clearSelectedDevices = useCallback(() => {
    if (isBulkOperationRunning) {
      return;
    }

    setSelectedDeviceUuids(() => new Set());
    setBulkActionError(null);
  }, [isBulkOperationRunning]);

  const handleRefreshDeviceList = useCallback(() => {
    void deviceListQuery.refetch();
  }, [deviceListQuery]);

  const vfdModelsQuery = useQuery({
    queryKey: ['vfd-models', queueForm.protocolVersionIdFilter],
    queryFn: () =>
      fetchVfdModels(
        queueForm.protocolVersionIdFilter.trim()
          ? queueForm.protocolVersionIdFilter.trim()
          : undefined,
      ),
    staleTime: 5 * 60 * 1000,
  });

  const statusDeviceUuid = queueForm.deviceUuid.trim();
  const statusPollingGate = usePollingGate('device-config-status', {
    isActive: Boolean(statusDeviceUuid),
  });
  const deviceStatusQuery = useQuery<DeviceStatusResponse, Error>({
    queryKey: ['device-status', statusDeviceUuid],
    queryFn: () => fetchDeviceStatus(statusDeviceUuid),
    enabled: Boolean(statusDeviceUuid) && statusPollingGate.enabled,
    refetchInterval: statusPollingGate.enabled ? 60_000 : false,
  });
  const deviceStatus = deviceStatusQuery.data ?? null;
  const activeDevice = deviceStatus?.device ?? null;
  const activeLocalCredentials = deviceStatus?.activeCredentials?.local ?? null;
  const activeGovernmentCredentials = deviceStatus?.activeCredentials?.government ?? null;
  const governmentDefaults = useMemo(
    () => extractGovernmentCredentialDefaults(deviceStatus?.device.protocolVersion?.metadata),
    [deviceStatus?.device.protocolVersion?.metadata],
  );
  const localMockDashboardBundle = useMemo(
    () => buildMockDashboardBundle(activeDevice, activeLocalCredentials),
    [activeDevice, activeLocalCredentials],
  );
  const governmentMockDashboardBundle = useMemo(
    () => buildMockDashboardBundle(activeDevice, activeGovernmentCredentials),
    [activeDevice, activeGovernmentCredentials],
  );
  const rotationCredentials = rotationResult?.credentials ?? null;
  const provisioningInfo = deviceStatus?.mqttProvisioning ?? null;

  useEffect(() => {
    setBundleCopyFeedback({
      local: { message: null, error: null },
      government: { message: null, error: null },
    });
  }, [activeDevice?.uuid]);

  useEffect(() => {
    setProvisioningRetryMessage(null);
    setProvisioningRetryError(null);
  }, [statusDeviceUuid]);

  useEffect(() => {
    const currentStatus = deviceStatus?.mqttProvisioning?.status;
    if (!currentStatus || currentStatus === 'applied') {
      setProvisioningRetryMessage(null);
      setProvisioningRetryError(null);
    }
  }, [deviceStatus?.mqttProvisioning?.status]);

  useEffect(() => {
    if (!commandHistoryFilters.deviceUuid.trim()) {
      setCommandHistoryResult(null);
      setCommandHistoryError(null);
    }
  }, [commandHistoryFilters.deviceUuid]);

  useEffect(() => {
    if (!activeView) {
      return;
    }

    const targetIdMap: Record<DeviceConfigurationView, string> = {
      government: 'device-config-government-credentials-heading',
      internal: 'device-config-regenerate-credentials-heading',
      drive: 'device-config-rms-drive-heading',
    };

    const targetId = targetIdMap[activeView];
    const node = document.getElementById(targetId);
    if (node) {
      node.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [activeView]);

  const queueMutation = useMutation<
    QueueDeviceConfigurationResponse,
    Error,
    { deviceUuid: string; payload: QueueDeviceConfigurationPayload }
  >({
    mutationFn: ({ deviceUuid, payload }) => queueDeviceConfiguration(deviceUuid, payload),
    onSuccess: (data, variables) => {
      setQueueResult(data);
      setPendingState({ deviceUuid: variables.deviceUuid, lastFetchedAt: null });
      setAckForm((prev) => ({ ...prev, deviceUuid: variables.deviceUuid }));
    },
    onError: (error) => {
      setQueueError(
        formatDeviceConfigActionError('queue-config', error, 'Unable to queue configuration'),
      );
    },
  });

  const acknowledgeMutation = useMutation<
    DeviceConfigurationRecord,
    Error,
    { deviceUuid: string; payload: Parameters<typeof acknowledgeDeviceConfiguration>[1] }
  >({
    mutationFn: ({ deviceUuid, payload }) => acknowledgeDeviceConfiguration(deviceUuid, payload),
    onSuccess: (data) => {
      setAckResult(data);
    },
    onError: (error) => {
      setAckError(
        formatDeviceConfigActionError('ack-config', error, 'Unable to acknowledge configuration'),
      );
    },
  });

  const csvMutation = useMutation<
    ImportDeviceConfigurationsResult,
    Error,
    Parameters<typeof importDeviceConfigurationsCsv>[0]
  >({
    mutationFn: importDeviceConfigurationsCsv,
    onSuccess: (data) => {
      setCsvResult(data);
    },
    onError: (error) => {
      setCsvError(
        formatDeviceConfigActionError('csv-import', error, 'Unable to import configurations'),
      );
    },
  });

  const rotateMutation = useMutation<
    RotateDeviceCredentialsResponse,
    Error,
    { deviceUuid: string; payload: RotateDeviceCredentialsPayload }
  >({
    mutationFn: ({ deviceUuid, payload }) => rotateDeviceCredentials(deviceUuid, payload),
    onSuccess: (data) => {
      setRotationResult(data);
      setRotationError(null);

      const rotatedUuid = data.device.id;

      setQueueForm((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setPendingState({ deviceUuid: rotatedUuid, lastFetchedAt: null });
      setAckForm((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setCommandIssueForm((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setCommandAckForm((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setCommandHistoryFilters((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setGovernmentForm((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setRevokeForm((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setResyncForm((prev) => ({ ...prev, deviceUuid: rotatedUuid }));
      setGovernmentResult(null);
      setGovernmentSuccess(null);
      setRevokeResult(null);
      setRevokeError(null);
      setResyncResult(null);
      setResyncError(null);
      setCommandIssueResult(null);
      setCommandIssueError(null);
      setCommandAckResult(null);
      setCommandAckError(null);
      setCommandHistoryResult(null);
      setCommandHistoryError(null);
      setRotationForm({ ...defaultRotationForm });

      queryClient.invalidateQueries({ queryKey: ['device-status', rotatedUuid] });
    },
    onError: (error) => {
      setRotationError(
        formatDeviceConfigActionError(
          'rotate-credentials',
          error,
          'Unable to rotate credentials',
        ),
      );
    },
  });

  const revokeMutation = useMutation<
    RevokeDeviceCredentialsResponse,
    Error,
    { deviceUuid: string; payload: RevokeDeviceCredentialsPayload }
  >({
    mutationFn: ({ deviceUuid, payload }) => revokeDeviceCredentials(deviceUuid, payload),
    onSuccess: (data) => {
      setRevokeResult(data);
      setRevokeError(null);
      setRevokeForm((prev) => ({
        ...prev,
        deviceUuid: data.device.id,
        reason: '',
        issuedBy: '',
      }));
      queryClient.invalidateQueries({ queryKey: ['device-status', data.device.id] });
    },
    onError: (error) => {
      setRevokeResult(null);
      setRevokeError(
        formatDeviceConfigActionError(
          'revoke-credentials',
          error,
          'Unable to revoke credentials',
        ),
      );
    },
  });

  const resyncMutation = useMutation<
    ResyncDeviceMqttProvisioningResponse,
    Error,
    { deviceUuid: string; reason?: string }
  >({
    mutationFn: ({ deviceUuid, reason }) => resyncDeviceMqttProvisioning({ deviceUuid, reason }),
    onSuccess: (data) => {
      setResyncResult(data);
      setResyncError(null);
      setResyncForm((prev) => ({ ...prev, deviceUuid: data.device.id, reason: '' }));
      setProvisioningRetryMessage(null);
      setProvisioningRetryError(null);
      queryClient.invalidateQueries({ queryKey: ['device-status', data.device.id] });
    },
    onError: (error) => {
      setResyncResult(null);
      setResyncError(
        formatDeviceConfigActionError('resync-broker', error, 'Unable to queue broker resync'),
      );
    },
  });

  const issueCommandMutation = useMutation<
    IssueDeviceCommandResponse,
    Error,
    { deviceUuid: string; payload: IssueDeviceCommandPayload }
  >({
    mutationFn: ({ deviceUuid, payload }) => issueDeviceCommand(deviceUuid, payload),
    onSuccess: (data) => {
      setCommandIssueResult(data);
      setCommandIssueError(null);
      setCommandIssueForm((prev) => ({ ...prev, deviceUuid: data.device.uuid }));
      setCommandAckForm((prev) => ({ ...prev, deviceUuid: data.device.uuid, msgid: data.msgid }));
      setCommandHistoryFilters((prev) => ({ ...prev, deviceUuid: data.device.uuid }));
    },
    onError: (error) => {
      setCommandIssueResult(null);
      setCommandIssueError(
        formatDeviceConfigActionError('issue-command', error, 'Unable to issue device command'),
      );
    },
  });

  const acknowledgeCommandMutation = useMutation<
    AcknowledgeDeviceCommandResponse,
    Error,
    { deviceUuid: string; payload: AcknowledgeDeviceCommandPayload }
  >({
    mutationFn: ({ deviceUuid, payload }) => acknowledgeDeviceCommand(deviceUuid, payload),
    onSuccess: (data, variables) => {
      setCommandAckResult(data);
      setCommandAckError(null);
      setCommandAckForm((prev) => ({
        ...prev,
        deviceUuid: variables.deviceUuid,
        msgid: data.msgid,
        status: data.status,
      }));
      setCommandHistoryFilters((prev) => ({ ...prev, deviceUuid: variables.deviceUuid }));
    },
    onError: (error) => {
      setCommandAckResult(null);
      setCommandAckError(
        formatDeviceConfigActionError(
          'ack-command',
          error,
          'Unable to acknowledge device command',
        ),
      );
    },
  });

  const governmentMutation = useMutation<
    { device: { id: string; imei: string }; credentials: GovernmentCredentialBundle },
    Error,
    { deviceUuid: string; payload: GovernmentCredentialPayload }
  >({
    mutationFn: ({ deviceUuid, payload }) => upsertDeviceGovernmentCredentials(deviceUuid, payload),
    onSuccess: (data) => {
      const deviceUuid = data.device.id;
      setGovernmentResult(data.credentials);
      setGovernmentSuccess('Government credentials saved successfully.');
      setGovernmentError(null);
      setGovernmentForm((prev) => ({
        ...prev,
        deviceUuid,
        clientId: data.credentials.clientId,
        username: data.credentials.username,
        password: data.credentials.password,
        endpointsJson: JSON.stringify(data.credentials.endpoints, null, 2),
        publishTopicsRaw: data.credentials.topics.publish.join('\n'),
        subscribeTopicsRaw: data.credentials.topics.subscribe.join('\n'),
      }));

      queryClient.invalidateQueries({ queryKey: ['device-status', deviceUuid] });
    },
    onError: (error) => {
      setGovernmentError(
        formatDeviceConfigActionError(
          'save-government-credentials',
          error,
          'Unable to save government credentials',
        ),
      );
    },
  });

  const provisioningRetryMutation = useMutation<
    RetryMqttProvisioningResponse,
    Error,
    { deviceUuid: string }
  >({
    mutationFn: ({ deviceUuid }) => retryDeviceMqttProvisioning(deviceUuid),
    onSuccess: (data, variables) => {
      setProvisioningRetryError(null);
      setProvisioningRetryMessage(
        data.attemptsReset
          ? 'Provisioning retry queued and attempt counter reset.'
          : 'Provisioning retry queued.',
      );
      queryClient.invalidateQueries({ queryKey: ['device-status', variables.deviceUuid] });
    },
    onError: (error) => {
      setProvisioningRetryMessage(null);
      setProvisioningRetryError(
        formatDeviceConfigActionError(
          'retry-provisioning',
          error,
          'Unable to retry MQTT provisioning',
        ),
      );
    },
  });

  const vfdModels = useMemo<VfdModel[]>(() => vfdModelsQuery.data ?? [], [vfdModelsQuery.data]);

  const selectedModel = useMemo<VfdModel | undefined>(
    () => vfdModels.find((model) => model.id === queueForm.vfdModelId),
    [vfdModels, queueForm.vfdModelId],
  );

  const vfdModelsError = (vfdModelsQuery.error ?? null) as (Error & { status?: number }) | null;
  const vfdModelsCapabilityError = vfdModelsError?.status === 403;

  const handleQueueInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>,
  ) => {
    const { name, value } = event.target;
    setQueueForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    if (name === 'deviceUuid') {
      setPendingState((prev) => ({ deviceUuid: value, lastFetchedAt: prev.lastFetchedAt }));
      setAckForm((prev) => ({ ...prev, deviceUuid: value }));
      setCommandIssueForm((prev) => ({ ...prev, deviceUuid: value }));
      setCommandAckForm((prev) => ({ ...prev, deviceUuid: value }));
      setCommandHistoryFilters((prev) => ({ ...prev, deviceUuid: value }));
      setCommandHistoryResult(null);
      setCommandHistoryError(null);
      setCommandIssueResult(null);
      setCommandIssueError(null);
      setCommandAckResult(null);
      setCommandAckError(null);
      setGovernmentForm((prev) => ({ ...prev, deviceUuid: value }));
      setRevokeForm((prev) => ({ ...prev, deviceUuid: value }));
      setResyncForm((prev) => ({ ...prev, deviceUuid: value }));
      setGovernmentResult(null);
      setGovernmentSuccess(null);
      setRevokeResult(null);
      setRevokeError(null);
      setResyncResult(null);
      setResyncError(null);
    }
    setQueueError(null);
  };

  const handleProvisioningRetry = (deviceUuid: string) => {
    if (!deviceUuid) {
      return;
    }

    setProvisioningRetryMessage(null);
    setProvisioningRetryError(null);
    provisioningRetryMutation.mutate({ deviceUuid });
  };

  const handleQueueSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!queueForm.deviceUuid.trim()) {
      setQueueError('Provide a device UUID to queue configuration.');
      return;
    }

    if (!queueForm.vfdModelId) {
      setQueueError('Select a VFD model to queue configuration.');
      return;
    }

    let overrides: QueueDeviceConfigurationPayload['overrides'];
    if (queueForm.overridesJson.trim()) {
      try {
        overrides = JSON.parse(
          queueForm.overridesJson,
        ) as QueueDeviceConfigurationPayload['overrides'];
      } catch {
        setQueueError('Overrides JSON is invalid. Provide a valid JSON object.');
        return;
      }
    }

    const qosValue = queueForm.qos.trim() ? Number(queueForm.qos) : undefined;
    if (qosValue !== undefined && (Number.isNaN(qosValue) || qosValue < 0 || qosValue > 2)) {
      setQueueError('QoS must be 0, 1, or 2 when provided.');
      return;
    }

    const payload: QueueDeviceConfigurationPayload = {
      vfdModelId: queueForm.vfdModelId,
      overrides,
      transport: queueForm.transport,
      issuedBy: queueForm.issuedBy.trim() || undefined,
      qos: qosValue,
    };

    setQueueError(null);
    setQueueResult(null);

    queueMutation.mutate({ deviceUuid: queueForm.deviceUuid.trim(), payload });
  };

  const handleCommandIssueInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    setCommandIssueForm((prev) => ({ ...prev, [name]: value }));
    setCommandIssueError(null);
  };

  const handleCommandIssueSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const deviceUuid = commandIssueForm.deviceUuid.trim();
    if (!deviceUuid) {
      setCommandIssueError('Provide a device UUID before issuing a command.');
      return;
    }

    const commandName = commandIssueForm.commandName.trim();
    if (!commandName) {
      setCommandIssueError('Provide a command name before issuing.');
      return;
    }

    let payload: Record<string, unknown> | undefined;
    if (commandIssueForm.payloadJson.trim()) {
      try {
        const parsed = JSON.parse(commandIssueForm.payloadJson);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          setCommandIssueError('Command payload must be a JSON object.');
          return;
        }
        payload = parsed as Record<string, unknown>;
      } catch {
        setCommandIssueError('Command payload must be valid JSON.');
        return;
      }
    }

    let qosValue: number | undefined;
    if (commandIssueForm.qos.trim()) {
      const parsed = Number(commandIssueForm.qos.trim());
      if (!Number.isFinite(parsed) || parsed < 0 || parsed > 2) {
        setCommandIssueError('QoS must be 0, 1, or 2 when provided.');
        return;
      }
      qosValue = parsed;
    }

    let timeoutValue: number | undefined;
    if (commandIssueForm.timeoutSeconds.trim()) {
      const parsed = Number(commandIssueForm.timeoutSeconds.trim());
      if (!Number.isFinite(parsed) || parsed <= 0) {
        setCommandIssueError('Timeout seconds must be a positive number.');
        return;
      }
      timeoutValue = parsed;
    }

    const payloadBody: IssueDeviceCommandPayload = {
      command: { name: commandName },
    };

    if (payload) {
      payloadBody.command.payload = payload;
    }
    if (typeof qosValue === 'number') {
      payloadBody.qos = qosValue;
    }
    if (typeof timeoutValue === 'number') {
      payloadBody.timeoutSeconds = timeoutValue;
    }

    const issuedByValue = commandIssueForm.issuedBy.trim();
    if (issuedByValue) {
      payloadBody.issuedBy = issuedByValue;
    }

    const simulatorToken = commandIssueForm.simulatorSessionToken.trim();
    if (simulatorToken) {
      payloadBody.simulatorSessionToken = simulatorToken;
    }

    setCommandIssueError(null);
    setCommandIssueResult(null);
    setCommandAckResult(null);
    setCommandAckError(null);

    issueCommandMutation.mutate({ deviceUuid, payload: payloadBody });
  };

  const handleCommandIssueReset = () => {
    setCommandIssueForm((prev) => ({ ...defaultCommandIssueForm, deviceUuid: prev.deviceUuid }));
    setCommandIssueResult(null);
    setCommandIssueError(null);
    issueCommandMutation.reset();
  };

  const configActions: Record<
    ConfigKind,
    {
      set: typeof setVfdConfig;
      get: typeof getVfdConfig;
      label: string;
    }
  > = {
    vfd: { set: setVfdConfig, get: getVfdConfig, label: 'VFD configuration' },
    beneficiary: { set: setBeneficiary, get: getBeneficiary, label: 'Beneficiary profile' },
    installation: { set: setInstallation, get: getInstallation, label: 'Installation details' },
  };

  const handleConfigInputChange = (
    kind: ConfigKind,
    field: 'deviceUuid' | 'payloadJson',
    value: string,
  ) => {
    setConfigForms((prev) => ({ ...prev, [kind]: { ...prev[kind], [field]: value } }));
    setConfigErrors((prev) => ({ ...prev, [kind]: null }));
  };

  const handleConfigAction = async (kind: ConfigKind, action: 'set' | 'get') => {
    const form = configForms[kind];
    const deviceUuid = (form.deviceUuid || singleSelectedDeviceUuid).trim();
    if (!deviceUuid) {
      setConfigErrors((prev) => ({ ...prev, [kind]: 'Select or enter a device UUID first.' }));
      return;
    }

    let payload: Record<string, unknown> | undefined;
    if (form.payloadJson.trim()) {
      try {
        const parsed = JSON.parse(form.payloadJson);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          throw new Error('Payload must be a JSON object.');
        }
        payload = parsed as Record<string, unknown>;
      } catch (error) {
        setConfigErrors((prev) => ({
          ...prev,
          [kind]:
            error instanceof Error ? error.message : 'Payload must be valid JSON (object only).',
        }));
        return;
      }
    }

    setConfigLoading((prev) => ({ ...prev, [kind]: true }));
    setConfigErrors((prev) => ({ ...prev, [kind]: null }));
    try {
      const api = configActions[kind][action];
      const response =
        action === 'set'
          ? await (api as typeof setVfdConfig)(deviceUuid, payload ?? {})
          : await (api as typeof getVfdConfig)(deviceUuid, payload ?? {});
      setConfigResults((prev) => ({
        ...prev,
        [kind]: response,
      }));
    } catch (caughtError) {
      const actionKeyByKind: Record<ConfigKind, Record<'set' | 'get', DeviceConfigActionKey>> = {
        vfd: { set: 'set-vfd-config', get: 'get-vfd-config' },
        beneficiary: { set: 'set-beneficiary', get: 'get-beneficiary' },
        installation: { set: 'set-installation', get: 'get-installation' },
      };
      const fallbackLabel = `${configActions[kind].label} request failed.`;
      const message = formatDeviceConfigActionError(
        actionKeyByKind[kind][action],
        caughtError,
        fallbackLabel,
      );
      setConfigErrors((prev) => ({ ...prev, [kind]: message }));
    } finally {
      setConfigLoading((prev) => ({ ...prev, [kind]: false }));
    }
  };

  const handleCommandAckInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    setCommandAckForm((prev) => ({ ...prev, [name]: value }));
    setCommandAckError(null);
  };

  const handleCommandAckSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const deviceUuid = commandAckForm.deviceUuid.trim();
    if (!deviceUuid) {
      setCommandAckError('Provide a device UUID before acknowledging a command.');
      return;
    }

    const msgid = commandAckForm.msgid.trim();
    if (!msgid) {
      setCommandAckError('Provide a message ID to acknowledge.');
      return;
    }

    let payload: Record<string, unknown> | undefined;
    if (commandAckForm.payloadJson.trim()) {
      try {
        const parsed = JSON.parse(commandAckForm.payloadJson);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          setCommandAckError('Acknowledgement payload must be a JSON object.');
          return;
        }
        payload = parsed as Record<string, unknown>;
      } catch {
        setCommandAckError('Acknowledgement payload must be valid JSON.');
        return;
      }
    }

    const receivedAtValue = commandAckForm.receivedAt.trim();

    const payloadBody: AcknowledgeDeviceCommandPayload = {
      msgid,
      status: commandAckForm.status,
    };

    if (payload) {
      payloadBody.payload = payload;
    }
    if (receivedAtValue) {
      payloadBody.receivedAt = receivedAtValue;
    }

    setCommandAckError(null);
    setCommandAckResult(null);

    acknowledgeCommandMutation.mutate({ deviceUuid, payload: payloadBody });
  };

  const handleCommandAckReset = () => {
    setCommandAckForm((prev) => ({ ...defaultCommandAckForm, deviceUuid: prev.deviceUuid }));
    setCommandAckResult(null);
    setCommandAckError(null);
    acknowledgeCommandMutation.reset();
  };

  const handleCommandHistoryInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    if (name === 'deviceUuid') {
      setCommandHistoryFilters((prev) => ({ ...prev, deviceUuid: value }));
    } else if (name === 'limit') {
      setCommandHistoryFilters((prev) => ({ ...prev, limit: value }));
    }
    setCommandHistoryError(null);
  };

  const toggleCommandHistoryStatus = (status: CommandHistoryStatusKey) => {
    setCommandHistoryFilters((prev) => ({
      ...prev,
      statuses: {
        ...prev.statuses,
        [status]: !prev.statuses[status],
      },
    }));
    setCommandHistoryError(null);
  };

  const loadCommandHistory = useCallback(
    async ({ cursor, append }: { cursor?: string | null; append?: boolean } = {}) => {
      const deviceUuid = commandHistoryFilters.deviceUuid.trim();
      if (!deviceUuid) {
        setCommandHistoryError('Provide a device UUID to load command history.');
        return;
      }

      let limitValue: number | undefined;
      const trimmedLimit = commandHistoryFilters.limit.trim();
      if (trimmedLimit) {
        const parsed = Number(trimmedLimit);
        if (!Number.isFinite(parsed) || parsed <= 0) {
          setCommandHistoryError('History limit must be a positive number.');
          return;
        }
        if (parsed > 100) {
          setCommandHistoryError('History limit cannot exceed 100.');
          return;
        }
        limitValue = parsed;
      }

      const selectedStatuses = Object.entries(commandHistoryFilters.statuses)
        .filter(([, enabled]) => enabled)
        .map(([status]) => status as CommandHistoryStatusKey);

      const params: FetchDeviceCommandHistoryParams = {};
      if (typeof limitValue === 'number') {
        params.limit = limitValue;
      }
      if (cursor) {
        params.cursor = cursor;
      }
      if (selectedStatuses.length > 0) {
        params.statuses = selectedStatuses as Array<'pending' | 'acknowledged' | 'failed'>;
      }

      setCommandHistoryLoading(true);
      setCommandHistoryError(null);
      if (!append) {
        setCommandHistoryResult(null);
      }

      try {
        const response = await fetchDeviceCommandHistory(deviceUuid, params);
        setCommandHistoryResult((prev) => {
          if (append && prev) {
            return {
              device: response.device,
              commands: [...prev.commands, ...response.commands],
              nextCursor: response.nextCursor,
            };
          }
          return response;
        });
      } catch (caughtError) {
        setCommandHistoryError(
          formatDeviceConfigActionError(
            'load-command-history',
            caughtError,
            'Unable to load command history',
          ),
        );
      } finally {
        setCommandHistoryLoading(false);
      }
    },
    [commandHistoryFilters],
  );

  const handleCommandHistorySubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void loadCommandHistory({ append: false });
  };

  const handleCommandHistoryReset = () => {
    setCommandHistoryFilters((prev) => ({
      ...defaultCommandHistoryFilters,
      deviceUuid: prev.deviceUuid,
    }));
    setCommandHistoryResult(null);
    setCommandHistoryError(null);
  };

  const handleCommandHistoryLoadMore = () => {
    const cursor = commandHistoryResult?.nextCursor;
    if (!cursor) {
      return;
    }
    void loadCommandHistory({ cursor, append: true });
  };

  const handleLoadLastCommand = async () => {
    const candidate =
      (singleSelectedDeviceUuid && singleSelectedDeviceUuid.trim()) ||
      commandIssueForm.deviceUuid.trim() ||
      commandHistoryFilters.deviceUuid.trim();

    if (!candidate) {
      setLastCommandError('Provide or select a device UUID first.');
      return;
    }

    setLastCommandLoading(true);
    setLastCommandError(null);

    try {
      const result = await fetchDeviceCommandHistory(candidate, { limit: 1 });
      setCommandHistoryResult(result);
      setCommandHistoryFilters((prev) => ({
        ...prev,
        deviceUuid: candidate,
        limit: '1',
      }));

      if (!result.commands.length) {
        setLastCommandError('No command history recorded yet.');
        return;
      }

      const [latest] = result.commands;
      setCommandIssueForm((prev) => ({
        ...prev,
        deviceUuid: candidate,
        commandName: latest.command.name ?? prev.commandName,
        payloadJson:
          latest.command.payload && Object.keys(latest.command.payload).length > 0
            ? JSON.stringify(latest.command.payload, null, 2)
            : '',
      }));
    } catch (error) {
      const message =
        error instanceof Error ? error.message : 'Failed to load the most recent command.';
      setLastCommandError(message);
    } finally {
      setLastCommandLoading(false);
    }
  };

  const handleRefreshPending = async () => {
    if (!pendingState.deviceUuid.trim()) {
      setPendingError('Provide a device UUID to load pending configuration.');
      return;
    }

    setPendingLoading(true);
    setPendingError(null);
    setPendingRecord(null);

    try {
      const record = await fetchPendingDeviceConfiguration(pendingState.deviceUuid.trim());
      setPendingRecord(record);
      setPendingState((prev) => ({ ...prev, lastFetchedAt: new Date() }));
    } catch (caughtError) {
      setPendingError(
        formatDeviceConfigActionError(
          'load-pending-config',
          caughtError,
          'Unable to load configuration',
        ),
      );
    } finally {
      setPendingLoading(false);
    }
  };

  const handlePendingUuidChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const value = event.target.value;
    setPendingState((prev) => ({ deviceUuid: value, lastFetchedAt: prev.lastFetchedAt }));
    setPendingError(null);
  };

  const handleAckChange = (
    event: ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>,
  ) => {
    const { name, value } = event.target;
    setAckForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    setAckError(null);
  };

  const handleAckSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!ackForm.deviceUuid.trim()) {
      setAckError('Provide a device UUID before sending acknowledgement.');
      return;
    }

    let payloadJson: Record<string, unknown> | undefined;
    if (ackForm.payloadJson.trim()) {
      try {
        payloadJson = JSON.parse(ackForm.payloadJson) as Record<string, unknown>;
      } catch {
        setAckError('Acknowledgement payload must be valid JSON.');
        return;
      }
    }

    const receivedAtValue = ackForm.receivedAt.trim() || undefined;

    setAckError(null);
    setAckResult(null);

    acknowledgeMutation.mutate({
      deviceUuid: ackForm.deviceUuid.trim(),
      payload: {
        status: ackForm.status,
        msgid: ackForm.msgid.trim() || undefined,
        receivedAt: receivedAtValue,
        acknowledgementPayload: payloadJson,
      },
    });
  };

  const handleCsvChange = (
    event: ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>,
  ) => {
    const { name, value } = event.target;
    setCsvForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    setCsvError(null);
  };

  const handleCsvSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!csvForm.csv.trim()) {
      setCsvError('Provide CSV data before importing.');
      return;
    }

    setCsvError(null);
    setCsvResult(null);

    csvMutation.mutate({
      csv: csvForm.csv,
      transport: csvForm.transport,
      issuedBy: csvForm.issuedBy.trim() || undefined,
    });
  };

  const handleRotationInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    setRotationForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    setRotationError(null);
  };

  const handleRotationSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!canManageDeviceCredentials) {
      setRotationError(DEVICE_CREDENTIAL_GUARD_MESSAGE);
      return;
    }

    const deviceUuid = rotationForm.deviceUuid.trim();
    if (!deviceUuid) {
      setRotationError('Provide a device UUID before rotating credentials.');
      return;
    }

    const reasonValue = rotationForm.reason.trim();
    if (reasonValue && reasonValue.length > 256) {
      setRotationError('Rotation reason must be 256 characters or fewer.');
      return;
    }

    const payload: RotateDeviceCredentialsPayload = {};
    if (reasonValue) {
      payload.reason = reasonValue;
    }

    const issuedByValue = rotationForm.issuedBy.trim();
    if (issuedByValue) {
      payload.issuedBy = issuedByValue;
    }

    setRotationError(null);
    setRotationResult(null);

    rotateMutation.mutate({ deviceUuid, payload });
  };

  const handleRevokeInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    setRevokeForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    setRevokeError(null);
    setRevokeResult(null);

    if (name === 'deviceUuid') {
      setQueueForm((prev) => ({ ...prev, deviceUuid: value }));
      setPendingState((prev) => ({ ...prev, deviceUuid: value }));
      setAckForm((prev) => ({ ...prev, deviceUuid: value }));
      setRotationForm((prev) => ({ ...prev, deviceUuid: value }));
      setGovernmentForm((prev) => ({ ...prev, deviceUuid: value }));
      setResyncForm((prev) => ({ ...prev, deviceUuid: value }));
      setGovernmentResult(null);
      setGovernmentSuccess(null);
      setResyncResult(null);
      setResyncError(null);
    }
  };

  const handleRevokeSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!canManageDeviceCredentials) {
      setRevokeError(DEVICE_CREDENTIAL_GUARD_MESSAGE);
      return;
    }

    const deviceUuid = revokeForm.deviceUuid.trim();
    if (!deviceUuid) {
      setRevokeError('Provide a device UUID before revoking credentials.');
      return;
    }

    const reasonValue = revokeForm.reason.trim();
    if (reasonValue.length > 512) {
      setRevokeError('Revocation reason must be 512 characters or fewer.');
      return;
    }

    const payload: RevokeDeviceCredentialsPayload = {
      type: revokeForm.credentialType,
    };

    if (reasonValue) {
      payload.reason = reasonValue;
    }

    const issuedByValue = revokeForm.issuedBy.trim();
    if (issuedByValue) {
      payload.issuedBy = issuedByValue;
    }

    setRevokeError(null);
    setRevokeResult(null);

    revokeMutation.mutate({ deviceUuid, payload });
  };

  const handleRevokeReset = () => {
    setRevokeForm((prev) => ({ ...defaultRevokeForm, deviceUuid: prev.deviceUuid }));
    setRevokeResult(null);
    setRevokeError(null);
    revokeMutation.reset();
  };

  const handleResyncInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    setResyncForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    setResyncError(null);
    setResyncResult(null);

    if (name === 'deviceUuid') {
      setQueueForm((prev) => ({ ...prev, deviceUuid: value }));
      setPendingState((prev) => ({ ...prev, deviceUuid: value }));
      setAckForm((prev) => ({ ...prev, deviceUuid: value }));
      setRotationForm((prev) => ({ ...prev, deviceUuid: value }));
      setGovernmentForm((prev) => ({ ...prev, deviceUuid: value }));
      setRevokeForm((prev) => ({ ...prev, deviceUuid: value }));
      setGovernmentResult(null);
      setGovernmentSuccess(null);
      setRevokeResult(null);
      setRevokeError(null);
    }
  };

  const handleResyncSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const deviceUuid = resyncForm.deviceUuid.trim();
    if (!deviceUuid) {
      setResyncError('Provide a device UUID before requesting a broker resync.');
      return;
    }

    const reasonValue = resyncForm.reason.trim();
    if (reasonValue.length > 512) {
      setResyncError('Resync reason must be 512 characters or fewer.');
      return;
    }

    setResyncError(null);
    setResyncResult(null);

    resyncMutation.mutate({ deviceUuid, reason: reasonValue || undefined });
  };

  const handleResyncReset = () => {
    setResyncForm((prev) => ({ ...defaultResyncForm, deviceUuid: prev.deviceUuid }));
    setResyncResult(null);
    setResyncError(null);
    resyncMutation.reset();
  };

  const processBulkCredentialRotation = useCallback(
    async (targets: DeviceListItem[]) => {
      for (const device of targets) {
        if (bulkRotationCancelRef.current) {
          break;
        }

        setBulkRotationState((prev) => {
          if (!prev) {
            return prev;
          }

          const nextResults = { ...prev.results };
          const existing = nextResults[device.uuid];
          nextResults[device.uuid] = {
            ...(existing ?? { deviceImei: device.imei ?? null }),
            status: 'in_progress',
            error: undefined,
          };

          return {
            ...prev,
            results: nextResults,
          } satisfies BulkRotationState;
        });

        try {
          const response = await rotateDeviceCredentials(device.uuid, {});
          const credentials = response.credentials ?? null;

          setBulkRotationState((prev) => {
            if (!prev) {
              return prev;
            }

            const nextResults = { ...prev.results };
            nextResults[device.uuid] = {
              status: 'success',
              credentials,
              deviceImei: response.device.imei ?? device.imei ?? null,
            } satisfies BulkCredentialResult;

            return {
              ...prev,
              results: nextResults,
            } satisfies BulkRotationState;
          });

          queryClient.invalidateQueries({ queryKey: ['device-status', device.uuid] });
        } catch (caughtError) {
          const message = formatDeviceConfigActionError(
            'bulk-rotate-credentials',
            caughtError,
            'Unable to rotate credentials',
          );

          setBulkRotationState((prev) => {
            if (!prev) {
              return prev;
            }

            const nextResults = { ...prev.results };
            const existing = nextResults[device.uuid];
            nextResults[device.uuid] = {
              ...(existing ?? {}),
              status: 'error',
              error: message,
              deviceImei: existing?.deviceImei ?? device.imei ?? null,
            } satisfies BulkCredentialResult;

            return {
              ...prev,
              results: nextResults,
            } satisfies BulkRotationState;
          });
        }
      }

      const wasCancelled = bulkRotationCancelRef.current;

      setBulkRotationState((prev) => {
        if (!prev) {
          return prev;
        }

        return {
          ...prev,
          status: 'completed',
          completedAt: new Date(),
          cancelled: wasCancelled,
        } satisfies BulkRotationState;
      });

      if (!wasCancelled) {
        setSelectedDeviceUuids(() => new Set());
      }

      bulkRotationCancelRef.current = false;
    },
    [queryClient],
  );

  const handleBulkRegenerateCredentials = () => {
    if (isBulkOperationRunning) {
      return;
    }

    if (!canManageDeviceCredentials) {
      setBulkActionError(DEVICE_CREDENTIAL_GUARD_MESSAGE);
      return;
    }

    const targetMap = new Map(devices.map((device) => [device.uuid, device]));
    const targets: DeviceListItem[] = [];

    selectedDeviceUuids.forEach((uuid) => {
      const entry = targetMap.get(uuid);
      if (entry) {
        targets.push(entry);
      }
    });

    if (!targets.length) {
      setBulkActionError('Select at least one device before running bulk actions.');
      return;
    }

    const initialResults = targets.reduce<Record<string, BulkCredentialResult>>((acc, device) => {
      acc[device.uuid] = {
        status: 'pending',
        deviceImei: device.imei ?? null,
      } satisfies BulkCredentialResult;
      return acc;
    }, {});

    bulkRotationCancelRef.current = false;
    setBulkActionError(null);
    setBulkRotationState({
      status: 'running',
      devices: targets,
      results: initialResults,
      startedAt: new Date(),
      completedAt: null,
      cancelled: false,
    });

    void processBulkCredentialRotation(targets);
  };

  const handleCancelBulkRotation = () => {
    if (bulkRotationState?.status !== 'running') {
      return;
    }

    bulkRotationCancelRef.current = true;
    setBulkRotationState((prev) => (prev ? { ...prev, cancelled: true } : prev));
  };

  const handleCloseBulkRotationModal = () => {
    if (bulkRotationState?.status === 'running') {
      return;
    }

    setBulkRotationState(null);
    bulkRotationCancelRef.current = false;
  };

  const handleDownloadBulkCredentialZip = () => {
    if (!bulkRotationState || bulkRotationState.status !== 'completed') {
      return;
    }

    const generatedAt = new Date().toISOString();
    const files = bulkRotationState.devices
      .map((device) => {
        const result = bulkRotationState.results[device.uuid];
        if (!result || result.status !== 'success' || !result.credentials) {
          return null;
        }

        const imei = result.deviceImei ?? device.imei ?? null;
        const payload = buildBulkCredentialPayload(
          device.uuid,
          imei,
          result.credentials,
          generatedAt,
        );
        const safeId = (imei ?? device.uuid).replace(/[^a-zA-Z0-9_-]/g, '-');

        return {
          filename: `device-${safeId}-credentials.json`,
          contents: JSON.stringify(payload, null, 2),
        };
      })
      .filter(Boolean) as Array<{ filename: string; contents: string }>;

    if (!files.length) {
      setBulkActionError(
        'Credential bundles are not available yet. Retry after provisioning confirms each device.',
      );
      return;
    }

    const zipBlob = createZipBlob(files);
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    downloadBlob(`bulk-device-credentials-${timestamp}.zip`, zipBlob);
  };

  const handleGovernmentInputChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    setGovernmentForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    setGovernmentError(null);
    setGovernmentSuccess(null);

    if (name === 'deviceUuid') {
      setQueueForm((prev) => ({ ...prev, deviceUuid: value }));
      setPendingState((prev) => ({ ...prev, deviceUuid: value }));
      setAckForm((prev) => ({ ...prev, deviceUuid: value }));
      setRotationForm((prev) => ({ ...prev, deviceUuid: value }));
      setRevokeForm((prev) => ({ ...prev, deviceUuid: value }));
      setResyncForm((prev) => ({ ...prev, deviceUuid: value }));
      setRevokeResult(null);
      setRevokeError(null);
      setResyncResult(null);
      setResyncError(null);
    }
  };

  const applyActiveGovernmentBundle = () => {
    if (!activeGovernmentCredentials) {
      setGovernmentError('No active government credential bundle available to copy.');
      return;
    }

    setGovernmentForm((prev) => ({
      ...prev,
      clientId: activeGovernmentCredentials.clientId,
      username: activeGovernmentCredentials.username,
      password: activeGovernmentCredentials.password,
      endpointsJson: JSON.stringify(activeGovernmentCredentials.endpoints, null, 2),
      publishTopicsRaw: activeGovernmentCredentials.topics.publish.join('\n'),
      subscribeTopicsRaw: activeGovernmentCredentials.topics.subscribe.join('\n'),
      issuedBy: activeGovernmentCredentials.issuedBy ?? prev.issuedBy,
    }));
    setGovernmentError(null);
    setGovernmentSuccess(null);
    setGovernmentResult(null);
  };

  const applyProtocolDefaultsToGovernmentForm = () => {
    if (!governmentDefaults) {
      setGovernmentError('Protocol metadata does not define government credential defaults.');
      return;
    }

    setGovernmentForm((prev) => ({
      ...prev,
      endpointsJson: JSON.stringify(governmentDefaults.endpoints, null, 2),
      publishTopicsRaw: governmentDefaults.topics.publish.join('\n'),
      subscribeTopicsRaw: governmentDefaults.topics.subscribe.join('\n'),
    }));
    setGovernmentError(null);
    setGovernmentSuccess(null);
    setGovernmentResult(null);
  };

  const handleCopyCredentialBundle = useCallback(
    async (flavor: BundleFlavor, bundle: MockDashboardBundleExport | null) => {
      const setFeedback = (message: string | null, error: string | null) => {
        setBundleCopyFeedback((prev) => ({
          ...prev,
          [flavor]: { message, error },
        }));
      };

      setFeedback(null, null);

      if (!bundle) {
        const missingMessage =
          flavor === 'local'
            ? 'Load a device with an active platform credential bundle before copying the mock dashboard JSON.'
            : 'Load a device with an active government credential bundle before copying the mock dashboard JSON.';
        setFeedback(null, missingMessage);
        return;
      }

      const clipboard = typeof navigator !== 'undefined' ? navigator.clipboard : null;
      const serialized = JSON.stringify(bundle, null, 2);

      if (!clipboard || typeof clipboard.writeText !== 'function') {
        setFeedback(
          null,
          'Clipboard access is unavailable. Copy the bundle JSON from the preview manually.',
        );
        return;
      }

      try {
        await clipboard.writeText(serialized);
        setFeedback(
          'Bundle JSON copied. Paste it into the mock dashboard import field to prefill MQTT settings.',
          null,
        );
      } catch (error) {
        console.error('Unable to copy mock dashboard bundle JSON', error);
        setFeedback(null, 'Clipboard copy failed. Copy the JSON from the preview manually.');
      }
    },
    [],
  );

  const handleGovernmentReset = () => {
    setGovernmentForm((prev) => ({ ...defaultGovernmentForm, deviceUuid: prev.deviceUuid }));
    setGovernmentResult(null);
    setGovernmentError(null);
    setGovernmentSuccess(null);
    governmentMutation.reset();
  };

  const handleGovernmentSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!canManageDeviceCredentials) {
      setGovernmentError(DEVICE_CREDENTIAL_GUARD_MESSAGE);
      setGovernmentSuccess(null);
      return;
    }

    const deviceUuid = governmentForm.deviceUuid.trim();
    if (!deviceUuid) {
      setGovernmentError('Provide a device UUID before saving government credentials.');
      return;
    }

    const clientId = governmentForm.clientId.trim();
    const username = governmentForm.username.trim();
    const password = governmentForm.password.trim();

    if (!clientId || !username || !password) {
      setGovernmentError(
        'Client ID, username, and password are required for government credentials.',
      );
      return;
    }

    if (!governmentForm.endpointsJson.trim()) {
      setGovernmentError('Provide at least one endpoint in JSON format.');
      return;
    }

    let endpointEntries: unknown;
    try {
      endpointEntries = JSON.parse(governmentForm.endpointsJson);
    } catch {
      setGovernmentError('Endpoints JSON is invalid. Provide a valid JSON array.');
      return;
    }

    if (!Array.isArray(endpointEntries)) {
      setGovernmentError('Endpoints JSON must be an array of endpoint objects.');
      return;
    }

    const normalizedEndpoints = (endpointEntries as unknown[])
      .map(normalizeGovernmentCredentialEndpoint)
      .filter(
        (endpoint): endpoint is GovernmentCredentialDefaults['endpoints'][number] =>
          endpoint !== null,
      );

    if (!normalizedEndpoints.length) {
      setGovernmentError('Provide at least one valid endpoint with protocol, host, and port.');
      return;
    }

    let metadata: Record<string, unknown> | undefined;
    if (governmentForm.metadataJson.trim()) {
      try {
        const parsedMetadata = JSON.parse(governmentForm.metadataJson);
        if (parsedMetadata && typeof parsedMetadata === 'object') {
          metadata = parsedMetadata as Record<string, unknown>;
        } else {
          setGovernmentError('Metadata JSON must resolve to an object.');
          return;
        }
      } catch {
        setGovernmentError('Metadata JSON is invalid. Provide a valid JSON object.');
        return;
      }
    }

    const publishTopics = parseTopicsInput(governmentForm.publishTopicsRaw);
    const subscribeTopics = parseTopicsInput(governmentForm.subscribeTopicsRaw);

    const payload: GovernmentCredentialPayload = {
      clientId,
      username,
      password,
      endpoints: normalizedEndpoints,
    };

    if (publishTopics.length || subscribeTopics.length) {
      payload.topics = {
        publish: publishTopics,
        subscribe: subscribeTopics,
      };
    }

    if (metadata) {
      payload.metadata = metadata;
    }

    const issuedByValue = governmentForm.issuedBy.trim();
    if (issuedByValue) {
      payload.issuedBy = issuedByValue;
    }

    setGovernmentError(null);
    setGovernmentResult(null);
    setGovernmentSuccess(null);

    governmentMutation.mutate({ deviceUuid, payload });
  };

  const handleRotationReset = () => {
    setRotationForm(defaultRotationForm);
    setRotationResult(null);
    setRotationError(null);
    rotateMutation.reset();
  };

  return (
    <div className="space-y-6">
      {showInternalSections && (
        <>
          <BulkDeviceOperationsSection
            deviceSearch={deviceSearch}
            onSearchChange={handleDeviceSearchInputChange}
            onRefresh={handleRefreshDeviceList}
            isLoading={deviceListQuery.isLoading}
            isFetching={deviceListQuery.isFetching}
            error={deviceListError}
            devices={devices}
            filteredDevices={filteredDevices}
            totalSelected={totalSelected}
            displayedSelectedCount={displayedSelectedCount}
            allDisplayedSelected={allDisplayedSelected}
            selectedDeviceUuids={selectedDeviceUuids}
            onToggleDevice={toggleDeviceSelection}
            onToggleAllDisplayed={toggleAllDisplayedSelections}
            onClearSelection={clearSelectedDevices}
            onBulkRegenerate={handleBulkRegenerateCredentials}
            bulkActionError={bulkActionError}
            canManageDeviceCredentials={canManageDeviceCredentials}
            isBulkOperationRunning={isBulkOperationRunning}
            headerCheckboxRef={headerCheckboxRef}
          />
          <section
            aria-labelledby="device-config-regenerate-credentials-heading"
            className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
          >
            <h2 id="device-config-regenerate-credentials-heading" className="text-lg font-semibold">
              Regenerate Device Credentials
            </h2>
            <p className="mt-2 text-sm text-slate-600">
              Request new MQTT credentials for a device while preserving the protocol-version tuple
              and updating the broker access control list.
            </p>
            {!canManageDeviceCredentials && (
              <div className="mt-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
                {DEVICE_CREDENTIAL_GUARD_MESSAGE}
              </div>
            )}
            <form className="mt-4 grid gap-4 md:grid-cols-2" onSubmit={handleRotationSubmit}>
              <InputField
                label="Device UUID"
                name="deviceUuid"
                value={rotationForm.deviceUuid}
                onChange={handleRotationInputChange}
                required
              />
              <InputField
                label="Rotation Reason (optional)"
                name="reason"
                value={rotationForm.reason}
                onChange={handleRotationInputChange}
                placeholder="scheduled-rotation"
              />
              <InputField
                label="Issued By (optional)"
                name="issuedBy"
                value={rotationForm.issuedBy}
                onChange={handleRotationInputChange}
                placeholder="Operator ID"
              />
              <div className="flex items-center gap-3 md:col-span-2">
                <button
                  type="submit"
                  className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                  disabled={!canManageDeviceCredentials || rotateMutation.isPending}
                >
                  {rotateMutation.isPending ? 'Rotating…' : 'Rotate Credentials'}
                </button>
                <button
                  type="button"
                  className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                  onClick={handleRotationReset}
                >
                  Reset
                </button>
              </div>
            </form>
            {rotationError && (
              <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {rotationError}
              </div>
            )}
            {rotationResult && (
              <div className="mt-6 space-y-4">
                <h3 className="text-base font-semibold text-emerald-700">
                  Credentials regenerated
                </h3>
                <div className="grid gap-4 md:grid-cols-2">
                  <Detail label="Device UUID" value={rotationResult.device.id} />
                  <Detail label="IMEI" value={rotationResult.device.imei} />
                </div>
                {rotationCredentials ? (
                  <>
                    <div className="grid gap-4 md:grid-cols-2">
                      <Detail label="Client ID" value={rotationCredentials.clientId} />
                      <Detail label="Username" value={rotationCredentials.username} />
                      <Detail label="Password" value={rotationCredentials.password} />
                      <Detail
                        label="Broker Access Applied"
                        value={rotationCredentials.mqttAccess?.applied ? 'Yes' : 'No'}
                      />
                    </div>
                    {rotationCredentials.endpoints.length > 0 && (
                      <div className="grid gap-3 md:grid-cols-2">
                        {rotationCredentials.endpoints.map((endpoint, index) => (
                          <Detail
                            key={`${endpoint.protocol}-${endpoint.host}-${endpoint.port}-${index}`}
                            label={`${endpoint.protocol.toUpperCase()} Endpoint`}
                            value={`${endpoint.url} (${endpoint.host}:${endpoint.port})`}
                          />
                        ))}
                      </div>
                    )}
                    <JsonPreview title="Publish Topics" data={rotationCredentials.topics.publish} />
                    <JsonPreview
                      title="Subscribe Topics"
                      data={rotationCredentials.topics.subscribe}
                    />
                  </>
                ) : (
                  <p className="rounded-md border border-slate-200 bg-slate-50 p-3 text-sm text-slate-600">
                    MQTT credentials will appear once the provisioning worker applies the latest
                    bundle.
                  </p>
                )}
              </div>
            )}
          </section>

          <section
            aria-labelledby="device-config-revoke-credentials-heading"
            className="mt-8 rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
          >
            <h3
              id="device-config-revoke-credentials-heading"
              className="text-base font-semibold text-slate-700"
            >
              Revoke Device Credentials
            </h3>
            <p className="mt-2 text-sm text-slate-600">
              Close active credential bundles when a device is decommissioned or its secrets are
              compromised. This marks the lifecycle as revoked and prevents future authentication.
            </p>
            {!canManageDeviceCredentials && (
              <div className="mt-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
                {DEVICE_CREDENTIAL_GUARD_MESSAGE}
              </div>
            )}
            <form className="mt-4 grid gap-4 md:grid-cols-2" onSubmit={handleRevokeSubmit}>
              <InputField
                label="Device UUID"
                name="deviceUuid"
                value={revokeForm.deviceUuid}
                onChange={handleRevokeInputChange}
              />
              <div className="flex flex-col gap-2 text-sm font-medium text-slate-700">
                <span>Credential Type</span>
                <div className="flex gap-4">
                  <label className="flex items-center gap-2 text-slate-600">
                    <input
                      type="radio"
                      name="credentialType"
                      value="local"
                      checked={revokeForm.credentialType === 'local'}
                      onChange={handleRevokeInputChange}
                    />
                    Local
                  </label>
                  <label className="flex items-center gap-2 text-slate-600">
                    <input
                      type="radio"
                      name="credentialType"
                      value="government"
                      checked={revokeForm.credentialType === 'government'}
                      onChange={handleRevokeInputChange}
                    />
                    Government
                  </label>
                </div>
              </div>
              <InputField
                label="Issued By (optional)"
                name="issuedBy"
                value={revokeForm.issuedBy}
                onChange={handleRevokeInputChange}
                placeholder="Mongo ObjectId"
              />
              <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-2">
                <span>Revocation Reason (optional)</span>
                <textarea
                  className="min-h-[96px] rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  name="reason"
                  value={revokeForm.reason}
                  onChange={handleRevokeInputChange}
                  placeholder="Explain why the credentials were revoked"
                />
              </label>
              <div className="flex items-center gap-3 md:col-span-2">
                <button
                  type="submit"
                  className="rounded-md bg-red-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-red-500 focus:outline-none focus:ring-2 focus:ring-red-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={!canManageDeviceCredentials || revokeMutation.isPending}
                >
                  {revokeMutation.isPending ? 'Revoking…' : 'Revoke Credentials'}
                </button>
                <button
                  type="button"
                  className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                  onClick={handleRevokeReset}
                  disabled={revokeMutation.isPending}
                >
                  Reset
                </button>
              </div>
            </form>
            {revokeError && (
              <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
                {revokeError}
              </div>
            )}
            {revokeResult && (
              <div className="mt-4 space-y-2 rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
                <p className="font-semibold">Revocation complete</p>
                <p>
                  Revoked {revokeResult.revokedCount}{' '}
                  {revokeResult.revokedCount === 1 ? 'credential bundle.' : 'credential bundles.'}
                </p>
                {revokeResult.lifecycleTransitions.length > 0 && (
                  <ul className="list-disc pl-5 text-emerald-900">
                    {revokeResult.lifecycleTransitions.map((transition, index) => (
                      <li key={`${transition.type}-${transition.from}-${transition.to}-${index}`}>
                        {transition.type === 'government' ? 'Government' : 'Local'}:{' '}
                        {` ${formatLifecycleText(transition.from)} -> ${formatLifecycleText(transition.to)} (${transition.count})`}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}
            <div className="mt-8">
              <h3 className="text-base font-semibold text-slate-700">Credential History</h3>
              {!statusDeviceUuid ? (
                <p className="mt-2 text-sm text-slate-500">
                  Enter a device UUID to load credential history.
                </p>
              ) : deviceStatusQuery.isLoading ? (
                <p className="mt-2 text-sm text-slate-500">Loading credential history…</p>
              ) : deviceStatus?.credentialsHistory?.length ? (
                <div className="mt-3 overflow-x-auto">
                  <table className="min-w-full divide-y divide-slate-200 text-sm">
                    <thead className="bg-slate-50">
                      <tr>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">Type</th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          Username
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          Client ID
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          Valid From
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          Valid To
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          Lifecycle
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">Reason</th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          MQTT Access
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          Import Job
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-600">
                          Protocol Selector
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200">
                      {deviceStatus.credentialsHistory.map((entry: DeviceCredentialHistoryItem) => {
                        const validFrom = new Date(entry.validFrom).toLocaleString();
                        const validTo = entry.validTo
                          ? new Date(entry.validTo).toLocaleString()
                          : 'Active';
                        const lifecycleLabel = formatLifecycleLabel(entry.lifecycle);
                        const lifecycleTooltip = formatLifecycleHistoryTooltip(
                          entry.lifecycleHistory,
                        );
                        const mqttAccessLabel = formatAccessStatus(entry.mqttAccessApplied);
                        const mqttJobId = entry.mqttAccess?.jobId ?? null;
                        const mqttJobLine = mqttJobId ? `Job ${mqttJobId}` : null;
                        const importJobId = entry.originImportJobId ?? null;
                        const protocolLabel = formatProtocolSelectorLabel(entry.protocolSelector);
                        const protocolTitle = formatProtocolSelectorTitle(entry.protocolSelector);

                        return (
                          <tr key={`${entry.clientId}-${entry.validFrom}`}>
                            <td className="px-3 py-2 uppercase text-slate-500">{entry.type}</td>
                            <td className="px-3 py-2 text-slate-800">{entry.username}</td>
                            <td className="px-3 py-2 text-slate-800">{entry.clientId}</td>
                            <td className="px-3 py-2 text-slate-600">{validFrom}</td>
                            <td className="px-3 py-2 text-slate-600">{validTo}</td>
                            <td className="px-3 py-2 text-slate-600" title={lifecycleTooltip}>
                              {lifecycleLabel}
                            </td>
                            <td className="px-3 py-2 text-slate-600">
                              {entry.rotationReason ?? '—'}
                            </td>
                            <td className="px-3 py-2 text-slate-600">
                              <div>{mqttAccessLabel}</div>
                              {mqttJobLine ? (
                                <div className="text-[11px] text-slate-500">{mqttJobLine}</div>
                              ) : null}
                            </td>
                            <td className="px-3 py-2 text-slate-600">
                              {importJobId ? <ImportJobLink jobId={importJobId} /> : '—'}
                            </td>
                            <td className="px-3 py-2 text-slate-600" title={protocolTitle}>
                              {protocolLabel}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="mt-2 text-sm text-slate-500">No credential history recorded yet.</p>
              )}
            </div>
          </section>
        </>
      )}

      {showGovernmentSection && (
        <section
          aria-labelledby="device-config-government-credentials-heading"
          className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
        >
          <h2 id="device-config-government-credentials-heading" className="text-lg font-semibold">
            Manage Government Credentials
          </h2>
          <p className="mt-2 text-sm text-slate-600">
            Update the government broker credentials for an enrolled device. Use the current bundle
            or protocol defaults as a starting point before saving the changes.
          </p>
          {!canManageDeviceCredentials && (
            <div className="mt-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
              {DEVICE_CREDENTIAL_GUARD_MESSAGE}
            </div>
          )}
          <div className="mt-4 flex flex-wrap gap-3">
            <button
              type="button"
              className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={applyActiveGovernmentBundle}
              disabled={!activeGovernmentCredentials}
            >
              Use Active Bundle
            </button>
            <button
              type="button"
              className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={applyProtocolDefaultsToGovernmentForm}
              disabled={!governmentDefaults}
            >
              Apply Protocol Defaults
            </button>
          </div>
          <form className="mt-4 grid gap-4 md:grid-cols-2" onSubmit={handleGovernmentSubmit}>
            <div>
              <InputField
                label="Device UUID"
                name="deviceUuid"
                value={governmentForm.deviceUuid}
                onChange={handleGovernmentInputChange}
                required
              />
            </div>
            <div>
              <InputField
                label="Issued By (optional)"
                name="issuedBy"
                value={governmentForm.issuedBy}
                onChange={handleGovernmentInputChange}
                placeholder="Operator ID"
              />
            </div>
            <div>
              <InputField
                label="Client ID"
                name="clientId"
                value={governmentForm.clientId}
                onChange={handleGovernmentInputChange}
                required
                placeholder="Government client ID"
              />
            </div>
            <div>
              <InputField
                label="Username"
                name="username"
                value={governmentForm.username}
                onChange={handleGovernmentInputChange}
                required
                placeholder="Government username"
              />
            </div>
            <div>
              <InputField
                label="Password"
                name="password"
                value={governmentForm.password}
                onChange={handleGovernmentInputChange}
                required
                placeholder="Government password"
              />
            </div>
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-2">
              <span>Endpoints JSON (array)</span>
              <textarea
                className="h-36 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                name="endpointsJson"
                value={governmentForm.endpointsJson}
                onChange={handleGovernmentInputChange}
                placeholder='[{"protocol":"mqtt","host":"broker.gov","port":1886,"url":"mqtt://broker.gov:1886"}]'
              />
            </label>
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-1">
              <span>Publish Topics (newline or comma separated)</span>
              <textarea
                className="h-28 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                name="publishTopicsRaw"
                value={governmentForm.publishTopicsRaw}
                onChange={handleGovernmentInputChange}
                placeholder="123456789012345/heartbeat"
              />
            </label>
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-1">
              <span>Subscribe Topics (newline or comma separated)</span>
              <textarea
                className="h-28 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                name="subscribeTopicsRaw"
                value={governmentForm.subscribeTopicsRaw}
                onChange={handleGovernmentInputChange}
                placeholder="123456789012345/ondemand"
              />
            </label>
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-2">
              <span>Metadata JSON (optional)</span>
              <textarea
                className="h-28 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                name="metadataJson"
                value={governmentForm.metadataJson}
                onChange={handleGovernmentInputChange}
                placeholder='{"source":"ui"}'
              />
            </label>
            <div className="flex items-center gap-3 md:col-span-2">
              <button
                type="submit"
                className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!canManageDeviceCredentials || governmentMutation.isPending}
              >
                {governmentMutation.isPending ? 'Saving…' : 'Save Government Credentials'}
              </button>
              <button
                type="button"
                className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={handleGovernmentReset}
              >
                Reset
              </button>
            </div>
          </form>
          {governmentError && (
            <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              {governmentError}
            </div>
          )}
          {governmentSuccess && (
            <div className="mt-4 rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
              {governmentSuccess}
            </div>
          )}
          {governmentResult && (
            <div className="mt-6 space-y-4">
              <h3 className="text-base font-semibold text-emerald-700">
                Government credentials saved
              </h3>
              <div className="grid gap-4 md:grid-cols-2">
                <Detail label="Client ID" value={governmentResult.clientId} />
                <Detail label="Username" value={governmentResult.username} />
                <Detail label="Password" value={governmentResult.password} />
              </div>
              <JsonPreview title="Endpoints" data={governmentResult.endpoints} />
              <JsonPreview title="Topics" data={governmentResult.topics} />
            </div>
          )}
        </section>
      )}

      {showInternalSections && (
        <>
          <section
            aria-labelledby="device-config-queue-heading"
            className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
          >
            <h2 id="device-config-queue-heading" className="text-lg font-semibold">
              Queue Device Configuration
            </h2>
            <p className="mt-2 text-sm text-slate-600">
              Select a VFD model and push the configuration payload ahead of telemetry onboarding.
            </p>
            {statusDeviceUuid && (
              <div className="mt-4 rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="font-semibold text-slate-800">Device Status Snapshot</div>
                  <span className="text-xs text-slate-500">UUID {statusDeviceUuid}</span>
                </div>
                {deviceStatusQuery.isLoading && (
                  <p className="mt-2 text-xs text-slate-500">Loading status…</p>
                )}
                {deviceStatusQuery.isError && (
                  <p className="mt-2 text-xs text-red-600">
                    {deviceStatusQuery.error?.message || 'Unable to load device status'}
                  </p>
                )}
                {deviceStatus && (
                  <>
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
                      <div>
                        <span className="block text-[11px] uppercase tracking-wide text-slate-500">
                          Import Job
                        </span>
                        {deviceStatus.device.originImportJobId ? (
                          <ImportJobLink jobId={deviceStatus.device.originImportJobId} showLabel />
                        ) : (
                          '—'
                        )}
                      </div>
                    </div>
                    {localMockDashboardBundle && (
                      <MockDashboardBundlePanel
                        title="Mock Dashboard Bundle (Platform)"
                        description="Use these credentials to exercise the RMS stack without touching the government broker."
                        bundle={localMockDashboardBundle}
                        onCopy={() => handleCopyCredentialBundle('local', localMockDashboardBundle)}
                        successMessage={bundleCopyFeedback.local.message}
                        errorMessage={bundleCopyFeedback.local.error}
                      />
                    )}
                    <GovernmentCredentialSummary
                      government={activeGovernmentCredentials}
                      defaults={governmentDefaults}
                    />
                    {governmentMockDashboardBundle && (
                      <MockDashboardBundlePanel
                        title="Mock Dashboard Bundle (Government)"
                        description="Only use when validating the government MQTT path."
                        bundle={governmentMockDashboardBundle}
                        onCopy={() =>
                          handleCopyCredentialBundle('government', governmentMockDashboardBundle)
                        }
                        successMessage={bundleCopyFeedback.government.message}
                        errorMessage={bundleCopyFeedback.government.error}
                      />
                    )}
                    {provisioningInfo && (
                      <ProvisioningStatusPanel
                        provisioning={provisioningInfo}
                        onRetry={() => handleProvisioningRetry(deviceStatus.device.uuid)}
                        retryPending={provisioningRetryMutation.isPending}
                        retryError={provisioningRetryError}
                        retryMessage={provisioningRetryMessage}
                      />
                    )}
                  </>
                )}
              </div>
            )}
            <div className="mt-4 rounded-md border border-slate-200 bg-white p-4 shadow-sm">
              <form
                className="space-y-4"
                aria-labelledby="device-config-broker-resync-heading"
                aria-label="Queue Broker Resync"
                onSubmit={handleResyncSubmit}
              >
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <h3
                      id="device-config-broker-resync-heading"
                      className="text-sm font-semibold text-slate-700"
                    >
                      Queue Broker Resync
                    </h3>
                    <p className="mt-1 text-xs text-slate-500">
                      Use this when EMQX credentials were reset or ACL updates need to be replayed.
                    </p>
                  </div>
                  <button
                    type="submit"
                    className="rounded-md bg-emerald-600 px-3 py-1.5 text-sm font-semibold text-white shadow-sm transition hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
                    disabled={resyncMutation.isPending}
                  >
                    {resyncMutation.isPending ? 'Queuing…' : 'Queue Resync'}
                  </button>
                </div>
                {!statusDeviceUuid && (
                  <p className="rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
                    Provide a device UUID to load the current provisioning snapshot before queueing
                    a broker resync.
                  </p>
                )}
                <div className="grid gap-3 md:grid-cols-1">
                  <InputField
                    label="Device UUID (Broker Resync)"
                    name="deviceUuid"
                    value={resyncForm.deviceUuid}
                    onChange={handleResyncInputChange}
                    ariaLabel="Device UUID (Broker Resync)"
                  />
                </div>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Resync Reason (optional)</span>
                  <textarea
                    className="min-h-[88px] rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    name="reason"
                    value={resyncForm.reason}
                    onChange={handleResyncInputChange}
                    placeholder="Document why the broker sync must be replayed"
                  />
                </label>
                <div className="flex flex-wrap items-center gap-3">
                  <button
                    type="button"
                    className="rounded-md border border-slate-300 px-3 py-1.5 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                    onClick={handleResyncReset}
                    disabled={resyncMutation.isPending}
                  >
                    Reset
                  </button>
                </div>
                {resyncError && (
                  <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
                    {resyncError}
                  </div>
                )}
                {resyncResult && (
                  <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800">
                    <p className="font-semibold">Broker resync queued</p>
                    <p className="mt-1">
                      Job {resyncResult.mqttProvisioning.jobId} queued (resync count{' '}
                      {resyncResult.resyncCount}).
                    </p>
                  </div>
                )}
              </form>
            </div>
            <form className="mt-4 grid gap-4 md:grid-cols-2" onSubmit={handleQueueSubmit}>
              <InputField
                label="Device UUID"
                name="deviceUuid"
                value={queueForm.deviceUuid}
                onChange={handleQueueInputChange}
                required
              />
              <InputField
                label="Protocol Version Filter (optional)"
                name="protocolVersionIdFilter"
                value={queueForm.protocolVersionIdFilter}
                onChange={handleQueueInputChange}
                placeholder="Protocol Version ObjectId"
              />
              <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-2">
                <span>VFD Model</span>
                <select
                  className="rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  name="vfdModelId"
                  value={queueForm.vfdModelId}
                  onChange={handleQueueInputChange}
                  required
                  disabled={vfdModelsQuery.isLoading}
                >
                  <option value="" disabled>
                    {vfdModelsQuery.isLoading ? 'Loading VFD models…' : 'Select VFD model'}
                  </option>
                  {vfdModels.map((model) => (
                    <option key={model.id} value={model.id}>
                      {model.manufacturer} — {model.model} {formatVersionLabel(model.version)}
                    </option>
                  ))}
                </select>
              </label>
              {vfdModelsQuery.isError && (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 md:col-span-2">
                  {vfdModelsCapabilityError
                    ? 'Current session is missing the catalog:drives capability (and catalog:rs485 for command imports). Contact an administrator to enable the required access before managing VFD models.'
                    : vfdModelsError?.message || 'Unable to load VFD models'}
                </div>
              )}
              {selectedModel && (
                <div className="md:col-span-2">
                  <VfdModelSummary model={selectedModel} />
                </div>
              )}
              <SelectField
                label="Transport"
                name="transport"
                value={queueForm.transport}
                onChange={handleQueueInputChange}
                options={[
                  { value: 'mqtt', label: 'MQTT (default)' },
                  { value: 'https', label: 'HTTPS mirror' },
                ]}
              />
              <InputField
                label="Issued By"
                name="issuedBy"
                value={queueForm.issuedBy}
                onChange={handleQueueInputChange}
                placeholder="Operator ID"
              />
              <InputField
                label="QoS (0-2)"
                name="qos"
                value={queueForm.qos}
                onChange={handleQueueInputChange}
                placeholder="Overrides MQTT QoS"
              />
              <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-2">
                <span>Overrides JSON (optional)</span>
                <textarea
                  className="h-28 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  name="overridesJson"
                  value={queueForm.overridesJson}
                  onChange={handleQueueInputChange}
                  placeholder='{"rs485": {"baudRate": 9600}}'
                />
              </label>
              <div className="flex items-center gap-3 md:col-span-2">
                <button
                  type="submit"
                  className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                  disabled={queueMutation.isPending}
                >
                  {queueMutation.isPending ? 'Queuing…' : 'Queue Configuration'}
                </button>
                <button
                  type="button"
                  className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                  onClick={() => {
                    setQueueForm(defaultQueueForm);
                    setQueueResult(null);
                    setQueueError(null);
                    setPendingState({ deviceUuid: '', lastFetchedAt: null });
                    setAckForm(defaultAckForm);
                    setGovernmentForm(defaultGovernmentForm);
                    setGovernmentResult(null);
                    setGovernmentError(null);
                    setGovernmentSuccess(null);
                  }}
                >
                  Reset
                </button>
              </div>
            </form>
            {queueError && (
              <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {queueError}
              </div>
            )}
            {queueResult && selectedModel && (
              <div className="mt-6 space-y-4">
                <h3 className="text-base font-semibold text-emerald-700">Configuration queued</h3>
                <Detail label="Transport" value={queueResult.transport.toUpperCase()} />
                <Detail label="Message ID" value={queueResult.msgid ?? 'Pending assignment'} />
                <div className="grid gap-4 md:grid-cols-2">
                  <Detail label="Manufacturer" value={selectedModel.manufacturer} />
                  <Detail
                    label="Model"
                    value={`${selectedModel.model} ${formatVersionLabel(selectedModel.version)}`}
                  />
                </div>
                <JsonPreview title="Payload" data={queueResult.configuration} />
              </div>
            )}
          </section>
          <section
            aria-labelledby="device-config-commands-heading"
            className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
          >
            <h2 id="device-config-commands-heading" className="text-lg font-semibold">
              Device Command Controls
            </h2>
            <p className="mt-2 text-sm text-slate-600">
              Command issuance, acknowledgements, and history now live in the dedicated Command
              Center. Use the link below to manage commands for any device.
            </p>
            <div className="mt-4">
              <Link
                to="/operations/command-center"
                className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
              >
                Open Command Center
              </Link>
              <p className="mt-2 text-xs text-slate-600">
                View and issue commands, record responses, and inspect history from the centralized
                Command Center.
              </p>
            </div>
          </section>

          <section
            aria-labelledby="device-config-pending-status-heading"
            className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
          >
            <h2 id="device-config-pending-status-heading" className="text-lg font-semibold">
              Pending Configuration Status
            </h2>
            <p className="mt-2 text-sm text-slate-600">
              Poll for pending records by device UUID. A 204 response indicates the device has no
              outstanding configuration.
            </p>
            <div className="mt-4 flex flex-col gap-3 md:flex-row md:items-end">
              <InputField
                label="Device UUID"
                name="pendingDeviceUuid"
                value={pendingState.deviceUuid}
                onChange={handlePendingUuidChange}
                required
              />
              <button
                type="button"
                className="h-10 rounded-md bg-emerald-600 px-4 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                onClick={handleRefreshPending}
                disabled={pendingLoading}
              >
                {pendingLoading ? 'Refreshing…' : 'Refresh'}
              </button>
              {pendingState.lastFetchedAt && (
                <p className="text-xs text-slate-500">
                  Last fetched {pendingState.lastFetchedAt.toLocaleTimeString()}
                </p>
              )}
            </div>
            {pendingError && (
              <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {pendingError}
              </div>
            )}
            {!pendingError && !pendingLoading && pendingState.lastFetchedAt && !pendingRecord && (
              <div className="mt-4 rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">
                No pending configuration for this device.
              </div>
            )}
            {pendingRecord && (
              <div className="mt-6 space-y-4">
                <Detail label="Status" value={pendingRecord.status.toUpperCase()} />
                <Detail label="Message ID" value={pendingRecord.msgid ?? 'N/A'} />
                <Detail label="Transport" value={pendingRecord.transport.toUpperCase()} />
                <Detail
                  label="Requested"
                  value={new Date(pendingRecord.requestedAt).toLocaleString()}
                />
                {pendingRecord.acknowledgedAt && (
                  <Detail
                    label="Acknowledged"
                    value={new Date(pendingRecord.acknowledgedAt).toLocaleString()}
                  />
                )}
                <JsonPreview title="Configuration" data={pendingRecord.configuration} />
                {pendingRecord.acknowledgementPayload && (
                  <JsonPreview
                    title="Acknowledgement Payload"
                    data={pendingRecord.acknowledgementPayload}
                  />
                )}
              </div>
            )}
          </section>

          <section
            aria-labelledby="device-config-ack-override-heading"
            className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
          >
            <h2 id="device-config-ack-override-heading" className="text-lg font-semibold">
              Config Ack Override
            </h2>
            <p className="mt-2 text-sm text-slate-600">
              Store acknowledgement details received from MQTT worker, field tools, or manual
              testers when the platform needs a hand-closing a job.
            </p>
            <form className="mt-4 grid gap-4 md:grid-cols-2" onSubmit={handleAckSubmit}>
              <InputField
                label="Device UUID"
                name="deviceUuid"
                value={ackForm.deviceUuid}
                onChange={handleAckChange}
                required
              />
              <SelectField
                label="Status"
                name="status"
                value={ackForm.status}
                onChange={handleAckChange}
                options={[
                  { value: 'acknowledged', label: 'Acknowledged' },
                  { value: 'failed', label: 'Failed' },
                ]}
              />
              <InputField
                label="Message ID (optional)"
                name="msgid"
                value={ackForm.msgid}
                onChange={handleAckChange}
                placeholder="Match device response msgid"
              />
              <InputField
                label="Received At (ISO 8601)"
                name="receivedAt"
                value={ackForm.receivedAt}
                onChange={handleAckChange}
                placeholder="2025-04-01T10:00:00Z"
              />
              <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-2">
                <span>Acknowledgement Payload (JSON)</span>
                <textarea
                  className="h-28 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  name="payloadJson"
                  value={ackForm.payloadJson}
                  onChange={handleAckChange}
                  placeholder='{"status":"ok"}'
                />
              </label>
              <div className="flex items-center gap-3 md:col-span-2">
                <button
                  type="submit"
                  className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                  disabled={acknowledgeMutation.isPending}
                >
                  {acknowledgeMutation.isPending ? 'Submitting…' : 'Record Acknowledgement'}
                </button>
                <button
                  type="button"
                  className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                  onClick={() => {
                    setAckForm(defaultAckForm);
                    setAckResult(null);
                    setAckError(null);
                  }}
                >
                  Reset
                </button>
              </div>
            </form>
            {ackError && (
              <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {ackError}
              </div>
            )}
            {ackResult && (
              <div className="mt-6 space-y-4">
                <h3 className="text-base font-semibold text-emerald-700">Configuration updated</h3>
                <Detail label="Status" value={ackResult.status.toUpperCase()} />
                {ackResult.acknowledgedAt && (
                  <Detail
                    label="Acknowledged At"
                    value={new Date(ackResult.acknowledgedAt).toLocaleString()}
                  />
                )}
                {ackResult.acknowledgementPayload && (
                  <JsonPreview title="Payload" data={ackResult.acknowledgementPayload} />
                )}
              </div>
            )}
          </section>
        </>
      )}

      {showDriveSection && (
        <section
          aria-labelledby="device-config-rms-drive-heading"
          className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
        >
          <h2 id="device-config-rms-drive-heading" className="text-lg font-semibold">
            RMS &amp; Drive Config
          </h2>
          <p className="mt-2 text-sm text-slate-600">
            Paste CSV content with device UUID, VFD model, and optional overrides. Payload mirrors
            the backend import helper.
          </p>
          <p className="mt-2 text-sm text-slate-500">
            Looking for device enrollment or government credential CSV uploads? Visit the{' '}
            <a
              href="/devices/import"
              className="font-medium text-emerald-600 underline-offset-2 hover:underline"
            >
              Import CSVs
            </a>{' '}
            page for those bulk actions.
          </p>
          <form className="mt-4 grid gap-4 md:grid-cols-2" onSubmit={handleCsvSubmit}>
            <SelectField
              label="Transport"
              name="transport"
              value={csvForm.transport}
              onChange={handleCsvChange}
              options={[
                { value: 'mqtt', label: 'MQTT' },
                { value: 'https', label: 'HTTPS' },
              ]}
            />
            <InputField
              label="Issued By"
              name="issuedBy"
              value={csvForm.issuedBy}
              onChange={handleCsvChange}
              placeholder="Operator ID"
            />
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-2">
              <span>CSV Payload</span>
              <textarea
                className="h-40 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                name="csv"
                value={csvForm.csv}
                onChange={handleCsvChange}
                placeholder="uuid,vfdModelId,transport"
              />
            </label>
            <div className="flex items-center gap-3 md:col-span-2">
              <button
                type="submit"
                className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                disabled={csvMutation.isPending}
              >
                {csvMutation.isPending ? 'Importing…' : 'Import CSV'}
              </button>
              <button
                type="button"
                className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={() => {
                  setCsvForm(defaultCsvForm);
                  setCsvResult(null);
                  setCsvError(null);
                }}
              >
                Reset
              </button>
            </div>
          </form>
          {csvError && (
            <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              {csvError}
            </div>
          )}
          {csvResult && (
            <div className="mt-6 space-y-3">
              <Detail label="Processed" value={csvResult.processed.toString()} />
              <Detail label="Queued" value={csvResult.queued.toString()} />
              {csvResult.errors.length > 0 && (
                <div className="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                  <p className="font-semibold">Errors</p>
                  <ul className="mt-2 space-y-1">
                    {csvResult.errors.map((error) => (
                      <li key={error.row}>
                        Row {error.row}: {error.message}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </section>
      )}

      {bulkRotationState && (
        <BulkProgressOverlay
          state={bulkRotationState}
          onCancel={handleCancelBulkRotation}
          onClose={handleCloseBulkRotationModal}
          onDownload={handleDownloadBulkCredentialZip}
        />
      )}
    </div>
  );
}

export function DeviceInternalCredentialsPage() {
  return <DeviceConfigurationPage view="internal" />;
}

export function DeviceGovernmentCredentialsPage() {
  return <DeviceConfigurationPage view="government" />;
}

export function DeviceDriveConfigPage() {
  return <DeviceConfigurationPage view="drive" />;
}

type BulkDeviceOperationsSectionProps = {
  deviceSearch: string;
  onSearchChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onRefresh: () => void;
  isLoading: boolean;
  isFetching: boolean;
  error: Error | null;
  devices: DeviceListItem[];
  filteredDevices: DeviceListItem[];
  totalSelected: number;
  displayedSelectedCount: number;
  allDisplayedSelected: boolean;
  selectedDeviceUuids: Set<string>;
  onToggleDevice: (uuid: string) => void;
  onToggleAllDisplayed: () => void;
  onClearSelection: () => void;
  onBulkRegenerate: () => void;
  bulkActionError: string | null;
  canManageDeviceCredentials: boolean;
  isBulkOperationRunning: boolean;
  headerCheckboxRef: RefObject<HTMLInputElement>;
};

function BulkDeviceOperationsSection({
  deviceSearch,
  onSearchChange,
  onRefresh,
  isLoading,
  isFetching,
  error,
  devices,
  filteredDevices,
  totalSelected,
  displayedSelectedCount,
  allDisplayedSelected,
  selectedDeviceUuids,
  onToggleDevice,
  onToggleAllDisplayed,
  onClearSelection,
  onBulkRegenerate,
  bulkActionError,
  canManageDeviceCredentials,
  isBulkOperationRunning,
  headerCheckboxRef,
}: BulkDeviceOperationsSectionProps) {
  const totalDevices = devices.length;
  const visibleDevices = filteredDevices;
  const hasSelection = totalSelected > 0;
  const selectionSummary = hasSelection
    ? visibleDevices.length > 0
      ? displayedSelectedCount === visibleDevices.length
        ? `${totalSelected} selected`
        : `${totalSelected} selected (${displayedSelectedCount}/${visibleDevices.length} visible)`
      : `${totalSelected} selected (hidden by filters)`
    : null;
  const showGuard = hasSelection && !canManageDeviceCredentials;
  const headerDisabled = isBulkOperationRunning || visibleDevices.length === 0;
  const refreshLabel = isFetching ? 'Refreshing…' : 'Refresh';

  return (
    <section
      aria-labelledby="device-config-bulk-operations-heading"
      className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"
    >
      <div className="flex flex-col gap-2">
        <div>
          <h2 id="device-config-bulk-operations-heading" className="text-lg font-semibold">
            Bulk Device Operations
          </h2>
          <p className="mt-1 text-sm text-slate-600">
            Select multiple devices to rotate credentials or prepare command queues without
            repeating manual steps.
          </p>
        </div>
        <div className="mt-2 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center">
            <input
              type="search"
              value={deviceSearch}
              onChange={onSearchChange}
              className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500 sm:max-w-xs"
              placeholder="Search IMEI, UUID, or protocol"
              disabled={isBulkOperationRunning}
              aria-label="Filter devices"
            />
            <button
              type="button"
              onClick={onRefresh}
              className="inline-flex h-10 items-center justify-center rounded-md border border-slate-300 px-3 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={isFetching}
            >
              {refreshLabel}
            </button>
          </div>
          <div className="text-xs text-slate-500">
            {isFetching && !isLoading
              ? 'Syncing latest device list…'
              : `Showing ${visibleDevices.length} of ${totalDevices} devices`}
          </div>
        </div>
        {bulkActionError && (
          <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {bulkActionError}
          </div>
        )}
        {hasSelection && (
          <div className="flex flex-wrap items-center gap-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            <span className="font-semibold">{selectionSummary}</span>
            <button
              type="button"
              onClick={onBulkRegenerate}
              className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canManageDeviceCredentials || isBulkOperationRunning}
            >
              {isBulkOperationRunning ? 'Running…' : 'Regenerate credentials'}
            </button>
            <button
              type="button"
              onClick={onClearSelection}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              disabled={isBulkOperationRunning}
            >
              Clear selection
            </button>
            {showGuard && (
              <span className="text-xs text-amber-700">{DEVICE_CREDENTIAL_GUARD_MESSAGE}</span>
            )}
          </div>
        )}
        {isLoading ? (
          <div className="rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600">
            Loading devices…
          </div>
        ) : error ? (
          <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {error.message || 'Unable to load device list'}
          </div>
        ) : visibleDevices.length === 0 ? (
          <div className="rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600">
            {totalDevices === 0
              ? 'No devices enrolled yet. Use the enrollment workflow to add your first device.'
              : 'No devices match the current filters.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50">
                <tr>
                  <th className="px-3 py-2 text-left">
                    <input
                      ref={headerCheckboxRef}
                      type="checkbox"
                      className="size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                      checked={allDisplayedSelected}
                      onChange={onToggleAllDisplayed}
                      disabled={headerDisabled}
                      aria-label="Select all visible devices"
                    />
                  </th>
                  <th className="px-3 py-2 text-left font-semibold text-slate-600">IMEI</th>
                  <th className="px-3 py-2 text-left font-semibold text-slate-600">UUID</th>
                  <th className="px-3 py-2 text-left font-semibold text-slate-600">Lifecycle</th>
                  <th className="px-3 py-2 text-left font-semibold text-slate-600">Connectivity</th>
                  <th className="px-3 py-2 text-left font-semibold text-slate-600">Protocol</th>
                  <th className="px-3 py-2 text-left font-semibold text-slate-600">
                    Last Telemetry
                  </th>
                  <th className="px-3 py-2 text-left font-semibold text-slate-600">
                    Last Heartbeat
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200">
                {visibleDevices.map((device) => {
                  const isSelected = selectedDeviceUuids.has(device.uuid);
                  return (
                    <tr
                      key={device.uuid}
                      className={`transition-colors ${isSelected ? 'bg-emerald-50' : 'hover:bg-slate-50'}`}
                    >
                      <td className="px-3 py-2">
                        <input
                          type="checkbox"
                          className="size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                          checked={isSelected}
                          onChange={() => onToggleDevice(device.uuid)}
                          disabled={isBulkOperationRunning}
                          aria-label={`Select device ${device.imei ?? device.uuid}`}
                        />
                      </td>
                      <td className="px-3 py-2 text-slate-800">{device.imei ?? '—'}</td>
                      <td className="px-3 py-2 font-mono text-xs text-slate-600">{device.uuid}</td>
                      <td className="px-3 py-2 text-slate-600">
                        {formatDeviceStatus(device.status)}
                      </td>
                      <td className="px-3 py-2 text-slate-600">
                        <div className="flex flex-col gap-1">
                          <StatusBadge status={device.connectivityStatus} />
                          <span className="text-xs text-slate-500">
                            {formatOptionalTimestamp(device.connectivityUpdatedAt)}
                          </span>
                        </div>
                      </td>
                      <td className="px-3 py-2 text-slate-600">
                        {device.protocolVersion ? (
                          <div className="flex flex-col">
                            <span>{device.protocolVersion.version}</span>
                            {device.protocolVersion.name ? (
                              <span className="text-xs text-slate-500">
                                {device.protocolVersion.name}
                              </span>
                            ) : null}
                          </div>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td className="px-3 py-2 text-slate-600">
                        {formatOptionalTimestamp(device.lastTelemetryAt)}
                      </td>
                      <td className="px-3 py-2 text-slate-600">
                        {formatOptionalTimestamp(device.lastHeartbeatAt)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
        {isFetching && !isLoading && (
          <p className="text-xs text-slate-500">Refreshing device data…</p>
        )}
      </div>
    </section>
  );
}

type BulkProgressOverlayProps = {
  state: BulkRotationState;
  onCancel: () => void;
  onClose: () => void;
  onDownload: () => void;
};

function BulkProgressOverlay({ state, onCancel, onClose, onDownload }: BulkProgressOverlayProps) {
  const entries = state.devices.map((device) => ({
    device,
    result: state.results[device.uuid],
  }));
  const total = entries.length;
  const completedCount = entries.reduce((count, entry) => {
    if (entry.result && (entry.result.status === 'success' || entry.result.status === 'error')) {
      return count + 1;
    }
    return count;
  }, 0);
  const successCount = entries.reduce(
    (count, entry) => (entry.result?.status === 'success' ? count + 1 : count),
    0,
  );
  const failureCount = entries.reduce(
    (count, entry) => (entry.result?.status === 'error' ? count + 1 : count),
    0,
  );
  const percentComplete = total === 0 ? 0 : Math.round((completedCount / total) * 100);
  const canDownload =
    state.status === 'completed' &&
    entries.some((entry) => entry.result?.status === 'success' && entry.result.credentials);
  const cancellationMessage =
    state.cancelled && state.status === 'running'
      ? 'Cancellation requested. Finishing in-flight requests…'
      : state.cancelled && state.status === 'completed'
        ? 'Operation cancelled before all devices were processed.'
        : null;

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/60 px-4 py-6">
      <div className="w-full max-w-3xl rounded-lg bg-white p-6 shadow-2xl">
        <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
          <div>
            <h3 className="text-lg font-semibold text-slate-800">Bulk credential regeneration</h3>
            <p className="text-sm text-slate-600">
              {state.status === 'running'
                ? `Processing ${total} device${total === 1 ? '' : 's'}…`
                : `Processed ${total} device${total === 1 ? '' : 's'}.`}
            </p>
            {cancellationMessage && <p className="text-xs text-amber-600">{cancellationMessage}</p>}
          </div>
        </div>
        <div className="mt-4">
          <div className="flex items-center justify-between text-sm text-slate-600">
            <span>
              {completedCount} / {total} completed
            </span>
            <span>{percentComplete}%</span>
          </div>
          <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-slate-100">
            <div
              className="h-full rounded-full bg-emerald-500 transition-all"
              style={{ width: `${percentComplete}%` }}
            />
          </div>
        </div>
        <div className="mt-5 max-h-72 overflow-y-auto rounded-md border border-slate-200">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50">
              <tr>
                <th className="px-3 py-2 text-left font-semibold text-slate-600">Device</th>
                <th className="px-3 py-2 text-left font-semibold text-slate-600">Status</th>
                <th className="px-3 py-2 text-left font-semibold text-slate-600">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {entries.map(({ device, result }) => {
                const status = result?.status ?? 'pending';
                const statusLabel = formatBulkStatusLabel(status);
                const statusClassName =
                  status === 'success'
                    ? 'text-emerald-600'
                    : status === 'error'
                      ? 'text-red-600'
                      : 'text-slate-600';
                const detail = (() => {
                  if (!result) {
                    return 'Queued';
                  }
                  if (result.status === 'error') {
                    return result.error ?? 'Failed';
                  }
                  if (result.status === 'success') {
                    return result.credentials ? 'Credentials captured' : 'Awaiting broker sync';
                  }
                  if (result.status === 'in_progress') {
                    return 'Rotating credentials…';
                  }
                  return 'Queued';
                })();

                return (
                  <tr key={device.uuid} className="align-top">
                    <td className="px-3 py-2 text-slate-800">
                      <div className="font-medium">{device.imei ?? 'Unknown IMEI'}</div>
                      <div className="font-mono text-xs text-slate-500">{device.uuid}</div>
                    </td>
                    <td className={`px-3 py-2 text-sm font-semibold ${statusClassName}`}>
                      {statusLabel}
                    </td>
                    <td className="px-3 py-2 text-sm text-slate-600">{detail}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="mt-6 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="text-sm text-slate-600">
            <span className="font-semibold text-emerald-600">{successCount}</span> success
            <span className="ml-3 font-semibold text-red-600">{failureCount}</span> failed
            {state.cancelled && state.status === 'completed' && (
              <span className="ml-3 text-amber-600">Cancelled</span>
            )}
          </div>
          {state.status === 'running' ? (
            <div className="flex gap-2">
              <button
                type="button"
                onClick={onCancel}
                className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={state.cancelled}
              >
                Cancel
              </button>
            </div>
          ) : (
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={onDownload}
                className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!canDownload}
              >
                Download credentials ZIP
              </button>
              <button
                type="button"
                onClick={onClose}
                className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              >
                Close
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function formatDeviceStatus(status: string | null | undefined) {
  if (!status) {
    return '—';
  }

  const normalized = status.replace(/_/g, ' ').trim();
  if (!normalized) {
    return '—';
  }

  return normalized.replace(/\b\w/g, (char) => char.toUpperCase());
}

function formatBulkStatusLabel(status: BulkCredentialStatus) {
  switch (status) {
    case 'success':
      return 'Success';
    case 'error':
      return 'Failed';
    case 'in_progress':
      return 'In progress';
    default:
      return 'Pending';
  }
}

function buildBulkCredentialPayload(
  deviceUuid: string,
  imei: string | null,
  bundle: DeviceCredentialBundle,
  generatedAt: string,
) {
  return {
    deviceUuid,
    imei,
    clientId: bundle.clientId,
    username: bundle.username,
    password: bundle.password,
    endpoints: bundle.endpoints,
    topics: bundle.topics,
    mqttAccessApplied: bundle.mqttAccess?.applied ?? null,
    generatedAt,
  };
}

type InputFieldProps = {
  label: string;
  name: string;
  value: string;
  onChange: (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => void;
  required?: boolean;
  placeholder?: string;
  ariaLabel?: string;
};

function InputField({
  label,
  name,
  value,
  onChange,
  required,
  placeholder,
  ariaLabel,
}: InputFieldProps) {
  return (
    <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
      <span>{label}</span>
      <input
        className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
        name={name}
        value={value}
        onChange={onChange}
        required={required}
        placeholder={placeholder}
        aria-label={ariaLabel}
      />
    </label>
  );
}

type SelectFieldProps = {
  label: string;
  name: string;
  value: string;
  onChange: (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => void;
  options: Array<{ value: string; label: string }>;
};

function SelectField({ label, name, value, onChange, options }: SelectFieldProps) {
  return (
    <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
      <span>{label}</span>
      <select
        className="rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
        name={name}
        value={value}
        onChange={onChange}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

type DetailProps = {
  label: string;
  value: ReactNode;
  title?: string;
  monospace?: boolean;
};

function Detail({ label, value, title, monospace = false }: DetailProps) {
  return (
    <div className="rounded border border-slate-200 p-3">
      <p className="text-xs uppercase tracking-wide text-slate-500">{label}</p>
      <p className={`mt-1 text-sm text-slate-800${monospace ? ' font-mono' : ''}`} title={title}>
        {value}
      </p>
    </div>
  );
}

type JsonPreviewProps = {
  title: string;
  data: unknown;
};

function JsonPreview({ title, data }: JsonPreviewProps) {
  return (
    <div>
      <h4 className="text-sm font-semibold text-slate-700">{title}</h4>
      <pre className="mt-2 max-h-60 overflow-auto rounded border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700">
        {JSON.stringify(data, null, 2)}
      </pre>
    </div>
  );
}

type CommandDictionaryTableProps = {
  commands: CommandDefinition[];
};

function CommandDictionaryTable({ commands }: CommandDictionaryTableProps) {
  if (!commands.length) {
    return (
      <div>
        <h4 className="text-sm font-semibold text-slate-700">Command Dictionary</h4>
        <p className="mt-2 text-sm text-slate-600">
          No command entries defined yet. Users with the catalog:rs485 capability can import command
          dictionaries to enrich this model.
        </p>
      </div>
    );
  }

  return (
    <div>
      <h4 className="text-sm font-semibold text-slate-700">Command Dictionary</h4>
      <div className="mt-2 overflow-x-auto">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-600">
            <tr>
              <th className="px-3 py-2">Command</th>
              <th className="px-3 py-2">Address</th>
              <th className="px-3 py-2">Function</th>
              <th className="px-3 py-2">Description</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200 bg-white">
            {commands.map((command, index) => {
              const metadataKeys = command.metadata ? Object.keys(command.metadata) : [];
              return (
                <tr key={`${command.commandName}-${index}`}>
                  <td className="px-3 py-2 font-medium text-slate-800">{command.commandName}</td>
                  <td className="px-3 py-2 text-slate-700">{command.address}</td>
                  <td className="px-3 py-2 text-slate-700">
                    {command.functionCode !== undefined && command.functionCode !== null
                      ? String(command.functionCode)
                      : '—'}
                  </td>
                  <td className="px-3 py-2 text-slate-700">
                    <div>{command.description ?? '—'}</div>
                    {metadataKeys.length > 0 && (
                      <div className="mt-1 text-xs text-slate-500">
                        Metadata keys: {metadataKeys.join(', ')}
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

type VfdModelSummaryProps = {
  model: VfdModel;
};

function VfdModelSummary({ model }: VfdModelSummaryProps) {
  const metadata = model.rs485.metadata;
  const hasRs485Metadata = Boolean(metadata && Object.keys(metadata).length > 0);

  return (
    <section className="rounded-md border border-slate-200 bg-slate-50 p-4">
      <h3 className="text-sm font-semibold text-slate-700">Selected VFD Model</h3>
      <div className="mt-3 grid gap-3 md:grid-cols-3">
        <Detail label="Manufacturer" value={model.manufacturer} />
        <Detail label="Model" value={model.model} />
        <Detail label="Version" value={model.version} />
      </div>
      <div className="mt-3 grid gap-3 md:grid-cols-3">
        <Detail label="Baud Rate" value={String(model.rs485.baudRate)} />
        <Detail label="Data Bits" value={String(model.rs485.dataBits)} />
        <Detail label="Stop Bits" value={String(model.rs485.stopBits)} />
      </div>
      <div className="mt-3 grid gap-3 md:grid-cols-3">
        <Detail label="Parity" value={model.rs485.parity} />
        <Detail label="Flow Control" value={model.rs485.flowControl} />
        <Detail label="Command Count" value={String(model.commandDictionary.length)} />
      </div>
      <div className="mt-3 grid gap-3 md:grid-cols-3">
        <Detail label="Realtime Parameters" value={String(model.realtimeParameters.length)} />
        <Detail label="Fault Codes" value={String(model.faultMap.length)} />
        <Detail label="Active Assignments" value={String(model.assignments.length)} />
      </div>
      {hasRs485Metadata && (
        <div className="mt-4">
          <JsonPreview title="RS485 Metadata" data={metadata} />
        </div>
      )}
      <div className="mt-4">
        <CommandDictionaryTable commands={model.commandDictionary} />
      </div>
    </section>
  );
}

type ProvisioningStatusPanelProps = {
  provisioning: MqttProvisioningInfo;
  onRetry: () => void;
  retryPending: boolean;
  retryError: string | null;
  retryMessage: string | null;
};

function formatProvisioningStatusLabel(status: MqttProvisioningInfo['status']) {
  switch (status) {
    case 'in_progress':
      return 'In Progress';
    case 'applied':
      return 'Applied';
    case 'failed':
      return 'Failed';
    default:
      return 'Pending';
  }
}

function ProvisioningStatusPanel({
  provisioning,
  onRetry,
  retryPending,
  retryError,
  retryMessage,
}: ProvisioningStatusPanelProps) {
  const statusLabel = formatProvisioningStatusLabel(provisioning.status);
  const retryDisabled = retryPending || provisioning.status === 'in_progress';
  const nextAttemptDate = useMemo(
    () => (provisioning.nextAttemptAt ? new Date(provisioning.nextAttemptAt) : null),
    [provisioning.nextAttemptAt],
  );
  const now = useNow(30_000);
  const isStalled = useMemo(() => {
    if (provisioning.status === 'applied' || provisioning.status === 'failed') {
      return false;
    }

    if (!nextAttemptDate) {
      return true;
    }

    return now - nextAttemptDate.getTime() > PROVISIONING_STALLED_THRESHOLD_MS;
  }, [nextAttemptDate, now, provisioning.status]);

  return (
    <section className="mt-4 space-y-4 rounded-md border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-xs uppercase tracking-wide text-slate-500">MQTT Provisioning</p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <StatusBadge status={provisioning.status} label={statusLabel} />
            <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              Job {provisioning.jobId}
            </span>
          </div>
        </div>
        <button
          type="button"
          className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-sm font-semibold text-emerald-700 shadow-sm transition hover:bg-emerald-100 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
          onClick={onRetry}
          disabled={retryDisabled}
        >
          {retryPending ? 'Retrying…' : 'Retry Provisioning'}
        </button>
      </div>
      <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
        <Detail
          label="Attempts"
          value={`${provisioning.attemptCount} / ${provisioning.maxAttempts}`}
        />
        <Detail
          label="Base Retry Delay"
          value={formatRetryDelayMs(provisioning.baseRetryDelayMs)}
        />
        <Detail
          label="Last Attempt"
          value={formatTimestampWithRelativeLabel(provisioning.lastAttemptAt)}
        />
        <Detail
          label="Next Attempt"
          value={formatTimestampWithRelativeLabel(provisioning.nextAttemptAt)}
        />
      </div>
      {isStalled && (
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900">
          Provisioning appears stalled. Review broker logs or trigger a retry.
        </div>
      )}
      {provisioning.lastError && (
        <div className="space-y-1 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
          <p className="font-semibold">Last Error</p>
          <p>{provisioning.lastError.message}</p>
          <ul className="mt-1 space-y-1 text-xs">
            {provisioning.lastError.status ? (
              <li>Status: {provisioning.lastError.status}</li>
            ) : null}
            {provisioning.lastError.endpoint ? (
              <li>Endpoint: {provisioning.lastError.endpoint}</li>
            ) : null}
          </ul>
        </div>
      )}
      {retryMessage && (
        <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800">
          {retryMessage}
        </div>
      )}
      {retryError && (
        <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {retryError}
        </div>
      )}
    </section>
  );
}

type GovernmentCredentialDefaults = {
  endpoints: Array<{ protocol: 'mqtt' | 'mqtts'; host: string; port: number; url: string }>;
  topics: {
    publish: string[];
    subscribe: string[];
  };
};

type MockDashboardBundleExport = {
  version: string;
  exportedAt: string;
  credentialType: ActiveCredentialSummary['type'];
  device: {
    uuid: string;
    imei: string | null;
    protocolVersionId: string | null;
    protocolVersion: string | null;
  };
  bundle: {
    type: ActiveCredentialSummary['type'];
    clientId: string;
    username: string;
    password: string;
    issuedBy: string | null;
    lifecycle: CredentialLifecycleState | null;
    validFrom: string;
    topics: {
      publish: string[];
      subscribe: string[];
    };
    endpoints: DeviceCredentialBundle['endpoints'];
  };
  connection: {
    brokerHost: string | null;
    brokerPort: number | null;
    protocol: DeviceCredentialBundle['endpoints'][number]['protocol'] | null;
    websocketUrl: string | null;
    username: string;
    password: string;
    clientId: string;
    topicPrefix: string | null;
    publishTopics: string[];
    subscribeTopics: string[];
  };
};

function extractGovernmentCredentialDefaults(
  metadata: Record<string, unknown> | null | undefined,
): GovernmentCredentialDefaults | null {
  if (!metadata || typeof metadata !== 'object') {
    return null;
  }

  const section = (metadata as Record<string, unknown>).governmentCredentials;
  if (!section || typeof section !== 'object') {
    return null;
  }

  const record = section as Record<string, unknown>;
  const endpointsRaw = record.endpointDefaults;
  const endpoints: GovernmentCredentialDefaults['endpoints'] = Array.isArray(endpointsRaw)
    ? (endpointsRaw as unknown[])
        .map(normalizeGovernmentCredentialEndpoint)
        .filter((endpoint): endpoint is GovernmentCredentialDefaults['endpoints'][number] =>
          Boolean(endpoint),
        )
    : [];

  const topicsRaw = record.topics;
  const topics: GovernmentCredentialDefaults['topics'] = {
    publish: [],
    subscribe: [],
  };

  if (topicsRaw && typeof topicsRaw === 'object') {
    const topicsRecord = topicsRaw as Record<string, unknown>;
    const publishRaw = topicsRecord.publish;
    const subscribeRaw = topicsRecord.subscribe;

    if (Array.isArray(publishRaw)) {
      topics.publish = publishRaw
        .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
        .map((value) => value.trim());
    }

    if (Array.isArray(subscribeRaw)) {
      topics.subscribe = subscribeRaw
        .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
        .map((value) => value.trim());
    }
  }

  if (!endpoints.length && topics.publish.length === 0 && topics.subscribe.length === 0) {
    return null;
  }

  return {
    endpoints,
    topics,
  };
}

function selectPreferredEndpoint(
  endpoints: DeviceCredentialBundle['endpoints'],
): DeviceCredentialBundle['endpoints'][number] | null {
  if (!endpoints.length) {
    return null;
  }

  const websocketEndpoint = endpoints.find((endpoint) => {
    const url = endpoint.url?.toLowerCase();
    return Boolean(url && (url.startsWith('ws://') || url.startsWith('wss://')));
  });

  if (websocketEndpoint) {
    return websocketEndpoint;
  }

  const secureEndpoint = endpoints.find((endpoint) => endpoint.protocol === 'mqtts');
  if (secureEndpoint) {
    return secureEndpoint;
  }

  return endpoints[0];
}

function deriveWebsocketUrl(
  endpoint: DeviceCredentialBundle['endpoints'][number] | null,
): string | null {
  if (!endpoint) {
    return null;
  }

  const sourceUrl = endpoint.url ?? '';
  const normalized = sourceUrl.toLowerCase();

  if (normalized.startsWith('wss://') || normalized.startsWith('ws://')) {
    return sourceUrl;
  }

  if (normalized.startsWith('mqtts://')) {
    return `wss://${sourceUrl.slice('mqtts://'.length)}`;
  }

  if (normalized.startsWith('mqtt://')) {
    return `ws://${sourceUrl.slice('mqtt://'.length)}`;
  }

  if (normalized.startsWith('ssl://')) {
    return `wss://${sourceUrl.slice('ssl://'.length)}`;
  }

  if (normalized.startsWith('tcp://')) {
    return `ws://${sourceUrl.slice('tcp://'.length)}`;
  }

  if (!endpoint.host) {
    return null;
  }

  const scheme = endpoint.protocol === 'mqtts' ? 'wss' : 'ws';
  const portSegment = endpoint.port ? `:${endpoint.port}` : '';
  return `${scheme}://${endpoint.host}${portSegment}/mqtt`;
}

function deriveTopicPrefix(
  device: DeviceStatusResponse['device'] | null | undefined,
  topics: { publish: string[]; subscribe: string[] },
): string | null {
  if (device?.imei) {
    return device.imei;
  }

  const topicCandidates = [...topics.publish, ...topics.subscribe];
  for (const candidate of topicCandidates) {
    if (typeof candidate !== 'string') {
      continue;
    }
    const parts = candidate.trim().split('/');
    if (parts[0]) {
      return parts[0];
    }
  }

  return null;
}

function buildMockDashboardBundle(
  device: DeviceStatusResponse['device'] | null,
  credential: ActiveCredentialSummary | null,
): MockDashboardBundleExport | null {
  if (!device || !credential) {
    return null;
  }

  const preferredEndpoint = selectPreferredEndpoint(credential.endpoints);
  const websocketUrl = deriveWebsocketUrl(preferredEndpoint);
  const topicPrefix = deriveTopicPrefix(device, credential.topics);

  const endpoints = credential.endpoints.map((endpoint) => ({ ...endpoint }));

  return {
    version: 'rms.dashboard.bundle/v1',
    exportedAt: new Date().toISOString(),
    credentialType: credential.type,
    device: {
      uuid: device.uuid,
      imei: device.imei ?? null,
      protocolVersionId: device.protocolVersion?.id ?? null,
      protocolVersion: device.protocolVersion?.version ?? null,
    },
    bundle: {
      type: credential.type,
      clientId: credential.clientId,
      username: credential.username,
      password: credential.password,
      issuedBy: credential.issuedBy ?? null,
      lifecycle: credential.lifecycle ?? null,
      validFrom: credential.validFrom,
      topics: {
        publish: [...credential.topics.publish],
        subscribe: [...credential.topics.subscribe],
      },
      endpoints,
    },
    connection: {
      brokerHost: preferredEndpoint?.host ?? null,
      brokerPort: preferredEndpoint?.port ?? null,
      protocol: preferredEndpoint?.protocol ?? null,
      websocketUrl,
      username: credential.username,
      password: credential.password,
      clientId: credential.clientId,
      topicPrefix,
      publishTopics: [...credential.topics.publish],
      subscribeTopics: [...credential.topics.subscribe],
    },
  } satisfies MockDashboardBundleExport;
}

function normalizeGovernmentCredentialEndpoint(entry: unknown) {
  if (!entry || typeof entry !== 'object') {
    return null;
  }

  const record = entry as Record<string, unknown>;
  const protocolValue = record.protocol;
  const protocol = protocolValue === 'mqtt' || protocolValue === 'mqtts' ? protocolValue : null;
  if (!protocol) {
    return null;
  }

  const hostValue = record.host;
  const host = typeof hostValue === 'string' ? hostValue.trim() : '';
  if (!host) {
    return null;
  }

  const portValue = record.port;
  const port =
    typeof portValue === 'number'
      ? portValue
      : typeof portValue === 'string'
        ? Number.parseInt(portValue, 10)
        : Number.NaN;

  if (!Number.isFinite(port) || port <= 0) {
    return null;
  }

  const urlValue = record.url;
  const url =
    typeof urlValue === 'string' && urlValue.trim().length > 0
      ? urlValue.trim()
      : `${protocol}://${host}:${port}`;

  return {
    protocol,
    host,
    port,
    url,
  } satisfies GovernmentCredentialDefaults['endpoints'][number];
}

function parseTopicsInput(rawValue: string): string[] {
  if (!rawValue.trim()) {
    return [];
  }

  return rawValue
    .split(/[\n,]/)
    .map((value) => value.trim())
    .filter((value) => value.length > 0);
}

type GovernmentCredentialSummaryProps = {
  government: DeviceStatusResponse['activeCredentials']['government'] | null;
  defaults: GovernmentCredentialDefaults | null;
};

function GovernmentCredentialSummary({ government, defaults }: GovernmentCredentialSummaryProps) {
  if (!government && !defaults) {
    return null;
  }

  return (
    <div className="mt-4 space-y-4">
      <h4 className="text-sm font-semibold text-slate-700">Government Credentials</h4>
      {government ? (
        <div className="space-y-3 rounded-md border border-slate-200 bg-white p-4 shadow-sm">
          <p className="text-xs uppercase tracking-wide text-slate-500">Active Bundle</p>
          <div className="grid gap-3 md:grid-cols-2">
            <Detail label="Client ID" value={government.clientId} />
            <Detail label="Username" value={government.username} />
            <Detail label="Password" value={government.password} />
            <Detail label="Issued By" value={government.issuedBy ?? '—'} />
            <Detail label="Lifecycle" value={formatLifecycleLabel(government.lifecycle)} />
            <Detail label="Valid From" value={formatOptionalTimestamp(government.validFrom)} />
            <Detail
              label="Import Job"
              value={
                government.originImportJobId ? (
                  <ImportJobLink jobId={government.originImportJobId} showLabel />
                ) : (
                  '—'
                )
              }
            />
            <Detail
              label="Protocol Selector"
              value={formatProtocolSelectorLabel(government.protocolSelector)}
              title={formatProtocolSelectorTitle(government.protocolSelector)}
            />
          </div>
          {government.endpoints.length > 0 && (
            <JsonPreview title="Active Endpoints" data={government.endpoints} />
          )}
          <JsonPreview
            title="Active Topics"
            data={{ publish: government.topics.publish, subscribe: government.topics.subscribe }}
          />
        </div>
      ) : (
        <p className="rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600">
          No active government credential bundle recorded for this device.
        </p>
      )}
      {defaults ? (
        <div className="space-y-3 rounded-md border border-slate-200 bg-white p-4 shadow-sm">
          <p className="text-xs uppercase tracking-wide text-slate-500">Protocol Defaults</p>
          {defaults.endpoints.length > 0 ? (
            <JsonPreview title="Default Endpoints" data={defaults.endpoints} />
          ) : (
            <p className="text-xs text-slate-600">No endpoint defaults provided.</p>
          )}
          {defaults.topics.publish.length > 0 || defaults.topics.subscribe.length > 0 ? (
            <JsonPreview title="Default Topics" data={defaults.topics} />
          ) : (
            <p className="text-xs text-slate-600">No topic defaults provided.</p>
          )}
        </div>
      ) : (
        <p className="rounded-md border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600">
          Protocol metadata does not define government credential defaults.
        </p>
      )}
    </div>
  );
}

type MockDashboardBundlePanelProps = {
  title: string;
  description: string;
  bundle: MockDashboardBundleExport;
  onCopy: () => void;
  successMessage: string | null;
  errorMessage: string | null;
};

function MockDashboardBundlePanel({
  title,
  description,
  bundle,
  onCopy,
  successMessage,
  errorMessage,
}: MockDashboardBundlePanelProps) {
  return (
    <div className="mt-4 space-y-3 rounded-md border border-slate-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-slate-700">{title}</p>
          <p className="text-xs text-slate-500">{description}</p>
        </div>
        <button
          type="button"
          className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100"
          onClick={onCopy}
        >
          Copy Bundle JSON
        </button>
      </div>
      <JsonPreview title="Bundle Payload" data={bundle} />
      {successMessage && (
        <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-800">
          {successMessage}
        </div>
      )}
      {errorMessage && (
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800">
          {errorMessage}
        </div>
      )}
    </div>
  );
}

function formatLifecycleText(state: string | null | undefined) {
  if (!state) {
    return 'Unknown';
  }

  switch (state.toLowerCase()) {
    case 'pending':
      return 'Pending';
    case 'active':
      return 'Active';
    case 'revoked':
      return 'Revoked';
    case 'expired':
      return 'Expired';
    default:
      return state;
  }
}

function formatLifecycleLabel(state: CredentialLifecycleState | null | undefined) {
  return formatLifecycleText(state ?? null);
}

function formatLifecycleHistoryTooltip(history: CredentialLifecycleHistoryEntry[]) {
  if (!history.length) {
    return 'No lifecycle history recorded';
  }

  return history
    .map((entry) => {
      const label = formatLifecycleText(entry.state);
      const timestamp = formatOptionalTimestamp(entry.occurredAt);
      const reason = entry.reason ? ` — ${entry.reason}` : '';
      const actor = entry.actorId ? ` by ${entry.actorId}` : '';
      return `${label} @ ${timestamp}${reason}${actor}`;
    })
    .join('\n');
}

function formatAccessStatus(applied: boolean | null) {
  if (applied === true) {
    return 'Applied';
  }

  if (applied === false) {
    return 'Not Applied';
  }

  return 'Unknown';
}

function formatProtocolSelectorLabel(selector: ProtocolSelectorSummary | null | undefined) {
  if (!selector) {
    return 'No selector linked';
  }

  return [selector.stateId, selector.stateAuthorityId, selector.projectId, selector.serverVendorId]
    .filter((value) => Boolean(value))
    .join(' • ');
}

function formatProtocolSelectorTitle(selector: ProtocolSelectorSummary | null | undefined) {
  if (!selector) {
    return 'No protocol selector available';
  }

  const parts = [
    `State: ${selector.stateId ?? '—'}`,
    `Authority: ${selector.stateAuthorityId ?? '—'}`,
    `Project: ${selector.projectId ?? '—'}`,
    `Server: ${selector.serverVendorId ?? '—'}`,
    `Protocol: ${selector.protocolVersionId ?? '—'}`,
    `Version: ${formatVersionLabel(selector.version)}`,
  ];

  return parts.join(' | ');
}

function formatOptionalTimestamp(value: string | null | undefined) {
  if (!value) {
    return '—';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }

  return date.toLocaleString();
}

function formatRetryDelayMs(value: number | null | undefined) {
  if (!value || value <= 0) {
    return '—';
  }

  if (value < 1000) {
    return `${value} ms`;
  }

  const seconds = value / 1000;
  if (seconds < 60) {
    return `${seconds.toFixed(seconds % 1 === 0 ? 0 : 1)} s`;
  }

  const minutes = seconds / 60;
  if (minutes < 60) {
    return `${minutes.toFixed(minutes % 1 === 0 ? 0 : 1)} min`;
  }

  const hours = minutes / 60;
  return `${hours.toFixed(hours % 1 === 0 ? 0 : 1)} hr`;
}

function formatDurationMs(value: number | null | undefined) {
  if (!value || value <= 0) {
    return '—';
  }

  const minutes = Math.round(value / 60000);
  if (minutes < 60) {
    return `${minutes} min${minutes === 1 ? '' : 's'}`;
  }

  const hours = value / 3_600_000;
  if (hours < 48) {
    return `${hours.toFixed(hours % 1 === 0 ? 0 : 1)} hr`;
  }

  const days = value / 86_400_000;
  return `${days.toFixed(days % 1 === 0 ? 0 : 1)} day${days >= 2 ? 's' : ''}`;
}

function formatTimestampWithRelativeLabel(value: string | null | undefined) {
  if (!value) {
    return '—';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }

  const base = date.toLocaleString();
  const diffMs = Date.now() - date.getTime();

  if (!Number.isFinite(diffMs)) {
    return base;
  }

  const absMs = Math.abs(diffMs);
  let relative: string;
  if (absMs < 60_000) {
    relative = `${Math.round(absMs / 1000)}s`;
  } else if (absMs < 3_600_000) {
    relative = `${Math.round(absMs / 60_000)}m`;
  } else if (absMs < 86_400_000) {
    relative = `${Math.round(absMs / 3_600_000)}h`;
  } else {
    relative = `${Math.round(absMs / 86_400_000)}d`;
  }

  const suffix = diffMs >= 0 ? `${relative} ago` : `in ${relative}`;
  return `${base} (${suffix})`;
}

function formatVersionLabel(version: string | null | undefined) {
  if (!version) {
    return 'vN/A';
  }

  return version.toLowerCase().startsWith('v') ? version : `v${version}`;
}

function useNow(intervalMs: number) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = window.setInterval(
      () => {
        setNow(Date.now());
      },
      Math.max(intervalMs, 1_000),
    );

    return () => {
      window.clearInterval(id);
    };
  }, [intervalMs]);

  return now;
}
