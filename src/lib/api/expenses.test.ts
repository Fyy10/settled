import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { expenseListFixture } from '../../tests/fixtures/api-contract';
import { listExpenses } from './expenses';

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

describe('expense API', () => {
	it('lists expenses and member summaries for an encoded group', async () => {
		const controller = new AbortController();
		const encodedGroupFixture = {
			...expenseListFixture,
			expenses: expenseListFixture.expenses.map((expense) => ({
				...expense,
				groupId: 'group/id'
			}))
		};
		fetchMock.mockResolvedValue(jsonResponse(encodedGroupFixture));

		await expect(
			listExpenses('group/id', { signal: controller.signal })
		).resolves.toEqual(encodedGroupFixture);
		expect(String(fetchMock.mock.calls[0][0])).toMatch(
			/\/api\/groups\/group%2Fid\/expenses$/
		);
		expect(fetchMock.mock.calls[0][1]).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});
	});

	it('rejects null or malformed list fields instead of inventing defaults', async () => {
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({ ...expenseListFixture, expenses: null })
			)
			.mockResolvedValueOnce(
				jsonResponse({ ...expenseListFixture, members: null })
			);

		await expect(listExpenses('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
		await expect(listExpenses('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it('rejects expenses attributed to another group', async () => {
		fetchMock.mockResolvedValue(jsonResponse(expenseListFixture));

		await expect(listExpenses('different-group')).rejects.toMatchObject({
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
