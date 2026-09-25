import { fetchCsrfToken } from '$lib/api/csrf';

type GetCsrfTokenOptions = {
	force?: boolean;
};

let csrfToken: string | null = null;
let refreshPromise: Promise<string> | null = null;
let generation = 0;

export function getCsrfToken({ force = false }: GetCsrfTokenOptions = {}): Promise<string> {
	if (!force && csrfToken !== null) {
		return Promise.resolve(csrfToken);
	}

	if (refreshPromise !== null) {
		return refreshPromise;
	}

	const refreshGeneration = generation;
	let currentRefresh: Promise<string>;
	currentRefresh = fetchCsrfToken()
		.then((token) => {
			if (generation !== refreshGeneration) {
				if (csrfToken !== null) {
					return csrfToken;
				}

				throw new Error('The CSRF token changed while it was being refreshed.');
			}

			csrfToken = token;
			return token;
		})
		.finally(() => {
			if (refreshPromise === currentRefresh) {
				refreshPromise = null;
			}
		});

	refreshPromise = currentRefresh;
	return currentRefresh;
}

export function setCsrfToken(token: string): void {
	if (token.length === 0) {
		throw new TypeError('The CSRF token must not be empty.');
	}

	generation += 1;
	csrfToken = token;
	refreshPromise = null;
}

export function clearCsrfToken(): void {
	generation += 1;
	csrfToken = null;
	refreshPromise = null;
}
