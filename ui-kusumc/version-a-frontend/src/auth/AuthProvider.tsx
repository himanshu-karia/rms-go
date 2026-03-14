import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import {
  login as loginRequest,
  logout as logoutRequest,
  refreshSession as refreshSessionRequest,
  type LoginPayload,
} from '../api/auth';
import {
  hasCapabilities,
  type CapabilityKey,
  type CapabilityMatchOptions,
} from '../api/capabilities';
import {
  clearSessionSnapshot,
  getSessionSnapshot,
  setSessionSnapshot,
  type SessionSnapshot,
} from '../api/session';

export type AuthContextValue = {
  session: SessionSnapshot | null;
  user: { username: string; displayName: string } | null;
  isAuthenticated: boolean;
  login: (payload: LoginPayload) => Promise<SessionSnapshot>;
  logout: () => Promise<void>;
  refresh: () => Promise<SessionSnapshot>;
  capabilities: CapabilityKey[];
  hasCapability: (
    required: CapabilityKey | CapabilityKey[],
    options?: CapabilityMatchOptions,
  ) => boolean;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);
const FORCED_LOGOUT_EVENT = 'pmkusum:forced-logout';

function isSessionValid(session: SessionSnapshot | null): boolean {
  if (!session) {
    return false;
  }

  if (!session.expiresAt) {
    return true;
  }

  const expiresAtMs = Date.parse(session.expiresAt);
  if (Number.isNaN(expiresAtMs)) {
    return true;
  }

  return expiresAtMs > Date.now();
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionSnapshot | null>(() => getSessionSnapshot());

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    const handleForcedLogout = () => {
      clearSessionSnapshot();
      setSession(null);
      if (window.location.pathname !== '/login') {
        window.location.replace('/login');
      }
    };

    window.addEventListener(FORCED_LOGOUT_EVENT, handleForcedLogout);
    return () => {
      window.removeEventListener(FORCED_LOGOUT_EVENT, handleForcedLogout);
    };
  }, []);

  const login = useCallback(async (payload: LoginPayload) => {
    const response = await loginRequest(payload);

    const snapshot: SessionSnapshot = {
      token: response.tokens.access.token,
      username: response.user.username,
      displayName: response.user.displayName,
      expiresAt: response.tokens.access.expiresAt,
      sessionId: response.session.id,
      capabilities: Array.isArray(response.user.capabilities) ? response.user.capabilities : [],
    };

    setSessionSnapshot(snapshot);
    setSession(snapshot);

    return snapshot;
  }, []);

  const logout = useCallback(async () => {
    try {
      await logoutRequest();
    } finally {
      clearSessionSnapshot();
      setSession(null);
      if (typeof window !== 'undefined') {
        window.location.replace('/login');
      }
    }
  }, []);

  const refresh = useCallback(async () => {
    const response = await refreshSessionRequest();

    const snapshot: SessionSnapshot = {
      token: response.tokens.access.token,
      username: response.user.username,
      displayName: response.user.displayName,
      expiresAt: response.tokens.access.expiresAt,
      sessionId: response.session.id,
      capabilities: Array.isArray(response.user.capabilities) ? response.user.capabilities : [],
    };

    setSessionSnapshot(snapshot);
    setSession(snapshot);

    return snapshot;
  }, []);

  const value = useMemo<AuthContextValue>(() => {
    const valid = isSessionValid(session);
    const capabilities = valid && session ? session.capabilities : [];
    const hasCapability = (
      required: CapabilityKey | CapabilityKey[],
      options?: CapabilityMatchOptions,
    ) => hasCapabilities(capabilities, required, options);

    return {
      session: valid ? session : null,
      user:
        valid && session ? { username: session.username, displayName: session.displayName } : null,
      isAuthenticated: valid,
      login,
      logout,
      refresh,
      capabilities,
      hasCapability,
    };
  }, [session, login, logout, refresh]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }

  return context;
}
