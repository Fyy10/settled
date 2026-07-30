import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { LOCAL_API_BASE_URL } from '$lib/config/api-base-url';
import { clearCsrfToken, setCsrfToken } from '$lib/state/csrf';

import {
	ApiError,
	onUnauthorized,
	request,
	type RequestOptions
} from './client';

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

describe('credentialed API transport', () => {
	it('builds an absolute API URL and always sends credentials and Accept', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ user: { id: 'user-one' } }));

		await expect(request('/api/me')).resolves.toEqual({ user: { id: 'user-one' } });

		const [url, init] = fetchMock.mock.calls[0];
		const headers = new Headers(init?.headers);
		expect(url).toBe(`${LOCAL_API_BASE_URL}/api/me`);
		expect(init).toMatchObject({
			method: 'GET',
			credentials: 'include'
		});
		expect(headers.get('Accept')).toBe('application/json');
		expect(headers.has('Content-Type')).toBe(false);
	});

	it('serializes JSON only when a body exists and passes the AbortSignal', async () => {
		const controller = new AbortController();
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'csrf-one' }))
			.mockResolvedValueOnce(jsonResponse({ group: { id: 'group-one' } }, 201));

		await request('/api/groups', {
			method: 'POST',
			body: { name: 'Lake Trip' },
			signal: controller.signal
		});

		const csrfInit = fetchMock.mock.calls[0][1];
		const mutationInit = fetchMock.mock.calls[1][1];
		const csrfHeaders = new Headers(csrfInit?.headers);
		const mutationHeaders = new Headers(mutationInit?.headers);

		expect(csrfHeaders.has('Content-Type')).toBe(false);
		expect(mutationInit?.body).toBe('{"name":"Lake Trip"}');
		expect(mutationInit?.signal).toBe(controller.signal);
		expect(mutationHeaders.get('Content-Type')).toBe('application/json');
		expect(mutationHeaders.get('X-CSRF-Token')).toBe('csrf-one');
	});

	it('returns undefined for 204 without attempting JSON parsing', async () => {
		fetchMock.mockResolvedValue(new Response(undefined, { status: 204 }));

		await expect(request<void>('/api/health/live')).resolves.toBeUndefined();
	});

	it.each([
		['a non-JSON success', new Response('ok', { status: 200 })],
		[
			'malformed success JSON',
			new Response('{', {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
		],
		['a primitive JSON success', jsonResponse('ok')]
	])('rejects %s as an invalid response', async (_description, response) => {
		fetchMock.mockResolvedValue(response);

		await expect(request('/api/health/live')).rejects.toMatchObject({
			name: 'ApiError',
			status: 200,
			code: 'invalid_response',
			source: 'invalid-response'
		});
	});

	it('parses stable HTTP errors, fields, and request IDs', async () => {
		fetchMock.mockResolvedValue(
			jsonResponse(
				{
					error: {
						code: 'validation_failed',
						message: 'One or more fields are invalid.',
						fields: { email: 'Email is required.' }
					}
				},
				422,
				{ 'X-Request-ID': 'request-123' }
			)
		);

		await expect(request('/api/me')).rejects.toMatchObject({
			name: 'ApiError',
			status: 422,
			code: 'validation_failed',
			message: 'One or more fields are invalid.',
			fields: { email: 'Email is required.' },
			requestId: 'request-123',
			source: 'http'
		});
	});

	it.each([
		new Response('Service unavailable', { status: 503 }),
		new Response('{', {
			status: 500,
			headers: { 'Content-Type': 'application/json' }
		}),
		jsonResponse({ message: 'Missing stable envelope.' }, 500)
	])('turns an unknown HTTP error shape into a stable unknown error', async (response) => {
		fetchMock.mockResolvedValue(response);

		await expect(request('/api/me')).rejects.toMatchObject({
			name: 'ApiError',
			code: 'unknown_error',
			message: 'The server returned an unexpected error response.',
			source: 'http'
		});
	});

	it('distinguishes network failure and preserves AbortError', async () => {
		fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch'));

		await expect(request('/api/me')).rejects.toMatchObject({
			name: 'ApiError',
			status: 0,
			code: 'network_error',
			source: 'network'
		});

		const abortError = new DOMException('Aborted', 'AbortError');
		fetchMock.mockRejectedValueOnce(abortError);
		await expect(request('/api/me')).rejects.toBe(abortError);
	});

	it.each([
		['https://evil.example/api/me', {}],
		['//evil.example/api/me', {}],
		['/health/live', {}],
		['/api/groups', { method: 'POST', csrf: false }]
	] satisfies Array<[string, RequestOptions]>)(
		'rejects an API path or CSRF bypass before fetching: %s',
		async (path, options) => {
			await expect(request(path, options)).rejects.toBeInstanceOf(TypeError);
			expect(fetchMock).not.toHaveBeenCalled();
		}
	);
});

describe('CSRF request lifecycle', () => {
	it('coalesces simultaneous token acquisition for unsafe requests', async () => {
		const csrfResponse = deferred<Response>();
		fetchMock.mockImplementation((input) => {
			if (String(input).endsWith('/api/auth/csrf')) {
				return csrfResponse.promise;
			}

			return Promise.resolve(jsonResponse({ ok: true }));
		});

		const first = request('/api/groups', { method: 'POST', body: { name: 'One' } });
		const second = request('/api/groups', { method: 'POST', body: { name: 'Two' } });

		await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
		csrfResponse.resolve(jsonResponse({ csrfToken: 'shared-token' }));
		await Promise.all([first, second]);

		expect(fetchMock).toHaveBeenCalledTimes(3);
		for (const call of fetchMock.mock.calls.slice(1)) {
			expect(new Headers(call[1]?.headers).get('X-CSRF-Token')).toBe('shared-token');
		}
	});

	it('uses an explicitly rotated token without fetching another one', async () => {
		setCsrfToken('rotated-token');
		fetchMock.mockResolvedValue(jsonResponse({ ok: true }));

		await request('/api/groups', { method: 'POST', body: { name: 'Lake Trip' } });

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(new Headers(fetchMock.mock.calls[0][1]?.headers).get('X-CSRF-Token')).toBe(
			'rotated-token'
		);
	});

	it('refreshes a rejected token and retries the original request once', async () => {
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'old-token' }))
			.mockResolvedValueOnce(csrfError('csrf_invalid'))
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'new-token' }))
			.mockResolvedValueOnce(jsonResponse({ group: { id: 'group-one' } }, 201));

		await expect(
			request('/api/groups', { method: 'POST', body: { name: 'Lake Trip' } })
		).resolves.toEqual({ group: { id: 'group-one' } });

		expect(fetchMock).toHaveBeenCalledTimes(4);
		expect(new Headers(fetchMock.mock.calls[1][1]?.headers).get('X-CSRF-Token')).toBe(
			'old-token'
		);
		expect(new Headers(fetchMock.mock.calls[3][1]?.headers).get('X-CSRF-Token')).toBe(
			'new-token'
		);
	});

	it.each(['csrf_required', 'csrf_invalid'] as const)(
		'surfaces a second %s response without a retry loop',
		async (code) => {
			fetchMock
				.mockResolvedValueOnce(jsonResponse({ csrfToken: 'old-token' }))
				.mockResolvedValueOnce(csrfError(code))
				.mockResolvedValueOnce(jsonResponse({ csrfToken: 'new-token' }))
				.mockResolvedValueOnce(csrfError(code));

			await expect(
				request('/api/groups', { method: 'POST', body: { name: 'Lake Trip' } })
			).rejects.toMatchObject({ status: 403, code });
			expect(fetchMock).toHaveBeenCalledTimes(4);
		}
	);

	it('does not retry CSRF errors when explicitly disabled', async () => {
		setCsrfToken('old-token');
		fetchMock.mockResolvedValue(csrfError('csrf_required'));

		await expect(
			request('/api/groups', {
				method: 'POST',
				body: { name: 'Lake Trip' },
				retryCsrf: false
			})
		).rejects.toMatchObject({ code: 'csrf_required' });
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it('does not retry network or server failures', async () => {
		setCsrfToken('current-token');
		fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch'));

		await expect(request('/api/groups', { method: 'POST' })).rejects.toMatchObject({
			code: 'network_error'
		});
		expect(fetchMock).toHaveBeenCalledTimes(1);

		fetchMock.mockResolvedValueOnce(
			jsonResponse(
				{ error: { code: 'internal_error', message: 'Unexpected failure.' } },
				500
			)
		);
		await expect(request('/api/groups', { method: 'POST' })).rejects.toMatchObject({
			code: 'internal_error'
		});
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it('notifies on 401 and clears the cached token, but not on network failure', async () => {
		setCsrfToken('session-token');
		const unauthorized = vi.fn();
		const unsubscribe = onUnauthorized(unauthorized);
		fetchMock.mockResolvedValueOnce(
			jsonResponse(
				{ error: { code: 'unauthorized', message: 'Authentication is required.' } },
				401
			)
		);

		await expect(request('/api/me')).rejects.toMatchObject({ status: 401 });
		expect(unauthorized).toHaveBeenCalledOnce();

		fetchMock
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'anonymous-token' }))
			.mockResolvedValueOnce(jsonResponse({ ok: true }));
		await request('/api/groups/join', { method: 'POST', body: { joinCode: 'ABCD1234' } });
		expect(String(fetchMock.mock.calls[1][0])).toBe(
			`${LOCAL_API_BASE_URL}/api/auth/csrf`
		);

		fetchMock.mockRejectedValueOnce(new TypeError('Failed to fetch'));
		await expect(request('/api/me')).rejects.toBeInstanceOf(ApiError);
		expect(unauthorized).toHaveBeenCalledOnce();
		unsubscribe();
	});
});

function jsonResponse(
	body: unknown,
	status = 200,
	headers: Record<string, string> = {}
): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: {
			'Content-Type': 'application/json',
			...headers
		}
	});
}

function csrfError(code: 'csrf_required' | 'csrf_invalid'): Response {
	return jsonResponse(
		{
			error: {
				code,
				message: 'The CSRF token is missing or invalid.'
			}
		},
		403
	);
}

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (reason?: unknown) => void;
} {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((resolvePromise, rejectPromise) => {
		resolve = resolvePromise;
		reject = rejectPromise;
	});

	return { promise, resolve, reject };
}
