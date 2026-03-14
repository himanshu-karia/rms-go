import { FormEvent, useEffect, useMemo, useReducer, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';
import mqtt, { type MqttClient } from 'mqtt';
import { Link } from 'react-router-dom';

import type { LoginPayload } from '../api/auth';
import {
  fetchAuthorities,
  fetchProjects,
  fetchStates,
  type AuthorityOption,
  type ProjectOption,
  type ProtocolVersionOption,
  type StateOption,
} from '../api/lookups';
import {
  fetchAdminVendors,
  fetchAdminStates,
  createAdminState,
  fetchAdminStateAuthorities,
  createAdminStateAuthority,
  fetchAdminProjects,
  createAdminProject,
  fetchAdminProtocolVersions,
  createAdminProtocolVersion,
  createAdminVendor,
  type AdminVendor,
} from '../api/admin';
import {
  registerDevice,
  type DeviceCredentialBundle,
  type GovernmentCredentialBundle,
  type RegisterDevicePayload,
  type RegisterDeviceResponse,
} from '../api/devices';
import { type SessionSnapshot } from '../api/session';
import { useAuth } from '../auth';
import { StatusBadge } from '../components/StatusBadge';

type EnrollmentContext = {
  state?: StateOption;
  authority?: AuthorityOption;
  project?: ProjectOption;
  protocol?: ProtocolVersionOption;
  solarPumpVendor?: AdminVendor | null;
};

type EnrollmentOutcome = {
  device: RegisterDeviceResponse['device'];
  credentials: RegisterDeviceResponse['credentials'];
  governmentCredentials: GovernmentCredentialBundle | null;
  context: EnrollmentContext;
  asn: string;
};

type SimulatorLogEntry = {
  id: string;
  timestamp: string;
  direction: 'system' | 'outbound' | 'inbound';
  topic?: string;
  message: string;
  payload?: Record<string, unknown> | null;
};

type ReceivedCommand = {
  topic: string;
  msgid: string;
  cmd: string;
  payload: Record<string, unknown>;
  raw: Record<string, unknown>;
};

type StoredSimulatorState = {
  asn: string;
  websocketUrl: string;
  autoRespond: boolean;
  keepalive: number;
  device?: RegisterDeviceResponse['device'];
  credentials?: DeviceCredentialBundle | null;
  governmentCredentials?: GovernmentCredentialBundle | null;
  context?: EnrollmentContext;
};

type BootstrapSummary = {
  stateName: string;
  authorityName: string;
  projectName: string;
  protocolVersion: string;
  serverVendorName: string;
  solarVendorName: string;
  deviceImei?: string;
  clientId?: string;
  username?: string;
};

type BootstrapTraceEntry = {
  step: string;
  status: 'running' | 'done' | 'failed';
  detail?: string;
  at: string;
};

const SIMULATOR_STORAGE_KEY = 'pmkusum.simulator.state.v1';
const DEFAULT_WEBSOCKET_URL =
  import.meta.env?.VITE_MQTT_WS_URL ?? `wss://${window.location.host}/mqtt`;
const DEFAULT_BOOTSTRAP_CONFIG = {
  stateName: 'Maharashtra',
  stateIsoCode: 'MH',
  authorityName: 'MSEDCL',
  projectName: 'PM_KUSUM_SolarPump_RMS',
  serverVendorName: 'Local RMS Server',
  solarVendorName: 'Generic Solar Pump Vendor',
  protocolVersion: 'MSEDCL-v1',
  protocolName: 'MSEDCL Phase 1',
} as const;

const initialLoginForm: LoginPayload = {
  username: '',
  password: '',
};

const initialSelection = {
  stateId: '',
  authorityId: '',
  projectId: '',
  protocolVersionId: '',
  solarPumpVendorId: '',
};

type SelectionState = typeof initialSelection;

type GovernmentFormState = {
  enabled: boolean;
  protocol: 'mqtt' | 'mqtts';
  host: string;
  port: string;
  clientId: string;
  username: string;
  password: string;
};

const initialGovernmentForm: GovernmentFormState = {
  enabled: false,
  protocol: 'mqtts',
  host: '',
  port: '',
  clientId: '',
  username: '',
  password: '',
};

// Disable persistence to ensure every session uses fresh credentials and settings.
function loadStoredSimulatorState(): StoredSimulatorState | null {
  return null;
}

function persistSimulatorState(_state: StoredSimulatorState | null) {
  return;
}

type TopicMap = {
  heartbeat?: string;
  pump?: string;
  data?: string;
  daq?: string;
  ondemand?: string;
};

type PublishTopics = TopicMap & { [key: string]: string | undefined };

type SubscribeTopics = TopicMap & {
  all: string[];
  [key: string]: string | string[] | undefined;
};

type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error';

type PublishResult = { success: boolean; error?: Error | null };

const EMPTY_GOVERNMENT_TOPICS: GovernmentCredentialBundle['topics'] = {
  publish: [],
  subscribe: [],
};

type BootstrapResult = {
  stateOption: StateOption;
  authorityOption: AuthorityOption;
  projectOption: ProjectOption;
  protocolOption: ProtocolVersionOption;
  serverVendor: AdminVendor;
  solarVendor: AdminVendor;
  summary: BootstrapSummary;
};

function formatBootstrapError(error: unknown): string {
  const raw = error instanceof Error ? error.message : 'Failed to bootstrap simulator defaults.';
  const match = /^\[([^\]]+)\]\s*(.*)$/i.exec(raw.trim());
  if (!match) {
    return raw;
  }

  const step = match[1].trim().toLowerCase();
  const detail = match[2]?.trim() || 'Unknown error';

  const stepHintMap: Record<string, string> = {
    'states:list': 'Check /api/admin/states response and your hierarchy:manage capability.',
    'states:create': 'Verify state payload fields and backend state create validation.',
    'vendors:server:list': 'Check /api/admin/server-vendors permissions and response shape.',
    'vendors:server:create': 'Verify vendor create payload and catalog permission mapping.',
    'vendors:solar:list': 'Check /api/admin/solar-pump-vendors permissions and response shape.',
    'vendors:solar:create': 'Verify vendor create payload and catalog permission mapping.',
    'authorities:list': 'Check /api/admin/state-authorities filter params and response envelope.',
    'authorities:create': 'Validate state_id/stateId and authority envelope compatibility.',
    'projects:list': 'Check /api/admin/projects query compatibility and hierarchy links.',
    'projects:create': 'Verify authority linkage and project create payload fields.',
    'protocol-versions:list': 'Check protocol versions list query params and vendor linkage.',
    'protocol-versions:create': 'Verify protocol payload fields and server vendor id.',
    'lookups:refresh': 'Check lookup endpoints (/lookup/states, /lookup/authorities, /lookup/projects).',
  };

  const hint = stepHintMap[step] ?? 'Inspect network response for this step and compare payload keys.';
  return `Step ${step} failed: ${detail} Suggestion: ${hint}`;
}

async function ensureSimulatorBootstrap(
  queryClient: QueryClient,
  onStep?: (entry: BootstrapTraceEntry) => void,
): Promise<BootstrapResult> {
  const config = DEFAULT_BOOTSTRAP_CONFIG;
  const normalize = (value: string) => value.trim().toLowerCase();
  const withStep = async <T,>(step: string, action: () => Promise<T>): Promise<T> => {
    onStep?.({ step, status: 'running', at: new Date().toISOString() });
    try {
      const result = await action();
      onStep?.({ step, status: 'done', at: new Date().toISOString() });
      return result;
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      onStep?.({ step, status: 'failed', detail, at: new Date().toISOString() });
      throw new Error(`[${step}] ${detail}`);
    }
  };
  const metadata: Record<string, unknown> = {
    source: 'simulator-bootstrap',
    specRef: 'RMS_Server_Preprompt.txt#Device Provisioning & Credentials',
  };

  let adminStates = await withStep('states:list', () => fetchAdminStates());
  let state = adminStates.find((entity) => normalize(entity.name) === normalize(config.stateName));

  if (!state) {
    state = await withStep('states:create', () =>
      createAdminState({
        name: config.stateName,
        isoCode: config.stateIsoCode,
        metadata,
      }),
    );
    adminStates = [...adminStates, state];
  }

  let serverVendors = await withStep('vendors:server:list', () => fetchAdminVendors('server'));
  let serverVendor =
    serverVendors.find((entity) => normalize(entity.name) === normalize(config.serverVendorName)) ??
    null;

  if (!serverVendor) {
    serverVendor = await withStep('vendors:server:create', () =>
      createAdminVendor('server', {
        name: config.serverVendorName,
        metadata,
      }),
    );
    serverVendors = [...serverVendors, serverVendor];
  }

  let solarVendors = await withStep('vendors:solar:list', () => fetchAdminVendors('solarPump'));
  let solarVendor =
    solarVendors.find((entity) => normalize(entity.name) === normalize(config.solarVendorName)) ??
    null;

  if (!solarVendor) {
    solarVendor = await withStep('vendors:solar:create', () =>
      createAdminVendor('solarPump', {
        name: config.solarVendorName,
        metadata,
      }),
    );
    solarVendors = [...solarVendors, solarVendor];
  }

  let authorities = await withStep('authorities:list', () =>
    fetchAdminStateAuthorities({ stateId: state.id }),
  );
  let authority =
    authorities.find((entity) => normalize(entity.name) === normalize(config.authorityName)) ??
    null;

  if (!authority) {
    authority = await withStep('authorities:create', () =>
      createAdminStateAuthority({
        stateId: state.id,
        name: config.authorityName,
        metadata,
      }),
    );
    authorities = [...authorities, authority];
  }

  let projects = await withStep('projects:list', () =>
    fetchAdminProjects({ stateAuthorityId: authority.id }),
  );
  let project =
    projects.find((entity) => normalize(entity.name) === normalize(config.projectName)) ?? null;

  if (!project) {
    project = await withStep('projects:create', () =>
      createAdminProject({
        authorityId: authority.id,
        name: config.projectName,
        metadata,
      }),
    );
    projects = [...projects, project];
  }

  let protocolVersions = await withStep('protocol-versions:list', () =>
    fetchAdminProtocolVersions({
      stateId: state.id,
      stateAuthorityId: authority.id,
      projectId: project.id,
    }),
  );

  let protocol =
    protocolVersions.find(
      (entity) => normalize(entity.version) === normalize(config.protocolVersion),
    ) ?? null;

  if (!protocol) {
    protocol = await withStep('protocol-versions:create', () =>
      createAdminProtocolVersion({
        stateId: state.id,
        stateAuthorityId: authority.id,
        projectId: project.id,
        serverVendorId: serverVendor.id,
        version: config.protocolVersion,
        name: config.protocolName,
        metadata,
      }),
    );
    protocolVersions = [...protocolVersions, protocol];
  }

  const [stateOptions, authorityOptions, projectOptions] = await withStep('lookups:refresh', () =>
    Promise.all([
      fetchStates(),
      fetchAuthorities(state.id),
      fetchProjects({ stateId: state.id, stateAuthorityId: authority.id }),
    ]),
  );

  const stateOption =
    stateOptions.find((option) => option.id === state.id) ??
    stateOptions.find((option) => normalize(option.name) === normalize(config.stateName));

  if (!stateOption) {
    throw new Error('Bootstrap completed but state lookup is missing.');
  }

  const authorityOption =
    authorityOptions.find((option) => option.id === authority.id) ??
    authorityOptions.find((option) => normalize(option.name) === normalize(config.authorityName));

  if (!authorityOption) {
    throw new Error('Bootstrap completed but authority lookup is missing.');
  }

  const projectOption =
    projectOptions.find((option) => option.id === project.id) ??
    projectOptions.find((option) => normalize(option.name) === normalize(config.projectName));

  if (!projectOption) {
    throw new Error('Bootstrap completed but project lookup is missing.');
  }

  let protocolOption =
    projectOption.protocolVersions.find((option) => option.id === protocol.id) ??
    projectOption.protocolVersions.find(
      (option) => normalize(option.version) === normalize(config.protocolVersion),
    );

  if (!protocolOption) {
    protocolOption = {
      id: protocol.id,
      version: protocol.version,
      serverVendorId: protocol.serverVendorId,
      serverVendorName: protocol.serverVendorName,
    } satisfies ProtocolVersionOption;
  }

  queryClient.setQueryData(['states'], stateOptions);
  queryClient.setQueryData(['authorities', state.id], authorityOptions);
  queryClient.setQueryData(['projects', state.id, authority.id], projectOptions);
  queryClient.setQueryData(['vendors', 'solarPump'], solarVendors);
  queryClient.setQueryData(['vendors', 'server'], serverVendors);

  return {
    stateOption,
    authorityOption,
    projectOption,
    protocolOption,
    serverVendor,
    solarVendor,
    summary: {
      stateName: stateOption.name,
      authorityName: authorityOption.name,
      projectName: projectOption.name,
      protocolVersion: protocolOption.version,
      serverVendorName: protocolOption.serverVendorName ?? serverVendor.name,
      solarVendorName: solarVendor.name,
    },
  } satisfies BootstrapResult;
}

export function SimulatorPage() {
  const queryClient = useQueryClient();
  const { session, login: authenticate, logout: revokeSession } = useAuth();
  const [loginForm, setLoginForm] = useState<LoginPayload>(initialLoginForm);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [logoutError, setLogoutError] = useState<string | null>(null);
  const [selections, setSelections] = useState<SelectionState>(initialSelection);
  const [governmentForm, setGovernmentForm] = useState<GovernmentFormState>(initialGovernmentForm);
  const storedSimulatorState = useMemo(loadStoredSimulatorState, []);
  const [asn, setAsn] = useState<string>(storedSimulatorState?.asn ?? 'MH-2025-00001');
  const [enrollmentOutcome, setEnrollmentOutcome] = useState<EnrollmentOutcome | null>(() => {
    if (!storedSimulatorState?.device || !storedSimulatorState?.credentials) {
      return null;
    }

    return {
      device: storedSimulatorState.device,
      credentials: storedSimulatorState.credentials,
      governmentCredentials: storedSimulatorState.governmentCredentials ?? null,
      context: storedSimulatorState.context ?? {},
      asn: storedSimulatorState.asn ?? 'MH-2025-00001',
    } satisfies EnrollmentOutcome;
  });
  const [websocketUrl, setWebsocketUrl] = useState<string>(() => {
    const candidate = storedSimulatorState?.websocketUrl;
    if (candidate && candidate.startsWith('wss://')) {
      return candidate;
    }

    return String(DEFAULT_WEBSOCKET_URL);
  });
  const [autoRespond, setAutoRespond] = useState<boolean>(
    storedSimulatorState?.autoRespond ?? true,
  );
  const [keepalive, setKeepalive] = useState<number>(storedSimulatorState?.keepalive ?? 30);

  // Reset WebSocket URL if it's set to incorrect ports
  useEffect(() => {
    if (!websocketUrl.startsWith('wss://')) {
      setWebsocketUrl(DEFAULT_WEBSOCKET_URL);
    }
  }, [websocketUrl]);
  const [imei, setImei] = useState<string>(() => generateRandomImei());
  const [issuedBy, setIssuedBy] = useState<string>('');
  const [formError, setFormError] = useState<string | null>(null);
  const [mqttStatus, setMqttStatus] = useState<ConnectionStatus>('disconnected');
  const [mqttError, setMqttError] = useState<string | null>(null);
  const [logEntries, setLogEntries] = useState<SimulatorLogEntry[]>([]);
  const [streaming, setStreaming] = useState<boolean>(false);
  const [streamIntervalSeconds, setStreamIntervalSeconds] = useState<number>(5);
  const mqttClientRef = useRef<MqttClient | null>(null);
  const streamTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const telemetryRuntimeRef = useRef(createSimulatorTelemetryRuntime());
  const [lastCommand, setLastCommand] = useState<ReceivedCommand | null>(null);
  const [bootstrapStatus, setBootstrapStatus] = useState<'idle' | 'running' | 'success' | 'error'>(
    'idle',
  );
  const [bootstrapSummary, setBootstrapSummary] = useState<BootstrapSummary | null>(null);
  const [bootstrapError, setBootstrapError] = useState<string | null>(null);
  const [bootstrapTrace, setBootstrapTrace] = useState<BootstrapTraceEntry[]>([]);
  const bootstrapSessionRef = useRef<string | null>(null);
  const [bootstrapTrigger, triggerBootstrap] = useReducer((count: number) => count + 1, 0);

  const loginMutation = useMutation<SessionSnapshot, Error, LoginPayload>({
    mutationFn: authenticate,
    onSuccess: (snapshot) => {
      setLoginError(null);
      bootstrapSessionRef.current = snapshot.token;
      queryClient.clear();
    },
    onError: (error) => {
      setLoginError(error.message);
    },
  });

  const logoutMutation = useMutation<void, Error>({
    mutationFn: revokeSession,
    onSuccess: () => {
      setLoginForm(initialLoginForm);
      setSelections(initialSelection);
      setGovernmentForm(initialGovernmentForm);
      setEnrollmentOutcome(null);
      setLastCommand(null);
      setLogEntries([]);
      disconnectMqtt();
      setLogoutError(null);
      bootstrapSessionRef.current = null;
      queryClient.clear();
    },
    onError: (error) => {
      setLogoutError(error.message);
    },
  });

  const statesQuery = useQuery<StateOption[], Error>({
    queryKey: ['states'],
    queryFn: fetchStates,
    enabled: Boolean(session),
    staleTime: 5 * 60 * 1000,
  });

  const authoritiesQuery = useQuery<AuthorityOption[], Error>({
    queryKey: ['authorities', selections.stateId],
    queryFn: () => fetchAuthorities(selections.stateId),
    enabled: Boolean(session && selections.stateId),
    staleTime: 5 * 60 * 1000,
  });

  const projectsQuery = useQuery<ProjectOption[], Error>({
    queryKey: ['projects', selections.stateId, selections.authorityId],
    queryFn: () =>
      fetchProjects({
        stateId: selections.stateId,
        stateAuthorityId: selections.authorityId,
      }),
    enabled: Boolean(session && selections.stateId && selections.authorityId),
    staleTime: 5 * 60 * 1000,
  });

  const solarPumpVendorsQuery = useQuery<AdminVendor[], Error>({
    queryKey: ['vendors', 'solarPump'],
    queryFn: () => fetchAdminVendors('solarPump'),
    enabled: Boolean(session),
    staleTime: 10 * 60 * 1000,
  });

  useEffect(() => {
    if (!enrollmentOutcome) {
      persistSimulatorState({
        asn,
        websocketUrl,
        autoRespond,
        keepalive,
      });
      return;
    }

    persistSimulatorState({
      asn,
      websocketUrl,
      autoRespond,
      keepalive,
      device: enrollmentOutcome.device,
      credentials: enrollmentOutcome.credentials ?? null,
      governmentCredentials: enrollmentOutcome.governmentCredentials,
      context: enrollmentOutcome.context,
    });
  }, [asn, autoRespond, websocketUrl, keepalive, enrollmentOutcome]);

  useEffect(() => {
    telemetryRuntimeRef.current = createSimulatorTelemetryRuntime();
  }, [enrollmentOutcome?.device?.imei, enrollmentOutcome?.asn]);

  useEffect(() => {
    return () => {
      disconnectMqtt();
    };
  }, []);

  useEffect(() => {
    if (!session) {
      setBootstrapStatus('idle');
      setBootstrapSummary(null);
      setBootstrapError(null);
      setBootstrapTrace([]);
      bootstrapSessionRef.current = null;
      return;
    }

    if (bootstrapSessionRef.current === session.token) {
      return;
    }

    bootstrapSessionRef.current = session.token;
    setBootstrapStatus('idle');
    setBootstrapSummary(null);
    setBootstrapError(null);
    setBootstrapTrace([]);
    triggerBootstrap();
  }, [session]);

  useEffect(() => {
    if (!session) {
      return;
    }

    let active = true;
    setBootstrapStatus('running');
    setBootstrapError(null);
    setBootstrapTrace([]);

    const handleStep = (entry: BootstrapTraceEntry) => {
      if (!active) {
        return;
      }
      setBootstrapTrace((prev) => [...prev, entry]);
    };

    ensureSimulatorBootstrap(queryClient, handleStep)
      .then((result) => {
        if (!active) {
          return;
        }

        setSelections({
          stateId: result.stateOption.id,
          authorityId: result.authorityOption.id,
          projectId: result.projectOption.id,
          protocolVersionId: result.protocolOption.id,
          solarPumpVendorId: result.solarVendor.id,
        });
        setBootstrapSummary(result.summary);
        setBootstrapStatus('success');
      })
      .catch((error) => {
        if (!active) {
          return;
        }

        const message = formatBootstrapError(error);
        setBootstrapError(message);
        setBootstrapStatus('error');
      });

    return () => {
      active = false;
    };
  }, [session, bootstrapTrigger, queryClient]);

  const states = useMemo(() => statesQuery.data ?? [], [statesQuery.data]);
  const authorities = useMemo(() => authoritiesQuery.data ?? [], [authoritiesQuery.data]);
  const projects = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);
  const solarPumpVendors = useMemo(
    () => solarPumpVendorsQuery.data ?? [],
    [solarPumpVendorsQuery.data],
  );

  const selectedState = useMemo(
    () => states.find((state) => state.id === selections.stateId),
    [states, selections.stateId],
  );
  const selectedAuthority = useMemo(
    () => authorities.find((authority) => authority.id === selections.authorityId),
    [authorities, selections.authorityId],
  );
  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selections.projectId),
    [projects, selections.projectId],
  );
  const protocolOptions = useMemo(() => selectedProject?.protocolVersions ?? [], [selectedProject]);
  const selectedProtocol = useMemo(
    () => protocolOptions.find((protocol) => protocol.id === selections.protocolVersionId),
    [protocolOptions, selections.protocolVersionId],
  );

  useEffect(() => {
    if (!governmentForm.enabled) {
      return;
    }

    const defaults = selectedProtocol?.governmentCredentialDefaults ?? null;
    if (!defaults || defaults.endpoints.length === 0) {
      return;
    }

    const endpoint = defaults.endpoints[0];
    setGovernmentForm((prev) => {
      const port = endpoint.port ? String(endpoint.port) : '';
      if (
        prev.host === endpoint.host &&
        prev.port === port &&
        prev.protocol === endpoint.protocol
      ) {
        return prev;
      }

      return {
        ...prev,
        protocol: endpoint.protocol,
        host: endpoint.host,
        port,
      };
    });
  }, [governmentForm.enabled, selectedProtocol]);
  const selectedSolarVendor = useMemo(
    () => solarPumpVendors.find((vendor) => vendor.id === selections.solarPumpVendorId) ?? null,
    [solarPumpVendors, selections.solarPumpVendorId],
  );

  const publishTopics = useMemo<PublishTopics>(() => {
    const topics: PublishTopics = {};
    const list = enrollmentOutcome?.credentials?.topics.publish ?? [];
    for (const topic of list) {
      const suffix = extractTopicSuffix(topic);
      if (suffix) {
        topics[suffix] = topic;
      }
    }
    return topics;
  }, [enrollmentOutcome?.credentials?.topics.publish]);

  const subscribeTopics = useMemo<SubscribeTopics>(() => {
    const list = enrollmentOutcome?.credentials?.topics.subscribe ?? [];
    const map: SubscribeTopics = { all: list };
    for (const topic of list) {
      const suffix = extractTopicSuffix(topic);
      if (suffix) {
        map[suffix] = topic;
      }
    }
    return map;
  }, [enrollmentOutcome?.credentials?.topics.subscribe]);

  const handleLoginSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (loginMutation.isPending) {
      return;
    }
    setLoginError(null);
    loginMutation.mutate(loginForm);
  };

  const handleLogout = () => {
    if (logoutMutation.isPending) {
      return;
    }
    logoutMutation.mutate();
  };

  function handleBootstrapRetry() {
    if (!session) {
      return;
    }

    setBootstrapSummary(null);
    setBootstrapError(null);
    setBootstrapStatus('idle');
    setBootstrapTrace([]);
    triggerBootstrap();
  }

  function handleSelectionChange(field: keyof SelectionState, value: string) {
    setSelections((prev) => {
      if (field === 'stateId') {
        return {
          stateId: value,
          authorityId: '',
          projectId: '',
          protocolVersionId: '',
          solarPumpVendorId: '',
        };
      }

      if (field === 'authorityId') {
        return {
          stateId: prev.stateId,
          authorityId: value,
          projectId: '',
          protocolVersionId: '',
          solarPumpVendorId: '',
        };
      }

      if (field === 'projectId') {
        return {
          ...prev,
          projectId: value,
          protocolVersionId: '',
        };
      }

      return {
        ...prev,
        [field]: value,
      };
    });
    setFormError(null);
  }

  function handleGovernmentToggle(enabled: boolean) {
    if (!enabled) {
      setGovernmentForm(initialGovernmentForm);
      return;
    }

    const defaults = selectedProtocol?.governmentCredentialDefaults ?? null;
    const endpoint = defaults?.endpoints?.[0];
    setGovernmentForm({
      enabled: true,
      protocol: endpoint?.protocol ?? 'mqtts',
      host: endpoint?.host ?? '',
      port: endpoint?.port ? String(endpoint.port) : '',
      clientId: '',
      username: '',
      password: '',
    });
  }

  function validateEnrollment(): string | null {
    if (!selectedProtocol) {
      return 'Select a protocol version before enrolling the device.';
    }

    const required: Array<[string, string]> = [
      ['IMEI', imei.trim()],
      ['State', selections.stateId],
      ['State authority', selections.authorityId],
      ['Project', selections.projectId],
      ['Protocol version', selections.protocolVersionId],
      ['Solar pump vendor', selections.solarPumpVendorId],
    ];

    const missing = required.find(([, value]) => !value);
    if (missing) {
      return `${missing[0]} is required before provisioning.`;
    }

    if (governmentForm.enabled) {
      const requiredGov: Array<[string, string]> = [
        ['Government host', governmentForm.host.trim()],
        ['Government port', governmentForm.port.trim()],
        ['Government client ID', governmentForm.clientId.trim()],
        ['Government username', governmentForm.username.trim()],
        ['Government password', governmentForm.password],
      ];

      const missingGov = requiredGov.find(([, value]) => !value);
      if (missingGov) {
        return `${missingGov[0]} is required when providing government credentials.`;
      }

      const portValue = Number(governmentForm.port.trim());
      if (!Number.isFinite(portValue) || portValue <= 0) {
        return 'Government port must be a positive number.';
      }
    }

    return null;
  }

  const enrollmentMutation = useMutation<RegisterDeviceResponse, Error, RegisterDevicePayload>({
    mutationFn: registerDevice,
    onSuccess: (data) => {
      const context: EnrollmentContext = {
        state: selectedState,
        authority: selectedAuthority,
        project: selectedProject,
        protocol: selectedProtocol,
        solarPumpVendor: selectedSolarVendor,
      };
      setEnrollmentOutcome({
        device: data.device,
        credentials: data.credentials,
        governmentCredentials: data.governmentCredentials,
        context,
        asn,
      });
      setBootstrapSummary((prev) => {
        if (!prev || !data.credentials) {
          return prev;
        }

        return {
          ...prev,
          deviceImei: data.device.imei,
          clientId: data.credentials.clientId,
          username: data.credentials.username,
        };
      });
      setFormError(null);
      setLogEntries((prev) => [createLogEntry('system', 'Device enrolled successfully'), ...prev]);
    },
    onError: (error) => {
      setFormError(error.message);
    },
  });

  const handleEnrollmentSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const validationError = validateEnrollment();
    if (validationError) {
      setFormError(validationError);
      return;
    }

    if (!selectedProtocol) {
      setFormError('Select a protocol version before provisioning.');
      return;
    }

    const payload: RegisterDevicePayload = {
      imei: imei.trim(),
      solarPumpVendorId: selections.solarPumpVendorId,
      stateId: selections.stateId,
      stateAuthorityId: selections.authorityId,
      projectId: selections.projectId,
      protocolVersionId: selections.protocolVersionId,
      serverVendorId: selectedProtocol.serverVendorId,
      issuedBy: issuedBy.trim() || undefined,
    };

    if (governmentForm.enabled) {
      payload.governmentCredentials = {
        clientId: governmentForm.clientId.trim(),
        username: governmentForm.username.trim(),
        password: governmentForm.password,
        endpoints: [
          {
            protocol: governmentForm.protocol,
            host: governmentForm.host.trim(),
            port: Number(governmentForm.port.trim()),
            url: `${governmentForm.protocol}://${governmentForm.host.trim()}:${governmentForm.port.trim()}`,
          },
        ],
        topics: selectedProtocol.governmentCredentialDefaults?.topics ?? EMPTY_GOVERNMENT_TOPICS,
        metadata: {
          source: 'browser-simulator',
          defaultsApplied: Boolean(selectedProtocol.governmentCredentialDefaults),
        },
      } satisfies GovernmentCredentialBundle;
    }

    enrollmentMutation.mutate(payload);
  };

  function createLogEntry(
    direction: SimulatorLogEntry['direction'],
    message: string,
    topic?: string,
    payload?: Record<string, unknown> | null,
  ): SimulatorLogEntry {
    return {
      id: crypto.randomUUID(),
      timestamp: new Date().toISOString(),
      direction,
      topic,
      message,
      payload: payload ?? null,
    };
  }

  function appendLog(
    direction: SimulatorLogEntry['direction'],
    message: string,
    topic?: string,
    payload?: Record<string, unknown> | null,
  ) {
    setLogEntries((prev) => {
      const next = [createLogEntry(direction, message, topic, payload), ...prev];
      return next.slice(0, 200);
    });
  }

  function connectToBroker() {
    if (!enrollmentOutcome) {
      appendLog('system', 'Enroll a device to load credentials before connecting.');
      return;
    }

    if (mqttStatus === 'connecting' || mqttStatus === 'connected') {
      appendLog('system', 'Already connected to broker.');
      return;
    }

    const credentials = enrollmentOutcome.credentials;
    if (!credentials) {
      appendLog(
        'system',
        'Local MQTT credentials are unavailable. Rotate credentials before connecting.',
      );
      return;
    }

    const { clientId, username, password } = credentials;

    if (!websocketUrl.trim()) {
      appendLog('system', 'Provide a WebSocket URL for the MQTT broker.');
      return;
    }

    try {
      const client = mqtt.connect(websocketUrl.trim(), {
        clientId,
        username,
        password,
        reconnectPeriod: 0,
        keepalive: keepalive,
        clean: true,
      });

      mqttClientRef.current = client;
      setMqttStatus('connecting');
      setMqttError(null);

      client.on('connect', () => {
        setMqttStatus('connected');
        appendLog('system', 'Connected to MQTT broker.');

        for (const topic of subscribeTopics.all) {
          client.subscribe(topic, { qos: 0 }, (error) => {
            if (error) {
              appendLog('system', `Failed to subscribe to ${topic}`, topic, {
                error: error.message,
              });
            } else {
              appendLog('system', `Subscribed to ${topic}`, topic);
            }
          });
        }
      });

      client.on('error', (error) => {
        setMqttStatus('error');
        setMqttError(error.message);
        appendLog('system', `MQTT connection error: ${error.message}`);
        client.end(true);
        mqttClientRef.current = null;
      });

      client.on('close', () => {
        setMqttStatus('disconnected');
        appendLog('system', 'MQTT connection closed.');
        stopTelemetryStream();
        mqttClientRef.current = null;
      });

      client.on('message', (topic, buffer) => {
        const payloadText = decodePayload(buffer);
        let payload: Record<string, unknown> | null = null;
        try {
          payload = payloadText ? (JSON.parse(payloadText) as Record<string, unknown>) : null;
        } catch {
          appendLog('inbound', 'Received non-JSON payload', topic, {
            payload: payloadText,
          });
          return;
        }

        appendLog('inbound', 'Message received', topic, payload ?? undefined);
        handleIncomingMessage(topic, payload ?? {});
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unexpected error while connecting.';
      setMqttStatus('error');
      setMqttError(message);
      appendLog('system', message);
    }
  }

  function disconnectMqtt() {
    if (streamTimerRef.current) {
      clearInterval(streamTimerRef.current);
      streamTimerRef.current = null;
    }
    setStreaming(false);

    if (mqttClientRef.current) {
      mqttClientRef.current.end(true);
      mqttClientRef.current.removeAllListeners();
      mqttClientRef.current = null;
    }

    setMqttStatus('disconnected');
  }

  function decodePayload(buffer: Uint8Array): string {
    if (typeof TextDecoder !== 'undefined') {
      return new TextDecoder('utf-8').decode(buffer);
    }
    // Fallback for environments without TextDecoder support.
    let result = '';
    for (let i = 0; i < buffer.length; i += 1) {
      result += String.fromCharCode(buffer[i]);
    }
    return result;
  }

  function handleIncomingMessage(topic: string, payload: Record<string, unknown>) {
    if ('cmd' in payload) {
      const msgid = typeof payload.msgid === 'string' ? payload.msgid : crypto.randomUUID();
      const commandPayload = inferCommandPayload(payload);

      const command: ReceivedCommand = {
        topic,
        msgid,
        cmd: String(payload.cmd ?? 'unknown'),
        payload: commandPayload,
        raw: payload,
      };

      setLastCommand(command);
      appendLog('system', `Ondemand command ${command.cmd}`, topic, commandPayload);

      if (autoRespond) {
        sendOndemandAcknowledgement(command);
      }
      return;
    }

    if ('status' in payload && typeof payload.status === 'string') {
      appendLog('system', `Ondemand status: ${payload.status}`, topic, payload);
      return;
    }
  }

  function inferCommandPayload(payload: Record<string, unknown>): Record<string, unknown> {
    if (payload.payload && typeof payload.payload === 'object') {
      return payload.payload as Record<string, unknown>;
    }

    return payload;
  }

  function publish(topic: string | undefined, message: Record<string, unknown>): PublishResult {
    if (!topic) {
      appendLog('system', 'Missing topic mapping for this payload');
      return { success: false };
    }

    if (!mqttClientRef.current || mqttStatus !== 'connected') {
      appendLog('system', 'Connect to the broker before publishing telemetry.');
      return { success: false };
    }

    return {
      success: mqttClientRef.current.publish(
        topic,
        JSON.stringify(message),
        { qos: 0 },
        (error) => {
          if (error) {
            appendLog('system', `Failed to publish message: ${error.message}`, topic, message);
          } else {
            appendLog('outbound', 'Message published', topic, message);
          }
        },
      ) as unknown as boolean,
    };
  }

  function sendHeartbeat() {
    if (!enrollmentOutcome) {
      appendLog('system', 'Enroll a device first to generate telemetry.');
      return;
    }

    const topic = publishTopics.heartbeat;
    const payload = buildHeartbeatPayload(enrollmentOutcome.device.imei, enrollmentOutcome.asn);
    publish(topic, payload);
  }

  function sendPump() {
    if (!enrollmentOutcome) {
      appendLog('system', 'Enroll a device first to generate telemetry.');
      return;
    }

    const runtime = telemetryRuntimeRef.current;
    const dataTopic = publishTopics.data;
    if (!dataTopic) {
      appendLog('system', 'Missing data topic mapping for pump telemetry payload.');
      return;
    }

    const payload = buildPumpPayload(enrollmentOutcome.device.imei, enrollmentOutcome.asn, runtime);
    publish(dataTopic, payload);
  }

  function sendDaq() {
    if (!enrollmentOutcome) {
      appendLog('system', 'Enroll a device first to generate telemetry.');
      return;
    }

    const topic = publishTopics.daq;
    const payload = buildDaqPayload(
      enrollmentOutcome.device.imei,
      enrollmentOutcome.asn,
      telemetryRuntimeRef.current,
    );
    publish(topic, payload);
  }

  async function copyCurrentImei() {
    const currentImei = enrollmentOutcome?.device.imei ?? imei;
    const value = currentImei.trim();
    if (!value) {
      appendLog('system', 'Enter or enroll an IMEI before copying.');
      return;
    }

    try {
      await navigator.clipboard.writeText(value);
      appendLog('system', `Copied IMEI ${value} to clipboard.`);
    } catch {
      appendLog('system', 'Unable to copy IMEI in this browser context.');
    }
  }

  function publishLegacyError() {
    const currentImei = enrollmentOutcome?.device.imei ?? imei;
    const value = currentImei.trim();
    if (!value) {
      appendLog('system', 'Enter or enroll an IMEI before publishing errors.');
      return;
    }

    publish(`${value}/errors`, {
      action: 'E',
      errorCode: 'SIM-FAULT',
      message: 'Simulator generated test fault event',
      ts: Date.now(),
    });
  }

  function publishLegacyOndemandSample() {
    const currentImei = enrollmentOutcome?.device.imei ?? imei;
    const value = currentImei.trim();
    if (!value) {
      appendLog('system', 'Enter or enroll an IMEI before publishing ondemand sample.');
      return;
    }

    publish(`${value}/ondemand`, {
      status: 'completed',
      cmd: 'ON_DEMAND_DATA',
      msgid: crypto.randomUUID(),
      payload: {
        imei: value,
        source: 'simulator-quick-action',
        ts: Date.now(),
      },
    });
  }

  function toggleTelemetryStream() {
    if (!streaming) {
      const intervalMs = Math.max(500, Math.min(streamIntervalSeconds * 1000, 60_000));
      sendHeartbeat();
      sendPump();
      sendDaq();
      streamTimerRef.current = setInterval(() => {
        // Send all telemetry types on each tick for consistent coverage.
        sendHeartbeat();
        sendPump();
        sendDaq();
      }, intervalMs);
      setStreaming(true);
      appendLog(
        'system',
        `Telemetry stream started (${(intervalMs / 1000).toFixed(1)}s interval).`,
      );
      return;
    }

    stopTelemetryStream();
    appendLog('system', 'Telemetry stream stopped.');
  }

  function stopTelemetryStream() {
    if (streamTimerRef.current) {
      clearInterval(streamTimerRef.current);
      streamTimerRef.current = null;
    }
    setStreaming(false);
  }

  function sendOndemandAcknowledgement(command: ReceivedCommand) {
    const topic = subscribeTopics.ondemand ?? command.topic;
    const toggleValue = deriveToggleValue(command.payload);
    const response = buildOndemandResponse(command.msgid, toggleValue);
    publish(topic, response);
  }

  function handleManualAck() {
    if (!lastCommand) {
      appendLog('system', 'No ondemand command received yet.');
      return;
    }

    sendOndemandAcknowledgement(lastCommand);
  }

  function deriveToggleValue(payload: Record<string, unknown>): number {
    const direct = payload.DO1;
    if (typeof direct === 'number') {
      return direct;
    }
    if (typeof direct === 'string') {
      const parsed = Number(direct);
      if (Number.isFinite(parsed)) {
        return parsed;
      }
    }

    const nested = payload.payload;
    if (nested && typeof nested === 'object') {
      return deriveToggleValue(nested as Record<string, unknown>);
    }

    return 0;
  }

  const canConnect = Boolean(enrollmentOutcome);

  const loginSessionExpiry = useMemo(() => {
    if (!session?.expiresAt) {
      return null;
    }
    const expiry = Date.parse(session.expiresAt);
    if (Number.isNaN(expiry)) {
      return null;
    }
    return new Date(expiry).toLocaleString();
  }, [session?.expiresAt]);

  const governmentDefaults = selectedProtocol?.governmentCredentialDefaults ?? null;

  return (
    <div className="space-y-8">
      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold text-slate-900">Simulator Session</h2>
            <p className="text-sm text-slate-600">
              Log in with provisioning credentials to call enrollment APIs and generate MQTT
              bundles.
            </p>
          </div>
          <StatusBadge status={session ? 'online' : 'offline'} />
        </div>
        {!session && (
          <form className="mt-6 grid gap-4 md:grid-cols-3" onSubmit={handleLoginSubmit}>
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
              <span>Username</span>
              <input
                type="text"
                value={loginForm.username}
                onChange={(event) =>
                  setLoginForm((prev) => ({ ...prev, username: event.target.value }))
                }
                className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                placeholder="Him"
                required
              />
            </label>
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
              <span>Password</span>
              <input
                type="password"
                value={loginForm.password}
                onChange={(event) =>
                  setLoginForm((prev) => ({ ...prev, password: event.target.value }))
                }
                className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                placeholder="0554"
                required
              />
            </label>
            <div className="flex items-end gap-3">
              <button
                type="submit"
                className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                disabled={loginMutation.isPending}
              >
                {loginMutation.isPending ? 'Authenticating…' : 'Log In'}
              </button>
              <button
                type="button"
                className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={() => setLoginForm(initialLoginForm)}
                disabled={loginMutation.isPending}
              >
                Clear
              </button>
            </div>
          </form>
        )}
        {session && (
          <div className="mt-6 flex flex-wrap items-center justify-between gap-4">
            <div className="space-y-1 text-sm text-slate-700">
              <div>
                <span className="font-semibold">Signed in as:</span> {session.displayName}{' '}
                <span className="text-slate-500">({session.username})</span>
              </div>
              <div>
                <span className="font-semibold">Session expires:</span> {loginSessionExpiry ?? '—'}
              </div>
            </div>
            <button
              type="button"
              className="rounded-md border border-red-200 px-4 py-2 text-sm font-semibold text-red-600 hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2"
              onClick={handleLogout}
              disabled={logoutMutation.isPending}
            >
              {logoutMutation.isPending ? 'Ending session…' : 'Log Out'}
            </button>
          </div>
        )}
        {loginError && (
          <p className="mt-4 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {loginError}
          </p>
        )}
        {logoutError && (
          <p className="mt-4 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {logoutError}
          </p>
        )}
        {session && (
          <div className="mt-6 rounded-md border border-slate-200 bg-slate-50 p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h4 className="text-sm font-semibold text-slate-800">Default Simulator Setup</h4>
                <p className="text-xs text-slate-600">
                  Ensures Maharashtra -&gt; MSEDCL hierarchy, vendors, and protocol metadata exist
                  before enrollment.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <span
                  className={`text-xs font-semibold ${
                    bootstrapStatus === 'success'
                      ? 'text-emerald-600'
                      : bootstrapStatus === 'error'
                        ? 'text-red-600'
                        : 'text-slate-600'
                  }`}
                >
                  {bootstrapStatus === 'running'
                    ? 'Preparing…'
                    : bootstrapStatus === 'success'
                      ? 'Ready'
                      : bootstrapStatus === 'error'
                        ? 'Needs attention'
                        : 'Queued'}
                </span>
                <button
                  type="button"
                  className="rounded border border-slate-300 px-2 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
                  onClick={handleBootstrapRetry}
                  disabled={bootstrapStatus === 'running'}
                >
                  {bootstrapStatus === 'error' ? 'Retry' : 'Re-run'}
                </button>
              </div>
            </div>
            {bootstrapStatus === 'running' && (
              <p className="mt-3 text-sm text-slate-600">
                Provisioning defaults from RMS_Server_Preprompt.txt. This includes hierarchy
                records, server vendor, solar vendor, and protocol metadata.
              </p>
            )}
            {bootstrapTrace.length > 0 && (
              <div className="mt-3 rounded border border-slate-200 bg-white p-3">
                <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                  Bootstrap Steps
                </p>
                <ul className="mt-2 space-y-1 text-xs text-slate-700">
                  {bootstrapTrace.map((entry, index) => (
                    <li key={`${entry.step}-${entry.at}-${index}`} className="flex items-start gap-2">
                      <span
                        className={
                          entry.status === 'done'
                            ? 'text-emerald-600'
                            : entry.status === 'failed'
                              ? 'text-red-600'
                              : 'text-slate-500'
                        }
                      >
                        {entry.status === 'done' ? 'OK' : entry.status === 'failed' ? 'ERR' : 'RUN'}
                      </span>
                      <span className="font-mono">{entry.step}</span>
                      {entry.detail ? <span className="text-slate-500">- {entry.detail}</span> : null}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {bootstrapStatus === 'error' && bootstrapError && (
              <p className="mt-3 rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">
                {bootstrapError}
              </p>
            )}
            {bootstrapStatus === 'success' && bootstrapSummary && (
              <div className="mt-3 grid gap-3 text-sm text-slate-700 md:grid-cols-2">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    State
                  </p>
                  <p className="text-slate-900">{bootstrapSummary.stateName}</p>
                </div>
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Authority
                  </p>
                  <p className="text-slate-900">{bootstrapSummary.authorityName}</p>
                </div>
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Project
                  </p>
                  <p className="text-slate-900">{bootstrapSummary.projectName}</p>
                </div>
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Protocol
                  </p>
                  <p className="text-slate-900">{bootstrapSummary.protocolVersion}</p>
                </div>
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Server Vendor
                  </p>
                  <p className="text-slate-900">{bootstrapSummary.serverVendorName}</p>
                </div>
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Solar Vendor
                  </p>
                  <p className="text-slate-900">{bootstrapSummary.solarVendorName}</p>
                </div>
                {bootstrapSummary.deviceImei && (
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                      Last Enrolled IMEI
                    </p>
                    <p className="text-slate-900">{bootstrapSummary.deviceImei}</p>
                  </div>
                )}
                {bootstrapSummary.clientId && (
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                      Last Client ID
                    </p>
                    <p className="text-slate-900">{bootstrapSummary.clientId}</p>
                  </div>
                )}
                {bootstrapSummary.username && (
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                      Last Username
                    </p>
                    <p className="text-slate-900">{bootstrapSummary.username}</p>
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-base font-semibold text-slate-900">Provision Device via API</h3>
        <p className="mt-1 text-sm text-slate-600">
          Submit the enrollment form to mirror the provisioning workflow described in the preprompt
          (Device Provisioning & Credentials section).
        </p>
        {!session && (
          <p className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
            Log in above to load lookup options and access the enrollment endpoint.
          </p>
        )}
        <form className="mt-6 grid gap-4 md:grid-cols-3" onSubmit={handleEnrollmentSubmit}>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>IMEI</span>
            <input
              type="text"
              value={imei}
              onChange={(event) => setImei(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              placeholder="86963…"
              required
              disabled={!session}
            />
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <button
                type="button"
                className="w-max rounded border border-slate-300 px-2 py-1 text-xs text-slate-600 hover:bg-slate-100"
                onClick={() => setImei(generateRandomImei())}
                disabled={!session}
              >
                Generate random IMEI
              </button>
              <button
                type="button"
                className="w-max rounded border border-slate-300 px-2 py-1 text-xs text-slate-600 hover:bg-slate-100"
                onClick={copyCurrentImei}
              >
                Copy IMEI
              </button>
            </div>
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>Application Serial Number (ASN)</span>
            <input
              type="text"
              value={asn}
              onChange={(event) => setAsn(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              placeholder="MH-2025-00001"
              required
              disabled={!session}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>Issued By (optional)</span>
            <input
              type="text"
              value={issuedBy}
              onChange={(event) => setIssuedBy(event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              placeholder="operator-id"
              disabled={!session}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>State</span>
            <select
              value={selections.stateId}
              onChange={(event) => handleSelectionChange('stateId', event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              disabled={!session || statesQuery.isLoading || !states.length}
              required
            >
              <option value="">Select state</option>
              {states.map((state) => (
                <option key={state.id} value={state.id}>
                  {state.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>State Authority</span>
            <select
              value={selections.authorityId}
              onChange={(event) => handleSelectionChange('authorityId', event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              disabled={!session || !selections.stateId || !authorities.length}
              required
            >
              <option value="">
                {selections.stateId ? 'Select authority' : 'Select state first'}
              </option>
              {authorities.map((authority) => (
                <option key={authority.id} value={authority.id}>
                  {authority.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>Project</span>
            <select
              value={selections.projectId}
              onChange={(event) => handleSelectionChange('projectId', event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              disabled={!session || !selections.authorityId || !projects.length}
              required
            >
              <option value="">
                {selections.authorityId ? 'Select project' : 'Select authority first'}
              </option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>Protocol Version</span>
            <select
              value={selections.protocolVersionId}
              onChange={(event) => handleSelectionChange('protocolVersionId', event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              disabled={!session || !selections.projectId || !protocolOptions.length}
              required
            >
              <option value="">
                {selections.projectId
                  ? protocolOptions.length
                    ? 'Select protocol version'
                    : 'No protocols configured'
                  : 'Select project first'}
              </option>
              {protocolOptions.map((protocol) => (
                <option key={protocol.id} value={protocol.id}>
                  {protocol.version} —{' '}
                  {protocol.serverVendorName ?? `Vendor ${protocol.serverVendorId}`}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
            <span>Solar Pump Vendor</span>
            <select
              value={selections.solarPumpVendorId}
              onChange={(event) => handleSelectionChange('solarPumpVendorId', event.target.value)}
              className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              disabled={!session || solarPumpVendorsQuery.isLoading || !solarPumpVendors.length}
              required
            >
              <option value="">Select solar pump vendor</option>
              {solarPumpVendors.map((vendor) => (
                <option key={vendor.id} value={vendor.id}>
                  {vendor.name}
                </option>
              ))}
            </select>
          </label>
          <div className="rounded-md border border-slate-200 bg-slate-50 p-4 md:col-span-3">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <label className="flex items-center gap-2 text-sm font-semibold text-slate-700">
                <input
                  type="checkbox"
                  className="size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
                  checked={governmentForm.enabled}
                  onChange={(event) => handleGovernmentToggle(event.target.checked)}
                  disabled={!session}
                />
                Provide government credential bundle
              </label>
              {governmentDefaults?.endpoints && governmentDefaults.endpoints.length > 0 && (
                <span className="text-xs text-slate-500">
                  Defaults:{' '}
                  {governmentDefaults.endpoints
                    .map((endpoint) => `${endpoint.protocol}://${endpoint.host}:${endpoint.port}`)
                    .join(', ')}
                </span>
              )}
            </div>
            {governmentForm.enabled && (
              <div className="mt-4 grid gap-3 md:grid-cols-3">
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Protocol</span>
                  <select
                    value={governmentForm.protocol}
                    onChange={(event) =>
                      setGovernmentForm((prev) => ({
                        ...prev,
                        protocol: event.target.value as GovernmentFormState['protocol'],
                      }))
                    }
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
                    value={governmentForm.host}
                    onChange={(event) =>
                      setGovernmentForm((prev) => ({ ...prev, host: event.target.value }))
                    }
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    placeholder="gov.example"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Port</span>
                  <input
                    type="text"
                    value={governmentForm.port}
                    onChange={(event) =>
                      setGovernmentForm((prev) => ({ ...prev, port: event.target.value }))
                    }
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    placeholder="8883"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700 md:col-span-3">
                  <span>Client ID</span>
                  <input
                    type="text"
                    value={governmentForm.clientId}
                    onChange={(event) =>
                      setGovernmentForm((prev) => ({ ...prev, clientId: event.target.value }))
                    }
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Username</span>
                  <input
                    type="text"
                    value={governmentForm.username}
                    onChange={(event) =>
                      setGovernmentForm((prev) => ({ ...prev, username: event.target.value }))
                    }
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    required
                  />
                </label>
                <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                  <span>Password</span>
                  <input
                    type="text"
                    value={governmentForm.password}
                    onChange={(event) =>
                      setGovernmentForm((prev) => ({ ...prev, password: event.target.value }))
                    }
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                    required
                  />
                </label>
              </div>
            )}
          </div>
          <div className="flex items-center gap-3 pt-2 md:col-span-3">
            <button
              type="submit"
              className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
              disabled={!session || enrollmentMutation.isPending}
            >
              {enrollmentMutation.isPending ? 'Enrolling…' : 'Enroll Device'}
            </button>
            <button
              type="button"
              className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              onClick={() => {
                setSelections(initialSelection);
                setGovernmentForm(initialGovernmentForm);
                setIssuedBy('');
                setImei(generateRandomImei());
                setFormError(null);
              }}
              disabled={!session || enrollmentMutation.isPending}
            >
              Reset Form
            </button>
          </div>
        </form>
        {formError && (
          <p className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
            {formError}
          </p>
        )}
        {enrollmentOutcome && (
          <div className="mt-6 space-y-4">
            <div className="grid gap-3 md:grid-cols-3">
              <Detail label="Device UUID" value={enrollmentOutcome.device.id} />
              <Detail label="IMEI" value={enrollmentOutcome.device.imei} />
              <Detail label="ASN" value={enrollmentOutcome.asn} />
              {enrollmentOutcome.context.state && (
                <Detail label="State" value={enrollmentOutcome.context.state.name} />
              )}
              {enrollmentOutcome.context.authority && (
                <Detail label="Authority" value={enrollmentOutcome.context.authority.name} />
              )}
              {enrollmentOutcome.context.project && (
                <Detail label="Project" value={enrollmentOutcome.context.project.name} />
              )}
              {enrollmentOutcome.context.protocol && (
                <Detail
                  label="Protocol"
                  value={`${enrollmentOutcome.context.protocol.version} — ${enrollmentOutcome.context.protocol.serverVendorName ?? `Vendor ${enrollmentOutcome.context.protocol.serverVendorId}`}`}
                />
              )}
              {enrollmentOutcome.context.solarPumpVendor && (
                <Detail
                  label="Solar Pump Vendor"
                  value={enrollmentOutcome.context.solarPumpVendor.name}
                />
              )}
            </div>
            {enrollmentOutcome.credentials ? (
              <CredentialBlock
                title="Local MQTT Credentials"
                bundle={enrollmentOutcome.credentials}
              />
            ) : (
              <p className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                Local MQTT credentials are unavailable. Rotate credentials to unblock simulator
                access.
              </p>
            )}
            {enrollmentOutcome.governmentCredentials && (
              <CredentialBlock
                title="Government Server Credentials"
                bundle={enrollmentOutcome.governmentCredentials}
              />
            )}
          </div>
        )}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="text-base font-semibold text-slate-900">MQTT Bridge</h3>
        <p className="mt-1 text-sm text-slate-600">
          Connect over the EMQX WebSocket listener (8080) to publish telemetry topics and receive
          ondemand commands.
        </p>
        <div className="mt-4 grid gap-4 md:grid-cols-4">
          <Detail label="Connection Status" value={mqttStatus} />
          <Detail label="Auto Respond" value={autoRespond ? 'Enabled' : 'Disabled'} />
          <Detail label="Heartbeat Topic" value={publishTopics.heartbeat ?? '—'} />
          <Detail label="Ondemand Topic" value={subscribeTopics.ondemand ?? '—'} />
        </div>
        <div className="mt-6 flex flex-wrap items-center gap-3">
          <label className="min-w-[240px] flex-1 text-sm font-medium text-slate-700">
            <span className="block">WebSocket URL</span>
            <input
              type="text"
              value={websocketUrl}
              onChange={(event) => setWebsocketUrl(event.target.value)}
              className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              placeholder="ws://localhost:8080"
            />
          </label>
          <label className="min-w-[120px] text-sm font-medium text-slate-700">
            <span className="block">Keepalive (s)</span>
            <input
              type="number"
              value={keepalive}
              onChange={(event) => setKeepalive(Number(event.target.value))}
              className="mt-1 w-full rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              placeholder="30"
              min="1"
              max="300"
            />
          </label>
          <div className="flex items-center gap-2">
            <input
              id="autoRespond"
              type="checkbox"
              className="size-4 rounded border-slate-300 text-emerald-600 focus:ring-emerald-500"
              checked={autoRespond}
              onChange={(event) => setAutoRespond(event.target.checked)}
            />
            <label htmlFor="autoRespond" className="text-sm text-slate-700">
              Auto-acknowledge ondemand commands
            </label>
          </div>
          <div className="flex items-center gap-3">
            <button
              type="button"
              className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
              onClick={connectToBroker}
              disabled={!canConnect || mqttStatus === 'connecting' || mqttStatus === 'connected'}
            >
              Connect
            </button>
            <button
              type="button"
              className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
              onClick={disconnectMqtt}
              disabled={mqttStatus === 'disconnected'}
            >
              Disconnect
            </button>
          </div>
        </div>
        {mqttError && (
          <p className="mt-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {mqttError}
          </p>
        )}
        <div className="mt-6 grid gap-4 md:grid-cols-3">
          <button
            type="button"
            className="rounded-md bg-slate-800 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-slate-800 focus:ring-offset-2"
            onClick={sendHeartbeat}
          >
            Publish Heartbeat
          </button>
          <button
            type="button"
            className="rounded-md bg-slate-800 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-slate-800 focus:ring-offset-2"
            onClick={sendPump}
          >
            Publish Pump/Data
          </button>
          <button
            type="button"
            className="rounded-md bg-slate-800 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-slate-800 focus:ring-offset-2"
            onClick={sendDaq}
          >
            Publish DAQ
          </button>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-3">
          <button
            type="button"
            className="rounded-md border border-amber-300 px-4 py-2 text-sm font-semibold text-amber-800 hover:bg-amber-50"
            onClick={publishLegacyError}
          >
            Quick Publish IMEI/errors
          </button>
          <button
            type="button"
            className="rounded-md border border-sky-300 px-4 py-2 text-sm font-semibold text-sky-800 hover:bg-sky-50"
            onClick={publishLegacyOndemandSample}
          >
            Quick Publish IMEI/ondemand
          </button>
          <Link
            to="/telemetry"
            className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          >
            Open Monitor
          </Link>
          <Link
            to="/telemetry/v2"
            className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
          >
            Open Monitor V2
          </Link>
        </div>
        <div className="mt-6 flex flex-wrap items-center gap-3">
          <label className="flex items-center gap-2 text-sm text-slate-700">
            <span>Stream interval (seconds)</span>
            <input
              type="number"
              min={1}
              max={60}
              value={streamIntervalSeconds}
              onChange={(event) => setStreamIntervalSeconds(Number(event.target.value) || 5)}
              className="w-20 rounded-md border border-slate-300 px-2 py-1 text-sm text-slate-900 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
            />
          </label>
          <button
            type="button"
            className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
            onClick={toggleTelemetryStream}
            disabled={mqttStatus !== 'connected'}
          >
            {streaming ? 'Stop Stream' : 'Start Stream'}
          </button>
          <button
            type="button"
            className="rounded-md border border-emerald-200 px-4 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50"
            onClick={handleManualAck}
            disabled={!lastCommand}
          >
            Acknowledge Last Command
          </button>
          {lastCommand && (
            <span className="text-xs text-slate-500">Last msgid: {lastCommand.msgid}</span>
          )}
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-slate-900">Activity Log</h3>
          <button
            type="button"
            className="rounded-md border border-slate-300 px-3 py-1 text-xs font-semibold text-slate-600 hover:bg-slate-100"
            onClick={() => setLogEntries([])}
          >
            Clear Log
          </button>
        </div>
        {logEntries.length === 0 ? (
          <p className="mt-4 text-sm text-slate-500">No simulator activity captured yet.</p>
        ) : (
          <ul className="mt-4 space-y-3">
            {logEntries.map((entry) => (
              <li
                key={entry.id}
                className="rounded-md border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700"
              >
                <div className="flex flex-wrap justify-between gap-2 text-xs uppercase tracking-wide text-slate-500">
                  <span>{entry.direction}</span>
                  <span>{new Date(entry.timestamp).toLocaleString()}</span>
                  {entry.topic && <span>{entry.topic}</span>}
                </div>
                <div className="mt-2 font-medium text-slate-800">{entry.message}</div>
                {entry.payload && (
                  <pre className="mt-2 max-h-60 overflow-x-auto whitespace-pre-wrap text-xs leading-relaxed text-slate-800">
                    {JSON.stringify(entry.payload, null, 2)}
                  </pre>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );

  function buildOndemandResponse(msgid: string, toggleValue: number) {
    return {
      msgid,
      status: toggleValue === 1 ? 'Pump ON' : 'Pump OFF',
      receivedAt: new Date().toISOString(),
      payload: {
        DO1: toggleValue,
        PRUNST1: toggleValue === 1 ? '1' : '0',
      },
    };
  }
}

type DetailProps = {
  label: string;
  value: string;
};

function Detail({ label, value }: DetailProps) {
  return (
    <div className="rounded-md border border-slate-200 bg-white p-3 text-sm text-slate-700">
      <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div>
      <div className="mt-1 break-all font-medium text-slate-900">{value}</div>
    </div>
  );
}

type CredentialBlockProps = {
  title: string;
  bundle: DeviceCredentialBundle | GovernmentCredentialBundle;
};

function CredentialBlock({ title, bundle }: CredentialBlockProps) {
  return (
    <article className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">
      <h4 className="text-sm font-semibold text-emerald-800">{title}</h4>
      <div className="mt-3 grid gap-3 md:grid-cols-2">
        <div>
          <span className="text-xs font-semibold uppercase tracking-wide text-emerald-700">
            Client ID
          </span>
          <div className="mt-1 break-all text-sm">{bundle.clientId}</div>
        </div>
        <div>
          <span className="text-xs font-semibold uppercase tracking-wide text-emerald-700">
            Username
          </span>
          <div className="mt-1 break-all text-sm">{bundle.username}</div>
        </div>
        <div>
          <span className="text-xs font-semibold uppercase tracking-wide text-emerald-700">
            Password
          </span>
          <div className="mt-1 break-all text-sm">{bundle.password}</div>
        </div>
        <div>
          <span className="text-xs font-semibold uppercase tracking-wide text-emerald-700">
            Publish Topics
          </span>
          <ul className="mt-1 space-y-1 text-xs">
            {bundle.topics.publish.map((topic) => (
              <li key={topic}>{topic}</li>
            ))}
          </ul>
        </div>
        <div>
          <span className="text-xs font-semibold uppercase tracking-wide text-emerald-700">
            Subscribe Topics
          </span>
          <ul className="mt-1 space-y-1 text-xs">
            {bundle.topics.subscribe.map((topic) => (
              <li key={topic}>{topic}</li>
            ))}
          </ul>
        </div>
        <div>
          <span className="text-xs font-semibold uppercase tracking-wide text-emerald-700">
            Endpoints
          </span>
          <ul className="mt-1 space-y-1 text-xs">
            {bundle.endpoints.map((endpoint) => (
              <li key={`${endpoint.protocol}-${endpoint.host}-${endpoint.port}`}>
                {endpoint.protocol}://{endpoint.host}:{endpoint.port}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </article>
  );
}

function extractTopicSuffix(topic: string | undefined): string | undefined {
  if (!topic) {
    return undefined;
  }
  const parts = topic.split('/');
  return parts[parts.length - 1]?.toLowerCase();
}

function generateRandomImei() {
  const base = `869${Math.floor(Math.random() * 1e12)
    .toString()
    .padStart(12, '0')}`;
  return base.slice(0, 15);
}

function isoDateParts() {
  const now = new Date();
  const timestamp = now.toISOString().replace('T', ' ').substring(0, 19);
  const date = `${now.getUTCFullYear().toString().slice(-2)}${String(now.getUTCMonth() + 1).padStart(2, '0')}`;
  return { timestamp, date };
}

function jitter(value: string, min: number, max: number, precision: number = 1) {
  const numeric = Number.parseFloat(value);
  if (!Number.isFinite(numeric)) {
    return value;
  }
  const delta = min + Math.random() * (max - min);
  return (numeric + delta).toFixed(precision);
}

export type SimulatorTelemetryRuntime = {
  lastUpdateMs: number | null;
  lastDateKey: string | null;
  dailyEnergyKwh: number;
  totalEnergyKwh: number;
  dailyWaterKl: number;
  totalWaterKl: number;
  dailyRunHours: number;
  totalRunHours: number;
  flowRateLpm: number;
  irradiance: number;
  waterLevelMeters: number;
  faultCode: number;
  resetRequired: boolean;
};

export function createSimulatorTelemetryRuntime(): SimulatorTelemetryRuntime {
  return {
    lastUpdateMs: null,
    lastDateKey: null,
    dailyEnergyKwh: 0,
    totalEnergyKwh: 1245,
    dailyWaterKl: 0,
    totalWaterKl: 460,
    dailyRunHours: 0,
    totalRunHours: 812,
    flowRateLpm: 0,
    irradiance: 640,
    waterLevelMeters: 58,
    faultCode: 0,
    resetRequired: false,
  } satisfies SimulatorTelemetryRuntime;
}

function computeDeltaSeconds(state: SimulatorTelemetryRuntime): number {
  const now = Date.now();
  if (state.lastUpdateMs === null) {
    state.lastUpdateMs = now;
    return 5;
  }
  const deltaSeconds = Math.max(3, (now - state.lastUpdateMs) / 1000);
  state.lastUpdateMs = now;
  return deltaSeconds;
}

function ensureDailyBucket(state: SimulatorTelemetryRuntime, nowDateKey: string) {
  if (state.lastDateKey === nowDateKey) {
    return;
  }
  state.lastDateKey = nowDateKey;
  state.dailyEnergyKwh = 0;
  state.dailyWaterKl = 0;
  state.dailyRunHours = 0;
}

function clampNumber(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function randomBetween(min: number, max: number, precision = 0) {
  const value = min + Math.random() * (max - min);
  return Number(value.toFixed(precision));
}

const HEARTBEAT_TEMPLATE = {
  VD: '0',
  TIMESTAMP: '',
  DATE: '',
  IMEI: '',
  ASN: '',
  RTCDATE: '',
  RTCTIME: '',
  LAT: '18.5204',
  LONG: '73.8567',
  RSSI: '-70',
  STINTERVAL: '5',
  POTP: '12345678',
  COTP: '87654321',
  GSM: '1',
  SIM: '1',
  NET: '1',
  GPRS: '1',
  SD: '1',
  ONLINE: '1',
  GPS: '1',
  GPSLOC: '1',
  RF: '1',
  TEMP: '25',
  SIMSLOT: '1',
  SIMCHNGCNT: '0',
  FLASH: '1',
  BATTST: '1',
  VBATT: 12.5,
  PST: 1,
};

export function buildHeartbeatPayload(imei: string, asn: string) {
  const { timestamp, date } = isoDateParts();
  const rtcDate = new Date().toISOString().substring(0, 10).replace(/-/g, '');
  const rtcTime = new Date().toTimeString().substring(0, 8);
  return {
    ...HEARTBEAT_TEMPLATE,
    TIMESTAMP: timestamp,
    DATE: date,
    IMEI: imei,
    ASN: asn,
    RTCDATE: rtcDate,
    RTCTIME: rtcTime,
    RSSI: Math.round(-55 - Math.random() * 25).toString(),
    TEMP: Math.round(20 + Math.random() * 10).toString(),
    VBATT: Number(jitter('12.5', -0.5, 0.5, 1)),
  };
}

const PUMP_TEMPLATE = {
  VD: '1',
  TIMESTAMP: '',
  DATE: '',
  IMEI: '',
  ASN: '',
  PDKWH1: '15.2',
  PTOTKWH1: '1234.5',
  PTDAYE1: '15.2',
  POPDWD1: '2500',
  POPTOTWD1: '125000',
  PDHR1: '6.5',
  PTOTHR1: '8760',
  POPKW1: '2.5',
  MAXINDEX: '95',
  INDEX: '3',
  LOAD: '0',
  STINTERVAL: '2',
  POTP: '12345678',
  COTP: '87654321',
  PMAXFREQ1: '60',
  PFREQLSP1: '45',
  PFREQHSP1: '55',
  PCNTRMODE1: '1',
  PRUNST1: '1',
  POPFREQ1: '50',
  POPI1: '5.2',
  POPV1: 380,
  PDC1V1: 350,
  PDC1I1: '3.5',
  PDCVOC1: '400',
  POPFLW1: '150',
  PFLWRT1: '150',
  PTDAYW1: '2.5',
  PTWTLV1: '40.0',
};

export function buildPumpPayload(imei: string, asn: string, runtime?: SimulatorTelemetryRuntime) {
  const { timestamp, date } = isoDateParts();
  const state = runtime ?? createSimulatorTelemetryRuntime();
  const deltaSeconds = computeDeltaSeconds(state);
  ensureDailyBucket(state, new Date().toISOString().slice(0, 10));

  const running = Math.random() > 0.3;
  const pumpPowerKw = running ? randomBetween(1.4, 3.4, 2) : 0;
  const outputFrequency = running ? randomBetween(45, 52, 1) : 0;
  const outputCurrent = running ? randomBetween(3.2, 6.3, 1) : 0;
  const outputVoltage = running ? Math.round(randomBetween(360, 420)) : 0;
  const dcVoltage = running ? Math.round(randomBetween(480, 620)) : 0;
  const dcCurrent = running ? randomBetween(4.0, 7.2, 1) : 0;
  const dcVoc = running ? dcVoltage + Math.round(randomBetween(35, 80)) : 0;
  const flowRate = running ? randomBetween(110, 240) : 0;
  const deltaHours = deltaSeconds / 3600;
  const deltaMinutes = deltaSeconds / 60;
  const deltaWaterKl = (flowRate * deltaMinutes) / 1000;

  if (running) {
    state.dailyEnergyKwh += pumpPowerKw * deltaHours;
    state.totalEnergyKwh += pumpPowerKw * deltaHours;
    state.dailyRunHours += deltaHours;
    state.totalRunHours += deltaHours;
    state.dailyWaterKl += deltaWaterKl;
    state.totalWaterKl += deltaWaterKl;
    state.flowRateLpm = flowRate;
    state.waterLevelMeters = clampNumber(state.waterLevelMeters - deltaWaterKl * 1.5, 2, 100);
  } else {
    state.flowRateLpm = 0;
    state.waterLevelMeters = clampNumber(state.waterLevelMeters + deltaMinutes / 90, 2, 100);
  }

  state.irradiance = clampNumber(
    state.irradiance + randomBetween(running ? -10 : -35, running ? 30 : 25),
    200,
    1000,
  );

  return {
    ...PUMP_TEMPLATE,
    TIMESTAMP: timestamp,
    DATE: date,
    IMEI: imei,
    ASN: asn,
    PDKWH1: state.dailyEnergyKwh.toFixed(2),
    PTDAYE1: state.dailyEnergyKwh.toFixed(2),
    PTOTKWH1: state.totalEnergyKwh.toFixed(1),
    POPDWD1: Math.round(state.dailyWaterKl * 1000).toString(),
    POPTOTWD1: Math.round(state.totalWaterKl * 1000).toString(),
    PDHR1: state.dailyRunHours.toFixed(2),
    PTOTHR1: state.totalRunHours.toFixed(1),
    POPKW1: running ? pumpPowerKw.toFixed(2) : '0',
    PRUNST1: running ? '1' : '0',
    POPFREQ1: running ? outputFrequency.toFixed(1) : '0',
    POPI1: running ? outputCurrent.toFixed(1) : '0',
    POPV1: outputVoltage,
    PDC1V1: dcVoltage,
    PDC1I1: running ? dcCurrent.toFixed(1) : '0',
    PDCVOC1: running ? dcVoc.toString() : '0',
    POPFLW1: running ? Math.round(flowRate).toString() : '0',
    PFLWRT1: Math.max(0, Math.round(state.flowRateLpm)).toString(),
    PTDAYW1: state.dailyWaterKl.toFixed(2),
    PTWTLV1: state.waterLevelMeters.toFixed(1),
  };
}

export function buildDataPayload(imei: string, asn: string, runtime?: SimulatorTelemetryRuntime) {
  const { timestamp, date } = isoDateParts();
  const state = runtime ?? createSimulatorTelemetryRuntime();
  return {
    VD: '5',
    TIMESTAMP: timestamp,
    DATE: date,
    IMEI: imei,
    ASN: asn,
    PFLWRT1: Math.max(0, Math.round(state.flowRateLpm)).toString(),
    PTDAYW1: state.dailyWaterKl.toFixed(2),
    PTWTLV1: state.waterLevelMeters.toFixed(1),
    PSOLIR1: Math.round(state.irradiance).toString(),
    PTDAYE1: state.dailyEnergyKwh.toFixed(2),
  };
}

const DAQ_TEMPLATE = {
  VD: '12',
  TIMESTAMP: '',
  DATE: '',
  IMEI: '',
  ASN: '',
  MAXINDEX: '98',
  INDEX: '5',
  LOAD: '0',
  STINTERVAL: '2',
  MSGID: '123',
  POTP: '12345678',
  COTP: '87654321',
  AI11: '3.2',
  AI21: '1.8',
  AI31: '4.1',
  AI41: '2.7',
  DI11: '1',
  DI21: '0',
  DI31: '1',
  DI41: '0',
  DO11: '1',
  DO21: '0',
  DO31: '0',
  DO41: '0',
  PDCBUS1: '720',
  PFAULT1: '0',
  PRESET1: '0',
};

export function buildDaqPayload(imei: string, asn: string, runtime?: SimulatorTelemetryRuntime) {
  const { timestamp, date } = isoDateParts();
  const state = runtime ?? createSimulatorTelemetryRuntime();

  if (state.faultCode !== 0) {
    if (Math.random() < 0.35) {
      state.faultCode = 0;
      state.resetRequired = false;
    }
  } else if (Math.random() < 0.08) {
    state.faultCode = Math.random() < 0.5 ? 201 : 412;
    state.resetRequired = true;
  }

  return {
    ...DAQ_TEMPLATE,
    TIMESTAMP: timestamp,
    DATE: date,
    IMEI: imei,
    ASN: asn,
    AI11: jitter('3', -1.5, 1.5, 1),
    AI21: jitter('2', -1.5, 1.5, 1),
    AI31: jitter('4', -1.5, 1.5, 1),
    AI41: jitter('2.5', -1.5, 1.5, 1),
    DI11: Math.random() > 0.5 ? '1' : '0',
    DI21: Math.random() > 0.5 ? '1' : '0',
    DI31: Math.random() > 0.5 ? '1' : '0',
    DI41: Math.random() > 0.5 ? '1' : '0',
    PDCBUS1: Math.round(randomBetween(680, 780)).toString(),
    PFAULT1: state.faultCode.toString(),
    PRESET1: state.resetRequired ? '1' : '0',
  };
}
