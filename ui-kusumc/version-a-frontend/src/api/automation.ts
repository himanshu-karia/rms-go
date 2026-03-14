import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type AutomationBundle = {
  nodes: unknown[];
  edges: unknown[];
  compiled_rules: unknown[];
  schema_version: string;
};

export type AutomationSaveDiagnostics = {
  saved: boolean;
  project_id: string;
  schema_version: string;
  compiled_count: number;
  errors: string[];
  warnings: string[];
  issues?: Array<{
    level: string;
    code: string;
    message: string;
    node_id?: string;
    path?: string;
  }>;
};

const DEFAULT_SCHEMA_VERSION = '1.0.0';

function normalizeBundle(flow: Partial<AutomationBundle> | null | undefined): AutomationBundle {
  return {
    nodes: Array.isArray(flow?.nodes) ? flow!.nodes : [],
    edges: Array.isArray(flow?.edges) ? flow!.edges : [],
    compiled_rules: Array.isArray(flow?.compiled_rules) ? flow!.compiled_rules : [],
    schema_version:
      typeof flow?.schema_version === 'string' && flow.schema_version.trim() !== ''
        ? flow.schema_version
        : DEFAULT_SCHEMA_VERSION,
  };
}

export async function getAutomationFlow(projectId: string): Promise<AutomationBundle> {
  const trimmed = projectId.trim();
  if (!trimmed) {
    throw new Error('projectId required');
  }

  const response = await apiFetch(`${API_BASE_URL}/config/automation/${encodeURIComponent(trimmed)}`);
  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      'Unable to load automation flow';
    throw new Error(message);
  }

  return normalizeBundle(body as Partial<AutomationBundle>);
}

export async function saveAutomationFlow(params: {
  projectId: string;
  bundle: AutomationBundle;
}): Promise<AutomationSaveDiagnostics> {
  const trimmed = params.projectId.trim();
  if (!trimmed) {
    throw new Error('projectId required');
  }

  const bundle = normalizeBundle(params.bundle);
  const payload = { project_id: trimmed, ...bundle };

  const response = await apiFetch(`${API_BASE_URL}/config/automation`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<any>(response);
  if (!response.ok || !body) {
    const message = (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      'Unable to save automation flow';
    throw new Error(message);
  }

  return body as AutomationSaveDiagnostics;
}
