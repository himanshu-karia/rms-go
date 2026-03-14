import { useEffect, useMemo, useRef, useState } from 'react';

export type GaugeDirection = 'low' | 'high' | null;
export type GaugeStatus = 'idle' | 'normal' | 'warn' | 'alert';

export type GaugeStatusInfo = {
  status: GaugeStatus;
  direction: GaugeDirection;
  threshold: number | null;
};

export type GaugeThresholds = {
  min?: number | null;
  max?: number | null;
  warnLow?: number | null;
  warnHigh?: number | null;
  alertLow?: number | null;
  alertHigh?: number | null;
};

export type SemiCircularGaugeProps = {
  value: number | null;
  min: number;
  max: number;
  warnLow?: number;
  warnHigh?: number;
  alertLow?: number;
  alertHigh?: number;
  unit?: string;
  decimalPlaces?: number;
};

const GAUGE_RADIUS = 56;
const GAUGE_WIDTH = 12;
const WARN_LOW_COLOR = '#F59E0B';
const WARN_HIGH_COLOR = '#F97316';
const ALERT_COLOR = '#DC2626';

type ThresholdSegment = {
  start: number;
  end: number;
  color: string;
};

export function SemiCircularGauge({
  value,
  min,
  max,
  warnLow,
  warnHigh,
  alertLow,
  alertHigh,
  unit,
  decimalPlaces = 1,
}: SemiCircularGaugeProps) {
  const [displayValue, setDisplayValue] = useState<number | null>(null);
  const animationRef = useRef<number | null>(null);
  const lastTargetRef = useRef<number | null>(null);
  const displayRef = useRef<number | null>(null);

  useEffect(() => {
    const safeMin = Number.isFinite(min) ? min : 0;
    const safeMax = Number.isFinite(max) && max !== null ? max : safeMin + 1;
    const span = Math.max(safeMax - safeMin, 1);
    const numericTarget = value === null || Number.isNaN(value) ? null : value;
    const clampedTarget =
      numericTarget === null ? null : Math.min(Math.max(numericTarget, safeMin), safeMax);

    if (clampedTarget === null) {
      if (animationRef.current !== null) {
        cancelAnimationFrame(animationRef.current);
        animationRef.current = null;
      }
      if (typeof queueMicrotask === 'function') {
        queueMicrotask(() => {
          setDisplayValue(null);
        });
      } else {
        setTimeout(() => {
          setDisplayValue(null);
        }, 0);
      }
      displayRef.current = null;
      lastTargetRef.current = null;
      return;
    }

    if (lastTargetRef.current !== null && Math.abs(lastTargetRef.current - clampedTarget) < 0.001) {
      return;
    }

    const startValue = displayRef.current ?? lastTargetRef.current ?? safeMin + span / 2;

    if (animationRef.current !== null) {
      cancelAnimationFrame(animationRef.current);
      animationRef.current = null;
    }

    const duration = 600;
    const startTime = performance.now();

    const step = (timestamp: number) => {
      const elapsed = Math.min((timestamp - startTime) / duration, 1);
      const eased = easeOutCubic(elapsed);
      const nextValue = startValue + (clampedTarget - startValue) * eased;
      displayRef.current = nextValue;
      setDisplayValue(nextValue);

      if (elapsed < 1) {
        animationRef.current = requestAnimationFrame(step);
      } else {
        animationRef.current = null;
        lastTargetRef.current = clampedTarget;
      }
    };

    animationRef.current = requestAnimationFrame(step);

    return () => {
      if (animationRef.current !== null) {
        cancelAnimationFrame(animationRef.current);
        animationRef.current = null;
      }
    };
  }, [value, min, max]);

  useEffect(() => {
    return () => {
      if (animationRef.current !== null) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, []);

  const {
    status,
    strokeDashoffset,
    formattedValue,
    warnSegments,
    alertSegments,
    minLabel,
    maxLabel,
    hasValue,
  } = useMemo(() => {
    const safeMin = Number.isFinite(min) ? min : 0;
    const safeMax = Number.isFinite(max) && max !== null ? max : safeMin + 1;
    const span = Math.max(safeMax - safeMin, 1);

    const numericTarget = value === null || Number.isNaN(value) ? null : value;
    const gaugeValue =
      displayValue !== null ? displayValue : numericTarget !== null ? safeMin + span / 2 : null;

    const normalizedValue =
      gaugeValue === null ? null : Math.min(Math.max((gaugeValue - safeMin) / span, 0), 1);

    const statusInfo = evaluateGaugeStatus(numericTarget, {
      min,
      max,
      warnLow,
      warnHigh,
      alertLow,
      alertHigh,
    });

    const circumference = Math.PI * GAUGE_RADIUS;
    const dashoffset =
      normalizedValue === null ? circumference : Math.max(0, (1 - normalizedValue) * circumference);
    const warnSegments = buildThresholdSegments({ low: warnLow, high: warnHigh }, safeMin, span, {
      lowColor: WARN_LOW_COLOR,
      highColor: WARN_HIGH_COLOR,
    });
    const alertSegments = buildThresholdSegments(
      { low: alertLow, high: alertHigh },
      safeMin,
      span,
      { lowColor: ALERT_COLOR, highColor: ALERT_COLOR },
    );

    return {
      normalized: normalizedValue,
      status: statusInfo.status,
      strokeDashoffset: dashoffset,
      formattedValue: formatGaugeValue(numericTarget, decimalPlaces),
      warnSegments,
      alertSegments,
      minLabel: safeMin.toLocaleString(),
      maxLabel: safeMax.toLocaleString(),
      hasValue: numericTarget !== null,
    };
  }, [value, displayValue, min, max, warnLow, warnHigh, alertLow, alertHigh, decimalPlaces]);

  const circumference = Math.PI * GAUGE_RADIUS;
  const trackStroke = '#E2E8F0';
  const progressStroke = !hasValue
    ? '#CBD5F5'
    : status === 'alert'
      ? ALERT_COLOR
      : status === 'warn'
        ? WARN_HIGH_COLOR
        : '#10B981';
  const warnStrokeWidth = GAUGE_WIDTH * 0.6;
  const alertStrokeWidth = GAUGE_WIDTH * 0.85;

  return (
    <div className="relative mx-auto size-44">
      <svg viewBox="0 0 140 90" className="size-full">
        <path
          d={buildArcPath(GAUGE_RADIUS)}
          fill="none"
          stroke={trackStroke}
          strokeWidth={GAUGE_WIDTH}
          strokeLinecap="round"
        />
        {alertSegments.map((segment, index) => (
          <path
            key={`alert-${segment.start}-${segment.end}-${index}`}
            d={buildArcPath(GAUGE_RADIUS)}
            fill="none"
            stroke={segment.color}
            strokeWidth={alertStrokeWidth}
            strokeLinecap="butt"
            strokeDasharray={`${(segment.end - segment.start) * circumference} ${circumference}`}
            strokeDashoffset={(1 - segment.end) * circumference}
            opacity={0.9}
          />
        ))}
        {warnSegments.map((segment, index) => (
          <path
            key={`warn-${segment.start}-${segment.end}-${index}`}
            d={buildArcPath(GAUGE_RADIUS)}
            fill="none"
            stroke={segment.color}
            strokeWidth={warnStrokeWidth}
            strokeLinecap="butt"
            strokeDasharray={`${(segment.end - segment.start) * circumference} ${circumference}`}
            strokeDashoffset={(1 - segment.end) * circumference}
          />
        ))}
        <path
          d={buildArcPath(GAUGE_RADIUS)}
          fill="none"
          stroke={progressStroke}
          strokeWidth={GAUGE_WIDTH}
          strokeLinecap="round"
          strokeDasharray={`${circumference} ${circumference}`}
          strokeDashoffset={strokeDashoffset}
          style={{ transition: 'stroke-dashoffset 0.6s ease, stroke 0.3s ease' }}
        />
      </svg>
      <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-end gap-1 pb-6">
        <span className="text-3xl font-semibold text-slate-900">{formattedValue}</span>
        {unit && hasValue && (
          <span className="text-xs uppercase tracking-wide text-slate-500">{unit}</span>
        )}
        {!hasValue && <span className="text-xs text-slate-500">Awaiting data…</span>}
      </div>
      <div className="pointer-events-none absolute inset-x-6 bottom-2 flex justify-between text-[11px] text-slate-500">
        <span>{minLabel}</span>
        <span>{maxLabel}</span>
      </div>
    </div>
  );
}

function buildArcPath(radius: number): string {
  const startX = 70 - radius;
  const endX = 70 + radius;
  const centerY = 70;
  return `M ${startX} ${centerY} A ${radius} ${radius} 0 0 1 ${endX} ${centerY}`;
}

function formatGaugeValue(value: number | null, decimalPlaces = 1): string {
  if (value === null || Number.isNaN(value)) {
    return '—';
  }

  const formatted = value.toLocaleString(undefined, {
    maximumFractionDigits: decimalPlaces,
    minimumFractionDigits: decimalPlaces > 0 ? Math.min(1, decimalPlaces) : 0,
  });

  return formatted;
}

function buildThresholdSegments(
  thresholds: { low?: number; high?: number },
  min: number,
  span: number,
  colors: { lowColor: string; highColor: string },
): ThresholdSegment[] {
  const segments: ThresholdSegment[] = [];
  const normalize = (value: number) => Math.min(Math.max((value - min) / span, 0), 1);

  if (typeof thresholds.low === 'number') {
    const normalized = normalize(thresholds.low);
    if (normalized > 0) {
      segments.push({ start: 0, end: normalized, color: colors.lowColor });
    }
  }

  if (typeof thresholds.high === 'number') {
    const normalized = normalize(thresholds.high);
    if (normalized < 1) {
      segments.push({ start: normalized, end: 1, color: colors.highColor });
    }
  }

  return segments;
}

function easeOutCubic(t: number): number {
  return 1 - Math.pow(1 - t, 3);
}

const coerceFinite = (value?: number | null): number | null => {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
};

const coalesceThreshold = (...values: Array<number | null | undefined>): number | null => {
  for (const value of values) {
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value;
    }
  }
  return null;
};

export function evaluateGaugeStatus(
  value: number | null,
  thresholds: GaugeThresholds,
): GaugeStatusInfo {
  const numericValue = typeof value === 'number' && Number.isFinite(value) ? value : null;
  if (numericValue === null) {
    return { status: 'idle', direction: null, threshold: null };
  }

  const lowAlert = coalesceThreshold(thresholds.alertLow, thresholds.min);
  if (lowAlert !== null && numericValue < lowAlert) {
    return { status: 'alert', direction: 'low', threshold: lowAlert };
  }

  const highAlert = coalesceThreshold(thresholds.alertHigh, thresholds.max);
  if (highAlert !== null && numericValue > highAlert) {
    return { status: 'alert', direction: 'high', threshold: highAlert };
  }

  const warnLow = coerceFinite(thresholds.warnLow);
  if (warnLow !== null && numericValue < warnLow) {
    return { status: 'warn', direction: 'low', threshold: warnLow };
  }

  const warnHigh = coerceFinite(thresholds.warnHigh);
  if (warnHigh !== null && numericValue > warnHigh) {
    return { status: 'warn', direction: 'high', threshold: warnHigh };
  }

  return { status: 'normal', direction: null, threshold: null };
}
