const DEFAULT_DEV_API_BASE = '/api';

const normalize = (value: string) => (value.endsWith('/') ? value.slice(0, -1) : value);

const resolveSameOrigin = () => {
	if (typeof window === 'undefined' || !window.location?.origin) {
		return DEFAULT_DEV_API_BASE;
	}

	try {
		const sameOriginUrl = new URL('/api', window.location.origin);
		return normalize(sameOriginUrl.toString());
	} catch {
		return DEFAULT_DEV_API_BASE;
	}
};

const resolveFromEnv = () => {
	const raw = import.meta.env.VITE_API_BASE_URL;
	if (typeof raw !== 'string') {
		return null;
	}

	const trimmed = raw.trim();
	if (!trimmed) {
		return null;
	}

	if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
		return normalize(trimmed);
	}

	if (trimmed.startsWith('/')) {
		return normalize(trimmed);
	}

	if (typeof window !== 'undefined' && window.location?.origin) {
		try {
			const relativeToOrigin = new URL(trimmed, window.location.origin);
			return normalize(relativeToOrigin.toString());
		} catch {
			return normalize(trimmed);
		}
	}

	return normalize(trimmed);
};

// Default to the same-origin API when no explicit URL is provided so that CSP policies remain satisfied.
export const API_BASE_URL = resolveFromEnv() ?? resolveSameOrigin();
