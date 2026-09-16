import type {
	Repayment,
	ReplaceRepaymentInput
} from '$lib/api/types';
import { copy } from '$lib/copy/en';
import { parseLocalDate } from '$lib/utils/dates';
import {
	formatCentsAsUsdInput,
	parseUsdToCents,
	type DecimalInputErrorCode
} from '$lib/utils/money';

const GO_UNICODE_SPACE =
	'\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000';
const GO_TRIM_SPACE_PATTERN = new RegExp(
	`^[${GO_UNICODE_SPACE}]+|[${GO_UNICODE_SPACE}]+$`,
	'g'
);
const REPAYMENT_NOTE_MAX_RUNES = 240;
const CONTROL_OR_BIDI_PATTERN =
	/[\p{Cc}\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/u;
const ASCII_INTEGER_PATTERN = /^\d+$/;

export type RepaymentDraft = {
	fromUserId: string;
	toUserId: string;
	amount: string;
	repaymentDate: string;
	note: string;
};

export type RepaymentDraftErrorField =
	| 'fromUserId'
	| 'toUserId'
	| 'amountCents'
	| 'repaymentDate'
	| 'note';

export type RepaymentDraftErrorCode =
	| 'from-required'
	| 'from-invalid'
	| 'to-required'
	| 'to-invalid'
	| 'users-same'
	| 'amount-required'
	| 'amount-invalid'
	| 'amount-not-positive'
	| 'amount-too-large'
	| 'date-invalid'
	| 'note-too-long'
	| 'note-invalid';

export type RepaymentDraftError = {
	code: RepaymentDraftErrorCode;
	field: RepaymentDraftErrorField;
	message: string;
};

export type RepaymentInputResult =
	| { ok: true; value: ReplaceRepaymentInput }
	| { ok: false; errors: RepaymentDraftError[] };

export function createRepaymentDraft(options: {
	fromUserId: string;
	repaymentDate: string;
}): RepaymentDraft {
	return {
		fromUserId: options.fromUserId,
		toUserId: '',
		amount: '',
		repaymentDate: options.repaymentDate,
		note: ''
	};
}

export function initializeRepaymentEditDraft(
	repayment: Pick<
		Repayment,
		| 'fromUserId'
		| 'toUserId'
		| 'amountCents'
		| 'repaymentDate'
		| 'note'
	>
): RepaymentDraft {
	if (
		repayment.fromUserId === '' ||
		repayment.toUserId === '' ||
		repayment.fromUserId.toLocaleLowerCase('en-US') ===
			repayment.toUserId.toLocaleLowerCase('en-US')
	) {
		throw new RangeError(
			'Persisted repayment members must be non-empty and different.'
		);
	}
	if (
		!Number.isSafeInteger(repayment.amountCents) ||
		repayment.amountCents <= 0
	) {
		throw new RangeError(
			'Persisted repayment amount must be a positive safe integer.'
		);
	}
	if (!parseLocalDate(repayment.repaymentDate).ok) {
		throw new RangeError(
			'Persisted repayment date must use YYYY-MM-DD.'
		);
	}
	if (
		repayment.note !== null &&
		(normalizeRepaymentNote(repayment.note) !== repayment.note ||
			repaymentNoteError(repayment.note) !== null)
	) {
		throw new RangeError('Persisted repayment note must be normalized.');
	}

	return {
		fromUserId: repayment.fromUserId,
		toUserId: repayment.toUserId,
		amount: formatCentsAsUsdInput(repayment.amountCents),
		repaymentDate: repayment.repaymentDate,
		note: repayment.note ?? ''
	};
}

export function applySettlementRepaymentPrefill(
	draft: Readonly<RepaymentDraft>,
	query: URLSearchParams,
	activeMemberIds: readonly string[]
): RepaymentDraft {
	const fromValues = query.getAll('from');
	const toValues = query.getAll('to');
	const amountValues = query.getAll('amountCents');
	if (
		fromValues.length !== 1 ||
		toValues.length !== 1 ||
		amountValues.length !== 1
	) {
		return { ...draft };
	}

	const fromUserId = fromValues[0];
	const toUserId = toValues[0];
	const amountText = amountValues[0];
	const activeIds = new Set(activeMemberIds);
	if (
		!activeIds.has(fromUserId) ||
		!activeIds.has(toUserId) ||
		fromUserId === toUserId ||
		!ASCII_INTEGER_PATTERN.test(amountText)
	) {
		return { ...draft };
	}

	const amountCents = BigInt(amountText);
	if (
		amountCents <= 0n ||
		amountCents > BigInt(Number.MAX_SAFE_INTEGER)
	) {
		return { ...draft };
	}

	return {
		...draft,
		fromUserId,
		toUserId,
		amount: formatCentsAsUsdInput(Number(amountCents))
	};
}

export function toRepaymentInput(
	draft: Readonly<RepaymentDraft>,
	activeMemberIds?: readonly string[]
): RepaymentInputResult {
	const errors: RepaymentDraftError[] = [];
	const activeIds =
		activeMemberIds === undefined ? null : new Set(activeMemberIds);

	if (draft.fromUserId === '') {
		errors.push({
			code: 'from-required',
			field: 'fromUserId',
			message: copy.repaymentDraft.fromRequired
		});
	} else if (activeIds !== null && !activeIds.has(draft.fromUserId)) {
		errors.push({
			code: 'from-invalid',
			field: 'fromUserId',
			message: copy.repaymentDraft.fromInvalid
		});
	}

	if (draft.toUserId === '') {
		errors.push({
			code: 'to-required',
			field: 'toUserId',
			message: copy.repaymentDraft.toRequired
		});
	} else if (activeIds !== null && !activeIds.has(draft.toUserId)) {
		errors.push({
			code: 'to-invalid',
			field: 'toUserId',
			message: copy.repaymentDraft.toInvalid
		});
	}

	if (
		draft.fromUserId !== '' &&
		draft.toUserId !== '' &&
		draft.fromUserId === draft.toUserId
	) {
		errors.push({
			code: 'users-same',
			field: 'toUserId',
			message: copy.repaymentDraft.usersDifferent
		});
	}

	const amountResult = parseUsdToCents(draft.amount);
	if (!amountResult.ok) {
		errors.push(amountDraftError(amountResult.code));
	}

	const dateResult = parseLocalDate(draft.repaymentDate);
	if (!dateResult.ok) {
		errors.push({
			code: 'date-invalid',
			field: 'repaymentDate',
			message: copy.repaymentDraft.dateInvalid
		});
	}

	const normalizedNote = normalizeRepaymentNote(draft.note);
	const noteError = repaymentNoteError(normalizedNote);
	if (noteError !== null) {
		errors.push(noteError);
	}

	if (
		errors.length > 0 ||
		!amountResult.ok ||
		!dateResult.ok
	) {
		return { ok: false, errors };
	}

	return {
		ok: true,
		value: {
			fromUserId: draft.fromUserId,
			toUserId: draft.toUserId,
			amountCents: amountResult.value,
			repaymentDate: dateResult.value,
			note: normalizedNote
		}
	};
}

export function isRepaymentDraftDirty(
	draft: Readonly<RepaymentDraft>,
	initial: Readonly<RepaymentDraft>
): boolean {
	return (
		draft.fromUserId !== initial.fromUserId ||
		draft.toUserId !== initial.toUserId ||
		amountFingerprint(draft.amount) !== amountFingerprint(initial.amount) ||
		draft.repaymentDate !== initial.repaymentDate ||
		noteFingerprint(draft.note) !== noteFingerprint(initial.note)
	);
}

export function normalizeRepaymentNote(
	value: string | null | undefined
): string | null {
	if (value === null || value === undefined) {
		return null;
	}

	const normalized = value.replace(GO_TRIM_SPACE_PATTERN, '');
	return normalized === '' ? null : normalized;
}

export function isValidPersistedRepaymentNote(
	value: string | null
): boolean {
	return (
		value === null ||
		(normalizeRepaymentNote(value) === value &&
			repaymentNoteError(value) === null)
	);
}

function repaymentNoteError(
	value: string | null
): RepaymentDraftError | null {
	if (value === null) {
		return null;
	}
	if (CONTROL_OR_BIDI_PATTERN.test(value)) {
		return {
			code: 'note-invalid',
			field: 'note',
			message: copy.repaymentDraft.noteInvalid
		};
	}
	if ([...value].length > REPAYMENT_NOTE_MAX_RUNES) {
		return {
			code: 'note-too-long',
			field: 'note',
			message: copy.repaymentDraft.noteTooLong
		};
	}

	return null;
}

function amountDraftError(
	code: DecimalInputErrorCode
): RepaymentDraftError {
	switch (code) {
		case 'required':
			return {
				code: 'amount-required',
				field: 'amountCents',
				message: copy.repaymentDraft.amountRequired
			};
		case 'invalid-format':
			return {
				code: 'amount-invalid',
				field: 'amountCents',
				message: copy.repaymentDraft.amountInvalid
			};
		case 'not-positive':
			return {
				code: 'amount-not-positive',
				field: 'amountCents',
				message: copy.repaymentDraft.amountPositive
			};
		case 'out-of-range':
			return {
				code: 'amount-too-large',
				field: 'amountCents',
				message: copy.repaymentDraft.amountTooLarge
			};
	}
}

function amountFingerprint(value: string): string {
	const parsed = parseUsdToCents(value);
	return parsed.ok ? `valid:${parsed.value}` : `invalid:${value}`;
}

function noteFingerprint(value: string): string {
	const normalized = normalizeRepaymentNote(value);
	return repaymentNoteError(normalized) === null
		? `valid:${normalized ?? ''}`
		: `invalid:${value}`;
}
