import { FormEvent, useMemo, useState } from 'react';
import {
  acknowledgeDeviceCommand,
  fetchDeviceCommandHistory,
  issueDeviceCommand,
  type AcknowledgeDeviceCommandPayload,
  type AcknowledgeDeviceCommandResponse,
  type DeviceCommandHistoryRecord,
  type FetchDeviceCommandHistoryParams,
  type FetchDeviceCommandHistoryResponse,
  type IssueDeviceCommandPayload,
  type IssueDeviceCommandResponse,
} from '../api/devices';
import { useAuth } from '../auth';

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

const defaultCommandHistoryFilters = {
  deviceUuid: '',
  limit: '25',
  statuses: {
    pending: true,
    acknowledged: true,
    failed: true,
  },
};

export function CommandCenterPage() {
  const { hasCapability } = useAuth();

  const [commandIssueForm, setCommandIssueForm] =
    useState<typeof defaultCommandIssueForm>(defaultCommandIssueForm);
  const [commandIssueResult, setCommandIssueResult] =
    useState<IssueDeviceCommandResponse | null>(null);
  const [commandIssueError, setCommandIssueError] = useState<string | null>(null);

  const [commandAckForm, setCommandAckForm] =
    useState<typeof defaultCommandAckForm>(defaultCommandAckForm);
  const [commandAckResult, setCommandAckResult] =
    useState<AcknowledgeDeviceCommandResponse | null>(null);
  const [commandAckError, setCommandAckError] = useState<string | null>(null);

  const [commandHistoryFilters, setCommandHistoryFilters] = useState(defaultCommandHistoryFilters);
  const [commandHistoryResult, setCommandHistoryResult] =
    useState<FetchDeviceCommandHistoryResponse | null>(null);
  const [commandHistoryLoading, setCommandHistoryLoading] = useState(false);
  const [commandHistoryError, setCommandHistoryError] = useState<string | null>(null);

  const [lastCommandError, setLastCommandError] = useState<string | null>(null);
  const [lastCommandLoading, setLastCommandLoading] = useState(false);

  const commandHistoryStatuses = useMemo(
    () =>
      Object.entries(commandHistoryFilters.statuses)
        .filter(([, checked]) => checked)
        .map(([status]) => status),
    [commandHistoryFilters.statuses],
  );

  const handleCommandIssueInputChange = (
    event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>,
  ) => {
    const { name, value } = event.target;
    setCommandIssueForm((prev) => ({ ...prev, [name]: value }));
  };

  const handleCommandIssueReset = () => {
    setCommandIssueForm(defaultCommandIssueForm);
    setCommandIssueResult(null);
    setCommandIssueError(null);
  };

  const handleCommandAckInputChange = (
    event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
  ) => {
    const { name, value } = event.target;
    setCommandAckForm((prev) => ({ ...prev, [name]: value }));
  };

  const handleCommandAckReset = () => {
    setCommandAckForm(defaultCommandAckForm);
    setCommandAckResult(null);
    setCommandAckError(null);
  };

  const handleCommandIssueSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setCommandIssueError(null);
    setCommandIssueResult(null);

    const deviceUuid = commandIssueForm.deviceUuid.trim();
    const commandName = commandIssueForm.commandName.trim();
    if (!deviceUuid || !commandName) {
      setCommandIssueError('Device UUID and command name are required.');
      return;
    }

    const payload: IssueDeviceCommandPayload = {
      command: { name: commandName },
    };

    if (commandIssueForm.payloadJson.trim()) {
      try {
        payload.command.payload = JSON.parse(commandIssueForm.payloadJson);
      } catch {
        setCommandIssueError('Payload must be valid JSON.');
        return;
      }
    }

    if (commandIssueForm.qos.trim()) {
      const parsed = Number(commandIssueForm.qos.trim());
      if (Number.isNaN(parsed) || parsed < 0 || parsed > 2) {
        setCommandIssueError('QoS must be 0, 1, or 2.');
        return;
      }
      payload.qos = parsed;
    }

    if (commandIssueForm.timeoutSeconds.trim()) {
      const parsed = Number(commandIssueForm.timeoutSeconds.trim());
      if (Number.isNaN(parsed) || parsed <= 0) {
        setCommandIssueError('Timeout seconds must be a positive number.');
        return;
      }
      payload.timeoutSeconds = parsed;
    }

    if (commandIssueForm.issuedBy.trim()) {
      payload.issuedBy = commandIssueForm.issuedBy.trim();
    }
    if (commandIssueForm.simulatorSessionToken.trim()) {
      payload.simulatorSessionToken = commandIssueForm.simulatorSessionToken.trim();
    }

    try {
      const result = await issueDeviceCommand(deviceUuid, payload);
      setCommandIssueResult(result);
    } catch (err) {
      setCommandIssueError(err instanceof Error ? err.message : 'Unable to issue command.');
    }
  };

  const handleCommandAckSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setCommandAckError(null);
    setCommandAckResult(null);

    const deviceUuid = commandAckForm.deviceUuid.trim();
    const msgid = commandAckForm.msgid.trim();
    if (!deviceUuid || !msgid) {
      setCommandAckError('Device UUID and message id are required.');
      return;
    }

    const payload: AcknowledgeDeviceCommandPayload = {
      msgid,
      status: commandAckForm.status,
    };

    if (commandAckForm.payloadJson.trim()) {
      try {
        payload.payload = JSON.parse(commandAckForm.payloadJson);
      } catch {
        setCommandAckError('Acknowledgement payload must be valid JSON.');
        return;
      }
    }

    if (commandAckForm.receivedAt.trim()) {
      payload.receivedAt = commandAckForm.receivedAt.trim();
    }

    try {
      const result = await acknowledgeDeviceCommand(deviceUuid, payload);
      setCommandAckResult(result);
    } catch (err) {
      setCommandAckError(
        err instanceof Error ? err.message : 'Unable to record command acknowledgement.',
      );
    }
  };

  const handleCommandHistoryInputChange = (
    event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>,
  ) => {
    const { name, value } = event.target;
    if (name === 'limit') {
      setCommandHistoryFilters((prev) => ({ ...prev, limit: value }));
      return;
    }
    if (name.startsWith('status.')) {
      const key = name.replace('status.', '') as keyof typeof defaultCommandHistoryFilters.statuses;
      setCommandHistoryFilters((prev) => ({
        ...prev,
        statuses: {
          ...prev.statuses,
          [key]: !prev.statuses[key],
        },
      }));
      return;
    }
    setCommandHistoryFilters((prev) => ({ ...prev, [name]: value }));
  };

  const loadCommandHistory = async ({
    append,
    cursor,
  }: {
    append: boolean;
    cursor?: string;
  }) => {
    const deviceUuid = commandHistoryFilters.deviceUuid.trim();
    if (!deviceUuid) {
      setCommandHistoryError('Provide a device UUID first.');
      return;
    }

    const params: FetchDeviceCommandHistoryParams = {};

    if (commandHistoryFilters.limit.trim()) {
      const parsed = Number(commandHistoryFilters.limit.trim());
      if (!Number.isNaN(parsed) && parsed > 0) {
        params.limit = parsed;
      }
    }

    const selectedStatuses = commandHistoryStatuses;
    if (selectedStatuses.length > 0 && selectedStatuses.length < 3) {
      params.statuses = selectedStatuses as Array<'pending' | 'acknowledged' | 'failed'>;
    }
    if (cursor) {
      params.cursor = cursor;
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
    } catch (err) {
      setCommandHistoryError(
        err instanceof Error ? err.message : 'Unable to load command history',
      );
    } finally {
      setCommandHistoryLoading(false);
    }
  };

  const handleCommandHistorySubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void loadCommandHistory({ append: false });
  };

  const handleCommandHistoryLoadMore = () => {
    const cursor = commandHistoryResult?.nextCursor;
    if (!cursor) {
      return;
    }
    void loadCommandHistory({ cursor, append: true });
  };

  const handleCommandHistoryReset = () => {
    setCommandHistoryFilters((prev) => ({
      ...defaultCommandHistoryFilters,
      deviceUuid: prev.deviceUuid,
    }));
    setCommandHistoryResult(null);
    setCommandHistoryError(null);
  };

  const handleLoadLastCommand = async () => {
    const candidate = commandIssueForm.deviceUuid.trim() || commandHistoryFilters.deviceUuid.trim();
    if (!candidate) {
      setLastCommandError('Provide a device UUID first.');
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

  return (
    <div className="space-y-6">
      <header className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h1 className="text-xl font-semibold text-slate-900">Command Center</h1>
        <p className="mt-1 text-sm text-slate-600">
          Issue device commands, record acknowledgements, and inspect command history.
        </p>
      </header>

      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:opacity-60"
            onClick={handleLoadLastCommand}
            disabled={lastCommandLoading}
          >
            {lastCommandLoading ? 'Loading last command.' : 'Load Last Command'}
          </button>
          {lastCommandError && <span className="text-xs font-medium text-amber-700">{lastCommandError}</span>}
        </div>

        <div className="mt-6 grid gap-6 lg:grid-cols-2">
          <div className="space-y-4">
            <form className="grid gap-4" onSubmit={handleCommandIssueSubmit}>
              <InputField
                label="Device UUID"
                name="deviceUuid"
                value={commandIssueForm.deviceUuid}
                onChange={handleCommandIssueInputChange}
                required
              />
              <InputField
                label="Command Name"
                name="commandName"
                value={commandIssueForm.commandName}
                onChange={handleCommandIssueInputChange}
                required
              />
              <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                <span>Command Payload (JSON, optional)</span>
                <textarea
                  className="h-28 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  name="payloadJson"
                  value={commandIssueForm.payloadJson}
                  onChange={handleCommandIssueInputChange}
                  placeholder='{"register": 100, "value": 1}'
                />
              </label>
              <div className="grid gap-4 md:grid-cols-2">
                <InputField
                  label="QoS (0-2)"
                  name="qos"
                  value={commandIssueForm.qos}
                  onChange={handleCommandIssueInputChange}
                  placeholder="Defaults to broker setting"
                />
                <InputField
                  label="Timeout Seconds"
                  name="timeoutSeconds"
                  value={commandIssueForm.timeoutSeconds}
                  onChange={handleCommandIssueInputChange}
                  placeholder="e.g. 30"
                />
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <InputField
                  label="Issued By (optional)"
                  name="issuedBy"
                  value={commandIssueForm.issuedBy}
                  onChange={handleCommandIssueInputChange}
                  placeholder="Operator ID"
                />
                <InputField
                  label="Simulator Session Token"
                  name="simulatorSessionToken"
                  value={commandIssueForm.simulatorSessionToken}
                  onChange={handleCommandIssueInputChange}
                  placeholder="Link simulator session"
                />
              </div>
              <div className="flex items-center gap-3">
                <button
                  type="submit"
                  className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                >
                  Issue Command
                </button>
                <button
                  type="button"
                  className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                  onClick={handleCommandIssueReset}
                >
                  Reset
                </button>
              </div>
            </form>
            {commandIssueError && (
              <div className="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {commandIssueError}
              </div>
            )}
            {commandIssueResult && (
              <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
                <p className="font-semibold">Command queued</p>
                <p className="mt-1">
                  Message {commandIssueResult.msgid} pending on topic{' '}
                  <span className="font-mono text-emerald-900">{commandIssueResult.topic}</span>.
                </p>
                {commandIssueResult.simulatorSessionId && (
                  <p className="mt-1 text-xs text-emerald-700">
                    Simulator session {commandIssueResult.simulatorSessionId}
                  </p>
                )}
              </div>
            )}
          </div>

          <div className="space-y-4">
            <form className="grid gap-4" onSubmit={handleCommandAckSubmit}>
              <InputField
                label="Device UUID"
                name="deviceUuid"
                value={commandAckForm.deviceUuid}
                onChange={handleCommandAckInputChange}
                required
              />
              <InputField
                label="Message ID"
                name="msgid"
                value={commandAckForm.msgid}
                onChange={handleCommandAckInputChange}
                required
              />
              <SelectField
                label="Status"
                name="status"
                value={commandAckForm.status}
                onChange={handleCommandAckInputChange}
                options={[
                  { value: 'acknowledged', label: 'Acknowledged' },
                  { value: 'failed', label: 'Failed' },
                ]}
              />
              <InputField
                label="Received At (ISO 8601)"
                name="receivedAt"
                value={commandAckForm.receivedAt}
                onChange={handleCommandAckInputChange}
                placeholder="2025-04-01T10:00:00Z"
              />
              <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
                <span>Acknowledgement Payload (JSON, optional)</span>
                <textarea
                  className="h-28 rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  name="payloadJson"
                  value={commandAckForm.payloadJson}
                  onChange={handleCommandAckInputChange}
                  placeholder='{"status":"ok"}'
                />
              </label>
              <div className="flex items-center gap-3">
                <button
                  type="submit"
                  className="rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2"
                >
                  Record Acknowledgement
                </button>
                <button
                  type="button"
                  className="rounded-md border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                  onClick={handleCommandAckReset}
                >
                  Reset
                </button>
              </div>
            </form>
            {commandAckError && (
              <div className="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {commandAckError}
              </div>
            )}
            {commandAckResult && (
              <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
                <p className="font-semibold">Command updated</p>
                <p className="mt-1">
                  Message {commandAckResult.msgid} marked {commandAckResult.status.toUpperCase()}.
                </p>
              </div>
            )}
          </div>
        </div>

        <div className="mt-8">
          <h3 className="text-base font-semibold text-slate-700">Command History</h3>
          <form
            className="mt-3 grid gap-4 md:grid-cols-2 lg:grid-cols-4"
            onSubmit={handleCommandHistorySubmit}
          >
            <InputField
              label="Device UUID"
              name="deviceUuid"
              value={commandHistoryFilters.deviceUuid}
              onChange={handleCommandHistoryInputChange}
              required
            />
            <InputField
              label="Limit"
              name="limit"
              value={commandHistoryFilters.limit}
              onChange={handleCommandHistoryInputChange}
              placeholder="Defaults to 25"
            />
            <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
              <span>Status Filters</span>
              <div className="grid grid-cols-3 gap-2 text-xs">
                {(['pending', 'acknowledged', 'failed'] as const).map((status) => (
                  <label key={status} className="flex items-center gap-1 font-semibold text-slate-700">
                    <input
                      type="checkbox"
                      name={`status.${status}`}
                      checked={commandHistoryFilters.statuses[status]}
                      onChange={handleCommandHistoryInputChange}
                    />
                    {status}
                  </label>
                ))}
              </div>
            </label>
            <div className="flex items-end gap-2">
              <button
                type="submit"
                className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white shadow hover:bg-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-600 focus:ring-offset-2 disabled:opacity-60"
                disabled={commandHistoryLoading}
              >
                {commandHistoryLoading ? 'Loading…' : 'Load History'}
              </button>
              <button
                type="button"
                className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={handleCommandHistoryReset}
                disabled={commandHistoryLoading}
              >
                Reset
              </button>
            </div>
          </form>

          {commandHistoryError && (
            <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              {commandHistoryError}
            </div>
          )}

          {commandHistoryResult && (
            <div className="mt-4 overflow-x-auto">
              <table className="min-w-full divide-y divide-slate-200 text-sm">
                <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                  <tr>
                    <th className="p-3 text-left">Message ID</th>
                    <th className="p-3 text-left">Command</th>
                    <th className="p-3 text-left">Status</th>
                    <th className="p-3 text-left">Requested</th>
                    <th className="p-3 text-left">Ack</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {commandHistoryResult.commands.map((command: DeviceCommandHistoryRecord) => (
                    <tr key={command.msgid} className="hover:bg-slate-50">
                      <td className="px-3 py-2 font-mono text-xs text-slate-700">{command.msgid}</td>
                      <td className="px-3 py-2 text-xs text-slate-700">
                        <div className="font-semibold text-slate-900">{command.command.name}</div>
                        {command.command.payload && Object.keys(command.command.payload).length > 0 && (
                          <div className="mt-1 text-[11px] text-slate-500">
                            Payload keys: {Object.keys(command.command.payload).join(', ')}
                          </div>
                        )}
                      </td>
                      <td className="px-3 py-2 text-xs text-slate-700">{command.status}</td>
                      <td className="px-3 py-2 text-xs text-slate-700">
                        {new Date(command.requestedAt).toLocaleString()}
                      </td>
                      <td className="px-3 py-2 text-xs text-slate-700">
                        {command.acknowledgedAt ? new Date(command.acknowledgedAt).toLocaleString() : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {commandHistoryResult.nextCursor && (
                <div className="mt-3">
                  <button
                    type="button"
                    className="rounded-md border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                    onClick={handleCommandHistoryLoadMore}
                    disabled={commandHistoryLoading}
                  >
                    {commandHistoryLoading ? 'Loading…' : 'Load More'}
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

// Local input helpers reused from DeviceConfigurationPage
type InputFieldProps = {
  label: string;
  name: string;
  value: string;
  onChange: (
    event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
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
    event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>,
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
