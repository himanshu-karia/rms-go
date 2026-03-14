export function formatDateTimeShort(value: string | number | Date): string {
  const date =
    value instanceof Date ? value : new Date(typeof value === 'number' ? value : String(value));

  if (Number.isNaN(date.getTime())) {
    return 'Invalid date';
  }

  return date.toLocaleString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    day: '2-digit',
    month: 'short',
  });
}

export function formatDateTimeWithSeconds(value: string | number | Date): string {
  const date =
    value instanceof Date ? value : new Date(typeof value === 'number' ? value : String(value));

  if (Number.isNaN(date.getTime())) {
    return 'Invalid date';
  }

  return date.toLocaleString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    day: '2-digit',
    month: 'short',
  });
}

export function subtractHours(base: Date, hours: number): Date {
  return new Date(base.getTime() - hours * 60 * 60 * 1000);
}

export function subtractDays(base: Date, days: number): Date {
  return new Date(base.getTime() - days * 24 * 60 * 60 * 1000);
}

export function toIsoString(date: Date): string {
  return date.toISOString();
}

export function formatRelativeDuration(milliseconds: number): string {
  if (!Number.isFinite(milliseconds)) {
    return '0s';
  }

  const totalSeconds = Math.max(0, Math.floor(milliseconds / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  const parts: string[] = [];
  if (hours > 0) {
    parts.push(`${hours}h`);
  }

  if (minutes > 0 || hours > 0) {
    parts.push(`${minutes}m`);
  }

  parts.push(`${seconds}s`);
  return parts.join(' ');
}
