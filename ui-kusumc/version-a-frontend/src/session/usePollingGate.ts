import { useCallback, useEffect, useId, useMemo } from 'react';

import { useSessionTimers } from './SessionTimersProvider';

type PollingGateOptions = {
  isActive?: boolean;
};

type PollingGateState = {
  enabled: boolean;
  isIdle: boolean;
  remainingMs: number | null;
  resume: () => void;
};

export function usePollingGate(
  sourceKey?: string,
  options: PollingGateOptions = {},
): PollingGateState {
  const {
    registerPollingSource,
    isSessionIdle,
    sessionIdleRemainingMs,
    resumeSessionActivity,
    isTabVisible,
  } = useSessionTimers();

  const id = useId();
  const key = sourceKey ?? `polling-${id}`;
  const isActive = options.isActive ?? true;

  useEffect(() => {
    if (!isActive) {
      return;
    }

    return registerPollingSource(key);
  }, [isActive, key, registerPollingSource]);

  const resume = useCallback(() => {
    resumeSessionActivity();
  }, [resumeSessionActivity]);

  const enabled = isActive && isTabVisible && !isSessionIdle;
  const isIdleState = isActive && (!isTabVisible || isSessionIdle);
  const remaining = isActive ? sessionIdleRemainingMs : null;

  return useMemo(
    () => ({
      enabled,
      isIdle: isIdleState,
      remainingMs: remaining,
      resume,
    }),
    [enabled, isIdleState, remaining, resume],
  );
}
