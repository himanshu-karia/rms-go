export function parseMetadata(input: string) {
  if (!input.trim()) {
    return { metadata: null as Record<string, unknown> | null, error: null as string | null };
  }

  try {
    return { metadata: JSON.parse(input) as Record<string, unknown>, error: null as string | null };
  } catch {
    return { metadata: null, error: 'Metadata must be valid JSON' };
  }
}

export function stringifyMetadata(metadata: Record<string, unknown> | null) {
  if (!metadata) {
    return '';
  }
  try {
    return JSON.stringify(metadata, null, 2);
  } catch {
    return '';
  }
}

export function getMetadataPreview(metadata: Record<string, unknown> | null) {
  if (!metadata) {
    return '—';
  }
  try {
    return JSON.stringify(metadata);
  } catch {
    return '[unserializable metadata]';
  }
}
