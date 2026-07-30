import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
	groupDetailFixture,
	groupListFixture
} from '../../tests/fixtures/api-contract';
import { clearCsrfToken, setCsrfToken } from '../state/csrf';
import {
	createGroup,
	dissolveGroup,
	getGroupDetail,
	getGroupJoinCode,
	joinGroup,
	listGroups,
	removeGroupMember,
	renameGroup
} from './groups';

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

	it('loads one group detail with its stable member array', async () => {
		const controller = new AbortController();
		const encodedGroupFixture = {
			...groupDetailFixture,
			group: { ...groupDetailFixture.group, id: 'group/id' }
		};
		fetchMock.mockResolvedValue(jsonResponse(encodedGroupFixture));

		await expect(
			getGroupDetail('group/id', { signal: controller.signal })
		).resolves.toEqual(encodedGroupFixture);
		expect(String(fetchMock.mock.calls[0][0])).toMatch(
			/\/api\/groups\/group%2Fid$/
		);
		expect(fetchMock.mock.calls[0][1]).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});

		fetchMock.mockResolvedValueOnce(
			jsonResponse({ ...groupDetailFixture, members: null })
		);
		await expect(getGroupDetail(groupDetailFixture.group.id)).rejects.toMatchObject({
			code: 'invalid_response'
		});

		fetchMock.mockResolvedValueOnce(jsonResponse(groupDetailFixture));
		await expect(getGroupDetail('different-group')).rejects.toMatchObject({
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

	it('renames a group through the shared CSRF path and validates its identity', async () => {
		setCsrfToken('authenticated-token');
		const renamed = {
			...groupListFixture.groups[0],
			id: 'group/id',
			name: 'Beach Trip'
		};
		fetchMock.mockResolvedValue(jsonResponse({ group: renamed }));

		await expect(
			renameGroup('group/id', { name: 'Beach Trip' })
		).resolves.toEqual(renamed);
		expect(String(fetchMock.mock.calls[0][0])).toMatch(
			/\/api\/groups\/group%2Fid$/
		);
		expect(fetchMock.mock.calls[0][1]).toMatchObject({
			method: 'PATCH',
			body: '{"name":"Beach Trip"}'
		});
		expect(
			new Headers(fetchMock.mock.calls[0][1]?.headers).get('X-CSRF-Token')
		).toBe('authenticated-token');

		fetchMock.mockResolvedValueOnce(
			jsonResponse({ group: groupListFixture.groups[0] })
		);
		await expect(
			renameGroup('different-group', { name: 'Beach Trip' })
		).rejects.toMatchObject({ code: 'invalid_response' });
	});

	it('loads and validates the owner-only group code', async () => {
		const controller = new AbortController();
		fetchMock.mockResolvedValue(jsonResponse({ joinCode: 'ABCD1234' }));

		await expect(
			getGroupJoinCode('group/id', { signal: controller.signal })
		).resolves.toBe('ABCD1234');
		expect(String(fetchMock.mock.calls[0][0])).toMatch(
			/\/api\/groups\/group%2Fid\/join-code$/
		);
		expect(fetchMock.mock.calls[0][1]).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});

		fetchMock.mockResolvedValueOnce(jsonResponse({ joinCode: '' }));
		await expect(getGroupJoinCode('group/id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it('removes a member and dissolves a group without request bodies', async () => {
		setCsrfToken('authenticated-token');
		const memberController = new AbortController();
		const dissolveController = new AbortController();
		fetchMock.mockResolvedValue(new Response(null, { status: 204 }));

		await removeGroupMember('group/id', 'user/id', {
			signal: memberController.signal
		});
		expect(String(fetchMock.mock.calls[0][0])).toMatch(
			/\/api\/groups\/group%2Fid\/members\/user%2Fid$/
		);
		expect(fetchMock.mock.calls[0][1]).toMatchObject({
			method: 'DELETE',
			signal: memberController.signal
		});
		expect(fetchMock.mock.calls[0][1]?.body).toBeUndefined();

		fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
		await dissolveGroup('group/id', { signal: dissolveController.signal });
		expect(String(fetchMock.mock.calls[1][0])).toMatch(
			/\/api\/groups\/group%2Fid$/
		);
		expect(fetchMock.mock.calls[1][1]).toMatchObject({
			method: 'DELETE',
			signal: dissolveController.signal
		});
		expect(fetchMock.mock.calls[1][1]?.body).toBeUndefined();
	});

	it('rejects success payloads from no-content owner mutations', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ removed: true }))
			.mockResolvedValueOnce(jsonResponse({ dissolved: true }));

		await expect(
			removeGroupMember('group-id', 'user-id')
		).rejects.toMatchObject({ code: 'invalid_response' });
		await expect(dissolveGroup('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it('rejects wrong success statuses for every owner-management contract', async () => {
		setCsrfToken('authenticated-token');
		const group = groupListFixture.groups[0];
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ group }, 201))
			.mockResolvedValueOnce(jsonResponse({ joinCode: 'ABCD1234' }, 201))
			.mockResolvedValueOnce(jsonResponse({ removed: true }, 200))
			.mockResolvedValueOnce(jsonResponse({ dissolved: true }, 200));

		await expect(
			renameGroup(group.id, { name: group.name })
		).rejects.toMatchObject({ status: 201, code: 'invalid_response' });
		await expect(getGroupJoinCode(group.id)).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			removeGroupMember(group.id, groupDetailFixture.members[1].userId)
		).rejects.toMatchObject({ status: 200, code: 'invalid_response' });
		await expect(dissolveGroup(group.id)).rejects.toMatchObject({
			status: 200,
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
