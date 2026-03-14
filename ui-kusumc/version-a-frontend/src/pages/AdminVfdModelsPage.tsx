import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  createVfdModel,
  fetchVfdModels,
  type CreateVfdModelPayload,
  type VfdModel,
} from '../api/vfd';
import { formatDateTimeShort } from '../utils/datetime';

function Rs485Summary({ model }: { model: VfdModel }) {
  const rs = model.rs485;
  return (
    <div className="text-xs text-slate-600">
      <div>
        Baud {rs.baudRate} • {rs.dataBits}
        {rs.parity ? ` ${rs.parity}` : ''} • stop {rs.stopBits} • flow {rs.flowControl}
      </div>
    </div>
  );
}

export function AdminVfdModelsPage() {
  const [selected, setSelected] = useState<VfdModel | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const { data, isFetching, isError, error, refetch } = useQuery({
    queryKey: ['vfd-models-admin'],
    queryFn: () => fetchVfdModels(),
  });
  const queryClient = useQueryClient();

  const createMutation = useMutation({
    mutationFn: (payload: CreateVfdModelPayload) => createVfdModel(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vfd-models-admin'] });
      setShowCreate(false);
    },
  });

  const models = useMemo(() => data ?? [], [data]);

  function parseJson<T>(value: string, fallback: T): T {
    if (!value.trim()) return fallback;
    try {
      return JSON.parse(value) as T;
    } catch (err) {
      setFormError('Invalid JSON in command/fault/metadata fields');
      throw err;
    }
  }

  function handleCreateSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fd = new FormData(event.currentTarget);
    setFormError(null);

    const payload: CreateVfdModelPayload = {
      manufacturerId: String(fd.get('manufacturerId') ?? '').trim(),
      model: String(fd.get('model') ?? '').trim(),
      version: String(fd.get('version') ?? '').trim(),
      rs485: {
        baudRate: Number(fd.get('baudRate') ?? 0),
        dataBits: Number(fd.get('dataBits') ?? 0),
        stopBits: Number(fd.get('stopBits') ?? 0),
        parity: String(fd.get('parity') ?? '').trim(),
        flowControl: String(fd.get('flowControl') ?? '').trim(),
      },
      commandDictionary: parseJson(String(fd.get('commandDictionary') ?? '[]'), []),
      faultMap: parseJson(String(fd.get('faultMap') ?? '[]'), []),
      metadata: parseJson(String(fd.get('metadata') ?? '{}'), {}),
    };

    const protocolVersionId = String(fd.get('protocolVersionId') ?? '').trim();
    if (protocolVersionId) {
      payload.protocolVersionId = protocolVersionId;
    }

    if (!payload.manufacturerId || !payload.model || !payload.version) {
      setFormError('Manufacturer, model, and version are required.');
      return;
    }

    createMutation.mutate(payload);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-slate-900">VFD Drive Models</h2>
          <p className="text-sm text-slate-600">
            Catalog of RS485-capable VFD models with command/fault dictionaries.
          </p>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setShowCreate(true)}
            className="rounded bg-emerald-600 px-3 py-1 text-sm font-semibold text-white shadow hover:bg-emerald-500"
          >
            New model
          </button>
          <button
            type="button"
            onClick={() => refetch()}
            className="rounded border border-slate-300 px-3 py-1 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-60"
            disabled={isFetching}
          >
            {isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </div>

      {isError && (
        <div className="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          Failed to load models: {error instanceof Error ? error.message : 'Unknown error'}
        </div>
      )}

      {formError && (
        <div className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          {formError}
        </div>
      )}

      <div className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-600">
            <tr>
              <th className="px-4 py-3">Model</th>
              <th className="px-4 py-3">Manufacturer</th>
              <th className="px-4 py-3">RS485</th>
              <th className="px-4 py-3">Commands</th>
              <th className="px-4 py-3">Faults</th>
              <th className="px-4 py-3">Updated</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200">
            {models.map((model) => (
              <tr key={model.id} className="hover:bg-slate-50">
                <td className="px-4 py-3 font-semibold text-slate-900">
                  {model.model} <span className="text-slate-500">v{model.version}</span>
                </td>
                <td className="px-4 py-3 text-slate-700">{model.manufacturerName}</td>
                <td className="px-4 py-3">
                  <Rs485Summary model={model} />
                </td>
                <td className="px-4 py-3 text-slate-700">{model.commandDictionary.length}</td>
                <td className="px-4 py-3 text-slate-700">{model.faultMap.length}</td>
                <td className="px-4 py-3 text-slate-600">
                  {formatDateTimeShort(model.updatedAt)}
                </td>
                <td className="px-4 py-3 text-right">
                  <button
                    type="button"
                    className="rounded border border-slate-300 px-3 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100"
                    onClick={() => setSelected(model)}
                  >
                    View JSON
                  </button>
                </td>
              </tr>
            ))}
            {!models.length && !isFetching && (
              <tr>
                <td className="px-4 py-6 text-center text-sm text-slate-500" colSpan={7}>
                  No VFD models found. Use API import/CRUD to seed models.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {selected && (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/50 px-4">
          <div className="w-full max-w-3xl rounded-lg bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
              <div>
                <p className="text-xs uppercase tracking-wide text-slate-500">VFD model</p>
                <p className="text-sm font-semibold text-slate-900">
                  {selected.model} v{selected.version} — {selected.manufacturerName}
                </p>
              </div>
              <button
                type="button"
                className="rounded border border-slate-300 px-3 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100"
                onClick={() => setSelected(null)}
              >
                Close
              </button>
            </div>
            <div className="max-h-[75vh] overflow-auto px-4 py-3">
              <pre className="whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs text-slate-800">
                {JSON.stringify(selected, null, 2)}
              </pre>
            </div>
          </div>
        </div>
      )}

      {showCreate && (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/50 px-4">
          <div className="w-full max-w-4xl rounded-lg bg-white shadow-xl">
            <form onSubmit={handleCreateSubmit}>
              <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
                <div>
                  <p className="text-xs uppercase tracking-wide text-slate-500">Create VFD model</p>
                  <p className="text-sm font-semibold text-slate-900">
                    RS485 settings + command/fault dictionaries
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    type="button"
                    className="rounded border border-slate-300 px-3 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100"
                    onClick={() => {
                      setShowCreate(false);
                      setFormError(null);
                    }}
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="rounded bg-emerald-600 px-3 py-1 text-xs font-semibold text-white shadow hover:bg-emerald-500 disabled:opacity-70"
                    disabled={createMutation.isPending}
                  >
                    {createMutation.isPending ? 'Saving…' : 'Save model'}
                  </button>
                </div>
              </div>
              <div className="grid max-h-[75vh] grid-cols-2 gap-4 overflow-auto px-4 py-3 text-sm">
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs font-semibold uppercase text-slate-500">
                      Manufacturer ID
                    </label>
                    <input
                      name="manufacturerId"
                      className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm"
                      required
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs font-semibold uppercase text-slate-500">
                        Model
                      </label>
                      <input
                        name="model"
                        className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm"
                        required
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-semibold uppercase text-slate-500">
                        Version
                      </label>
                      <input
                        name="version"
                        className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm"
                        required
                      />
                    </div>
                  </div>
                  <div>
                    <label className="block text-xs font-semibold uppercase text-slate-500">
                      Protocol Version (optional)
                    </label>
                    <input
                      name="protocolVersionId"
                      className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm"
                      placeholder="Link to protocol version ID (optional)"
                    />
                  </div>
                  <fieldset className="rounded border border-slate-200 p-3">
                    <legend className="px-1 text-xs font-semibold uppercase text-slate-500">
                      RS485
                    </legend>
                    <div className="grid grid-cols-2 gap-2">
                      <input
                        name="baudRate"
                        type="number"
                        min={1}
                        step={1}
                        placeholder="Baud rate"
                        className="rounded border border-slate-300 px-3 py-2 text-sm"
                        defaultValue={9600}
                      />
                      <input
                        name="dataBits"
                        type="number"
                        min={5}
                        max={8}
                        step={1}
                        placeholder="Data bits"
                        className="rounded border border-slate-300 px-3 py-2 text-sm"
                        defaultValue={8}
                      />
                      <input
                        name="stopBits"
                        type="number"
                        min={1}
                        step={1}
                        placeholder="Stop bits"
                        className="rounded border border-slate-300 px-3 py-2 text-sm"
                        defaultValue={1}
                      />
                      <input
                        name="parity"
                        placeholder="Parity (NONE/EVEN/ODD)"
                        className="rounded border border-slate-300 px-3 py-2 text-sm"
                        defaultValue="NONE"
                      />
                      <input
                        name="flowControl"
                        placeholder="Flow control"
                        className="rounded border border-slate-300 px-3 py-2 text-sm"
                        defaultValue="NONE"
                      />
                    </div>
                  </fieldset>
                </div>
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs font-semibold uppercase text-slate-500">
                      Command dictionary (JSON array)
                    </label>
                    <textarea
                      name="commandDictionary"
                      className="mt-1 w-full rounded border border-slate-300 px-3 py-2 font-mono text-xs"
                      rows={6}
                      placeholder='[{"commandName":"start","address":1}]'
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold uppercase text-slate-500">
                      Fault map (JSON array)
                    </label>
                    <textarea
                      name="faultMap"
                      className="mt-1 w-full rounded border border-slate-300 px-3 py-2 font-mono text-xs"
                      rows={4}
                      placeholder='[{"faultCode":"E01","address":1,"faultName":"Overcurrent"}]'
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-semibold uppercase text-slate-500">
                      Metadata (JSON object, optional)
                    </label>
                    <textarea
                      name="metadata"
                      className="mt-1 w-full rounded border border-slate-300 px-3 py-2 font-mono text-xs"
                      rows={3}
                      placeholder='{"notes":"optional"}'
                    />
                  </div>
                </div>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
