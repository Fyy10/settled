import { getCurrentUser } from '$lib/api/auth';
import { isUnauthorizedApiError } from '$lib/api/errors';
import { onUnauthorized } from '$lib/api/client';
import type { User } from '$lib/api/types';

import { clearCsrfToken } from './csrf';

export type AuthState =
	| { status: 'unknown'; user: null }
	| { status: 'authenticated'; user: User }
	| { status: 'anonymous'; user: null };

type AuthStateContainer = {
	readonly current: AuthState;
};

type MutableAuthStateContainer = {
	current: AuthState;
};

type SessionExpiredListener = () => void;

const mutableAuthState = $state<MutableAuthStateContainer>({
	current: { status: 'unknown', user: null }
});
const sessionExpiredListeners = new Set<SessionExpiredListener>();
let ensureSessionPromise: Promise<AuthState> | null = null;

export const authState: AuthStateContainer = {
	get current() {
		return mutableAuthState.current;
	}
};

export function ensureSession(): Promise<AuthState> {
	if (mutableAuthState.current.status !== 'unknown') {
		return Promise.resolve(mutableAuthState.current);
	}

	if (ensureSessionPromise !== null) {
		return ensureSessionPromise;
	}

	let currentRequest: Promise<AuthState>;
	currentRequest = getCurrentUser()
		.then((user) => {
			setAuthenticated(user);
			return mutableAuthState.current;
		})
		.catch((error: unknown) => {
			if (isUnauthorizedApiError(error)) {
				return mutableAuthState.current;
			}

			throw error;
		})
		.finally(() => {
			if (ensureSessionPromise === currentRequest) {
				ensureSessionPromise = null;
			}
		});

	ensureSessionPromise = currentRequest;
	return currentRequest;
}

export function setAuthenticated(user: User): void {
	mutableAuthState.current = { status: 'authenticated', user };
}

export function clearSession(): void {
	clearCsrfToken();
	mutableAuthState.current = { status: 'anonymous', user: null };
}

export function onSessionExpired(listener: SessionExpiredListener): () => void {
	sessionExpiredListeners.add(listener);

	return () => {
		sessionExpiredListeners.delete(listener);
	};
}

onUnauthorized(() => {
	const sessionWasAuthenticated = mutableAuthState.current.status === 'authenticated';
	clearSession();

	if (sessionWasAuthenticated) {
		for (const listener of sessionExpiredListeners) {
			try {
				listener();
			} catch {
				// A navigation listener must not replace the authoritative API error.
			}
		}
	}
});
