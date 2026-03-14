export function parseTelemetryNumeric(value: unknown, scale = 1): number | null {
  if (typeof value === 'number') {
    return Number.isFinite(value) ? value * scale : null;
  }

  if (typeof value === 'string') {
    const normalized = value.trim();
    if (!normalized) {
      return null;
    }

    const sanitized = normalized.replace(/[^0-9.,+-]/g, '');
    if (!sanitized) {
      return null;
    }

    const containsDot = sanitized.includes('.');
    let numericString = sanitized;

    if (containsDot) {
      numericString = sanitized.replace(/,/g, '');
    } else if (sanitized.includes(',')) {
      const [integerPart, decimalPart] = sanitized.split(',', 2);
      numericString = `${integerPart.replace(/,/g, '')}.${(decimalPart ?? '').replace(/,/g, '')}`;
    }

    const parsed = Number(numericString);
    return Number.isFinite(parsed) ? parsed * scale : null;
  }

  return null;
}

export type TelemetrySeriesRecord = {
  receivedAt: string;
  payload?: Record<string, unknown> | null;
};

export type TelemetrySeriesPoint = {
  timestamp: string;
  value: number | null;
};

export function buildTelemetrySeries(
  records: TelemetrySeriesRecord[],
  payloadKey: string,
  options: {
    scale?: number;
    sortOrder?: 'asc' | 'desc';
  } = {},
): TelemetrySeriesPoint[] {
  if (!Array.isArray(records) || !payloadKey) {
    return [];
  }

  const { scale = 1, sortOrder = 'asc' } = options;
  const sorted = [...records].sort((a, b) => {
    const aTime = new Date(a.receivedAt).getTime();
    const bTime = new Date(b.receivedAt).getTime();
    if (Number.isNaN(aTime) || Number.isNaN(bTime)) {
      return 0;
    }
    return sortOrder === 'asc' ? aTime - bTime : bTime - aTime;
  });

  return sorted.map((record) => ({
    timestamp: record.receivedAt,
    value: parseTelemetryNumeric(record.payload?.[payloadKey], scale),
  }));
}
