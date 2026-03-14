import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useAuth } from '../auth';
import { useActiveProject } from '../activeProject';
import { ackAlert, fetchAlerts, type AlertRecord } from '../api/alerts';
import {
  createRule,
  deleteRule,
  fetchRules,
  type RuleAction,
  type RuleRecord,
  type RuleTrigger,
} from '../api/rules';

type TabKey = 'rules' | 'alerts';

type RuleDraft = {
  projectId: string;
  deviceId: string;
  name: string;
  triggerFormula: string;
  actionsJson: string;
};

const DEFAULT_ACTIONS_JSON = JSON.stringify(
  [{ type: 'ALERT', message: 'Rule triggered' }],
  null,
  2,
);

function toId(value: unknown): string | null {
  if (typeof value === 'string' && value.trim()) {
    return value.trim();
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value);
  }
  return null;
}

function describeJson(value: unknown): string {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function resolveRuleId(rule: RuleRecord): string | null {
  return toId(rule.id ?? (rule as { ID?: unknown }).ID);
}

function resolveAlertId(alert: AlertRecord): string | null {
  return toId(alert.id ?? (alert as { ID?: unknown }).ID);
}

export function RulesAlertsPage() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const { activeProjectId: globalProjectId } = useActiveProject();
  const canManageAlerts = hasCapability(['alerts:manage', 'admin:all'], { match: 'any' });

  const [activeTab, setActiveTab] = useState<TabKey>('rules');
  const [filters, setFilters] = useState(() => ({
    projectIdInput: globalProjectId ?? '',
    deviceIdInput: '',
    status: 'all',
  }));
  const [activeProjectId, setActiveProjectId] = useState<string>('');
  const [activeDeviceId, setActiveDeviceId] = useState<string>('');
  const [ruleDraft, setRuleDraft] = useState<RuleDraft>(() => ({
    projectId: globalProjectId ?? '',
    deviceId: '',
    name: '',
    triggerFormula: '',
    actionsJson: DEFAULT_ACTIONS_JSON,
  }));
  const [ruleError, setRuleError] = useState<string | null>(null);

  useEffect(() => {
    if (!globalProjectId) {
      return;
    }

    setFilters((prev) =>
      prev.projectIdInput.trim() ? prev : { ...prev, projectIdInput: globalProjectId },
    );

    setRuleDraft((prev) =>
      prev.projectId.trim() ? prev : { ...prev, projectId: globalProjectId },
    );
  }, [globalProjectId]);

  const handleApplyFilters = () => {
    const projectId = filters.projectIdInput.trim();
    const deviceId = filters.deviceIdInput.trim();
    setActiveProjectId(projectId);
    setActiveDeviceId(deviceId);

    setRuleDraft((prev) => ({
      ...prev,
      projectId,
      deviceId,
    }));
  };

  const rulesQuery = useQuery<RuleRecord[], Error>({
    queryKey: ['rules', activeProjectId, activeDeviceId],
    queryFn: () => fetchRules({ projectId: activeProjectId, deviceId: activeDeviceId || undefined }),
    enabled: canManageAlerts && Boolean(activeProjectId),
    refetchOnWindowFocus: false,
  });

  const alertsQuery = useQuery<AlertRecord[], Error>({
    queryKey: ['alerts', activeProjectId, filters.status],
    queryFn: () =>
      fetchAlerts({
        projectId: activeProjectId || undefined,
        status: filters.status === 'all' ? undefined : filters.status,
      }),
    enabled: canManageAlerts && Boolean(activeProjectId),
    refetchOnWindowFocus: false,
  });

  const createRuleMutation = useMutation({
    mutationFn: createRule,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['rules'] });
    },
  });

  const deleteRuleMutation = useMutation({
    mutationFn: deleteRule,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['rules'] });
    },
  });

  const ackAlertMutation = useMutation({
    mutationFn: ackAlert,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['alerts'] });
    },
  });

  const rules = rulesQuery.data ?? [];
  const alerts = alertsQuery.data ?? [];

  const submitRule = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!canManageAlerts) {
      setRuleError('Managing rules requires alerts:manage capability.');
      return;
    }

    const projectId = ruleDraft.projectId.trim();
    if (!projectId) {
      setRuleError('Project ID is required to create a rule.');
      return;
    }

    const name = ruleDraft.name.trim();
    if (!name) {
      setRuleError('Rule name is required.');
      return;
    }

    const formula = ruleDraft.triggerFormula.trim();
    if (!formula) {
      setRuleError('Trigger formula is required.');
      return;
    }

    let actions: RuleAction[] = [];
    try {
      const parsed = JSON.parse(ruleDraft.actionsJson) as unknown;
      if (!Array.isArray(parsed)) {
        setRuleError('Actions JSON must be an array.');
        return;
      }
      actions = parsed as RuleAction[];
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Invalid Actions JSON';
      setRuleError(message);
      return;
    }

    const trigger: RuleTrigger = { formula };

    setRuleError(null);
    await createRuleMutation.mutateAsync({
      projectId,
      deviceId: ruleDraft.deviceId.trim() || undefined,
      name,
      trigger,
      actions,
    });

    setRuleDraft((prev) => ({
      ...prev,
      name: '',
      triggerFormula: '',
      actionsJson: DEFAULT_ACTIONS_JSON,
    }));
  };

  const rulesError = rulesQuery.error?.message ?? null;
  const alertsError = alertsQuery.error?.message ?? null;

  const activeContext = useMemo(() => {
    if (!activeProjectId) {
      return 'No project loaded.';
    }
    if (activeDeviceId) {
      return `Loaded project: ${activeProjectId} • Device: ${activeDeviceId}`;
    }
    return `Loaded project: ${activeProjectId}`;
  }, [activeDeviceId, activeProjectId]);

  const loadHint = useMemo(() => {
    if (!canManageAlerts) {
      return 'Requires alerts:manage capability.';
    }
    return 'Enter a project ID and press Load to view rules and alerts.';
  }, [canManageAlerts]);

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold text-slate-900">Rules / Alerts</h1>
        <p className="text-sm text-slate-600">{loadHint}</p>
        <p className="text-xs text-slate-500" aria-label="Active context">
          {activeContext}
        </p>
      </header>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="grid gap-4 md:grid-cols-4">
          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Project ID</span>
            <input
              value={filters.projectIdInput}
              onChange={(e) => setFilters((prev) => ({ ...prev, projectIdInput: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. rms-pump-01"
              aria-label="Project ID (filters)"
              disabled={!canManageAlerts}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Device ID (optional)</span>
            <input
              value={filters.deviceIdInput}
              onChange={(e) => setFilters((prev) => ({ ...prev, deviceIdInput: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="device uuid (optional)"
              aria-label="Device ID (filters)"
              disabled={!canManageAlerts}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Alert status</span>
            <select
              value={filters.status}
              onChange={(e) => setFilters((prev) => ({ ...prev, status: e.target.value }))}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              aria-label="Alert status"
              disabled={!canManageAlerts}
            >
              <option value="all">All</option>
              <option value="active">Open</option>
              <option value="acknowledged">Acknowledged</option>
            </select>
          </label>

          <div className="flex items-end">
            <button
              type="button"
              onClick={handleApplyFilters}
              className="inline-flex w-full items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canManageAlerts}
            >
              Load
            </button>
          </div>
        </div>
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => setActiveTab('rules')}
            className={`rounded-full px-4 py-1 text-sm font-semibold transition-colors ${
              activeTab === 'rules'
                ? 'bg-emerald-600 text-white'
                : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
            Rules
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('alerts')}
            className={`rounded-full px-4 py-1 text-sm font-semibold transition-colors ${
              activeTab === 'alerts'
                ? 'bg-emerald-600 text-white'
                : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
            Alerts
          </button>
        </div>

        {activeTab === 'rules' ? (
          <div className="mt-6 space-y-6">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-slate-900">Create rule</h2>
              <p className="text-sm text-slate-600">
                Rules require a project ID. This page uses trigger.formula and a JSON actions array.
              </p>
              {!activeProjectId ? (
                <p className="text-xs text-slate-500">
                  Tip: Use the Load filters above to set the active project (and optionally device).
                </p>
              ) : null}
            </div>

            <form className="space-y-4" onSubmit={submitRule}>
              <div className="grid gap-4 md:grid-cols-2">
                <label className="flex flex-col gap-2 text-sm">
                  <span className="font-medium text-slate-800">Project ID</span>
                  <input
                    value={ruleDraft.projectId}
                    onChange={(e) => setRuleDraft((prev) => ({ ...prev, projectId: e.target.value }))}
                    className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                    placeholder="e.g. rms-pump-01"
                    disabled={!canManageAlerts}
                  />
                </label>

                <label className="flex flex-col gap-2 text-sm">
                  <span className="font-medium text-slate-800">Device ID (optional)</span>
                  <input
                    value={ruleDraft.deviceId}
                    onChange={(e) => setRuleDraft((prev) => ({ ...prev, deviceId: e.target.value }))}
                    className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                    placeholder="device uuid (optional)"
                    disabled={!canManageAlerts}
                  />
                </label>

                <label className="flex flex-col gap-2 text-sm md:col-span-2">
                  <span className="font-medium text-slate-800">Rule name</span>
                  <input
                    value={ruleDraft.name}
                    onChange={(e) => setRuleDraft((prev) => ({ ...prev, name: e.target.value }))}
                    className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                    placeholder="e.g. Pump offline alert"
                    disabled={!canManageAlerts}
                  />
                </label>

                <label className="flex flex-col gap-2 text-sm md:col-span-2">
                  <span className="font-medium text-slate-800">Trigger formula</span>
                  <input
                    value={ruleDraft.triggerFormula}
                    onChange={(e) =>
                      setRuleDraft((prev) => ({ ...prev, triggerFormula: e.target.value }))
                    }
                    className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                    placeholder="e.g. telemetry.POPFREQ1 > 55"
                    disabled={!canManageAlerts}
                  />
                </label>

                <label className="flex flex-col gap-2 text-sm md:col-span-2">
                  <span className="font-medium text-slate-800">Actions JSON</span>
                  <textarea
                    value={ruleDraft.actionsJson}
                    onChange={(e) =>
                      setRuleDraft((prev) => ({ ...prev, actionsJson: e.target.value }))
                    }
                    rows={6}
                    className="rounded border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
                    disabled={!canManageAlerts}
                  />
                </label>
              </div>

              {ruleError ? (
                <p className="text-sm text-rose-600" role="alert">
                  {ruleError}
                </p>
              ) : null}

              {createRuleMutation.isError ? (
                <p className="text-sm text-rose-600" role="alert">
                  {createRuleMutation.error instanceof Error
                    ? createRuleMutation.error.message
                    : 'Unable to create rule'}
                </p>
              ) : null}

              <button
                type="submit"
                className="inline-flex items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={!canManageAlerts || createRuleMutation.isPending}
              >
                {createRuleMutation.isPending ? 'Creating…' : 'Create rule'}
              </button>
            </form>

            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3">
                <h2 className="text-lg font-semibold text-slate-900">Rules</h2>
                <button
                  type="button"
                  onClick={() => rulesQuery.refetch()}
                  className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
                  disabled={!rulesQuery.isFetched || rulesQuery.isFetching}
                >
                  {rulesQuery.isFetching ? 'Refreshing…' : 'Refresh'}
                </button>
              </div>

              {rulesError ? (
                <p className="text-sm text-rose-600" role="alert">
                  {rulesError}
                </p>
              ) : null}

              {!activeProjectId ? (
                <p className="text-xs text-slate-500">Load a project to view rules.</p>
              ) : rulesQuery.isFetching && !rulesQuery.isFetched ? (
                <p className="text-xs text-slate-500">Loading rules…</p>
              ) : (
                <p className="text-xs text-slate-500">Loaded: {rules.length} rule(s).</p>
              )}

              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-slate-200 text-sm">
                  <thead className="bg-slate-100 text-slate-700">
                    <tr>
                      <th className="px-3 py-2 text-left text-xs font-semibold">ID</th>
                      <th className="px-3 py-2 text-left text-xs font-semibold">Name</th>
                      <th className="px-3 py-2 text-left text-xs font-semibold">Device</th>
                      <th className="px-3 py-2 text-left text-xs font-semibold">Trigger</th>
                      <th className="px-3 py-2 text-left text-xs font-semibold">Actions</th>
                      <th className="px-3 py-2 text-right text-xs font-semibold">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200">
                    {rules.map((rule, idx) => {
                      const id = resolveRuleId(rule);
                      return (
                        <tr key={id ?? `rule-${idx}`}> 
                          <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">
                            {id ?? '—'}
                          </td>
                          <td className="px-3 py-2">{String(rule.name ?? '—')}</td>
                          <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">
                            {String(rule.deviceId ?? rule.device_id ?? '—')}
                          </td>
                          <td className="px-3 py-2 text-xs text-slate-700">
                            {rule.trigger ? describeJson(rule.trigger) : '—'}
                          </td>
                          <td className="px-3 py-2 text-xs text-slate-700">
                            {rule.actions ? describeJson(rule.actions) : '—'}
                          </td>
                          <td className="px-3 py-2 text-right">
                            <button
                              type="button"
                              onClick={() => {
                                setRuleError(null);
                                if (!id) {
                                  setRuleError('Cannot delete rule: id missing from record.');
                                  return;
                                }
                                void deleteRuleMutation.mutateAsync(id);
                              }}
                              className="rounded border border-rose-200 bg-rose-50 px-3 py-1 text-xs font-medium text-rose-700 hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-60"
                              disabled={!canManageAlerts || !id || deleteRuleMutation.isPending}
                            >
                              {deleteRuleMutation.isPending ? 'Deleting…' : 'Delete'}
                            </button>
                          </td>
                        </tr>
                      );
                    })}
                    {!rules.length ? (
                      <tr>
                        <td className="px-3 py-3 text-sm text-slate-500" colSpan={6}>
                          {activeProjectId ? 'No rules found for the current filters.' : 'No rules loaded.'}
                        </td>
                      </tr>
                    ) : null}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        ) : null}

        {activeTab === 'alerts' ? (
          <div className="mt-6 space-y-3">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-lg font-semibold text-slate-900">Alerts</h2>
              <button
                type="button"
                onClick={() => alertsQuery.refetch()}
                className="rounded border border-slate-300 px-3 py-1 text-xs font-semibold uppercase tracking-wide text-slate-600 hover:bg-slate-100 disabled:opacity-60"
                disabled={!alertsQuery.isFetched || alertsQuery.isFetching}
              >
                {alertsQuery.isFetching ? 'Refreshing…' : 'Refresh'}
              </button>
            </div>

            {alertsError ? (
              <p className="text-sm text-rose-600" role="alert">
                {alertsError}
              </p>
            ) : null}

            {!activeProjectId ? (
              <p className="text-xs text-slate-500">Load a project to view alerts.</p>
            ) : alertsQuery.isFetching && !alertsQuery.isFetched ? (
              <p className="text-xs text-slate-500">Loading alerts…</p>
            ) : (
              <p className="text-xs text-slate-500">Loaded: {alerts.length} alert(s).</p>
            )}

            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-slate-200 text-sm">
                <thead className="bg-slate-100 text-slate-700">
                  <tr>
                    <th className="px-3 py-2 text-left text-xs font-semibold">ID</th>
                    <th className="px-3 py-2 text-left text-xs font-semibold">Status</th>
                    <th className="px-3 py-2 text-left text-xs font-semibold">Device</th>
                    <th className="px-3 py-2 text-left text-xs font-semibold">Message</th>
                    <th className="px-3 py-2 text-left text-xs font-semibold">Created</th>
                    <th className="px-3 py-2 text-right text-xs font-semibold">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200">
                  {alerts.map((alert, idx) => {
                    const id = resolveAlertId(alert);
                    const status = String(alert.status ?? '—');
                    const canAck = id && status.toLowerCase() !== 'acknowledged';
                    const created =
                      alert.triggeredAt ??
                      alert.triggered_at ??
                      alert.createdAt ??
                      alert.created_at ??
                      '—';
                    return (
                      <tr key={id ?? `alert-${idx}`}>
                        <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">
                          {id ?? '—'}
                        </td>
                        <td className="px-3 py-2">{status}</td>
                        <td className="px-3 py-2 font-mono text-[0.7rem] text-slate-600">
                          {String(alert.deviceId ?? alert.device_id ?? '—')}
                        </td>
                        <td className="px-3 py-2 text-xs text-slate-700">
                          {String(alert.message ?? alert.title ?? '—')}
                        </td>
                        <td className="px-3 py-2 text-xs text-slate-700">
                          {String(created)}
                        </td>
                        <td className="px-3 py-2 text-right">
                          <button
                            type="button"
                            onClick={() => {
                              if (!id) {
                                return;
                              }
                              void ackAlertMutation.mutateAsync(id);
                            }}
                            className="rounded border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-800 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-60"
                            disabled={!canManageAlerts || !canAck || ackAlertMutation.isPending}
                          >
                            {ackAlertMutation.isPending ? 'Acknowledging…' : 'Acknowledge'}
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                  {!alerts.length ? (
                    <tr>
                      <td className="px-3 py-3 text-sm text-slate-500" colSpan={6}>
                        {activeProjectId ? 'No alerts found for the current filters.' : 'No alerts loaded.'}
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </div>

            {ackAlertMutation.isError ? (
              <p className="text-sm text-rose-600" role="alert">
                {ackAlertMutation.error instanceof Error
                  ? ackAlertMutation.error.message
                  : 'Unable to acknowledge alert'}
              </p>
            ) : null}
          </div>
        ) : null}
      </section>
    </div>
  );
}
