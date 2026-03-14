import type { CapabilityKey } from './capabilities';

const STORAGE_KEY = 'pmkusum.session.v1';

type SessionSnapshot = {
  token: string;
  username: string;
  displayName: string;
  expiresAt: string;
  sessionId: string | null;
  capabilities: CapabilityKey[];
};

let cachedSession: SessionSnapshot | null = null;
let hydrated = false;

function readSessionFromStorage(): SessionSnapshot | null {
  if (typeof window === 'undefined' || !window.localStorage) {
    return null;
  }

  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return null;
    }

    const parsed = JSON.parse(raw) as Partial<SessionSnapshot> | null;
    if (!parsed || typeof parsed.token !== 'string') {
      return null;
    }

    return {
      token: parsed.token,
      username: parsed.username ?? 'admin',
      displayName: parsed.displayName ?? parsed.username ?? 'Admin',
      expiresAt: parsed.expiresAt ?? '',
      sessionId: typeof parsed.sessionId === 'string' ? parsed.sessionId : null,
      capabilities: Array.isArray(parsed.capabilities)
        ? (parsed.capabilities.filter(
            (value): value is CapabilityKey => typeof value === 'string',
          ) as CapabilityKey[])
        : [],
    } satisfies SessionSnapshot;
  } catch (error) {
    console.warn('[session] failed to parse stored session snapshot', error);
    return null;
  }
}

function writeSessionToStorage(session: SessionSnapshot | null) {
  if (typeof window === 'undefined' || !window.localStorage) {
    return;
  }

  if (!session) {
    window.localStorage.removeItem(STORAGE_KEY);
    return;
  }

  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  } catch (error) {
    console.warn('[session] failed to persist session snapshot', error);
  }
}

function resolveEnvSession(): SessionSnapshot | null {
  const token = import.meta.env?.VITE_API_SESSION_TOKEN;
  if (!token || typeof token !== 'string' || token.trim().length === 0) {
    return null;
  }

  const username = import.meta.env?.VITE_API_SESSION_USERNAME ?? 'env-admin';
  const displayName = import.meta.env?.VITE_API_SESSION_DISPLAY_NAME ?? username;
  const expiresAt = import.meta.env?.VITE_API_SESSION_EXPIRES_AT ?? '';
  const sessionIdEnv = import.meta.env?.VITE_API_SESSION_ID;
  const sessionId =
    typeof sessionIdEnv === 'string' && sessionIdEnv.trim().length > 0 ? sessionIdEnv.trim() : null;

  const capabilitiesEnv = import.meta.env?.VITE_API_SESSION_CAPABILITIES;
  const capabilities =
    typeof capabilitiesEnv === 'string' && capabilitiesEnv.trim().length > 0
      ? (capabilitiesEnv
          .split(',')
          .map((value) => value.trim())
          .filter((value) => value.length > 0) as CapabilityKey[])
      : [];

  return {
    token: token.trim(),
    username,
    displayName,
    expiresAt,
    sessionId,
    capabilities,
  } satisfies SessionSnapshot;
}

export function getSessionSnapshot(): SessionSnapshot | null {
  if (!hydrated) {
    cachedSession = readSessionFromStorage();
    hydrated = true;

    if (!cachedSession) {
      cachedSession = resolveEnvSession();
    }
  }

  return cachedSession;
}

export function setSessionSnapshot(
  session: SessionSnapshot | null,
  options: { persist?: boolean } = {},
): SessionSnapshot | null {
  const persist = options.persist ?? true;
  cachedSession = session;
  hydrated = true;

  if (persist) {
    writeSessionToStorage(session);
  }

  return cachedSession;
}

export function clearSessionSnapshot(): void {
  cachedSession = null;
  hydrated = true;
  writeSessionToStorage(null);
}

export function getSessionToken(): string | null {
  const snapshot = getSessionSnapshot();
  return snapshot?.token ?? null;
}

export function isSessionExpired(): boolean {
  const snapshot = getSessionSnapshot();
  if (!snapshot) {
    return true;
  }

  if (!snapshot.expiresAt) {
    return false;
  }

  const expiresAtMs = Date.parse(snapshot.expiresAt);
  if (Number.isNaN(expiresAtMs)) {
    return false;
  }

  return expiresAtMs <= Date.now();
}

export type { SessionSnapshot };
