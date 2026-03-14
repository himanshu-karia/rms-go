import { useMemo } from 'react';

import { formatDuration, useSessionTimers } from '../session';

type ChipTone = 'muted' | 'ok' | 'warn' | 'alert';

type StatusChipProps = {
  label: string;
  value: string;
  tone: ChipTone;
};

export function SessionStatusIndicators() {
  const {
    pollingActive,
    isSessionIdle,
    sessionIdleRemainingMs,
    idleLimitMs,
    jwtRemainingMs,
    jwtWarningThresholdMs,
    isJwtExpiringSoon,
    listMqttSummaries,
    mqttPacketCap,
    isTabVisible,
  } = useSessionTimers();

  const idleDisplay = !isTabVisible
    ? 'Tab hidden'
    : pollingActive
      ? formatDuration(sessionIdleRemainingMs ?? idleLimitMs)
      : '--';

  const idleTone: ChipTone = !isTabVisible
    ? 'warn'
    : !pollingActive
      ? 'muted'
      : isSessionIdle
        ? 'alert'
        : 'ok';

  const jwtTone: ChipTone = useMemo(() => {
    if (jwtRemainingMs === null) {
      return 'muted';
    }

    if (jwtRemainingMs <= 0) {
      return 'alert';
    }

    if (isJwtExpiringSoon || jwtRemainingMs <= jwtWarningThresholdMs) {
      return 'warn';
    }

    return 'ok';
  }, [jwtRemainingMs, isJwtExpiringSoon, jwtWarningThresholdMs]);

  const jwtDisplay = formatDuration(jwtRemainingMs);

  const liveFeed = useMemo(() => {
    const summaries = listMqttSummaries();
    if (!summaries.length) {
      return {
        value: '--',
        tone: 'muted' as ChipTone,
      };
    }

    const minRemaining = summaries.reduce((acc, entry) => {
      return Math.min(acc, entry.summary.remaining);
    }, Number.POSITIVE_INFINITY);
    const pausedEntry = summaries.find((entry) => entry.summary.isPaused);

    if (pausedEntry) {
      const reason = pausedEntry.summary.reason === 'cap' ? 'Cap reached' : 'Idle paused';
      return {
        value: reason,
        tone: 'alert' as ChipTone,
      };
    }

    const safeRemaining = Number.isFinite(minRemaining) ? minRemaining : mqttPacketCap;
    return {
      value: `${safeRemaining} pkt left`,
      tone:
        safeRemaining <= Math.floor(mqttPacketCap * 0.1)
          ? ('warn' as ChipTone)
          : ('ok' as ChipTone),
    };
  }, [listMqttSummaries, mqttPacketCap]);

  return (
    <div className="flex flex-wrap items-center gap-2 text-xs">
      <StatusChip label="REST polling" value={idleDisplay} tone={idleTone} />
      <StatusChip label="JWT" value={jwtDisplay} tone={jwtTone} />
      <StatusChip label="Live feed" value={liveFeed.value} tone={liveFeed.tone} />
    </div>
  );
}

export function SessionIdleBanner() {
  const { isSessionIdle, sessionIdleRemainingMs, resumeSessionActivity } = useSessionTimers();

  if (!isSessionIdle) {
    return null;
  }

  const lastReading = formatDuration(sessionIdleRemainingMs, { fallback: '00:00' });

  return (
    <div className="flex items-center justify-between gap-4 border-b border-amber-200 bg-amber-50 px-6 py-3 text-sm text-amber-800">
      <div>
        <p className="font-medium">Polling paused after 30 minutes of inactivity.</p>
        <p className="text-xs text-amber-700">Last timer reading: {lastReading}</p>
      </div>
      <button
        type="button"
        onClick={resumeSessionActivity}
        className="rounded bg-amber-600 px-3 py-1 text-xs font-semibold text-white shadow hover:bg-amber-500"
      >
        Resume polling
      </button>
    </div>
  );
}

function StatusChip({ label, value, tone }: StatusChipProps) {
  const toneClasses: Record<ChipTone, string> = {
    muted: 'border-slate-200 bg-slate-50 text-slate-500',
    ok: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    warn: 'border-amber-200 bg-amber-50 text-amber-700',
    alert: 'border-red-200 bg-red-50 text-red-700',
  };

  return (
    <div className={`flex items-center gap-2 rounded-full border px-3 py-1 ${toneClasses[tone]}`}>
      <span className="font-semibold uppercase tracking-wide">{label}</span>
      <span className="font-mono text-[0.7rem]">{value}</span>
    </div>
  );
}
