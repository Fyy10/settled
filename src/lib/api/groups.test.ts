import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { groupListFixture } from '../../tests/fixtures/api-contract';
import { clearCsrfToken, setCsrfToken } from '../state/csrf';
import { createGroup, joinGroup, listGroups } from './groups';

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

describe('group API', () => {
	it('lists and validates API-provided group summaries', async () => {
		const controller = new AbortController();
		fetchMock.mockResolvedValue(jsonResponse(groupListFixture));

		await expect(listGroups({ signal: controller.signal })).resolves.toEqual(
			groupListFixture.groups
		);
		expect(fetchMock.mock.calls[0][1]).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});

		fetchMock.mockResolvedValueOnce(
			jsonResponse({
				groups: [{ ...groupListFixture.groups[0], memberCount: 'two' }]
			})
		);
		await expect(listGroups()).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it('creates a group through the shared CSRF request path', async () => {
		setCsrfToken('authenticated-token');
		const created = groupListFixture.groups[0];
		fetchMock.mockResolvedValue(jsonResponse({ group: created }, 201));

		await expect(createGroup({ name: 'Lake Trip' })).resolves.toEqual(created);
		expect(fetchMock).toHaveBeenCalledOnce();
		expect(fetchMock.mock.calls[0][1]?.body).toBe('{"name":"Lake Trip"}');
		expect(
			new Headers(fetchMock.mock.calls[0][1]?.headers).get('X-CSRF-Token')
		).toBe('authenticated-token');
	});

	it('joins a group through the shared CSRF request path', async () => {
		setCsrfToken('authenticated-token');
		const joined = groupListFixture.groups[1];
		fetchMock.mockResolvedValue(jsonResponse({ group: joined }));

		await expect(joinGroup({ joinCode: 'ABCD1234' })).resolves.toEqual(joined);
		expect(fetchMock.mock.calls[0][1]?.body).toBe('{"joinCode":"ABCD1234"}');
		expect(String(fetchMock.mock.calls[0][0])).toMatch(/\/api\/groups\/join$/);
	});

	it('rejects malformed create and join responses', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ group: { id: 'only-an-id' } }, 201))
			.mockResolvedValueOnce(jsonResponse({ group: null }));

		await expect(createGroup({ name: 'Lake Trip' })).rejects.toMatchObject({
			code: 'invalid_response'
		});
		await expect(joinGroup({ joinCode: 'ABCD1234' })).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});
});

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}
