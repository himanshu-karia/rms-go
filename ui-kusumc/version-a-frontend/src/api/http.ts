import { clearSessionSnapshot, getSessionToken } from './session';

const DEFAULT_URL_BASE = 'http://localhost';
const FORCED_LOGOUT_EVENT = 'pmkusum:forced-logout';

function toSnakeCase(input: string): string {
  // Fast path.
  if (!/[A-Z]/.test(input)) {
    return input;
  }

  return input
    .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
    .replace(/([A-Z]+)([A-Z][a-z0-9]+)/g, '$1_$2')
    .toLowerCase();
}

function toCamelCase(input: string): string {
  if (!input.includes('_')) {
    return input;
  }

  return input.replace(/_([a-z0-9])/g, (_, c: string) => c.toUpperCase());
}

function normalizeQueryKeysToSnake(input: RequestInfo | URL): RequestInfo | URL {
  if (typeof input !== 'string' && !(input instanceof URL)) {
    return input;
  }

  const raw = input instanceof URL ? input.toString() : input;
  if (!raw.includes('?')) {
    return input;
  }

  try {
    const base = typeof window !== 'undefined' && window.location?.origin ? window.location.origin : DEFAULT_URL_BASE;
    const url = new URL(raw, base);

    const updated = new URLSearchParams();
    url.searchParams.forEach((value, key) => {
      updated.append(toSnakeCase(key), value);
    });

    url.search = updated.toString();
    return url.toString();
  } catch {
    return input;
  }
}

function normalizeJsonBodyKeysToSnake(init: RequestInit): RequestInit {
  if (!init.body || typeof init.body !== 'string') {
    return init;
  }

  const headers = buildHeaders(init.headers);
  const contentType = headers.get('Content-Type') ?? headers.get('content-type') ?? '';
  if (!contentType.toLowerCase().includes('application/json')) {
    return init;
  }

  try {
    const parsed = JSON.parse(init.body) as unknown;
    const normalized = normalizeKeysDeep(parsed, toSnakeCase);
    return {
      ...init,
      headers,
      body: JSON.stringify(normalized),
    };
  } catch {
    return init;
  }
}

function normalizeKeysDeep(value: unknown, keyTransform: (key: string) => string): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => normalizeKeysDeep(item, keyTransform));
  }

  if (!value || typeof value !== 'object') {
    return value;
  }

  if (value instanceof Date) {
    return value;
  }

  if (value instanceof Blob) {
    return value;
  }

  if (value instanceof File) {
    return value;
  }

  if (value instanceof FormData) {
    return value;
  }

  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    out[keyTransform(k)] = normalizeKeysDeep(v, keyTransform);
  }
  return out;
}

export function camelizeKeysDeep<T = unknown>(value: unknown): T {
  return normalizeKeysDeep(value, toCamelCase) as T;
}

function buildHeaders(input?: HeadersInit): Headers {
  if (input instanceof Headers) {
    return new Headers(input);
  }

  if (Array.isArray(input)) {
    return new Headers(input);
  }

  return new Headers(input ?? {});
}

export async function apiFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
): Promise<Response> {
  const normalizedInput = normalizeQueryKeysToSnake(input);

  const normalizedInit = normalizeJsonBodyKeysToSnake(init);
  const headers = buildHeaders(normalizedInit.headers);
  const token = headers.has('Authorization') ? null : getSessionToken();

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  if (!headers.has('Accept')) {
    headers.set('Accept', 'application/json');
  }

  const response = await fetch(normalizedInput, {
    ...normalizedInit,
    headers,
    credentials: 'include',
  });

  const hadAuthContext = headers.has('Authorization') || Boolean(token);
  if (hadAuthContext && response.status === 401) {
    clearSessionSnapshot();
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent(FORCED_LOGOUT_EVENT));
    }
  }

  return response;
}

export async function readJsonBody<T = unknown>(response: Response): Promise<T | null> {
  const contentType = (response.headers.get('Content-Type') ?? '').trim();
  if (contentType.length > 0 && !contentType.toLowerCase().includes('application/json')) {
    return null;
  }

  const parsed = await response.json().catch(() => null);
  if (parsed == null) {
    return null;
  }

  return camelizeKeysDeep<T>(parsed);
}
