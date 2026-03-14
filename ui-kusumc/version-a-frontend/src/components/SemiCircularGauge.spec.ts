import { describe, expect, it } from 'vitest';

import { evaluateGaugeStatus, type GaugeThresholds } from './SemiCircularGauge';

describe('evaluateGaugeStatus', () => {
  const baseThresholds: GaugeThresholds = {
    min: 0,
    max: 100,
    warnLow: 25,
    warnHigh: 75,
    alertLow: 10,
    alertHigh: 90,
  };

  it('returns idle when value is null or not finite', () => {
    expect(evaluateGaugeStatus(null, baseThresholds)).toEqual({
      status: 'idle',
      direction: null,
      threshold: null,
    });
    expect(evaluateGaugeStatus(Number.NaN, baseThresholds)).toEqual({
      status: 'idle',
      direction: null,
      threshold: null,
    });
  });

  it('flags low alert when value is below alertLow', () => {
    const result = evaluateGaugeStatus(5, baseThresholds);
    expect(result).toEqual({ status: 'alert', direction: 'low', threshold: 10 });
  });

  it('flags high alert when value is above alertHigh', () => {
    const result = evaluateGaugeStatus(95, baseThresholds);
    expect(result).toEqual({ status: 'alert', direction: 'high', threshold: 90 });
  });

  it('falls back to min/max when alert thresholds are undefined', () => {
    const thresholds: GaugeThresholds = {
      min: 0,
      max: 50,
      warnLow: 10,
      warnHigh: 40,
    };

    expect(evaluateGaugeStatus(-1, thresholds)).toEqual({
      status: 'alert',
      direction: 'low',
      threshold: 0,
    });
    expect(evaluateGaugeStatus(60, thresholds)).toEqual({
      status: 'alert',
      direction: 'high',
      threshold: 50,
    });
  });

  it('flags warn state when warn thresholds are crossed but alert thresholds are safe', () => {
    const lowWarn = evaluateGaugeStatus(20, baseThresholds);
    expect(lowWarn).toEqual({ status: 'warn', direction: 'low', threshold: 25 });

    const highWarn = evaluateGaugeStatus(80, baseThresholds);
    expect(highWarn).toEqual({ status: 'warn', direction: 'high', threshold: 75 });
  });

  it('returns normal when value is within tolerances', () => {
    const result = evaluateGaugeStatus(50, baseThresholds);
    expect(result).toEqual({ status: 'normal', direction: null, threshold: null });
  });
});
