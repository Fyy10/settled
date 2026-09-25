import { describe, expect, it } from 'vitest';

import type { Repayment } from '$lib/api/types';

import {
	applySettlementRepaymentPrefill,
	createRepaymentDraft,
	initializeRepaymentEditDraft,
	isRepaymentDraftDirty,
	normalizeRepaymentNote,
	toRepaymentInput,
	type RepaymentDraft
} from './repayment-draft';

const aliceId = '00000000-0000-4000-8000-000000000001';
const bobId = '00000000-0000-4000-8000-000000000002';
const activeIds = [aliceId, bobId];

describe('repayment draft', () => {
	it('creates the documented current-user and local-date defaults', () => {
		expect(
			createRepaymentDraft({
				fromUserId: aliceId,
				repaymentDate: '2026-07-30'
			})
		).toEqual({
			fromUserId: aliceId,
			toUserId: '',
			amount: '',
			repaymentDate: '2026-07-30',
			note: ''
		});
	});

	it('accepts a complete valid draft and normalizes Go Unicode-space notes', () => {
		const result = toRepaymentInput(
			{
				...validDraft(),
				note: '\u0085\u2000Venmo\u3000'
			},
			activeIds
		);

		expect(result).toEqual({
			ok: true,
			value: {
				fromUserId: aliceId,
				toUserId: bobId,
				amountCents: 2_000,
				repaymentDate: '2026-07-30',
				note: 'Venmo'
			}
		});
		expect(normalizeRepaymentNote('\u0085\u2000\u3000')).toBeNull();
	});

	it.each([
		['missing sender', { fromUserId: '' }, 'fromUserId'],
		['inactive sender', { fromUserId: 'removed' }, 'fromUserId'],
		['missing recipient', { toUserId: '' }, 'toUserId'],
		['inactive recipient', { toUserId: 'removed' }, 'toUserId'],
		['same sender and recipient', { toUserId: aliceId }, 'toUserId'],
		['empty amount', { amount: '' }, 'amountCents'],
		['non-decimal amount', { amount: '0x10' }, 'amountCents'],
		['zero amount', { amount: '0' }, 'amountCents'],
		[
			'unsafe amount',
			{ amount: '90071992547409.92' },
			'amountCents'
		],
		['invalid date', { repaymentDate: '2026-02-30' }, 'repaymentDate'],
		['control in note', { note: 'cash\npaid' }, 'note'],
		['bidi control in note', { note: 'cash\u202epaid' }, 'note'],
		['long note', { note: '🧾'.repeat(241) }, 'note']
	] as const)('rejects %s', (_label, override, expectedField) => {
		const result = toRepaymentInput(
			{ ...validDraft(), ...override },
			activeIds
		);

		expect(result.ok).toBe(false);
		if (!result.ok) {
			expect(result.errors.map((error) => error.field)).toContain(
				expectedField
			);
		}
	});

	it('counts note limits by Unicode code point and allows non-bidi format marks', () => {
		expect(
			toRepaymentInput(
				{ ...validDraft(), note: `${'🧾'.repeat(239)}\u200d` },
				activeIds
			).ok
		).toBe(true);
		expect(
			toRepaymentInput(
				{ ...validDraft(), note: '🧾'.repeat(240) },
				activeIds
			).ok
		).toBe(true);
	});

	it('hydrates a persisted repayment and rejects incoherent persisted data', () => {
		expect(initializeRepaymentEditDraft(persistedRepayment())).toEqual(
			validDraft()
		);
		expect(() =>
			initializeRepaymentEditDraft({
				...persistedRepayment(),
				toUserId: aliceId
			})
		).toThrow(/different/);
		expect(() =>
			initializeRepaymentEditDraft({
				...persistedRepayment(),
				note: ' Venmo'
			})
		).toThrow(/normalized/);
	});

	it('compares amount formatting and note trimming semantically', () => {
		const initial = validDraft();
		expect(
			isRepaymentDraftDirty(
				{ ...initial, amount: '20', note: '\u00a0Venmo\u3000' },
				initial
			)
		).toBe(false);
		expect(
			isRepaymentDraftDirty({ ...initial, amount: '20.01' }, initial)
		).toBe(true);
		expect(
			isRepaymentDraftDirty({ ...initial, note: 'Cash' }, initial)
		).toBe(true);
		expect(
			isRepaymentDraftDirty({ ...initial, note: 'Cash\n' }, initial)
		).toBe(true);
	});
});

describe('settlement repayment prefill', () => {
	it('applies one complete trusted hint as an all-or-nothing prefill', () => {
		const draft = blankDraft();

		expect(
			applySettlementRepaymentPrefill(
				draft,
				query({
					from: bobId,
					to: aliceId,
					amountCents: '2000'
				}),
				activeIds
			)
		).toEqual({
			...draft,
			fromUserId: bobId,
			toUserId: aliceId,
			amount: '20.00'
		});
	});

	it.each([
		['missing from', `to=${bobId}&amountCents=2000`],
		['missing to', `from=${aliceId}&amountCents=2000`],
		['missing amount', `from=${aliceId}&to=${bobId}`],
		[
			'duplicate from',
			`from=${aliceId}&from=${bobId}&to=${bobId}&amountCents=2000`
		],
		[
			'duplicate amount',
			`from=${aliceId}&to=${bobId}&amountCents=2000&amountCents=1`
		],
		[
			'inactive sender',
			`from=removed&to=${bobId}&amountCents=2000`
		],
		[
			'inactive recipient',
			`from=${aliceId}&to=removed&amountCents=2000`
		],
		[
			'same members',
			`from=${aliceId}&to=${aliceId}&amountCents=2000`
		],
		[
			'negative amount',
			`from=${aliceId}&to=${bobId}&amountCents=-1`
		],
		[
			'zero amount',
			`from=${aliceId}&to=${bobId}&amountCents=0`
		],
		[
			'hex amount',
			`from=${aliceId}&to=${bobId}&amountCents=0x10`
		],
		[
			'exponent amount',
			`from=${aliceId}&to=${bobId}&amountCents=2e3`
		],
		[
			'whitespace amount',
			`from=${aliceId}&to=${bobId}&amountCents=%202000`
		],
		[
			'decimal amount',
			`from=${aliceId}&to=${bobId}&amountCents=20.00`
		],
		[
			'unsafe amount',
			`from=${aliceId}&to=${bobId}&amountCents=9007199254740992`
		]
	])('ignores the whole hint when it contains %s', (_label, search) => {
		const draft = blankDraft();

		expect(
			applySettlementRepaymentPrefill(
				draft,
				new URLSearchParams(search),
				activeIds
			)
		).toEqual(draft);
	});
});

function blankDraft(): RepaymentDraft {
	return createRepaymentDraft({
		fromUserId: aliceId,
		repaymentDate: '2026-07-30'
	});
}

function validDraft(): RepaymentDraft {
	return {
		fromUserId: aliceId,
		toUserId: bobId,
		amount: '20.00',
		repaymentDate: '2026-07-30',
		note: 'Venmo'
	};
}

function persistedRepayment(): Repayment {
	return {
		id: '00000000-0000-4000-8000-000000000010',
		groupId: '00000000-0000-4000-8000-000000000020',
		fromUserId: aliceId,
		toUserId: bobId,
		amountCents: 2_000,
		currency: 'USD',
		note: 'Venmo',
		repaymentDate: '2026-07-30',
		createdByUserId: aliceId,
		createdAt: '2026-07-30T18:00:00Z',
		updatedAt: '2026-07-30T18:00:00Z'
	};
}

function query(values: Record<string, string>): URLSearchParams {
	return new URLSearchParams(values);
}
