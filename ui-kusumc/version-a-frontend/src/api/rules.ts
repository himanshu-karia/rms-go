import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';

export type RuleTrigger =
  | {
      formula: string;
    }
  | {
      field: string;
      operator: string;
      value: unknown;
    };

export type RuleAction = {
  type: string;
  [key: string]: unknown;
};

export type RuleRecord = {
  id?: string;
  name?: string;
  projectId?: string;
  deviceId?: string | null;
  trigger?: RuleTrigger;
  actions?: RuleAction[];
  [key: string]: unknown;
};

export type ListRulesResponse = {
  rules: RuleRecord[];
};

function normalizeRules(payload: unknown): RuleRecord[] {
  if (Array.isArray(payload)) {
    return payload as RuleRecord[];
  }

  if (payload && typeof payload === 'object') {
    const maybeRules = (payload as { rules?: unknown }).rules;
    if (Array.isArray(maybeRules)) {
      return maybeRules as RuleRecord[];
    }
  }

  return [];
}

export async function fetchRules(params: {
  projectId: string;
  deviceId?: string;
}): Promise<RuleRecord[]> {
  const query = new URLSearchParams({ projectId: params.projectId });
  if (params.deviceId) {
    query.set('deviceId', params.deviceId);
  }

  const response = await apiFetch(`${API_BASE_URL}/rules?${query.toString()}`);
  const body = await readJsonBody(response);

  if (!response.ok) {
    const message = (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      'Unable to load rules';
    throw new Error(message);
  }

  return normalizeRules(body);
}

export async function createRule(payload: {
  projectId: string;
  deviceId?: string;
  name: string;
  trigger: RuleTrigger;
  actions: RuleAction[];
}): Promise<{ id: string }> {
  const response = await apiFetch(`${API_BASE_URL}/rules`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody(response);
  if (!response.ok || !body) {
    const message = (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      'Unable to create rule';
    throw new Error(message);
  }

  const id = (body as { id?: unknown }).id;
  if (typeof id !== 'string' || !id.trim()) {
    throw new Error('Rule id missing from response');
  }

  return { id };
}

export async function deleteRule(ruleId: string): Promise<void> {
  const trimmed = ruleId.trim();
  if (!trimmed) {
    throw new Error('ruleId required');
  }

  const response = await apiFetch(`${API_BASE_URL}/rules/${encodeURIComponent(trimmed)}`, {
    method: 'DELETE',
  });

  if (!response.ok) {
    const body = await readJsonBody(response);
    const message = (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      'Unable to delete rule';
    throw new Error(message);
  }
}
