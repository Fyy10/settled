import { copy } from '$lib/copy/en';
import { parseLocalDate } from '$lib/utils/dates';
import {
	formatBasisPointsAsPercentageInput,
	formatCentsAsUsdInput,
	parsePercentageToBasisPoints,
	parseUsdToCents,
	type DecimalInputErrorCode
} from '$lib/utils/money';

export const PERCENTAGE_BASIS_POINT_TOTAL = 10_000;

const GO_UNICODE_SPACE =
	'\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000';
const GO_TRIM_SPACE_PATTERN = new RegExp(
	`^[${GO_UNICODE_SPACE}]+|[${GO_UNICODE_SPACE}]+$`,
	'g'
);

export type ExpenseSplitMode = 'equal' | 'exact' | 'percentage';

export type ExpenseParticipantDraft = {
	userId: string;
	exactAmount: string;
	seededExactAmount: string;
	percentage: string;
	seededPercentage: string;
};

export type ExpenseDraft = {
	description: string;
	amount: string;
	paidByUserId: string;
	expenseDate: string;
	splitMode: ExpenseSplitMode;
	participants: ExpenseParticipantDraft[];
};

export type PersistedExpenseDraftSource = {
	paidByUserId: string;
	description: string;
	amountCents: number;
	expenseDate: string;
	splits: ExpenseRequestSplit[];
};

export type ExpenseRequestSplit = {
	userId: string;
	amountCents: number;
};

export type PercentageExpenseRequestSplit = {
	userId: string;
	percentageBasisPoints: number;
};

export type ExpenseBaseInput = {
	paidByUserId: string;
	description: string;
	amountCents: number;
	expenseDate: string;
};

export type EqualExpenseInput = ExpenseBaseInput & {
	splitMode: 'equal';
	participantUserIds: string[];
};

export type ExactExpenseInput = ExpenseBaseInput & {
	splitMode: 'exact';
	splits: ExpenseRequestSplit[];
};

export type PercentageExpenseInput = ExpenseBaseInput & {
	splitMode: 'percentage';
	percentageSplits: PercentageExpenseRequestSplit[];
};

export type ExpenseInput = EqualExpenseInput | ExactExpenseInput | PercentageExpenseInput;

export type ExpenseDraftErrorField =
	| 'description'
	| 'amountCents'
	| 'paidByUserId'
	| 'expenseDate'
	| 'participantUserIds'
	| 'splits'
	| 'percentageSplits';

export type ExpenseDraftErrorCode =
	| 'description-required'
	| 'description-too-long'
	| 'amount-required'
	| 'amount-invalid'
	| 'amount-not-positive'
	| 'amount-too-large'
	| 'payer-required'
	| 'date-invalid'
	| 'participants-required'
	| 'participant-invalid'
	| 'participant-duplicate'
	| 'equal-zero-share'
	| 'exact-share-required'
	| 'exact-share-invalid'
	| 'exact-share-not-positive'
	| 'exact-share-too-large'
	| 'exact-total-invalid'
	| 'percentage-required'
	| 'percentage-invalid'
	| 'percentage-not-positive'
	| 'percentage-too-large'
	| 'percentage-total-invalid'
	| 'percentage-zero-share';

export type ExpenseDraftError = {
	code: ExpenseDraftErrorCode;
	field: ExpenseDraftErrorField;
	message: string;
	participantUserId?: string;
};

export type ExpenseSplitPreview = {
	splitMode: ExpenseSplitMode;
	amountCents: number | null;
	assignedCents: number | null;
	assignedBasisPoints: number | null;
	splits: ExpenseRequestSplit[];
	errors: ExpenseDraftError[];
	isValid: boolean;
};

export type ExpenseInputResult =
	| { ok: true; value: ExpenseInput }
	| { ok: false; errors: ExpenseDraftError[] };

export function trimExpenseDescription(value: string): string {
	return value.replace(GO_TRIM_SPACE_PATTERN, '');
}

export function createExpenseDraft(options: {
	paidByUserId: string;
	expenseDate: string;
	participantUserIds: readonly string[];
}): ExpenseDraft {
	return {
		description: '',
		amount: '',
		paidByUserId: options.paidByUserId,
		expenseDate: options.expenseDate,
		splitMode: 'equal',
		participants: options.participantUserIds.map(emptyParticipant)
	};
}

export function initializeExpenseEditDraft(
	expense: PersistedExpenseDraftSource,
	activeParticipantOrder?: readonly string[]
): ExpenseDraft {
	assertPositiveSafeInteger(expense.amountCents, 'Expense amount');
	if (!parseLocalDate(expense.expenseDate).ok) {
		throw new RangeError('Persisted expense date must use YYYY-MM-DD.');
	}
	if (expense.splits.length === 0) {
		throw new RangeError('Persisted expense must include at least one split.');
	}

	const splitByUserId = new Map<string, ExpenseRequestSplit>();
	let splitTotal = 0n;
	for (const split of expense.splits) {
		assertPositiveSafeInteger(split.amountCents, 'Persisted split amount');
		if (split.userId === '' || splitByUserId.has(split.userId)) {
			throw new RangeError('Persisted expense split user IDs must be non-empty and unique.');
		}
		splitByUserId.set(split.userId, split);
		splitTotal += BigInt(split.amountCents);
	}
	if (splitTotal !== BigInt(expense.amountCents)) {
		throw new RangeError('Persisted expense splits must total the expense amount.');
	}

	const orderedUserIds = orderPersistedParticipants(expense.splits, activeParticipantOrder);
	const participants = orderedUserIds.map((userId) => {
		const split = splitByUserId.get(userId);
		if (!split) {
			throw new Error('Persisted split order invariant failed.');
		}
		const exactAmount = formatCentsAsUsdInput(split.amountCents);

		return {
			userId,
			exactAmount,
			seededExactAmount: exactAmount,
			percentage: '',
			seededPercentage: ''
		};
	});

	return {
		description: expense.description,
		amount: formatCentsAsUsdInput(expense.amountCents),
		paidByUserId: expense.paidByUserId,
		expenseDate: expense.expenseDate,
		splitMode: 'exact',
		participants
	};
}

export function updateExpenseParticipantSelection(
	draft: ExpenseDraft,
	orderedSelectedUserIds: readonly string[]
): ExpenseDraft {
	const existing = new Map(
		draft.participants.map((participant) => [participant.userId, participant] as const)
	);

	return {
		...draft,
		participants: orderedSelectedUserIds.map((userId) => ({
			...(existing.get(userId) ?? emptyParticipant(userId))
		}))
	};
}

export function removeExpenseParticipant(draft: ExpenseDraft, userId: string): ExpenseDraft {
	return {
		...draft,
		participants: draft.participants
			.filter((participant) => participant.userId !== userId)
			.map((participant) => ({ ...participant }))
	};
}

export function participantRemovalDiscardsManualShare(
	draft: ExpenseDraft,
	userId: string
): boolean {
	const participant = draft.participants.find((item) => item.userId === userId);
	if (!participant) {
		return false;
	}

	return participantHasManualShare(participant, draft.splitMode);
}

export function modeSwitchDiscardsManualShares(
	draft: ExpenseDraft,
	nextMode: ExpenseSplitMode
): boolean {
	if (nextMode === draft.splitMode || draft.splitMode === 'equal') {
		return false;
	}

	return draft.participants.some((participant) =>
		participantHasManualShare(participant, draft.splitMode)
	);
}

export function switchExpenseSplitMode(
	draft: ExpenseDraft,
	nextMode: ExpenseSplitMode
): ExpenseDraft {
	if (nextMode === draft.splitMode) {
		return {
			...draft,
			participants: draft.participants.map((participant) => ({ ...participant }))
		};
	}

	if (nextMode === 'equal') {
		return {
			...draft,
			splitMode: nextMode,
			participants: draft.participants.map((participant) => ({
				...participant,
				exactAmount: '',
				seededExactAmount: '',
				percentage: '',
				seededPercentage: ''
			}))
		};
	}

	if (nextMode === 'percentage') {
		const basisPoints = seedEqualPercentageBasisPoints(draft.participants.length);

		return {
			...draft,
			splitMode: nextMode,
			participants: draft.participants.map((participant, index) => {
				const percentage = formatBasisPointsAsPercentageInput(basisPoints[index] ?? 0);

				return {
					...participant,
					percentage,
					seededPercentage: percentage
				};
			})
		};
	}

	const exactAmounts = exactSeedAmounts(draft);
	return {
		...draft,
		splitMode: nextMode,
		participants: draft.participants.map((participant, index) => {
			const exactAmount =
				exactAmounts === null ? '' : formatCentsAsUsdInput(exactAmounts[index] ?? 0);

			return {
				...participant,
				exactAmount,
				seededExactAmount: exactAmount
			};
		})
	};
}

export function seedEqualPercentageBasisPoints(participantCount: number): number[] {
	if (!Number.isSafeInteger(participantCount) || participantCount < 0) {
		throw new RangeError('Participant count must be a non-negative safe integer.');
	}
	if (participantCount === 0) {
		return [];
	}

	const count = BigInt(participantCount);
	const base = BigInt(PERCENTAGE_BASIS_POINT_TOTAL) / count;
	const remainder = BigInt(PERCENTAGE_BASIS_POINT_TOTAL) % count;

	return Array.from({ length: participantCount }, (_, index) =>
		Number(base + (BigInt(index) < remainder ? 1n : 0n))
	);
}

export function previewExpenseSplits(draft: ExpenseDraft): ExpenseSplitPreview {
	const errors: ExpenseDraftError[] = [];
	const amountResult = parseUsdToCents(draft.amount);
	if (!amountResult.ok) {
		errors.push(amountDraftError(amountResult.code, amountResult.error));
	}

	const splitField = splitFieldForMode(draft.splitMode);
	errors.push(...participantErrors(draft.participants, splitField));

	if (!amountResult.ok || errors.length > 0) {
		return previewResult(draft.splitMode, null, null, null, [], errors);
	}

	switch (draft.splitMode) {
		case 'equal':
			return previewEqualSplits(amountResult.value, draft.participants);
		case 'exact':
			return previewExactSplits(amountResult.value, draft.participants);
		case 'percentage':
			return previewPercentageSplits(amountResult.value, draft.participants);
	}
}

export function toExpenseInput(draft: ExpenseDraft): ExpenseInputResult {
	const errors: ExpenseDraftError[] = [];
	const description = trimExpenseDescription(draft.description);
	if (description === '') {
		errors.push({
			code: 'description-required',
			field: 'description',
			message: copy.expenseDraft.descriptionRequired
		});
	} else if ([...description].length > 240) {
		errors.push({
			code: 'description-too-long',
			field: 'description',
			message: copy.expenseDraft.descriptionTooLong
		});
	}

	if (draft.paidByUserId.trim() === '') {
		errors.push({
			code: 'payer-required',
			field: 'paidByUserId',
			message: copy.expenseDraft.payerRequired
		});
	}

	const dateResult = parseLocalDate(draft.expenseDate);
	if (!dateResult.ok) {
		errors.push({
			code: 'date-invalid',
			field: 'expenseDate',
			message: dateResult.error
		});
	}

	const preview = previewExpenseSplits(draft);
	errors.push(...preview.errors);
	if (errors.length > 0 || preview.amountCents === null) {
		return { ok: false, errors };
	}

	const base: ExpenseBaseInput = {
		paidByUserId: draft.paidByUserId,
		description,
		amountCents: preview.amountCents,
		expenseDate: draft.expenseDate
	};

	switch (draft.splitMode) {
		case 'equal':
			return {
				ok: true,
				value: {
					...base,
					splitMode: 'equal',
					participantUserIds: draft.participants.map((participant) => participant.userId)
				}
			};
		case 'exact':
			return {
				ok: true,
				value: {
					...base,
					splitMode: 'exact',
					splits: preview.splits.map((split) => ({ ...split }))
				}
			};
		case 'percentage':
			return {
				ok: true,
				value: {
					...base,
					splitMode: 'percentage',
					percentageSplits: draft.participants.map((participant) => {
						const result = parsePercentageToBasisPoints(participant.percentage);
						if (!result.ok) {
							throw new Error('Percentage request conversion invariant failed.');
						}

						return {
							userId: participant.userId,
							percentageBasisPoints: result.value
						};
					})
				}
			};
	}
}

export function persistedSplitsForExpenseInput(
	input: ExpenseInput
): ExpenseRequestSplit[] {
	switch (input.splitMode) {
		case 'equal': {
			if (input.participantUserIds.length === 0) {
				return [];
			}
			const amounts = calculateEqualAmounts(
				input.amountCents,
				input.participantUserIds.length
			);
			return input.participantUserIds.map((userId, index) => ({
				userId,
				amountCents: amounts[index]
			}));
		}
		case 'exact':
			return input.splits.map((split) => ({ ...split }));
		case 'percentage': {
			const amounts = calculatePercentageAmounts(
				input.amountCents,
				input.percentageSplits.map((split) => split.percentageBasisPoints)
			);
			return input.percentageSplits.map((split, index) => ({
				userId: split.userId,
				amountCents: amounts[index]
			}));
		}
	}
}

export function isExpenseDraftDirty(draft: ExpenseDraft, initial: ExpenseDraft): boolean {
	return draftFingerprint(draft) !== draftFingerprint(initial);
}

function previewEqualSplits(
	amountCents: number,
	participants: readonly ExpenseParticipantDraft[]
): ExpenseSplitPreview {
	if (amountCents < participants.length) {
		return previewResult(
			'equal',
			amountCents,
			null,
			null,
			[],
			[
				{
					code: 'equal-zero-share',
					field: 'participantUserIds',
					message: copy.expenseDraft.equalZeroShare
				}
			]
		);
	}

	const amounts = calculateEqualAmounts(amountCents, participants.length);
	const splits = participants.map((participant, index) => ({
		userId: participant.userId,
		amountCents: amounts[index]
	}));

	return previewResult('equal', amountCents, amountCents, null, splits, []);
}

function previewExactSplits(
	amountCents: number,
	participants: readonly ExpenseParticipantDraft[]
): ExpenseSplitPreview {
	const errors: ExpenseDraftError[] = [];
	const splits: ExpenseRequestSplit[] = [];
	let assigned = 0n;

	for (const participant of participants) {
		const result = parseUsdToCents(participant.exactAmount);
		if (!result.ok) {
			errors.push(exactShareError(result.code, participant.userId));
			continue;
		}

		assigned += BigInt(result.value);
		splits.push({ userId: participant.userId, amountCents: result.value });
	}

	if (errors.length === 0 && assigned !== BigInt(amountCents)) {
		errors.push({
			code: 'exact-total-invalid',
			field: 'splits',
			message: copy.expenseDraft.exactTotalInvalid
		});
	}

	return previewResult(
		'exact',
		amountCents,
		safeBigIntToNumber(assigned),
		null,
		splits,
		errors
	);
}

function previewPercentageSplits(
	amountCents: number,
	participants: readonly ExpenseParticipantDraft[]
): ExpenseSplitPreview {
	const errors: ExpenseDraftError[] = [];
	const basisPoints: number[] = [];
	let assignedBasisPoints = 0n;

	for (const participant of participants) {
		const result = parsePercentageToBasisPoints(participant.percentage);
		if (!result.ok) {
			errors.push(percentageShareError(result.code, participant.userId));
			continue;
		}

		assignedBasisPoints += BigInt(result.value);
		basisPoints.push(result.value);
	}

	const assignedBasisPointsNumber = safeBigIntToNumber(assignedBasisPoints);
	if (errors.length > 0) {
		return previewResult(
			'percentage',
			amountCents,
			null,
			assignedBasisPointsNumber,
			[],
			errors
		);
	}
	if (assignedBasisPoints !== BigInt(PERCENTAGE_BASIS_POINT_TOTAL)) {
		return previewResult(
			'percentage',
			amountCents,
			null,
			assignedBasisPointsNumber,
			[],
			[
				{
					code: 'percentage-total-invalid',
					field: 'percentageSplits',
					message: copy.expenseDraft.percentageTotalInvalid
				}
			]
		);
	}

	const amounts = calculatePercentageAmounts(amountCents, basisPoints);
	const splits = participants.map((participant, index) => ({
		userId: participant.userId,
		amountCents: amounts[index]
	}));
	for (const split of splits) {
		if (split.amountCents === 0) {
			errors.push({
				code: 'percentage-zero-share',
				field: 'percentageSplits',
				message: copy.expenseDraft.percentageZeroShare,
				participantUserId: split.userId
			});
		}
	}

	return previewResult(
		'percentage',
		amountCents,
		amountCents,
		PERCENTAGE_BASIS_POINT_TOTAL,
		splits,
		errors
	);
}

function exactSeedAmounts(draft: ExpenseDraft): number[] | null {
	const preview = previewExpenseSplits(draft);
	if (preview.isValid) {
		return preview.splits.map((split) => split.amountCents);
	}

	const amountResult = parseUsdToCents(draft.amount);
	if (!amountResult.ok || draft.participants.length === 0) {
		return null;
	}
	if (
		participantErrors(draft.participants, splitFieldForMode(draft.splitMode)).length > 0 ||
		amountResult.value < draft.participants.length
	) {
		return null;
	}

	return calculateEqualAmounts(amountResult.value, draft.participants.length);
}

function calculateEqualAmounts(amountCents: number, participantCount: number): number[] {
	const amount = BigInt(amountCents);
	const count = BigInt(participantCount);
	const base = amount / count;
	const remainder = amount % count;

	return Array.from({ length: participantCount }, (_, index) =>
		Number(base + (BigInt(index) < remainder ? 1n : 0n))
	);
}

function calculatePercentageAmounts(amountCents: number, basisPoints: readonly number[]): number[] {
	const amount = BigInt(amountCents);
	const denominator = BigInt(PERCENTAGE_BASIS_POINT_TOTAL);
	const amounts = basisPoints.map((value) => Number((amount * BigInt(value)) / denominator));
	const floorTotal = amounts.reduce((total, value) => total + BigInt(value), 0n);
	const remainder = Number(amount - floorTotal);

	for (let index = 0; index < remainder; index += 1) {
		amounts[index] += 1;
	}

	return amounts;
}

function participantErrors(
	participants: readonly ExpenseParticipantDraft[],
	field: ExpenseDraftErrorField
): ExpenseDraftError[] {
	if (participants.length === 0) {
		return [
			{
				code: 'participants-required',
				field,
				message: copy.expenseDraft.participantsRequired
			}
		];
	}

	const errors: ExpenseDraftError[] = [];
	const seen = new Set<string>();
	for (const participant of participants) {
		if (participant.userId === '') {
			errors.push({
				code: 'participant-invalid',
				field,
				message: copy.expenseDraft.participantInvalid
			});
			continue;
		}

		const identity = participant.userId.toLowerCase();
		if (seen.has(identity)) {
			errors.push({
				code: 'participant-duplicate',
				field,
				message: copy.expenseDraft.participantDuplicate,
				participantUserId: participant.userId
			});
		}
		seen.add(identity);
	}

	return errors;
}

function amountDraftError(code: DecimalInputErrorCode, message: string): ExpenseDraftError {
	const codeByInputError: Record<DecimalInputErrorCode, ExpenseDraftErrorCode> = {
		required: 'amount-required',
		'invalid-format': 'amount-invalid',
		'not-positive': 'amount-not-positive',
		'out-of-range': 'amount-too-large'
	};

	return {
		code: codeByInputError[code],
		field: 'amountCents',
		message
	};
}

function exactShareError(
	code: DecimalInputErrorCode,
	participantUserId: string
): ExpenseDraftError {
	const codeByInputError: Record<DecimalInputErrorCode, ExpenseDraftErrorCode> = {
		required: 'exact-share-required',
		'invalid-format': 'exact-share-invalid',
		'not-positive': 'exact-share-not-positive',
		'out-of-range': 'exact-share-too-large'
	};
	const messageByInputError: Record<DecimalInputErrorCode, string> = {
		required: copy.expenseDraft.exactShareRequired,
		'invalid-format': copy.expenseDraft.exactShareInvalid,
		'not-positive': copy.expenseDraft.exactSharePositive,
		'out-of-range': copy.expenseDraft.exactShareTooLarge
	};

	return {
		code: codeByInputError[code],
		field: 'splits',
		message: messageByInputError[code],
		participantUserId
	};
}

function percentageShareError(
	code: DecimalInputErrorCode,
	participantUserId: string
): ExpenseDraftError {
	const codeByInputError: Record<DecimalInputErrorCode, ExpenseDraftErrorCode> = {
		required: 'percentage-required',
		'invalid-format': 'percentage-invalid',
		'not-positive': 'percentage-not-positive',
		'out-of-range': 'percentage-too-large'
	};

	return {
		code: codeByInputError[code],
		field: 'percentageSplits',
		message:
			code === 'required'
				? copy.expenseDraft.percentageRequired
				: code === 'invalid-format'
					? copy.expenseDraft.percentageInvalid
					: code === 'not-positive'
						? copy.expenseDraft.percentagePositive
						: copy.expenseDraft.percentageTooLarge,
		participantUserId
	};
}

function previewResult(
	splitMode: ExpenseSplitMode,
	amountCents: number | null,
	assignedCents: number | null,
	assignedBasisPoints: number | null,
	splits: ExpenseRequestSplit[],
	errors: ExpenseDraftError[]
): ExpenseSplitPreview {
	return {
		splitMode,
		amountCents,
		assignedCents,
		assignedBasisPoints,
		splits,
		errors,
		isValid: errors.length === 0
	};
}

function splitFieldForMode(mode: ExpenseSplitMode): ExpenseDraftErrorField {
	switch (mode) {
		case 'equal':
			return 'participantUserIds';
		case 'exact':
			return 'splits';
		case 'percentage':
			return 'percentageSplits';
	}
}

function emptyParticipant(userId: string): ExpenseParticipantDraft {
	return {
		userId,
		exactAmount: '',
		seededExactAmount: '',
		percentage: '',
		seededPercentage: ''
	};
}

function participantHasManualShare(
	participant: ExpenseParticipantDraft,
	mode: ExpenseSplitMode
): boolean {
	switch (mode) {
		case 'equal':
			return false;
		case 'exact':
			return !decimalInputsAreEquivalent(
				participant.exactAmount,
				participant.seededExactAmount,
				parseUsdToCents
			);
		case 'percentage':
			return !decimalInputsAreEquivalent(
				participant.percentage,
				participant.seededPercentage,
				parsePercentageToBasisPoints
			);
	}
}

function decimalInputsAreEquivalent(
	first: string,
	second: string,
	parse: typeof parseUsdToCents
): boolean {
	if (first === second) {
		return true;
	}

	const firstResult = parse(first);
	const secondResult = parse(second);
	return (
		firstResult.ok &&
		secondResult.ok &&
		firstResult.value === secondResult.value
	);
}

function orderPersistedParticipants(
	splits: readonly ExpenseRequestSplit[],
	activeParticipantOrder?: readonly string[]
): string[] {
	if (!activeParticipantOrder) {
		return splits.map((split) => split.userId);
	}

	const splitIDs = new Set(splits.map((split) => split.userId));
	const ordered: string[] = [];
	const seen = new Set<string>();
	for (const userId of activeParticipantOrder) {
		if (seen.has(userId)) {
			throw new RangeError('Active participant order must not contain duplicate user IDs.');
		}
		seen.add(userId);
		if (splitIDs.has(userId)) {
			ordered.push(userId);
		}
	}
	for (const split of splits) {
		if (!seen.has(split.userId)) {
			ordered.push(split.userId);
		}
	}

	return ordered;
}

function draftFingerprint(draft: ExpenseDraft): string {
	const activeParticipants = draft.participants.map((participant) => {
		switch (draft.splitMode) {
			case 'equal':
				return { userId: participant.userId };
			case 'exact':
				return {
					userId: participant.userId,
					exactAmount: canonicalDecimalInput(participant.exactAmount, parseUsdToCents)
				};
			case 'percentage':
				return {
					userId: participant.userId,
					percentage: canonicalDecimalInput(
						participant.percentage,
						parsePercentageToBasisPoints
					)
				};
		}
	});

	return JSON.stringify({
		description: trimExpenseDescription(draft.description),
		amount: canonicalDecimalInput(draft.amount, parseUsdToCents),
		paidByUserId: draft.paidByUserId,
		expenseDate: draft.expenseDate,
		splitMode: draft.splitMode,
		participants: activeParticipants
	});
}

function canonicalDecimalInput(
	value: string,
	parse: typeof parseUsdToCents
): string {
	const result = parse(value);
	return result.ok ? `valid:${result.value}` : `invalid:${value}`;
}

function safeBigIntToNumber(value: bigint): number | null {
	return value <= BigInt(Number.MAX_SAFE_INTEGER) ? Number(value) : null;
}

function assertPositiveSafeInteger(value: number, label: string): void {
	if (!Number.isSafeInteger(value) || value <= 0) {
		throw new RangeError(`${label} must be a positive safe integer.`);
	}
}
