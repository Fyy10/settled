import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { currentUserFixture } from '../../tests/fixtures/api-contract';
import { clearCsrfToken, setCsrfToken } from '../state/csrf';
import { request } from './client';
import { getCurrentUser, login, logout, register } from './auth';

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

describe('authentication API', () => {
	it('logs in with UTF-8 Basic Auth, no body, and stores the rotated CSRF token', async () => {
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'anonymous-token' }))
			.mockResolvedValueOnce(
				jsonResponse({
					csrfToken: 'authenticated-token',
					user: currentUserFixture.user
				})
			)
			.mockResolvedValueOnce(jsonResponse({ ok: true }));

		await expect(
			login({ email: 'alice@example.com', password: 'påss🔐漢字' })
		).resolves.toEqual(currentUserFixture.user);

		const loginInit = fetchMock.mock.calls[1][1];
		const loginHeaders = new Headers(loginInit?.headers);
		expect(loginInit?.method).toBe('POST');
		expect(loginInit?.body).toBeUndefined();
		expect(loginHeaders.has('Content-Type')).toBe(false);
		expect(decodeBasicAuthorization(loginHeaders.get('Authorization'))).toBe(
			'alice@example.com:påss🔐漢字'
		);
		expect(loginHeaders.get('X-CSRF-Token')).toBe('anonymous-token');

		await request('/api/groups', { method: 'POST' });
		expect(new Headers(fetchMock.mock.calls[2][1]?.headers).get('X-CSRF-Token')).toBe(
			'authenticated-token'
		);
	});

	it('registers with the exact JSON body and stores the rotated CSRF token', async () => {
		const input = {
			email: 'alice@example.com',
			password: 'correct horse battery staple',
			displayName: 'Alice'
		};
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'anonymous-token' }))
			.mockResolvedValueOnce(
				jsonResponse(
					{
						csrfToken: 'registered-token',
						user: currentUserFixture.user
					},
					201
				)
			)
			.mockResolvedValueOnce(jsonResponse({ ok: true }));

		await expect(register(input)).resolves.toEqual(currentUserFixture.user);
		expect(fetchMock.mock.calls[1][1]?.body).toBe(JSON.stringify(input));

		await request('/api/groups', { method: 'POST' });
		expect(new Headers(fetchMock.mock.calls[2][1]?.headers).get('X-CSRF-Token')).toBe(
			'registered-token'
		);
	});

	it('clears CSRF only after logout succeeds', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(new Response(undefined, { status: 204 }))
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'anonymous-token' }))
			.mockResolvedValueOnce(jsonResponse({ ok: true }));

		await logout();
		await request('/api/groups/join', { method: 'POST' });

		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(String(fetchMock.mock.calls[1][0])).toMatch(/\/api\/auth\/csrf$/);
		expect(new Headers(fetchMock.mock.calls[2][1]?.headers).get('X-CSRF-Token')).toBe(
			'anonymous-token'
		);
	});

	it('retains the current CSRF token when logout fails over the network', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockRejectedValueOnce(new TypeError('Failed to fetch'))
			.mockResolvedValueOnce(jsonResponse({ ok: true }));

		await expect(logout()).rejects.toMatchObject({ code: 'network_error' });
		await request('/api/groups', { method: 'POST' });

		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(new Headers(fetchMock.mock.calls[1][1]?.headers).get('X-CSRF-Token')).toBe(
			'authenticated-token'
		);
	});

	it('loads and validates the current user response', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse(currentUserFixture));
		await expect(getCurrentUser()).resolves.toEqual(currentUserFixture.user);

		fetchMock.mockResolvedValueOnce(
			jsonResponse({ user: { ...currentUserFixture.user, displayName: 42 } })
		);
		await expect(getCurrentUser()).rejects.toMatchObject({
			code: 'invalid_response',
			source: 'invalid-response'
		});
	});

	it('rejects a malformed auth response before rotating CSRF', async () => {
		setCsrfToken('anonymous-token');
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({
					csrfToken: '',
					user: currentUserFixture.user
				})
			)
			.mockResolvedValueOnce(jsonResponse({ ok: true }));

		await expect(
			login({ email: 'alice@example.com', password: 'password' })
		).rejects.toMatchObject({ code: 'invalid_response' });

		await request('/api/groups', { method: 'POST' });
		expect(new Headers(fetchMock.mock.calls[1][1]?.headers).get('X-CSRF-Token')).toBe(
			'anonymous-token'
		);
	});
});

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function decodeBasicAuthorization(value: string | null): string {
	if (value === null || !value.startsWith('Basic ')) {
		throw new Error('Missing Basic Authorization header.');
	}

	const binary = atob(value.slice('Basic '.length));
	const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));

	return new TextDecoder().decode(bytes);
}
