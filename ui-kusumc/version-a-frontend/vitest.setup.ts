import '@testing-library/jest-dom/vitest';
import type { LoginResponse, SessionIntrospectionResponse } from './src/api/auth';
import { afterAll, afterEach, beforeEach, vi } from 'vitest';

const originalFetch = globalThis.fetch;
const fetchSpy = vi.spyOn(globalThis, 'fetch');

function resolveUrl(input: RequestInfo | URL): string {
	if (typeof input === 'string') {
		return input;
	}
	if (input instanceof URL) {
		return input.toString();
	}
	if (typeof Request !== 'undefined' && input instanceof Request) {
		return input.url;
	}
	return String(input);
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
	const headers = new Headers(init.headers ?? {});
	if (!headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}
	return new Response(JSON.stringify(body), {
		...init,
		headers,
	});
}

function buildSessionIntrospection(): SessionIntrospectionResponse {
	const now = Date.now();
	const issuedAt = new Date(now - 60_000);
	const expiresAt = new Date(now + 15 * 60 * 1000);
	return {
		session: {
			id: 'session-vitest',
			issuedAt: issuedAt.toISOString(),
			expiresAt: expiresAt.toISOString(),
			remainingSeconds: Math.max(0, Math.floor((expiresAt.getTime() - Date.now()) / 1000)),
		},
		user: {
			id: 'user-vitest',
			username: 'vitest',
			displayName: 'Vitest User',
			capabilities: [],
			mustRotatePassword: false,
			roles: [],
		},
		};
}

	function buildLoginResponse(): LoginResponse {
	const now = Date.now();
	const accessExpiresAt = new Date(now + 15 * 60 * 1000).toISOString();
	const refreshExpiresAt = new Date(now + 24 * 60 * 60 * 1000).toISOString();
	return {
		user: {
			username: 'vitest',
			displayName: 'Vitest User',
			capabilities: [],
		},
		session: {
			id: 'session-vitest',
		},
		tokens: {
			access: {
				token: 'access-token-vitest',
				expiresAt: accessExpiresAt,
			},
			refresh: {
				token: 'refresh-token-vitest',
				expiresAt: refreshExpiresAt,
			},
		},
		};
}

beforeEach(() => {
	fetchSpy.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = resolveUrl(input);

		if (url.includes('/auth/session')) {
			return jsonResponse(buildSessionIntrospection());
		}

		if (url.includes('/auth/refresh')) {
			return jsonResponse(buildLoginResponse());
		}

		return jsonResponse(
			{
				message: `Unhandled fetch in tests: ${url}`,
			},
			{ status: 404 },
		);
	});
});

afterEach(() => {
	fetchSpy.mockReset();
});

afterAll(() => {
	fetchSpy.mockRestore();
	if (originalFetch) {
		globalThis.fetch = originalFetch;
	}
});
