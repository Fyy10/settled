import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { clearCsrfToken, getCsrfToken, setCsrfToken } from './csrf';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
	clearCsrfToken();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('in-memory CSRF state', () => {
	it('returns a cached token until a forced refresh is requested', async () => {
		setCsrfToken('cached-token');

		await expect(getCsrfToken()).resolves.toBe('cached-token');
		expect(fetchMock).not.toHaveBeenCalled();

		fetchMock.mockResolvedValue(jsonResponse({ csrfToken: 'refreshed-token' }));
		await expect(getCsrfToken({ force: true })).resolves.toBe('refreshed-token');
		await expect(getCsrfToken()).resolves.toBe('refreshed-token');
		expect(fetchMock).toHaveBeenCalledOnce();
	});

	it('coalesces forced refreshes even when a cached token exists', async () => {
		setCsrfToken('cached-token');
		const response = deferred<Response>();
		fetchMock.mockReturnValue(response.promise);

		const first = getCsrfToken({ force: true });
		const second = getCsrfToken({ force: true });
		await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());

		response.resolve(jsonResponse({ csrfToken: 'rotated-token' }));
		await expect(Promise.all([first, second])).resolves.toEqual([
			'rotated-token',
			'rotated-token'
		]);
		expect(fetchMock).toHaveBeenCalledOnce();
	});

	it('clears a failed in-flight refresh so a later request can recover', async () => {
		fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch'));

		const first = getCsrfToken();
		const second = getCsrfToken();
		const failures = await Promise.allSettled([first, second]);

		expect(failures.every(({ status }) => status === 'rejected')).toBe(true);
		expect(fetchMock).toHaveBeenCalledOnce();

		fetchMock.mockResolvedValueOnce(jsonResponse({ csrfToken: 'recovered-token' }));
		await expect(getCsrfToken()).resolves.toBe('recovered-token');
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it('does not let an older refresh overwrite an authenticated rotation', async () => {
		const response = deferred<Response>();
		fetchMock.mockReturnValue(response.promise);
		const anonymousRefresh = getCsrfToken();
		await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());

		setCsrfToken('authenticated-token');
		response.resolve(jsonResponse({ csrfToken: 'stale-anonymous-token' }));

		await expect(anonymousRefresh).resolves.toBe('authenticated-token');
		await expect(getCsrfToken()).resolves.toBe('authenticated-token');
	});
});

function jsonResponse(body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});
}

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
} {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((resolvePromise) => {
		resolve = resolvePromise;
	});

	return { promise, resolve };
}
