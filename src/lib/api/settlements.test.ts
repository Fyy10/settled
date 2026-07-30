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
					toUserId: settlementListFixture.members[1].userId,
					amountCents: 500,
					currency: 'USD' as const
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
		fetchMock.mockResolvedValue(
			jsonResponse({ ...settlementListFixture, settlements: null })
		);

		await expect(listSettlements('group-id')).rejects.toMatchObject({
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
