import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { commonErrorFixtures, currentUserFixture } from '../../tests/fixtures/api-contract';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
	vi.resetModules();
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('current-user auth state', () => {
	it('coalesces concurrent authoritative session checks', async () => {
		const response = deferred<Response>();
		fetchMock.mockReturnValue(response.promise);
		const { authState, ensureSession } = await import('./auth.svelte');

		const first = ensureSession();
		const second = ensureSession();
		expect(first).toBe(second);
		await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());

		response.resolve(jsonResponse(currentUserFixture));
		await expect(Promise.all([first, second])).resolves.toEqual([
			{ status: 'authenticated', user: currentUserFixture.user },
			{ status: 'authenticated', user: currentUserFixture.user }
		]);
		expect(authState.current).toEqual({
			status: 'authenticated',
			user: currentUserFixture.user
		});
	});

	it('treats an initial 401 as anonymous without reporting session expiration', async () => {
		fetchMock.mockResolvedValue(
			jsonResponse(commonErrorFixtures.unauthorized, 401)
		);
		const { authState, ensureSession, onSessionExpired } = await import('./auth.svelte');
		const expired = vi.fn();
		onSessionExpired(expired);

		await expect(ensureSession()).resolves.toEqual({ status: 'anonymous', user: null });
		expect(authState.current).toEqual({ status: 'anonymous', user: null });
		expect(expired).not.toHaveBeenCalled();
	});

	it('leaves unknown state intact on a network failure and permits a later retry', async () => {
		fetchMock
			.mockRejectedValueOnce(new TypeError('Failed to fetch'))
			.mockResolvedValueOnce(jsonResponse(currentUserFixture));
		const { authState, ensureSession } = await import('./auth.svelte');

		await expect(ensureSession()).rejects.toMatchObject({ code: 'network_error' });
		expect(authState.current).toEqual({ status: 'unknown', user: null });

		await expect(ensureSession()).resolves.toEqual({
			status: 'authenticated',
			user: currentUserFixture.user
		});
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it('clears an authenticated session and reports expiration on a later 401', async () => {
		fetchMock.mockResolvedValue(
			jsonResponse(commonErrorFixtures.unauthorized, 401)
		);
		const { authState, onSessionExpired, setAuthenticated } = await import('./auth.svelte');
		const { request } = await import('$lib/api/client');
		const expired = vi.fn();

		setAuthenticated(currentUserFixture.user);
		onSessionExpired(expired);

		await expect(request('/api/groups')).rejects.toMatchObject({ status: 401 });
		expect(authState.current).toEqual({ status: 'anonymous', user: null });
		expect(expired).toHaveBeenCalledOnce();
	});

	it('retains an authenticated session on a later network failure', async () => {
		fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));
		const { authState, onSessionExpired, setAuthenticated } = await import('./auth.svelte');
		const { request } = await import('$lib/api/client');
		const expired = vi.fn();

		setAuthenticated(currentUserFixture.user);
		onSessionExpired(expired);

		await expect(request('/api/groups')).rejects.toMatchObject({ code: 'network_error' });
		expect(authState.current).toEqual({
			status: 'authenticated',
			user: currentUserFixture.user
		});
		expect(expired).not.toHaveBeenCalled();
	});
});

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
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
