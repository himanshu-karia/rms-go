import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import { useAuth } from '../auth';
import { useActiveProject } from '../activeProject';
import {
  getAutomationFlow,
  saveAutomationFlow,
  type AutomationBundle,
  type AutomationSaveDiagnostics,
} from '../api/automation';

function formatJson(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

function parseJsonArray(label: string, raw: string): unknown[] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Invalid JSON';
    throw new Error(`${label}: ${message}`);
  }

  if (!Array.isArray(parsed)) {
    throw new Error(`${label}: must be a JSON array`);
  }

  return parsed;
}

export function AutomationPage() {
  const { hasCapability } = useAuth();
  const { activeProjectId: globalProjectId } = useActiveProject();
  const canAccess = hasCapability(['alerts:manage', 'admin:all'], { match: 'any' });

  const [projectIdInput, setProjectIdInput] = useState('');
  const [activeProjectId, setActiveProjectId] = useState('');

  const [nodesJson, setNodesJson] = useState('[]');
  const [edgesJson, setEdgesJson] = useState('[]');
  const [compiledRulesJson, setCompiledRulesJson] = useState('[]');
  const [schemaVersion, setSchemaVersion] = useState('1.0.0');

  const [pageError, setPageError] = useState<string | null>(null);
  const [lastDiagnostics, setLastDiagnostics] = useState<AutomationSaveDiagnostics | null>(null);

  useEffect(() => {
    if (!globalProjectId) {
      return;
    }
    setProjectIdInput((prev) => (prev.trim() ? prev : globalProjectId));
  }, [globalProjectId]);

  const automationQuery = useQuery<AutomationBundle, Error>({
    queryKey: ['automation-flow', activeProjectId],
    queryFn: () => getAutomationFlow(activeProjectId),
    enabled: canAccess && Boolean(activeProjectId),
    refetchOnWindowFocus: false,
  });

  const canEditBundle = canAccess && Boolean(activeProjectId) && !automationQuery.isFetching;

  useEffect(() => {
    if (!automationQuery.data) {
      return;
    }

    setNodesJson(formatJson(automationQuery.data.nodes));
    setEdgesJson(formatJson(automationQuery.data.edges));
    setCompiledRulesJson(formatJson(automationQuery.data.compiled_rules));
    setSchemaVersion(automationQuery.data.schema_version);
  }, [automationQuery.data]);

  const saveMutation = useMutation({
    mutationFn: saveAutomationFlow,
    onSuccess: (diag) => {
      setLastDiagnostics(diag);
    },
  });

  const hint = useMemo(() => {
    if (!canAccess) {
      return 'Requires alerts:manage capability.';
    }
    return 'Load the project flow, edit nodes/edges, then save.';
  }, [canAccess]);

  const activeContext = useMemo(() => {
    if (!activeProjectId) {
      return 'No project loaded.';
    }
    return `Loaded project: ${activeProjectId}`;
  }, [activeProjectId]);

  const handleLoad = async () => {
    setPageError(null);
    setLastDiagnostics(null);

    const trimmed = projectIdInput.trim();
    if (!trimmed) {
      setPageError('Project ID is required.');
      return;
    }

    setActiveProjectId(trimmed);
  };

  const handleSave = async () => {
    setPageError(null);

    if (!canAccess) {
      setPageError('Saving automation requires alerts:manage capability.');
      return;
    }

    const trimmed = activeProjectId.trim();
    if (!trimmed) {
      setPageError('Load a project flow before saving.');
      return;
    }

    let nodes: unknown[];
    let edges: unknown[];
    let compiled_rules: unknown[];

    try {
      nodes = parseJsonArray('Nodes', nodesJson);
      edges = parseJsonArray('Edges', edgesJson);
      compiled_rules = parseJsonArray('Compiled rules', compiledRulesJson);
    } catch (error) {
      setPageError(error instanceof Error ? error.message : 'Invalid JSON');
      return;
    }

    const bundle: AutomationBundle = {
      nodes,
      edges,
      compiled_rules,
      schema_version: schemaVersion.trim() || '1.0.0',
    };

    // Normalize the editor text once JSON is valid (reduces accidental formatting errors).
    setNodesJson(formatJson(nodes));
    setEdgesJson(formatJson(edges));
    setCompiledRulesJson(formatJson(compiled_rules));
    setSchemaVersion(bundle.schema_version);

    const diagnostics = await saveMutation.mutateAsync({ projectId: trimmed, bundle });
    setLastDiagnostics(diagnostics);
  };

  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold text-slate-900">Automation</h1>
        <p className="text-sm text-slate-600">{hint}</p>
        <p className="text-xs text-slate-500" aria-label="Active context">
          {activeContext}
        </p>
      </header>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-2 text-sm md:col-span-2">
            <span className="font-medium text-slate-800">Project ID</span>
            <input
              value={projectIdInput}
              onChange={(e) => setProjectIdInput(e.target.value)}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder="e.g. rms-pump-01"
              aria-label="Project ID"
              disabled={!canAccess}
            />
          </label>

          <div className="flex items-end gap-3">
            <button
              type="button"
              onClick={() => void handleLoad()}
              className="inline-flex flex-1 items-center justify-center rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={!canAccess || automationQuery.isFetching || projectIdInput.trim() === ''}
            >
              {automationQuery.isFetching ? 'Loading…' : 'Load'}
            </button>
          </div>
        </div>

        {pageError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {pageError}
          </p>
        ) : null}

        {automationQuery.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {automationQuery.error instanceof Error
              ? automationQuery.error.message
              : 'Unable to load automation flow'}
          </p>
        ) : null}

        {activeProjectId && automationQuery.isFetching && !automationQuery.isError ? (
          <p className="mt-3 text-xs text-slate-500">Fetching automation flow…</p>
        ) : null}

        {activeProjectId && automationQuery.isFetched && !automationQuery.isError ? (
          <p className="mt-3 text-xs text-slate-500">
            {Array.isArray(automationQuery.data?.nodes) && automationQuery.data.nodes.length === 0
              ? 'No flow nodes saved yet for this project. Add nodes/edges below and press Save.'
              : 'Flow loaded. You can edit the bundle below and press Save.'}
          </p>
        ) : null}
      </section>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-slate-900">Flow bundle</h2>
          <button
            type="button"
            onClick={() => void handleSave()}
            className="rounded bg-emerald-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-emerald-700 focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={!canEditBundle || saveMutation.isPending}
          >
            {saveMutation.isPending ? 'Saving…' : 'Save'}
          </button>
        </div>

        {!activeProjectId ? (
          <p className="mt-2 text-xs text-slate-500">Load a project to edit its automation flow.</p>
        ) : null}

        <div className="mt-4 grid gap-4">
          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Schema version</span>
            <input
              value={schemaVersion}
              onChange={(e) => setSchemaVersion(e.target.value)}
              className="rounded border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              disabled={!canEditBundle}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Nodes (JSON array)</span>
            <textarea
              value={nodesJson}
              onChange={(e) => setNodesJson(e.target.value)}
              rows={10}
              className="rounded border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder='e.g. [{ "id": "t1", "type": "trigger", "data": { "type": "trigger" } }]'
              aria-label="Nodes JSON"
              disabled={!canEditBundle}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Edges (JSON array)</span>
            <textarea
              value={edgesJson}
              onChange={(e) => setEdgesJson(e.target.value)}
              rows={10}
              className="rounded border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder='e.g. [{ "source": "t1", "target": "a1" }]'
              aria-label="Edges JSON"
              disabled={!canEditBundle}
            />
          </label>

          <label className="flex flex-col gap-2 text-sm">
            <span className="font-medium text-slate-800">Compiled rules (JSON array)</span>
            <textarea
              value={compiledRulesJson}
              onChange={(e) => setCompiledRulesJson(e.target.value)}
              rows={8}
              className="rounded border border-slate-300 bg-white px-3 py-2 font-mono text-xs text-slate-700 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500"
              placeholder='e.g. [] (optional; backend can fall back to graph evaluation)'
              aria-label="Compiled rules JSON"
              disabled={!canEditBundle}
            />
          </label>
        </div>

        {saveMutation.isError ? (
          <p className="mt-3 text-sm text-rose-600" role="alert">
            {saveMutation.error instanceof Error ? saveMutation.error.message : 'Save failed'}
          </p>
        ) : null}

        {lastDiagnostics ? (
          <section
            role="region"
            aria-label="Automation save diagnostics"
            className="mt-6 rounded border border-slate-200 bg-slate-50 p-4 text-sm text-slate-800"
          >
            <p className="font-semibold">
              Save result: {lastDiagnostics.saved ? '✅ Saved' : '⚠️ Not saved'}
            </p>
            <p className="mt-1 text-xs text-slate-600">
              Project: {lastDiagnostics.project_id} • Schema: {lastDiagnostics.schema_version} •
              Compiled: {lastDiagnostics.compiled_count}
            </p>

            {lastDiagnostics.errors?.length ? (
              <div className="mt-3">
                <p className="font-medium text-rose-700">Errors</p>
                <ul className="mt-1 list-disc space-y-1 pl-5 text-xs text-rose-700">
                  {lastDiagnostics.errors.map((err) => (
                    <li key={err}>{err}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            {lastDiagnostics.warnings?.length ? (
              <div className="mt-3">
                <p className="font-medium text-amber-800">Warnings</p>
                <ul className="mt-1 list-disc space-y-1 pl-5 text-xs text-amber-800">
                  {lastDiagnostics.warnings.map((warn) => (
                    <li key={warn}>{warn}</li>
                  ))}
                </ul>
              </div>
            ) : null}

            {lastDiagnostics.issues?.length ? (
              <div className="mt-3">
                <p className="font-medium">Issues</p>
                <ul className="mt-1 space-y-1 text-xs">
                  {lastDiagnostics.issues.map((issue, idx) => (
                    <li key={`${issue.code}-${idx}`}>
                      [{issue.level}] {issue.code}: {issue.message}
                      {issue.path ? ` (${issue.path})` : ''}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
          </section>
        ) : null}
      </section>
    </div>
  );
}
