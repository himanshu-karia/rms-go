import { API_BASE_URL } from './config';
import { apiFetch, readJsonBody } from './http';
import type { CapabilityKey } from './capabilities';
import { getSessionSnapshot } from './session';

export type LoginPayload = {
  username: string;
  password: string;
};

export type LoginResponse = {
  user: {
    username: string;
    displayName: string;
    capabilities: CapabilityKey[];
  };
  session: {
    id: string;
  };
  tokens: {
    access: {
      token: string;
      expiresAt: string;
    };
    refresh: {
      token: string | null;
      expiresAt: string;
    };
  };
};

export type SessionIntrospectionResponse = {
  session: {
    id: string;
    issuedAt: string;
    expiresAt: string;
    remainingSeconds: number;
  };
  user: {
    id: string;
    username: string;
    displayName: string;
    capabilities: CapabilityKey[];
    mustRotatePassword: boolean;
    roles: Array<{
      id: string;
      key: string;
      name: string;
      capabilities: CapabilityKey[];
      scope: Record<string, unknown> | null;
    }>;
  };
};

type JwtPayload = {
  exp?: number;
  id?: string;
  name?: string;
  username?: string;
  session_id?: string;
  capabilities?: unknown;
};

let latestRefreshToken: string | null = null;

function decodeJwtPayload(token: string | null | undefined): JwtPayload | null {
  if (!token || typeof token !== 'string') {
    return null;
  }

  const parts = token.split('.');
  if (parts.length < 2 || !parts[1]) {
    return null;
  }

  try {
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
    const decoded = atob(padded);
    return JSON.parse(decoded) as JwtPayload;
  } catch {
    return null;
  }
}

function normalizeCapabilities(value: unknown): CapabilityKey[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter((item): item is CapabilityKey => typeof item === 'string');
}

function normalizeAuthResponse(body: any): LoginResponse {
  const accessToken =
    typeof body?.tokens?.access?.token === 'string'
      ? body.tokens.access.token
      : typeof body?.token === 'string'
        ? body.token
        : null;

  if (!accessToken) {
    throw new Error('Unexpected authentication response');
  }

  const jwt = decodeJwtPayload(accessToken);
  const accessExpiresAt =
    typeof body?.tokens?.access?.expiresAt === 'string'
      ? body.tokens.access.expiresAt
      : typeof body?.access?.expiresAt === 'string'
        ? body.access.expiresAt
        : typeof jwt?.exp === 'number'
          ? new Date(jwt.exp * 1000).toISOString()
          : '';

  const refreshToken =
    typeof body?.tokens?.refresh?.token === 'string'
      ? body.tokens.refresh.token
      : typeof body?.refresh?.token === 'string'
        ? body.refresh.token
        : null;

  if (refreshToken) {
    latestRefreshToken = refreshToken;
  }

  const refreshExpiresAt =
    typeof body?.tokens?.refresh?.expiresAt === 'string'
      ? body.tokens.refresh.expiresAt
      : typeof body?.refresh?.expiresAt === 'string'
        ? body.refresh.expiresAt
        : '';

  const username =
    (typeof body?.user?.username === 'string' && body.user.username) ||
    (typeof body?.name === 'string' && body.name) ||
    (typeof body?.email === 'string' && body.email) ||
    (typeof jwt?.username === 'string' && jwt.username) ||
    (typeof jwt?.name === 'string' && jwt.name) ||
    'admin';

  const userCapabilities = normalizeCapabilities(body?.user?.capabilities);
  const bodyCapabilities = normalizeCapabilities(body?.capabilities);
  const jwtCapabilities = normalizeCapabilities(jwt?.capabilities);
  const capabilities =
    userCapabilities.length > 0
      ? userCapabilities
      : bodyCapabilities.length > 0
        ? bodyCapabilities
        : jwtCapabilities;

  return {
    user: {
      username,
      displayName: username,
      capabilities,
    },
    session: {
      id:
        (typeof body?.session?.id === 'string' && body.session.id) ||
        (typeof jwt?.session_id === 'string' ? jwt.session_id : ''),
    },
    tokens: {
      access: {
        token: accessToken,
        expiresAt: accessExpiresAt,
      },
      refresh: {
        token: refreshToken,
        expiresAt: refreshExpiresAt,
      },
    },
  };
}

export async function login(payload: LoginPayload): Promise<LoginResponse> {
  const response = await apiFetch(`${API_BASE_URL}/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = (body as { message?: string } | null)?.message ?? 'Unable to authenticate';
    throw new Error(message);
  }

  return normalizeAuthResponse(body);
}

export async function logout(): Promise<void> {
  const response = await apiFetch(`${API_BASE_URL}/auth/logout`, {
    method: 'POST',
  });

  if (!response.ok) {
    const body = await readJsonBody<any>(response);
    const message = body?.message ?? 'Unable to log out';
    throw new Error(message);
  }
}

export async function refreshSession(): Promise<LoginResponse> {
  const snapshot = getSessionSnapshot();
  const refreshToken = latestRefreshToken;

  const response = await apiFetch(`${API_BASE_URL}/auth/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ refreshToken }),
  });

  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = (body as { message?: string } | null)?.message ?? 'Unable to refresh session';
    // If refresh endpoint shape differs or refresh token is unavailable, keep existing session active.
    if (snapshot?.token) {
      return {
        user: {
          username: snapshot.username,
          displayName: snapshot.displayName,
          capabilities: snapshot.capabilities,
        },
        session: {
          id: snapshot.sessionId ?? '',
        },
        tokens: {
          access: {
            token: snapshot.token,
            expiresAt: snapshot.expiresAt,
          },
          refresh: {
            token: refreshToken,
            expiresAt: '',
          },
        },
      };
    }

    throw new Error(message);
  }

  return normalizeAuthResponse(body);
}

export async function introspectSession(): Promise<SessionIntrospectionResponse> {
  const response = await apiFetch(`${API_BASE_URL}/auth/session`);
  const body = await readJsonBody<any>(response);

  if (!response.ok || !body) {
    const message = (body as { message?: string } | null)?.message ?? 'Unable to load session';
    throw new Error(message);
  }

  if (body?.session && body?.user) {
    return body as SessionIntrospectionResponse;
  }

  const snapshot = getSessionSnapshot();
  const jwt = decodeJwtPayload(snapshot?.token);
  const expiresAt =
    snapshot?.expiresAt && snapshot.expiresAt.length > 0
      ? snapshot.expiresAt
      : typeof jwt?.exp === 'number'
        ? new Date(jwt.exp * 1000).toISOString()
        : '';

  const nowMs = Date.now();
  const expiresMs = expiresAt ? Date.parse(expiresAt) : Number.NaN;
  const remainingSeconds = Number.isNaN(expiresMs)
    ? 0
    : Math.max(0, Math.floor((expiresMs - nowMs) / 1000));

  return {
    session: {
      id: snapshot?.sessionId ?? '',
      issuedAt: '',
      expiresAt,
      remainingSeconds,
    },
    user: {
      id: typeof body?.id === 'string' ? body.id : '',
      username:
        (typeof body?.name === 'string' && body.name) ||
        snapshot?.username ||
        (typeof body?.email === 'string' ? body.email : 'admin'),
      displayName:
        (typeof body?.name === 'string' && body.name) || snapshot?.displayName || 'Admin',
      capabilities: snapshot?.capabilities ?? normalizeCapabilities(jwt?.capabilities),
      mustRotatePassword: false,
      roles: [],
    },
  };
}
