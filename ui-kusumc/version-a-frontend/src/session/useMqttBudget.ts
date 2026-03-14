import { useCallback, useMemo } from 'react';

import { useSessionTimers, type MqttBudgetSummary } from './SessionTimersProvider';

export type MqttBudgetHook = {
  summary: MqttBudgetSummary;
  cap: number;
  recordPacket: () => MqttBudgetSummary;
  reset: () => void;
  pauseForIdle: () => void;
  pauseForCap: () => void;
  pauseForHidden: () => void;
};

export function useMqttBudget(scope: string): MqttBudgetHook {
  const { mqttPacketCap, recordMqttPacket, resetMqttPacket, getMqttSummary, markMqttPaused } =
    useSessionTimers();

  const recordPacket = useCallback(() => recordMqttPacket(scope), [recordMqttPacket, scope]);
  const reset = useCallback(() => resetMqttPacket(scope), [resetMqttPacket, scope]);
  const pauseForIdle = useCallback(() => markMqttPaused(scope, 'idle'), [markMqttPaused, scope]);
  const pauseForCap = useCallback(() => markMqttPaused(scope, 'cap'), [markMqttPaused, scope]);
  const pauseForHidden = useCallback(
    () => markMqttPaused(scope, 'hidden'),
    [markMqttPaused, scope],
  );

  const summary = getMqttSummary(scope);

  return useMemo(
    () => ({
      summary,
      cap: mqttPacketCap,
      recordPacket,
      reset,
      pauseForIdle,
      pauseForCap,
      pauseForHidden,
    }),
    [summary, mqttPacketCap, recordPacket, reset, pauseForIdle, pauseForCap, pauseForHidden],
  );
}
