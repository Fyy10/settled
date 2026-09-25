import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { repaymentListFixture } from '../../tests/fixtures/api-contract';
import { clearCsrfToken, setCsrfToken } from '../state/csrf';
import {
	createRepayment,
	deleteRepayment,
	getRepayment,
	listRepayments,
	replaceRepayment
} from './repayments';
import type {
	CreateRepaymentInput,
	ReplaceRepaymentInput
} from './types';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

const fetchMock = vi.fn<typeof fetch>();
const fixtureRepayment = repaymentListFixture.repayments[0];
const repaymentInput: ReplaceRepaymentInput = {
	fromUserId: fixtureRepayment.fromUserId,
	toUserId: fixtureRepayment.toUserId,
	amountCents: fixtureRepayment.amountCents,
	note: fixtureRepayment.note,
	repaymentDate: fixtureRepayment.repaymentDate
};

beforeEach(() => {
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
	clearCsrfToken();
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
		const [url, init] = fetchMock.mock.calls[0];
		expect(String(url)).toMatch(
			/\/api\/groups\/group%2Fid\/repayments$/
		);
		expect(init).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});
		expect(new Headers(init?.headers).has('X-CSRF-Token')).toBe(false);
	});

	it('rejects null arrays instead of converting them into empty data', async () => {
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({ ...repaymentListFixture, repayments: null })
			)
			.mockResolvedValueOnce(
				jsonResponse({ ...repaymentListFixture, members: null })
			);

		await expect(listRepayments('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
		await expect(listRepayments('group-id')).rejects.toMatchObject({
			code: 'invalid_response'
		});
	});

	it.each([
		[
			'a cross-group record',
			{
				...repaymentListFixture,
				repayments: [
					{
						...fixtureRepayment,
						groupId: 'different-group'
					}
				]
			}
		],
		[
			'duplicate repayment IDs',
			{
				...repaymentListFixture,
				repayments: [
					fixtureRepayment,
					{
						...fixtureRepayment,
						id: fixtureRepayment.id.toUpperCase()
					}
				]
			}
		],
		[
			'duplicate member IDs',
			{
				...repaymentListFixture,
				members: [
					...repaymentListFixture.members,
					{
						...repaymentListFixture.members[0],
						userId:
							repaymentListFixture.members[0].userId.toUpperCase()
					}
				]
			}
		],
		[
			'a missing sender summary',
			{
				...repaymentListFixture,
				members: repaymentListFixture.members.filter(
					(member) =>
						member.userId !== fixtureRepayment.fromUserId
				)
			}
		],
		[
			'a missing recipient summary',
			{
				...repaymentListFixture,
				members: repaymentListFixture.members.filter(
					(member) => member.userId !== fixtureRepayment.toUserId
				)
			}
		],
		[
			'a self-transfer',
			{
				...repaymentListFixture,
				repayments: [
					{
						...fixtureRepayment,
						toUserId: fixtureRepayment.fromUserId
					}
				]
			}
		],
		[
			'an unnormalized note',
			{
				...repaymentListFixture,
				repayments: [
					{ ...fixtureRepayment, note: ' Venmo' }
				]
			}
		],
		[
			'a control character in a note',
			{
				...repaymentListFixture,
				repayments: [
					{ ...fixtureRepayment, note: 'Venmo\n' }
				]
			}
		],
		[
			'a note over 240 code points',
			{
				...repaymentListFixture,
				repayments: [
					{ ...fixtureRepayment, note: '🧾'.repeat(241) }
				]
			}
		]
	])('rejects a repayment list with %s', async (_label, response) => {
		fetchMock.mockResolvedValue(jsonResponse(response));

		await expect(
			listRepayments(fixtureRepayment.groupId)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('does not require a repayment creator in current member summaries', async () => {
		const response = {
			...repaymentListFixture,
			repayments: [
				{ ...fixtureRepayment, createdByUserId: 'removed-creator' }
			]
		};
		fetchMock.mockResolvedValue(jsonResponse(response));

		await expect(
			listRepayments(fixtureRepayment.groupId)
		).resolves.toEqual(response);
	});

	it('gets one repayment with encoded identifiers and an AbortSignal', async () => {
		const controller = new AbortController();
		const repayment = {
			...fixtureRepayment,
			id: 'repayment/id',
			groupId: 'group/id'
		};
		fetchMock.mockResolvedValue(jsonResponse({ repayment }));

		await expect(
			getRepayment('group/id', 'repayment/id', {
				signal: controller.signal
			})
		).resolves.toEqual(repayment);
		const [url, init] = fetchMock.mock.calls[0];
		expect(String(url)).toMatch(
			/\/api\/groups\/group%2Fid\/repayments\/repayment%2Fid$/
		);
		expect(init).toMatchObject({
			method: 'GET',
			signal: controller.signal
		});
		expect(new Headers(init?.headers).has('X-CSRF-Token')).toBe(false);
	});

	it('rejects malformed or incoherent item responses', async () => {
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ repayment: null }))
			.mockResolvedValueOnce(
				jsonResponse({ repayment: fixtureRepayment })
			)
			.mockResolvedValueOnce(
				jsonResponse({ repayment: fixtureRepayment })
			);

		await expect(
			getRepayment(fixtureRepayment.groupId, fixtureRepayment.id)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			getRepayment('different-group', fixtureRepayment.id)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			getRepayment(fixtureRepayment.groupId, 'different-repayment')
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('creates a repayment on behalf of another member', async () => {
		const controller = new AbortController();
		const input = {
			...repaymentInput,
			note: '\u0085Venmo\u3000'
		};
		const repayment = {
			...fixtureRepayment,
			note: 'Venmo',
			createdByUserId: fixtureRepayment.toUserId
		};
		setCsrfToken('authenticated-token');
		fetchMock.mockResolvedValue(
			jsonResponse({ repayment }, 201)
		);

		await expect(
			createRepayment(fixtureRepayment.groupId, input, {
				signal: controller.signal
			})
		).resolves.toEqual(repayment);
		const [url, init] = fetchMock.mock.calls[0];
		const headers = new Headers(init?.headers);
		expect(String(url)).toMatch(
			new RegExp(
				`/api/groups/${fixtureRepayment.groupId}/repayments$`
			)
		);
		expect(init).toMatchObject({
			method: 'POST',
			body: JSON.stringify(input),
			signal: controller.signal
		});
		expect(headers.get('Content-Type')).toBe('application/json');
		expect(headers.get('X-CSRF-Token')).toBe('authenticated-token');
	});

	it.each([
		['sender', { fromUserId: fixtureRepayment.toUserId }],
		['recipient', { toUserId: fixtureRepayment.fromUserId }],
		['amount', { amountCents: fixtureRepayment.amountCents + 1 }],
		['date', { repaymentDate: '2026-07-01' }],
		['note', { note: 'Cash' }]
	] as const)(
		'rejects a create response with mismatched %s',
		async (_label, overrides) => {
			setCsrfToken('authenticated-token');
			fetchMock.mockResolvedValue(
				jsonResponse(
					{
						repayment: {
							...fixtureRepayment,
							...overrides
						}
					},
					201
				)
			);

			await expect(
				createRepayment(
					fixtureRepayment.groupId,
					repaymentInput
				)
			).rejects.toMatchObject({
				status: 201,
				code: 'invalid_response'
			});
		}
	);

	it.each([
		['missing note', { ...repaymentInput, note: undefined }],
		['null note', { ...repaymentInput, note: null }],
		['blank note', { ...repaymentInput, note: '\u0085\u3000' }]
	] as const)(
		'accepts a server null note after create with %s',
		async (_label, input) => {
			const repayment = { ...fixtureRepayment, note: null };
			setCsrfToken('authenticated-token');
			fetchMock.mockResolvedValue(
				jsonResponse({ repayment }, 201)
			);

			await expect(
				createRepayment(
					fixtureRepayment.groupId,
					input as CreateRepaymentInput
				)
			).resolves.toEqual(repayment);
		}
	);

	it('replaces a repayment with every editable field including null note', async () => {
		const controller = new AbortController();
		const input: ReplaceRepaymentInput = {
			...repaymentInput,
			note: null
		};
		const repayment = { ...fixtureRepayment, note: null };
		setCsrfToken('authenticated-token');
		fetchMock.mockResolvedValue(jsonResponse({ repayment }));

		await expect(
			replaceRepayment(
				'group/id',
				'repayment/id',
				input,
				{ signal: controller.signal }
			)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});

		fetchMock.mockResolvedValueOnce(
			jsonResponse({
				repayment: {
					...repayment,
					id: 'repayment/id',
					groupId: 'group/id'
				}
			})
		);
		await expect(
			replaceRepayment(
				'group/id',
				'repayment/id',
				input,
				{ signal: controller.signal }
			)
		).resolves.toMatchObject({ note: null });
		const [, init] = fetchMock.mock.calls[1];
		expect(init).toMatchObject({
			method: 'PUT',
			body: JSON.stringify(input),
			signal: controller.signal
		});
		expect(JSON.parse(String(init?.body))).toHaveProperty('note', null);
	});

	it('deletes an encoded repayment and requires an empty 204 response', async () => {
		const controller = new AbortController();
		setCsrfToken('authenticated-token');
		fetchMock.mockResolvedValue(
			new Response(undefined, { status: 204 })
		);

		await expect(
			deleteRepayment('group/id', 'repayment/id', {
				signal: controller.signal
			})
		).resolves.toBeUndefined();
		const [url, init] = fetchMock.mock.calls[0];
		const headers = new Headers(init?.headers);
		expect(String(url)).toMatch(
			/\/api\/groups\/group%2Fid\/repayments\/repayment%2Fid$/
		);
		expect(init).toMatchObject({
			method: 'DELETE',
			signal: controller.signal
		});
		expect(init?.body).toBeUndefined();
		expect(headers.has('Content-Type')).toBe(false);
		expect(headers.get('X-CSRF-Token')).toBe(
			'authenticated-token'
		);
	});

	it('enforces every documented success status', async () => {
		setCsrfToken('authenticated-token');
		fetchMock
			.mockResolvedValueOnce(jsonResponse(repaymentListFixture, 201))
			.mockResolvedValueOnce(
				jsonResponse({ repayment: fixtureRepayment }, 201)
			)
			.mockResolvedValueOnce(
				jsonResponse({ repayment: fixtureRepayment }, 200)
			)
			.mockResolvedValueOnce(
				jsonResponse({ repayment: fixtureRepayment }, 201)
			)
			.mockResolvedValueOnce(
				new Response(undefined, { status: 200 })
			);

		await expect(
			listRepayments(fixtureRepayment.groupId)
		).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			getRepayment(fixtureRepayment.groupId, fixtureRepayment.id)
		).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			createRepayment(fixtureRepayment.groupId, repaymentInput)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
		await expect(
			replaceRepayment(
				fixtureRepayment.groupId,
				fixtureRepayment.id,
				repaymentInput
			)
		).rejects.toMatchObject({
			status: 201,
			code: 'invalid_response'
		});
		await expect(
			deleteRepayment(
				fixtureRepayment.groupId,
				fixtureRepayment.id
			)
		).rejects.toMatchObject({
			status: 200,
			code: 'invalid_response'
		});
	});

	it('refreshes a rejected CSRF token and retries a repayment mutation once', async () => {
		const controller = new AbortController();
		setCsrfToken('stale-token');
		fetchMock
			.mockResolvedValueOnce(csrfError())
			.mockResolvedValueOnce(
				jsonResponse({ csrfToken: 'fresh-token' })
			)
			.mockResolvedValueOnce(
				jsonResponse({ repayment: fixtureRepayment }, 201)
			);

		await expect(
			createRepayment(
				fixtureRepayment.groupId,
				repaymentInput,
				{ signal: controller.signal }
			)
		).resolves.toEqual(fixtureRepayment);
		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(
			new Headers(
				fetchMock.mock.calls[0][1]?.headers
			).get('X-CSRF-Token')
		).toBe('stale-token');
		expect(
			new Headers(
				fetchMock.mock.calls[2][1]?.headers
			).get('X-CSRF-Token')
		).toBe('fresh-token');
		expect(fetchMock.mock.calls[2][1]).toMatchObject({
			method: 'POST',
			body: JSON.stringify(repaymentInput),
			signal: controller.signal
		});
	});
});

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function csrfError(): Response {
	return jsonResponse(
		{
			error: {
				code: 'csrf_invalid',
				message: 'The CSRF token is invalid.'
			}
		},
		403
	);
}
