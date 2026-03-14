import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { useLocation } from 'react-router-dom';

import { introspectSession } from '../api/auth';
import { getSessionSnapshot } from '../api/session';
import { useAuth } from '../auth';
import { formatDuration } from './time';

const SESSION_IDLE_LIMIT_MS = 30 * 60 * 1000;
const TIMER_TICK_MS = 5_000;
const JWT_WARNING_THRESHOLD_MS = 3 * 60 * 1000;
const MQTT_PACKET_CAP = 200;

const DEFAULT_BUDGET: MqttBudget = { count: 0, isPaused: false, reason: null };

type BudgetReason = 'cap' | 'idle' | 'hidden' | null;

type MqttBudget = {
  count: number;
  isPaused: boolean;
  reason: BudgetReason;
};

type SessionResumeToastVariant = 'http' | 'mqtt' | 'mixed';

export type MqttBudgetSummary = {
  count: number;
  remaining: number;
  cap: number;
  isPaused: boolean;
  reason: BudgetReason;
};

type SessionTimersContextValue = {
  idleLimitMs: number;
  pollingActive: boolean;
  lastActivityAt: number | null;
  sessionIdleDeadline: number | null;
  sessionIdleRemainingMs: number | null;
  isSessionIdle: boolean;
  markUserActivity: () => void;
  registerPollingSource: (key: string) => () => void;
  resumeSessionActivity: () => void;
  jwtExpiry: number | null;
  jwtRemainingMs: number | null;
  jwtWarningThresholdMs: number;
  isJwtExpiringSoon: boolean;
  isRenewalModalOpen: boolean;
  isRenewing: boolean;
  renewalError: string | null;
  openRenewalModal: () => void;
  attemptRenewal: () => Promise<boolean>;
  cancelRenewal: () => void;
  mqttPacketCap: number;
  recordMqttPacket: (scope: string) => MqttBudgetSummary;
  resetMqttPacket: (scope: string) => void;
  getMqttSummary: (scope: string) => MqttBudgetSummary;
  markMqttPaused: (scope: string, reason: 'cap' | 'idle' | 'hidden') => void;
  listMqttSummaries: () => Array<{ scope: string; summary: MqttBudgetSummary }>;
  isTabVisible: boolean;
  sessionGeneration: number;
};

const SessionTimersContext = createContext<SessionTimersContextValue | undefined>(undefined);

function normalizeScope(scope: string): string {
  const trimmed = scope?.trim?.() ?? '';
  return trimmed.length > 0 ? trimmed : 'default';
}

function summarizeBudget(budget: MqttBudget): MqttBudgetSummary {
  return {
    count: budget.count,
    remaining: Math.max(0, MQTT_PACKET_CAP - budget.count),
    cap: MQTT_PACKET_CAP,
    isPaused: budget.isPaused,
    reason: budget.reason,
  };
}

export function SessionTimersProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated, session, refresh, logout } = useAuth();
  const location = useLocation();

  const [lastActivityAt, setLastActivityAt] = useState<number | null>(null);
  const [pollingActive, setPollingActive] = useState(false);
  const [isSessionIdle, setIsSessionIdle] = useState(false);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [jwtExpiryMs, setJwtExpiryMs] = useState<number | null>(null);
  const [isRenewalModalOpen, setIsRenewalModalOpen] = useState(false);
  const [isRenewing, setIsRenewing] = useState(false);
  const [renewalError, setRenewalError] = useState<string | null>(null);
  const [isTabVisible, setIsTabVisible] = useState(() => {
    if (typeof document === 'undefined') {
      return true;
    }
    return !document.hidden;
  });
  const [showJwtWarningToast, setShowJwtWarningToast] = useState(false);
  const [showHiddenMqttToast, setShowHiddenMqttToast] = useState(false);
  const [showSessionResumeToast, setShowSessionResumeToast] = useState(false);
  const [resumeToastVariant, setResumeToastVariant] = useState<SessionResumeToastVariant | null>(
    null,
  );
  const [mqttVersion, setMqttVersion] = useState(0);
  const [sessionGeneration, setSessionGeneration] = useState(0);

  const pollingSourcesRef = useRef(new Set<string>());
  const mqttBudgetsRef = useRef(new Map<string, MqttBudget>());
  const hasIssuedJwtWarningRef = useRef(false);

  const markUserActivity = useCallback(() => {
    if (!isAuthenticated) {
      return;
    }

    const timestamp = Date.now();
    setLastActivityAt(timestamp);
    setIsSessionIdle(false);
  }, [isAuthenticated]);

  const resumeSessionActivity = useCallback(() => {
    markUserActivity();

    let changed = false;
    mqttBudgetsRef.current.forEach((budget, scope) => {
      if (budget.reason === 'idle' || budget.reason === 'hidden') {
        mqttBudgetsRef.current.set(scope, { count: budget.count, isPaused: false, reason: null });
        changed = true;
      }
    });

    if (changed) {
      setMqttVersion((value) => value + 1);
    }
  }, [markUserActivity]);

  const registerPollingSource = useCallback((key: string) => {
    const normalized =
      key && key.trim().length > 0 ? key.trim() : `polling-${Math.random().toString(36).slice(2)}`;

    if (!pollingSourcesRef.current.has(normalized)) {
      pollingSourcesRef.current.add(normalized);
      setPollingActive(true);
      setLastActivityAt((prev) => prev ?? Date.now());
    }

    return () => {
      pollingSourcesRef.current.delete(normalized);
      const hasSources = pollingSourcesRef.current.size > 0;
      setPollingActive(hasSources);
      if (!hasSources) {
        setIsSessionIdle(false);
      }
    };
  }, []);

  const updateBudget = useCallback(
    (scope: string, updater: (budget: MqttBudget) => MqttBudget): MqttBudget => {
      const normalized = normalizeScope(scope);
      const current = mqttBudgetsRef.current.get(normalized) ?? DEFAULT_BUDGET;
      const next = updater(current);
      mqttBudgetsRef.current.set(normalized, next);
      setMqttVersion((value) => value + 1);
      return next;
    },
    [],
  );

  const recordMqttPacket = useCallback(
    (scope: string) => {
      const next = updateBudget(scope, (budget) => {
        if (budget.isPaused) {
          return budget;
        }

        const nextCount = budget.count + 1;
        if (nextCount >= MQTT_PACKET_CAP) {
          return { count: nextCount, isPaused: true, reason: 'cap' };
        }

        return { count: nextCount, isPaused: false, reason: null };
      });

      return summarizeBudget(next);
    },
    [updateBudget],
  );

  const resetMqttPacket = useCallback(
    (scope: string) => {
      updateBudget(scope, () => ({ count: 0, isPaused: false, reason: null }));
    },
    [updateBudget],
  );

  const getMqttSummary = useCallback((scope: string) => {
    const normalized = normalizeScope(scope);
    const budget = mqttBudgetsRef.current.get(normalized) ?? DEFAULT_BUDGET;
    return summarizeBudget(budget);
  }, []);

  const markMqttPaused = useCallback(
    (scope: string, reason: 'cap' | 'idle' | 'hidden') => {
      updateBudget(scope, (budget) => {
        if (budget.isPaused && budget.reason === reason) {
          return budget;
        }

        return { count: budget.count, isPaused: true, reason };
      });
    },
    [updateBudget],
  );

  const listMqttSummaries = useCallback(() => {
    return Array.from(mqttBudgetsRef.current.entries()).map(([scope, budget]) => ({
      scope,
      summary: summarizeBudget(budget),
    }));
  }, []);

  const sessionIdleDeadline = useMemo(() => {
    if (!pollingActive || !lastActivityAt) {
      return null;
    }

    return lastActivityAt + SESSION_IDLE_LIMIT_MS;
  }, [pollingActive, lastActivityAt]);

  const sessionIdleRemainingMs = useMemo(() => {
    if (!sessionIdleDeadline) {
      return null;
    }

    return Math.max(0, sessionIdleDeadline - nowMs);
  }, [sessionIdleDeadline, nowMs]);

  const jwtRemainingMs = useMemo(() => {
    if (!jwtExpiryMs) {
      return null;
    }

    return Math.max(0, jwtExpiryMs - nowMs);
  }, [jwtExpiryMs, nowMs]);

  const isJwtExpiringSoon = useMemo(() => {
    if (jwtRemainingMs === null) {
      return false;
    }

    return jwtRemainingMs <= JWT_WARNING_THRESHOLD_MS;
  }, [jwtRemainingMs]);

  useEffect(() => {
    if (!pollingActive) {
      return;
    }

    if (sessionIdleDeadline && nowMs >= sessionIdleDeadline) {
      setIsSessionIdle(true);
    }
  }, [pollingActive, sessionIdleDeadline, nowMs]);

  useEffect(() => {
    if (!isSessionIdle) {
      return;
    }

    let changed = false;
    mqttBudgetsRef.current.forEach((budget, scope) => {
      if (budget.isPaused && budget.reason === 'cap') {
        return;
      }

      if (budget.isPaused && budget.reason === 'idle') {
        return;
      }

      mqttBudgetsRef.current.set(scope, { count: budget.count, isPaused: true, reason: 'idle' });
      changed = true;
    });

    if (changed) {
      setMqttVersion((value) => value + 1);
    }
  }, [isSessionIdle]);

  useEffect(() => {
    if (!isAuthenticated) {
      pollingSourcesRef.current.clear();
      mqttBudgetsRef.current.clear();
      setPollingActive(false);
      setIsSessionIdle(false);
      setLastActivityAt(null);
      setJwtExpiryMs(null);
      setIsRenewalModalOpen(false);
      setRenewalError(null);
      setMqttVersion((value) => value + 1);
      setSessionGeneration(0);
      setShowSessionResumeToast(false);
      setResumeToastVariant(null);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    setNowMs(Date.now());
  }, [isAuthenticated]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    if (!isTabVisible) {
      return;
    }

    const id = window.setInterval(() => {
      setNowMs(Date.now());
    }, TIMER_TICK_MS);

    return () => {
      window.clearInterval(id);
    };
  }, [isTabVisible]);

  useEffect(() => {
    if (typeof document === 'undefined') {
      return;
    }

    const handleVisibility = () => {
      const visible = !document.hidden;
      setIsTabVisible(visible);

      if (visible) {
        setShowHiddenMqttToast(false);
        setNowMs(Date.now());
        markUserActivity();
        resumeSessionActivity();
        hasIssuedJwtWarningRef.current = false;
        return;
      }

      let changed = false;
      mqttBudgetsRef.current.forEach((budget, scope) => {
        if (budget.isPaused && budget.reason === 'hidden') {
          return;
        }

        mqttBudgetsRef.current.set(scope, {
          count: budget.count,
          isPaused: true,
          reason: 'hidden',
        });
        changed = true;
      });

      if (changed) {
        setMqttVersion((value) => value + 1);
      }

      const hasMqttEntries = mqttBudgetsRef.current.size > 0;
      setShowHiddenMqttToast(hasMqttEntries);
    };

    document.addEventListener('visibilitychange', handleVisibility);

    return () => {
      document.removeEventListener('visibilitychange', handleVisibility);
    };
  }, [markUserActivity, resumeSessionActivity]);

  useEffect(() => {
    if (!isAuthenticated) {
      setShowJwtWarningToast(false);
      setShowHiddenMqttToast(false);
      setShowSessionResumeToast(false);
      setResumeToastVariant(null);
      hasIssuedJwtWarningRef.current = false;
      return;
    }

    if (jwtRemainingMs === null || jwtRemainingMs <= 0) {
      setShowJwtWarningToast(false);
      return;
    }

    if (jwtRemainingMs <= JWT_WARNING_THRESHOLD_MS) {
      if (!hasIssuedJwtWarningRef.current) {
        setShowJwtWarningToast(true);
        hasIssuedJwtWarningRef.current = true;
      }
    } else {
      setShowJwtWarningToast(false);
      hasIssuedJwtWarningRef.current = false;
    }
  }, [isAuthenticated, jwtRemainingMs]);

  useEffect(() => {
    if (isRenewalModalOpen) {
      setShowJwtWarningToast(false);
    }
  }, [isRenewalModalOpen]);

  useEffect(() => {
    if (!isAuthenticated || isRenewalModalOpen || !isTabVisible) {
      setShowSessionResumeToast(false);
      setResumeToastVariant(null);
      return;
    }

    const hasIdlePolling = pollingActive && isSessionIdle;
    const hasIdleMqtt = Array.from(mqttBudgetsRef.current.values()).some(
      (budget) => budget.isPaused && budget.reason === 'idle',
    );

    const variant: SessionResumeToastVariant | null =
      hasIdlePolling && hasIdleMqtt
        ? 'mixed'
        : hasIdlePolling
          ? 'http'
          : hasIdleMqtt
            ? 'mqtt'
            : null;

    if (variant) {
      setResumeToastVariant(variant);
      setShowSessionResumeToast(true);
    } else {
      setShowSessionResumeToast(false);
      setResumeToastVariant(null);
    }
  }, [
    isAuthenticated,
    isRenewalModalOpen,
    isTabVisible,
    pollingActive,
    isSessionIdle,
    mqttVersion,
  ]);

  useEffect(() => {
    if (typeof window === 'undefined' || !isAuthenticated) {
      return;
    }

    const activityEvents: Array<keyof WindowEventMap> = [
      'mousemove',
      'mousedown',
      'keydown',
      'scroll',
      'touchstart',
    ];

    const handleActivity = () => {
      markUserActivity();
    };

    activityEvents.forEach((event) => {
      window.addEventListener(event, handleActivity, { passive: true });
    });

    const handleFocus = () => {
      markUserActivity();
    };

    window.addEventListener('focus', handleFocus);

    return () => {
      activityEvents.forEach((event) => {
        window.removeEventListener(event, handleActivity);
      });
      window.removeEventListener('focus', handleFocus);
    };
  }, [isAuthenticated, markUserActivity]);

  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }

    markUserActivity();
  }, [location.pathname, isAuthenticated, markUserActivity]);

  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }

    const snapshot = getSessionSnapshot();
    if (snapshot?.expiresAt) {
      const parsed = Date.parse(snapshot.expiresAt);
      if (!Number.isNaN(parsed)) {
        setJwtExpiryMs(parsed);
      }
    }

    let cancelled = false;

    (async () => {
      try {
        const result = await introspectSession();
        if (cancelled) {
          return;
        }

        const parsed = Date.parse(result.session.expiresAt);
        if (!Number.isNaN(parsed)) {
          setJwtExpiryMs(parsed);
        }
      } catch (error) {
        console.warn('[session] failed to introspect active session', error);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, session?.sessionId]);

  useEffect(() => {
    if (!isAuthenticated) {
      setIsRenewalModalOpen(false);
      setRenewalError(null);
      return;
    }

    if (jwtRemainingMs === null) {
      return;
    }

    if (jwtRemainingMs <= 0) {
      setIsRenewalModalOpen(true);
    }
  }, [jwtRemainingMs, isAuthenticated]);

  const openRenewalModal = useCallback(() => {
    setRenewalError(null);
    setShowJwtWarningToast(false);
    setIsRenewalModalOpen(true);
  }, []);

  const cancelRenewal = useCallback(() => {
    setIsRenewalModalOpen(false);
    setRenewalError(null);
    void logout();
  }, [logout]);

  const attemptRenewal = useCallback(async () => {
    setIsRenewing(true);
    setRenewalError(null);

    try {
      const refreshed = await refresh();
      const parsed = Date.parse(refreshed.expiresAt);
      if (!Number.isNaN(parsed)) {
        setJwtExpiryMs(parsed);
      }

      setIsRenewalModalOpen(false);
      hasIssuedJwtWarningRef.current = false;
      setShowJwtWarningToast(false);
      setShowHiddenMqttToast(false);
      resumeSessionActivity();
      setSessionGeneration((value) => value + 1);
      return true;
    } catch (error) {
      if (error instanceof Error) {
        setRenewalError(error.message);
      } else {
        setRenewalError('Unable to refresh session');
      }
      return false;
    } finally {
      setIsRenewing(false);
    }
  }, [refresh, resumeSessionActivity]);

  const handleExtendSession = useCallback(() => {
    void attemptRenewal().then((success) => {
      if (success) {
        setShowJwtWarningToast(false);
        return;
      }

      setShowJwtWarningToast(false);
      setIsRenewalModalOpen(true);
    });
  }, [attemptRenewal]);

  const handleResumeLiveData = useCallback(() => {
    resumeSessionActivity();
    setShowSessionResumeToast(false);
  }, [resumeSessionActivity]);

  const contextValue = useMemo<SessionTimersContextValue>(
    () => ({
      idleLimitMs: SESSION_IDLE_LIMIT_MS,
      pollingActive,
      lastActivityAt,
      sessionIdleDeadline,
      sessionIdleRemainingMs,
      isSessionIdle,
      markUserActivity,
      registerPollingSource,
      resumeSessionActivity,
      jwtExpiry: jwtExpiryMs,
      jwtRemainingMs,
      jwtWarningThresholdMs: JWT_WARNING_THRESHOLD_MS,
      isJwtExpiringSoon,
      isRenewalModalOpen,
      isRenewing,
      renewalError,
      openRenewalModal,
      attemptRenewal,
      cancelRenewal,
      mqttPacketCap: MQTT_PACKET_CAP,
      recordMqttPacket,
      resetMqttPacket,
      getMqttSummary,
      markMqttPaused,
      listMqttSummaries,
      isTabVisible,
      sessionGeneration,
    }),
    [
      pollingActive,
      lastActivityAt,
      sessionIdleDeadline,
      sessionIdleRemainingMs,
      isSessionIdle,
      markUserActivity,
      registerPollingSource,
      resumeSessionActivity,
      jwtExpiryMs,
      jwtRemainingMs,
      isJwtExpiringSoon,
      isRenewalModalOpen,
      isRenewing,
      renewalError,
      openRenewalModal,
      attemptRenewal,
      cancelRenewal,
      recordMqttPacket,
      resetMqttPacket,
      getMqttSummary,
      markMqttPaused,
      listMqttSummaries,
      isTabVisible,
      sessionGeneration,
    ],
  );

  return (
    <SessionTimersContext.Provider value={contextValue}>
      {children}
      {showJwtWarningToast && !isRenewalModalOpen && (
        <SessionJwtWarningToast
          remainingMs={jwtRemainingMs}
          onDismiss={() => setShowJwtWarningToast(false)}
          onExtend={handleExtendSession}
          isExtending={isRenewing}
        />
      )}
      {showSessionResumeToast && resumeToastVariant && (
        <SessionResumeToast
          variant={resumeToastVariant}
          onResume={handleResumeLiveData}
          onDismiss={() => setShowSessionResumeToast(false)}
        />
      )}
      {showHiddenMqttToast && !isTabVisible && (
        <SessionHiddenMqttToast onDismiss={() => setShowHiddenMqttToast(false)} />
      )}
      <SessionRenewalDialog
        open={isRenewalModalOpen}
        onStaySignedIn={attemptRenewal}
        onSignOut={cancelRenewal}
        isSubmitting={isRenewing}
        error={renewalError}
        remainingMs={jwtRemainingMs}
      />
    </SessionTimersContext.Provider>
  );
}

export function useSessionTimers(): SessionTimersContextValue {
  const context = useContext(SessionTimersContext);

  if (!context) {
    throw new Error('useSessionTimers must be used within a SessionTimersProvider');
  }

  return context;
}

type SessionRenewalDialogProps = {
  open: boolean;
  onStaySignedIn: () => Promise<boolean>;
  onSignOut: () => void;
  isSubmitting: boolean;
  error: string | null;
  remainingMs: number | null;
};

function SessionRenewalDialog({
  open,
  onStaySignedIn,
  onSignOut,
  isSubmitting,
  error,
  remainingMs,
}: SessionRenewalDialogProps) {
  if (!open) {
    return null;
  }

  const remainingText = formatDuration(remainingMs, { fallback: 'Expired' });

  const handleStaySignedIn = () => {
    void onStaySignedIn();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 px-4">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <h2 className="text-lg font-semibold text-slate-900">Session expired</h2>
        <p className="mt-2 text-sm text-slate-600">
          Your access token has reached its 90 minute limit. Choose &ldquo;Stay signed in&rdquo; to
          request a fresh session or sign out safely.
        </p>
        <dl className="mt-4 space-y-2 text-sm text-slate-500">
          <div className="flex items-center justify-between">
            <dt className="font-medium text-slate-600">Last timer reading</dt>
            <dd className="font-mono text-slate-700">{remainingText}</dd>
          </div>
        </dl>
        {error && <p className="mt-3 rounded bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            className="rounded border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            onClick={onSignOut}
            disabled={isSubmitting}
          >
            Sign out
          </button>
          <button
            type="button"
            className="rounded bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-75"
            onClick={handleStaySignedIn}
            disabled={isSubmitting}
          >
            {isSubmitting ? 'Refreshing…' : 'Stay signed in'}
          </button>
        </div>
      </div>
    </div>
  );
}

type SessionJwtWarningToastProps = {
  remainingMs: number | null;
  onDismiss: () => void;
  onExtend: () => void;
  isExtending: boolean;
};

function SessionJwtWarningToast({
  remainingMs,
  onDismiss,
  onExtend,
  isExtending,
}: SessionJwtWarningToastProps) {
  const remainingText = formatDuration(remainingMs, { fallback: '< 3m' });

  return (
    <div className="fixed bottom-6 right-6 z-40 w-full max-w-sm rounded-lg border border-amber-300 bg-white shadow-xl">
      <div className="flex items-start gap-3 px-4 py-3">
        <span className="mt-0.5 inline-flex size-6 shrink-0 items-center justify-center rounded-full bg-amber-500 text-xs font-semibold text-white">
          !
        </span>
        <div className="flex-1">
          <p className="text-sm font-semibold text-amber-900">
            Session will expire in under 3 minutes
          </p>
          <p className="mt-1 text-xs text-amber-800">
            Your session will expire in about {remainingText}. Extend now to stay connected.
          </p>
          <div className="mt-3 flex flex-wrap gap-2">
            <button
              type="button"
              onClick={onExtend}
              disabled={isExtending}
              className="rounded bg-amber-600 px-3 py-1 text-xs font-semibold text-white shadow hover:bg-amber-500 disabled:cursor-not-allowed disabled:opacity-70"
            >
              {isExtending ? 'Extending…' : 'Extend session'}
            </button>
            <button
              type="button"
              onClick={onDismiss}
              className="rounded border border-amber-200 px-3 py-1 text-xs font-medium text-amber-700 hover:bg-amber-50"
            >
              Dismiss
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

type SessionHiddenMqttToastProps = {
  onDismiss: () => void;
};

function SessionHiddenMqttToast({ onDismiss }: SessionHiddenMqttToastProps) {
  return (
    <div className="fixed bottom-6 left-1/2 z-40 w-full max-w-md -translate-x-1/2 rounded-lg border border-slate-200 bg-white shadow-xl">
      <div className="flex items-start gap-3 px-4 py-3">
        <span className="mt-0.5 inline-flex size-6 shrink-0 items-center justify-center rounded-full bg-slate-600 text-xs font-semibold text-white">
          !
        </span>
        <div className="flex-1">
          <p className="text-sm font-semibold text-slate-900">Live streams paused</p>
          <p className="mt-1 text-xs text-slate-700">
            We disconnected MQTT feeds because this tab is hidden. Focus the tab and resume when
            you&apos;re ready.
          </p>
        </div>
        <button
          type="button"
          onClick={onDismiss}
          className="mt-0.5 inline-flex h-6 items-center justify-center rounded border border-slate-300 px-2 text-xs font-medium text-slate-600 hover:bg-slate-100"
        >
          Got it
        </button>
      </div>
    </div>
  );
}

type SessionResumeToastProps = {
  variant: SessionResumeToastVariant;
  onResume: () => void;
  onDismiss: () => void;
};

function SessionResumeToast({ variant, onResume, onDismiss }: SessionResumeToastProps) {
  const detail = (() => {
    switch (variant) {
      case 'http':
        return 'HTTP polling was paused after 30 minutes of inactivity.';
      case 'mqtt':
        return 'MQTT streams were paused because no activity was detected.';
      case 'mixed':
        return 'HTTP polling and MQTT streams were paused due to inactivity.';
      default:
        return '';
    }
  })();

  return (
    <div className="fixed bottom-6 left-6 z-40 w-full max-w-sm rounded-lg border border-red-200 bg-white shadow-xl">
      <div className="flex items-start gap-3 px-4 py-3">
        <span className="mt-0.5 inline-flex size-6 shrink-0 items-center justify-center rounded-full bg-red-600 text-xs font-semibold text-white">
          !
        </span>
        <div className="flex-1">
          <p className="text-sm font-semibold text-red-900">Session expired. Resume to continue.</p>
          <p className="mt-1 text-xs text-red-800">{detail}</p>
          <div className="mt-3 flex flex-wrap gap-2">
            <button
              type="button"
              onClick={onResume}
              className="rounded bg-red-600 px-3 py-1 text-xs font-semibold text-white shadow hover:bg-red-500"
            >
              Resume live data
            </button>
            <button
              type="button"
              onClick={onDismiss}
              className="rounded border border-red-200 px-3 py-1 text-xs font-medium text-red-700 hover:bg-red-50"
            >
              Dismiss
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
