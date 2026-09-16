import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { settlementListFixture } from '../../tests/fixtures/api-contract';
import { listSettlements } from './settlements';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('settlement API', () => {
	it('returns backend settlement order unchanged', async () => {
		const ordered = {
			...settlementListFixture,
			settlements: [
				...settlementListFixture.settlements,
				{
					fromUserId: settlementListFixture.members[0].userId,
					toUserId: '00000000-0000-4000-8000-000000000003',
					amountCents: 500,
					currency: 'USD' as const
				}
			],
			members: [
				...settlementListFixture.members,
				{
					userId: '00000000-0000-4000-8000-000000000003',
					displayName: 'Casey'
				}
			]
		};
		fetchMock.mockResolvedValue(jsonResponse(ordered));

		await expect(listSettlements('group/id')).resolves.toEqual(ordered);
		expect(String(fetchMock.mock.calls[0][0])).toMatch(
			/\/api\/groups\/group%2Fid\/settlements$/
		);
	});

	it('requires explicit non-null settlement and member arrays', async () => {
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({
					...settlementListFixture,
					settlements: null
				})
			)
			.mockResolvedValueOnce(
				jsonResponse({
					...settlementListFixture,
					members: null
				})
			);

		await expect(listSettlements('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
		await expect(listSettlements('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it.each([
		[
			'a duplicate member ID',
			{
				...settlementListFixture,
				members: [
					...settlementListFixture.members,
					{
						...settlementListFixture.members[0],
						userId:
							settlementListFixture.members[0].userId.toUpperCase()
					}
				]
			}
		],
		[
			'a self-transfer',
			{
				...settlementListFixture,
				settlements: [
					{
						...settlementListFixture.settlements[0],
						toUserId:
							settlementListFixture.settlements[0].fromUserId
					}
				]
			}
		],
		[
			'a missing sender summary',
			{
				...settlementListFixture,
				members: settlementListFixture.members.filter(
					(member) =>
						member.userId !==
						settlementListFixture.settlements[0].fromUserId
				)
			}
		],
		[
			'a missing recipient summary',
			{
				...settlementListFixture,
				members: settlementListFixture.members.filter(
					(member) =>
						member.userId !==
						settlementListFixture.settlements[0].toUserId
				)
			}
		],
		[
			'a duplicate unordered pair',
			{
				...settlementListFixture,
				settlements: [
					...settlementListFixture.settlements,
					{
						fromUserId:
							settlementListFixture.settlements[0].toUserId,
						toUserId:
							settlementListFixture.settlements[0].fromUserId,
						amountCents: 500,
						currency: 'USD' as const
					}
				]
			}
		]
	])('rejects settlement coherence with %s', async (_label, response) => {
		fetchMock.mockResolvedValue(jsonResponse(response));

		await expect(listSettlements('group-id')).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('enforces the documented 200 success status', async () => {
		fetchMock.mockResolvedValue(
			new Response(JSON.stringify(settlementListFixture), {
				status: 201,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await expect(listSettlements('group-id')).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
	});
});

function jsonResponse(body: unknown): Response {
	return new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});
}
