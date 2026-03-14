type FormatDurationOptions = {
  fallback?: string;
};

export function formatDuration(
  value: number | null | undefined,
  options: FormatDurationOptions = {},
): string {
  if (value === null || value === undefined) {
    return options.fallback ?? '--';
  }

  const totalSeconds = Math.max(0, Math.floor(value / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  const minutesText = minutes.toString().padStart(2, '0');
  const secondsText = seconds.toString().padStart(2, '0');

  if (hours > 0) {
    const hoursText = hours.toString().padStart(2, '0');
    return `${hoursText}:${minutesText}:${secondsText}`;
  }

  return `${minutesText}:${secondsText}`;
}
