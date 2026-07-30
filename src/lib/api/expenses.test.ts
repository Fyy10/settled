import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { expenseListFixture } from '../../tests/fixtures/api-contract';
import { clearCsrfToken, setCsrfToken } from '../state/csrf';
import {
	createExpense,
	deleteExpense,
	getExpense,
	listExpenses,
	replaceExpense
} from './expenses';
import type { ExpenseInput } from './types';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

const fetchMock = vi.fn<typeof fetch>();
const fixtureExpense = expenseListFixture.expenses[0];
const fixtureUserIds = fixtureExpense.splits.map((split) => split.userId);
const percentageBasisPoints = distributeBasisPoints(fixtureUserIds.length);
const expenseInputs: Array<{ label: string; input: ExpenseInput }> = [
	{
		label: 'equal',
		input: {
			paidByUserId: fixtureExpense.paidByUserId,
			description: 'Shared dinner',
			amountCents: 5400,
			expenseDate: '2026-06-30',
			splitMode: 'equal',
			participantUserIds: fixtureUserIds
		}
	},
	{
		label: 'exact',
		input: {
			paidByUserId: fixtureExpense.paidByUserId,
			description: 'Shared dinner',
			amountCents: 5400,
			expenseDate: '2026-06-30',
			splitMode: 'exact',
			splits: fixtureExpense.splits
		}
	},
	{
		label: 'percentage',
		input: {
			paidByUserId: fixtureExpense.paidByUserId,
			description: 'Shared dinner',
			amountCents: 5400,
			expenseDate: '2026-06-30',
			splitMode: 'percentage',
			percentageSplits: fixtureUserIds.map((userId, index) => ({
				userId,
				percentageBasisPoints: percentageBasisPoints[index]
			}))
		}
	}
];

beforeEach(() => {
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
	clearCsrfToken();
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
		expect(
			new Headers(fetchMock.mock.calls[0][1]?.headers).has('X-CSRF-Token')
		).toBe(false);
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

	it('rejects list entries attributed to another group', async () => {
		fetchMock.mockResolvedValue(jsonResponse(expenseListFixture));

		await expect(listExpenses('different-group')).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it.each([
		[
			'duplicate expense IDs',
			{
				...expenseListFixture,
				expenses: [
					fixtureExpense,
					{ ...fixtureExpense, id: fixtureExpense.id.toUpperCase() }
				]
			}
		],
		[
			'duplicate member IDs',
			{
				...expenseListFixture,
				members: [
					...expenseListFixture.members,
					{
						...expenseListFixture.members[0],
						userId: expenseListFixture.members[0].userId.toUpperCase()
					}
				]
			}
		],
		[
			'a missing payer summary',
			{
				...expenseListFixture,
				members: expenseListFixture.members.filter(
					(member) => member.userId !== fixtureExpense.paidByUserId
				)
			}
		],
		[
			'a missing participant summary',
			{
				...expenseListFixture,
				members: expenseListFixture.members.filter(
					(member) => member.userId !== fixtureExpense.splits[1].userId
				)
			}
		]
	])('rejects an expense list with %s', async (_label, response) => {
		fetchMock.mockResolvedValue(jsonResponse(response));

		await expect(listExpenses(fixtureExpense.groupId)).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('gets one expense with encoded identifiers and an AbortSignal', async () => {
		const controller = new AbortController();
		const expense = {
			...fixtureExpense,
			id: 'expense/id',
			groupId: 'group/id'
		};
		fetchMock.mockResolvedValue(jsonResponse({ expense }));

		await expect(
			getExpense('group/id', 'expense/id', { signal: controller.signal })
		).resolves.toEqual(expense);

		const [url, init] = fetchMock.mock.calls[0];
		expect(String(url)).toMatch(
			/\/api\/groups\/group%2Fid\/expenses\/expense%2Fid$/
		);
		expect(init).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});
		expect(new Headers(init?.headers).has('X-CSRF-Token')).toBe(false);
	});

	it('rejects malformed or incoherent item responses', async () => {
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ expense: null }))
			.mockResolvedValueOnce(jsonResponse({ expense: fixtureExpense }))
			.mockResolvedValueOnce(jsonResponse({ expense: fixtureExpense }))
			.mockResolvedValueOnce(
				jsonResponse({
					expense: {
						...fixtureExpense,
						splits: fixtureExpense.splits.map((split, index) => ({
							...split,
							amountCents: split.amountCents + (index === 0 ? 1 : 0)
						}))
					}
				})
			);

		await expect(
			getExpense(fixtureExpense.groupId, fixtureExpense.id)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			getExpense('different-group', fixtureExpense.id)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			getExpense(fixtureExpense.groupId, 'different-expense')
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			getExpense(fixtureExpense.groupId, fixtureExpense.id)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it.each(expenseInputs)(
		'creates an expense with a complete $label split payload',
		async ({ input }) => {
			const controller = new AbortController();
			const expense = matchingExpense(input, {
				groupId: 'group/id'
			});
			setCsrfToken('authenticated-token');
			fetchMock.mockResolvedValue(jsonResponse({ expense }, 201));

			await expect(
				createExpense('group/id', input, { signal: controller.signal })
			).resolves.toEqual(expense);

			const [url, init] = fetchMock.mock.calls[0];
			const headers = new Headers(init?.headers);
			expect(String(url)).toMatch(/\/api\/groups\/group%2Fid\/expenses$/);
			expect(init).toMatchObject({
				method: 'POST',
				body: JSON.stringify(input),
				signal: controller.signal
			});
			expect(headers.get('Content-Type')).toBe('application/json');
			expect(headers.get('X-CSRF-Token')).toBe('authenticated-token');
		}
	);

	it('rejects malformed or cross-group create responses', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ expense: null }, 201))
			.mockResolvedValueOnce(
				jsonResponse({ expense: fixtureExpense }, 201)
			);

		await expect(
			createExpense(fixtureExpense.groupId, expenseInputs[0].input)
		).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			createExpense('different-group', expenseInputs[0].input)
		).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
	});

	it('accepts Go-normalized Unicode-space descriptions after create and replace', async () => {
		const input = {
			...expenseInputs[0].input,
			description: '\u0085Dinner\u0085'
		};
		const persisted = matchingExpense(input, { description: 'Dinner' });
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ expense: persisted }, 201))
			.mockResolvedValueOnce(jsonResponse({ expense: persisted }));

		await expect(
			createExpense(fixtureExpense.groupId, input)
		).resolves.toEqual(persisted);
		await expect(
			replaceExpense(fixtureExpense.groupId, fixtureExpense.id, input)
		).resolves.toEqual(persisted);
	});

	it.each([
		['payer', { paidByUserId: fixtureUserIds[1] }],
		['trimmed description', { description: 'Groceries' }],
		['amount', { amountCents: 5_401, splits: [
			{ ...fixtureExpense.splits[0], amountCents: 2_701 },
			fixtureExpense.splits[1]
		] }],
		['date', { expenseDate: '2026-07-01' }],
		['persisted split output', { splits: [
			{ ...fixtureExpense.splits[0], amountCents: 2_699 },
			{ ...fixtureExpense.splits[1], amountCents: 2_701 }
		] }]
	] as const)(
		'rejects a create response with mismatched %s',
		async (_label, overrides) => {
			setCsrfToken('authenticated-token');
			fetchMock.mockResolvedValue(
				jsonResponse(
					{
						expense: {
							...matchingExpense(expenseInputs[0].input),
							...overrides
						}
					},
					201
				)
			);

			await expect(
				createExpense(fixtureExpense.groupId, expenseInputs[0].input)
			).rejects.toMatchObject({
				status: 201,
				code: 'invalid_response'
			});
		}
	);

	it.each(expenseInputs)(
		'replaces an expense with a complete $label split payload',
		async ({ input }) => {
			const controller = new AbortController();
			const expense = matchingExpense(input, {
				id: 'expense/id',
				groupId: 'group/id'
			});
			setCsrfToken('authenticated-token');
			fetchMock.mockResolvedValue(jsonResponse({ expense }));

			await expect(
				replaceExpense('group/id', 'expense/id', input, {
					signal: controller.signal
				})
			).resolves.toEqual(expense);

			const [url, init] = fetchMock.mock.calls[0];
			const headers = new Headers(init?.headers);
			expect(String(url)).toMatch(
				/\/api\/groups\/group%2Fid\/expenses\/expense%2Fid$/
			);
			expect(init).toMatchObject({
				method: 'PUT',
				body: JSON.stringify(input),
				signal: controller.signal
			});
			expect(headers.get('Content-Type')).toBe('application/json');
			expect(headers.get('X-CSRF-Token')).toBe('authenticated-token');
		}
	);

	it('rejects malformed or incoherent replacement responses', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ expense: null }))
			.mockResolvedValueOnce(jsonResponse({ expense: fixtureExpense }))
			.mockResolvedValueOnce(jsonResponse({ expense: fixtureExpense }));

		await expect(
			replaceExpense(
				fixtureExpense.groupId,
				fixtureExpense.id,
				expenseInputs[0].input
			)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			replaceExpense(
				'different-group',
				fixtureExpense.id,
				expenseInputs[0].input
			)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			replaceExpense(
				fixtureExpense.groupId,
				'different-expense',
				expenseInputs[0].input
			)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('rejects a replacement response whose content differs from the submitted input', async () => {
		setCsrfToken('authenticated-token');
		fetchMock.mockResolvedValue(
			jsonResponse({
				expense: {
					...matchingExpense(expenseInputs[0].input),
					description: 'Groceries'
				}
			})
		);

		await expect(
			replaceExpense(
				fixtureExpense.groupId,
				fixtureExpense.id,
				expenseInputs[0].input
			)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('deletes an encoded expense and requires an empty 204 response', async () => {
		const controller = new AbortController();
		setCsrfToken('authenticated-token');
		fetchMock.mockResolvedValue(new Response(undefined, { status: 204 }));

		await expect(
			deleteExpense('group/id', 'expense/id', {
				signal: controller.signal
			})
		).resolves.toBeUndefined();

		const [url, init] = fetchMock.mock.calls[0];
		const headers = new Headers(init?.headers);
		expect(String(url)).toMatch(
			/\/api\/groups\/group%2Fid\/expenses\/expense%2Fid$/
		);
		expect(init).toMatchObject({
			method: 'DELETE',
			signal: controller.signal
		});
		expect(init?.body).toBeUndefined();
		expect(headers.has('Content-Type')).toBe(false);
		expect(headers.get('X-CSRF-Token')).toBe('authenticated-token');
	});

	it('rejects an unexpected delete success status and preserves HTTP errors', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ deleted: true }))
			.mockResolvedValueOnce(
				jsonResponse(
					{
						error: {
							code: 'not_found',
							message: 'The requested resource was not found.'
						}
					},
					404
				)
			);

		await expect(
			deleteExpense(fixtureExpense.groupId, fixtureExpense.id)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			deleteExpense(fixtureExpense.groupId, fixtureExpense.id)
		).rejects.toMatchObject({
			status: 404,
			code: 'not_found'
		});
	});

	it('enforces each documented success status and reports the actual status', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse(expenseListFixture, 201))
			.mockResolvedValueOnce(
				jsonResponse({ expense: fixtureExpense }, 201)
			)
			.mockResolvedValueOnce(
				jsonResponse({ expense: fixtureExpense }, 200)
			)
			.mockResolvedValueOnce(
				jsonResponse({ expense: fixtureExpense }, 201)
			)
			.mockResolvedValueOnce(new Response(undefined, { status: 200 }));

		await expect(listExpenses(fixtureExpense.groupId)).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			getExpense(fixtureExpense.groupId, fixtureExpense.id)
		).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			createExpense(fixtureExpense.groupId, expenseInputs[0].input)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			replaceExpense(
				fixtureExpense.groupId,
				fixtureExpense.id,
				expenseInputs[0].input
			)
		).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			deleteExpense(fixtureExpense.groupId, fixtureExpense.id)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('refreshes a rejected CSRF token and retries an expense mutation once', async () => {
		const controller = new AbortController();
		setCsrfToken('stale-token');
		fetchMock
			.mockResolvedValueOnce(csrfError('csrf_invalid'))
			.mockResolvedValueOnce(jsonResponse({ csrfToken: 'fresh-token' }))
			.mockResolvedValueOnce(
				jsonResponse(
					{ expense: matchingExpense(expenseInputs[0].input) },
					201
				)
			);

		await expect(
			createExpense(
				fixtureExpense.groupId,
				expenseInputs[0].input,
				{ signal: controller.signal }
			)
		).resolves.toEqual(matchingExpense(expenseInputs[0].input));

		expect(fetchMock).toHaveBeenCalledTimes(3);
		const firstAttempt = fetchMock.mock.calls[0][1];
		const retry = fetchMock.mock.calls[2][1];
		expect(firstAttempt).toMatchObject({
			method: 'POST',
			body: JSON.stringify(expenseInputs[0].input),
			signal: controller.signal
		});
		expect(retry).toMatchObject({
			method: 'POST',
			body: JSON.stringify(expenseInputs[0].input),
			signal: controller.signal
		});
		expect(new Headers(firstAttempt?.headers).get('X-CSRF-Token')).toBe(
			'stale-token'
		);
		expect(new Headers(retry?.headers).get('X-CSRF-Token')).toBe(
			'fresh-token'
		);
	});
});

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
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

function distributeBasisPoints(count: number): number[] {
	const base = Math.floor(10_000 / count);
	const remainder = 10_000 % count;

	return Array.from(
		{ length: count },
		(_, index) => base + (index < remainder ? 1 : 0)
	);
}

function matchingExpense(
	input: ExpenseInput,
	overrides: Partial<typeof fixtureExpense> = {}
): typeof fixtureExpense {
	return {
		...fixtureExpense,
		paidByUserId: input.paidByUserId,
		description: input.description.trim(),
		amountCents: input.amountCents,
		expenseDate: input.expenseDate,
		// These fixtures intentionally use two equal 50% shares in every mode.
		splits: fixtureExpense.splits.map((split) => ({ ...split })),
		...overrides
	};
}
