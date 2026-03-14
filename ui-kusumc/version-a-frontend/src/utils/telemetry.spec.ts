import { describe, expect, it } from 'vitest';

import { buildTelemetrySeries, parseTelemetryNumeric } from './telemetry';

describe('parseTelemetryNumeric', () => {
  it('returns numeric values unchanged when finite numbers provided', () => {
    expect(parseTelemetryNumeric(42)).toBe(42);
    expect(parseTelemetryNumeric(3.5, 2)).toBe(7);
  });

  it('parses numeric strings including comma separators', () => {
    expect(parseTelemetryNumeric('123.45')).toBeCloseTo(123.45);
    expect(parseTelemetryNumeric('1,234.5')).toBeCloseTo(1234.5);
    expect(parseTelemetryNumeric('1.234,5')).toBeCloseTo(1.2345);
  });

  it('returns null for invalid or empty values', () => {
    expect(parseTelemetryNumeric('')).toBeNull();
    expect(parseTelemetryNumeric('not-a-number')).toBeNull();
    expect(parseTelemetryNumeric(undefined)).toBeNull();
  });
});

describe('buildTelemetrySeries', () => {
  const records = [
    {
      receivedAt: '2025-10-26T10:00:00.000Z',
      payload: { POPV1: '230.5' },
    },
    {
      receivedAt: '2025-10-26T08:00:00.000Z',
      payload: { POPV1: '229.75' },
    },
    {
      receivedAt: '2025-10-26T09:00:00.000Z',
      payload: { POPV1: null },
    },
  ];

  it('sorts records chronologically ascending by default', () => {
    const series = buildTelemetrySeries(records, 'POPV1');
    expect(series.map((point) => point.timestamp)).toEqual([
      '2025-10-26T08:00:00.000Z',
      '2025-10-26T09:00:00.000Z',
      '2025-10-26T10:00:00.000Z',
    ]);
  });

  it('parses values and applies optional scaling factor', () => {
    const series = buildTelemetrySeries(records, 'POPV1', { scale: 0.5 });
    expect(series[0]).toEqual({ timestamp: '2025-10-26T08:00:00.000Z', value: 114.875 });
    expect(series[1]).toEqual({ timestamp: '2025-10-26T09:00:00.000Z', value: null });
    expect(series[2]).toEqual({ timestamp: '2025-10-26T10:00:00.000Z', value: 115.25 });
  });

  it('returns empty array for invalid payload keys', () => {
    expect(buildTelemetrySeries(records, '')).toEqual([]);
  });
});
