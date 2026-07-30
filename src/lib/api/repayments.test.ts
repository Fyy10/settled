import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { repaymentListFixture } from '../../tests/fixtures/api-contract';
import { listRepayments } from './repayments';

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

describe('repayment API', () => {
	it('lists repayments and member summaries for an encoded group', async () => {
		const controller = new AbortController();
		const encodedGroupFixture = {
			...repaymentListFixture,
			repayments: repaymentListFixture.repayments.map((repayment) => ({
				...repayment,
				groupId: 'group/id'
			}))
		};
		fetchMock.mockResolvedValue(jsonResponse(encodedGroupFixture));

		await expect(
			listRepayments('group/id', { signal: controller.signal })
		).resolves.toEqual(encodedGroupFixture);
		expect(String(fetchMock.mock.calls[0][0])).toMatch(
			/\/api\/groups\/group%2Fid\/repayments$/
		);
		expect(fetchMock.mock.calls[0][1]).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});
	});

	it('rejects null arrays instead of converting them into empty data', async () => {
		fetchMock.mockResolvedValue(
			jsonResponse({ ...repaymentListFixture, repayments: null })
		);

		await expect(listRepayments('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it('rejects payment records attributed to another group', async () => {
		fetchMock.mockResolvedValue(jsonResponse(repaymentListFixture));

		await expect(listRepayments('different-group')).rejects.toMatchObject({
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
