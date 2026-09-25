import { describe, expect, it } from 'vitest';

import {
	backendExpenseSplitFixtures,
	backendParticipantA,
	backendParticipantB,
	backendParticipantC,
	backendParticipantD
} from '../../tests/fixtures/backend-expense-splits';
import {
	createExpenseDraft,
	initializeExpenseEditDraft,
	isExpenseDraftDirty,
	modeSwitchDiscardsManualShares,
	participantRemovalDiscardsManualShare,
	previewExpenseSplits,
	removeExpenseParticipant,
	seedEqualPercentageBasisPoints,
	switchExpenseSplitMode,
	toExpenseInput,
	updateExpenseParticipantSelection,
	type ExpenseDraft,
	type ExpenseParticipantDraft
} from './expense-draft';
import { formatCentsAsUsdInput } from './money';

const payerOnly = '00000000-0000-4000-8000-000000000099';

describe('backend split parity', () => {
	it('matches equal request-order remainder allocation', () => {
		const fixture = backendExpenseSplitFixtures.equalRemainder;
		const draft = validEqualDraft(fixture.amountCents, fixture.participantUserIds);

		const preview = previewExpenseSplits(draft);

		expect(preview.isValid).toBe(true);
		expect(preview.splits).toEqual(fixture.splits);
		expect(sumPreview(preview.splits)).toBe(fixture.amountCents);
	});

	it('matches percentage floor and request-order remainder allocation', () => {
		const fixture = backendExpenseSplitFixtures.percentageRemainder;
		const equalDraft = validEqualDraft(
			fixture.amountCents,
			fixture.percentageSplits.map((split) => split.userId)
		);
		const percentageDraft = switchExpenseSplitMode(equalDraft, 'percentage');

		const preview = previewExpenseSplits(percentageDraft);

		expect(
			percentageDraft.participants.map((participant) => participant.percentage)
		).toEqual(['33.34', '33.33', '33.33']);
		expect(preview.isValid).toBe(true);
		expect(preview.splits).toEqual(fixture.splits);
		expect(sumPreview(preview.splits)).toBe(fixture.amountCents);
	});

	it('keeps cross-mode equivalent previews identical', () => {
		const fixture = backendExpenseSplitFixtures.crossModeRemainder;
		const equalDraft = validEqualDraft(fixture.amountCents, fixture.participantUserIds);
		const exactDraft = switchExpenseSplitMode(equalDraft, 'exact');
		const percentageDraft = switchExpenseSplitMode(equalDraft, 'percentage');

		for (const draft of [equalDraft, exactDraft, percentageDraft]) {
			const preview = previewExpenseSplits(draft);
			expect(preview.isValid).toBe(true);
			expect(preview.splits).toEqual(fixture.splits);
			expect(sumPreview(preview.splits)).toBe(fixture.amountCents);
		}
	});

	it('matches backend arithmetic at the frontend safe-integer boundary', () => {
		const fixture = backendExpenseSplitFixtures.maximumSafePercentage;
		const draft = validEqualDraft(
			fixture.amountCents,
			fixture.percentageSplits.map((split) => split.userId)
		);
		const percentageDraft = switchExpenseSplitMode(draft, 'percentage');

		const preview = previewExpenseSplits(percentageDraft);

		expect(preview.isValid).toBe(true);
		expect(preview.splits).toEqual(fixture.splits);
		expect(sumPreview(preview.splits)).toBe(fixture.amountCents);
	});
});

describe('split preview errors', () => {
	it('requires participants and rejects case-insensitive duplicates', () => {
		const noParticipants = validEqualDraft(100, []);
		expect(previewExpenseSplits(noParticipants).errors).toContainEqual(
			expect.objectContaining({ code: 'participants-required' })
		);

		const duplicate = validEqualDraft(100, [
			backendParticipantD,
			backendParticipantD.toUpperCase()
		]);
		expect(previewExpenseSplits(duplicate).errors).toContainEqual(
			expect.objectContaining({ code: 'participant-duplicate' })
		);
	});

	it('surfaces an equal zero-share boundary', () => {
		const preview = previewExpenseSplits(
			validEqualDraft(2, [backendParticipantA, backendParticipantB, backendParticipantC])
		);

		expect(preview.isValid).toBe(false);
		expect(preview.errors).toContainEqual(
			expect.objectContaining({
				code: 'equal-zero-share',
				message: 'The expense must include at least one cent per participant.'
			})
		);
	});

	it('reports exact input and invalid-total errors with assigned cents', () => {
		const draft = switchExpenseSplitMode(
			validEqualDraft(5_400, [backendParticipantA, backendParticipantB]),
			'exact'
		);
		draft.participants[0].exactAmount = '53';
		draft.participants[1].exactAmount = '0';

		const fieldPreview = previewExpenseSplits(draft);
		expect(fieldPreview.errors).toContainEqual(
			expect.objectContaining({
				code: 'exact-share-not-positive',
				participantUserId: backendParticipantB
			})
		);

		draft.participants[1].exactAmount = '0.50';
		const totalPreview = previewExpenseSplits(draft);
		expect(totalPreview.assignedCents).toBe(5_350);
		expect(totalPreview.errors).toContainEqual(
			expect.objectContaining({
				code: 'exact-total-invalid',
				message: 'Exact shares must add up to the expense amount.'
			})
		);
	});

	it('reports percentage totals and post-remainder zero shares', () => {
		const draft = switchExpenseSplitMode(
			validEqualDraft(2, [backendParticipantA, backendParticipantB]),
			'percentage'
		);
		draft.participants[0].percentage = '50';
		draft.participants[1].percentage = '49.5';

		const totalPreview = previewExpenseSplits(draft);
		expect(totalPreview.assignedBasisPoints).toBe(9_950);
		expect(totalPreview.errors).toContainEqual(
			expect.objectContaining({
				code: 'percentage-total-invalid',
				message: 'Percentages must add up to 100%.'
			})
		);

		draft.participants[0].percentage = '99.99';
		draft.participants[1].percentage = '0.01';
		const zeroPreview = previewExpenseSplits(draft);
		expect(zeroPreview.splits).toEqual([
			{ userId: backendParticipantA, amountCents: 2 },
			{ userId: backendParticipantB, amountCents: 0 }
		]);
		expect(zeroPreview.errors).toContainEqual(
			expect.objectContaining({
				code: 'percentage-zero-share',
				participantUserId: backendParticipantB,
				message: 'Each percentage must produce at least a one-cent share.'
			})
		);
	});
});

describe('participant selection', () => {
	it('uses only the explicit selection and never adds the payer', () => {
		const draft = createExpenseDraft({
			paidByUserId: payerOnly,
			expenseDate: '2026-07-29',
			participantUserIds: [backendParticipantB, backendParticipantA]
		});

		expect(draft.participants.map((participant) => participant.userId)).toEqual([
			backendParticipantB,
			backendParticipantA
		]);
		expect(draft.participants.some((participant) => participant.userId === payerOnly)).toBe(
			false
		);
	});

	it('preserves retained shares in the supplied visible order and leaves additions blank', () => {
		const draft = switchExpenseSplitMode(
			validEqualDraft(900, [backendParticipantA, backendParticipantB]),
			'exact'
		);
		draft.participants[0].exactAmount = '6.00';
		draft.participants[1].exactAmount = '3.00';

		const selected = updateExpenseParticipantSelection(draft, [
			backendParticipantC,
			backendParticipantB
		]);

		expect(selected.participants).toEqual([
			emptyExpectedParticipant(backendParticipantC),
			expect.objectContaining({
				userId: backendParticipantB,
				exactAmount: '3.00'
			})
		]);
		expect(selected.paidByUserId).toBe(payerOnly);
		expect(selected.participants.some((participant) => participant.userId === payerOnly)).toBe(
			false
		);
	});

	it('detects manual share loss before removal without mutating the source', () => {
		const draft = switchExpenseSplitMode(
			validEqualDraft(10, [backendParticipantA, backendParticipantB]),
			'exact'
		);
		expect(participantRemovalDiscardsManualShare(draft, backendParticipantA)).toBe(false);

		draft.participants[0].exactAmount = '0.06';
		expect(participantRemovalDiscardsManualShare(draft, backendParticipantA)).toBe(true);

		const removed = removeExpenseParticipant(draft, backendParticipantA);
		expect(removed.participants.map((participant) => participant.userId)).toEqual([
			backendParticipantB
		]);
		expect(draft.participants).toHaveLength(2);
	});
});

describe('split-mode seeding', () => {
	it('seeds exact inputs from the current equal preview', () => {
		const equal = validEqualDraft(10, [
			backendParticipantC,
			backendParticipantA,
			backendParticipantB
		]);

		const exact = switchExpenseSplitMode(equal, 'exact');

		expect(exact.splitMode).toBe('exact');
		expect(exact.participants.map((participant) => participant.exactAmount)).toEqual([
			'0.04',
			'0.03',
			'0.03'
		]);
		expect(exact.participants.map((participant) => participant.seededExactAmount)).toEqual([
			'0.04',
			'0.03',
			'0.03'
		]);
	});

	it('seeds equal percentages with basis-point remainder in participant order', () => {
		expect(seedEqualPercentageBasisPoints(3)).toEqual([3_334, 3_333, 3_333]);
		expect(seedEqualPercentageBasisPoints(4)).toEqual([2_500, 2_500, 2_500, 2_500]);
		expect(seedEqualPercentageBasisPoints(0)).toEqual([]);

		const percentage = switchExpenseSplitMode(
			validEqualDraft(100, [
				backendParticipantC,
				backendParticipantA,
				backendParticipantB
			]),
			'percentage'
		);
		expect(percentage.participants.map((participant) => participant.percentage)).toEqual([
			'33.34',
			'33.33',
			'33.33'
		]);
	});

	it('seeds exact amounts from a valid percentage preview', () => {
		const percentage = switchExpenseSplitMode(
			validEqualDraft(100, [backendParticipantA, backendParticipantB]),
			'percentage'
		);
		percentage.participants[0].percentage = '60';
		percentage.participants[1].percentage = '40';

		const exact = switchExpenseSplitMode(percentage, 'exact');

		expect(exact.participants.map((participant) => participant.exactAmount)).toEqual([
			'0.60',
			'0.40'
		]);
	});

	it('detects discarded manual values and clears them on equal mode', () => {
		const exact = switchExpenseSplitMode(
			validEqualDraft(100, [backendParticipantA, backendParticipantB]),
			'exact'
		);
		exact.participants[0].exactAmount = '0.60';

		expect(modeSwitchDiscardsManualShares(exact, 'equal')).toBe(true);
		const equal = switchExpenseSplitMode(exact, 'equal');
		expect(equal.participants.every((participant) => participant.exactAmount === '')).toBe(true);
		expect(equal.participants.every((participant) => participant.percentage === '')).toBe(true);
	});
});

describe('persisted expense edit initialization', () => {
	it('always initializes exact mode in active member order without adding the payer', () => {
		const draft = initializeExpenseEditDraft(
			{
				paidByUserId: payerOnly,
				description: 'Dinner',
				amountCents: 1_000,
				expenseDate: '2026-07-29',
				splits: [
					{ userId: backendParticipantB, amountCents: 400 },
					{ userId: backendParticipantA, amountCents: 600 }
				]
			},
			[backendParticipantA, payerOnly, backendParticipantB]
		);

		expect(draft).toMatchObject({
			description: 'Dinner',
			amount: '10.00',
			paidByUserId: payerOnly,
			expenseDate: '2026-07-29',
			splitMode: 'exact'
		});
		expect(draft.participants).toEqual([
			{
				userId: backendParticipantA,
				exactAmount: '6.00',
				seededExactAmount: '6.00',
				percentage: '',
				seededPercentage: ''
			},
			{
				userId: backendParticipantB,
				exactAmount: '4.00',
				seededExactAmount: '4.00',
				percentage: '',
				seededPercentage: ''
			}
		]);
		expect(draft.participants.some((participant) => participant.userId === payerOnly)).toBe(
			false
		);
	});

	it('rejects an unsafe or internally inconsistent persisted model', () => {
		expect(() =>
			initializeExpenseEditDraft({
				paidByUserId: payerOnly,
				description: 'Dinner',
				amountCents: Number.MAX_SAFE_INTEGER + 1,
				expenseDate: '2026-07-29',
				splits: [{ userId: backendParticipantA, amountCents: 1 }]
			})
		).toThrow(RangeError);

		expect(() =>
			initializeExpenseEditDraft({
				paidByUserId: payerOnly,
				description: 'Dinner',
				amountCents: 100,
				expenseDate: '2026-07-29',
				splits: [{ userId: backendParticipantA, amountCents: 99 }]
			})
		).toThrow('Persisted expense splits must total the expense amount.');
	});
});

describe('dirty detection', () => {
	it('compares semantic numeric values and active request state', () => {
		const initial = switchExpenseSplitMode(
			validEqualDraft(1_000, [backendParticipantA, backendParticipantB]),
			'exact'
		);
		const equivalent = cloneDraft(initial);
		equivalent.amount = '10';
		equivalent.description = '\u0085 Dinner \u0085';
		equivalent.participants[0].exactAmount = '5';

		expect(isExpenseDraftDirty(equivalent, initial)).toBe(false);

		const changedShare = cloneDraft(initial);
		changedShare.participants[0].exactAmount = '5.01';
		expect(isExpenseDraftDirty(changedShare, initial)).toBe(true);

		const reordered = cloneDraft(initial);
		reordered.participants.reverse();
		expect(isExpenseDraftDirty(reordered, initial)).toBe(true);

		const changedMode = switchExpenseSplitMode(initial, 'equal');
		expect(isExpenseDraftDirty(changedMode, initial)).toBe(true);
	});

	it('ignores inactive-mode storage that cannot affect the request', () => {
		const initial = validEqualDraft(100, [backendParticipantA, backendParticipantB]);
		const changedInactiveValue = cloneDraft(initial);
		changedInactiveValue.participants[0].exactAmount = '0.99';

		expect(isExpenseDraftDirty(changedInactiveValue, initial)).toBe(false);
	});
});

describe('expense request DTO conversion', () => {
	it('trims the Unicode space set used by the Go API', () => {
		const draft = validEqualDraft(1_000, [
			backendParticipantA,
			backendParticipantB
		]);
		draft.description = '\u0085Dinner\u0085';

		expect(toExpenseInput(draft)).toMatchObject({
			ok: true,
			value: {
				description: 'Dinner'
			}
		});
	});

	it('emits the exact equal DTO and preserves participant order', () => {
		const draft = validEqualDraft(10, [
			backendParticipantC,
			backendParticipantA,
			backendParticipantB
		]);

		expect(toExpenseInput(draft)).toEqual({
			ok: true,
			value: {
				paidByUserId: payerOnly,
				description: 'Dinner',
				amountCents: 10,
				expenseDate: '2026-07-29',
				splitMode: 'equal',
				participantUserIds: [
					backendParticipantC,
					backendParticipantA,
					backendParticipantB
				]
			}
		});
	});

	it('emits only exact split fields for exact mode', () => {
		const draft = switchExpenseSplitMode(
			validEqualDraft(10, [backendParticipantA, backendParticipantB]),
			'exact'
		);
		draft.participants[0].exactAmount = '0.06';
		draft.participants[1].exactAmount = '0.04';

		expect(toExpenseInput(draft)).toEqual({
			ok: true,
			value: {
				paidByUserId: payerOnly,
				description: 'Dinner',
				amountCents: 10,
				expenseDate: '2026-07-29',
				splitMode: 'exact',
				splits: [
					{ userId: backendParticipantA, amountCents: 6 },
					{ userId: backendParticipantB, amountCents: 4 }
				]
			}
		});
	});

	it('emits only integer basis points for percentage mode', () => {
		const draft = switchExpenseSplitMode(
			validEqualDraft(5_400, [
				backendParticipantC,
				backendParticipantA,
				backendParticipantB
			]),
			'percentage'
		);

		expect(toExpenseInput(draft)).toEqual({
			ok: true,
			value: {
				paidByUserId: payerOnly,
				description: 'Dinner',
				amountCents: 5_400,
				expenseDate: '2026-07-29',
				splitMode: 'percentage',
				percentageSplits: [
					{ userId: backendParticipantC, percentageBasisPoints: 3_334 },
					{ userId: backendParticipantA, percentageBasisPoints: 3_333 },
					{ userId: backendParticipantB, percentageBasisPoints: 3_333 }
				]
			}
		});
	});

	it('returns field errors instead of coercing an invalid draft', () => {
		const draft = createExpenseDraft({
			paidByUserId: '',
			expenseDate: '2026-02-29',
			participantUserIds: []
		});

		const result = toExpenseInput(draft);

		expect(result.ok).toBe(false);
		if (result.ok) {
			throw new Error('Expected invalid expense draft.');
		}
		expect(result.errors.map((error) => error.code)).toEqual([
			'description-required',
			'payer-required',
			'date-invalid',
			'amount-required',
			'participants-required'
		]);
	});
});

function validEqualDraft(
	amountCents: number,
	participantUserIds: readonly string[]
): ExpenseDraft {
	const draft = createExpenseDraft({
		paidByUserId: payerOnly,
		expenseDate: '2026-07-29',
		participantUserIds
	});

	return {
		...draft,
		description: 'Dinner',
		amount: formatCentsAsUsdInput(amountCents)
	};
}

function emptyExpectedParticipant(userId: string): ExpenseParticipantDraft {
	return {
		userId,
		exactAmount: '',
		seededExactAmount: '',
		percentage: '',
		seededPercentage: ''
	};
}

function cloneDraft(draft: ExpenseDraft): ExpenseDraft {
	return {
		...draft,
		participants: draft.participants.map((participant) => ({ ...participant }))
	};
}

function sumPreview(splits: readonly { amountCents: number }[]): number {
	return Number(splits.reduce((total, split) => total + BigInt(split.amountCents), 0n));
}
