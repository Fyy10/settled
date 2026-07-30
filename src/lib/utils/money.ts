import { copy } from '$lib/copy/en';

export type DecimalInputErrorCode =
	| 'required'
	| 'invalid-format'
	| 'not-positive'
	| 'out-of-range';

export type DecimalInputResult =
	| { ok: true; value: number }
	| { ok: false; code: DecimalInputErrorCode; error: string };

const MAX_SAFE_INTEGER = BigInt(Number.MAX_SAFE_INTEGER);
const USD_SCALE = 100n;
const BASIS_POINT_SCALE = 100n;
const MAX_PERCENTAGE_BASIS_POINTS = 10_000n;
const decimalPattern = /^(\d+)(?:\.(\d{1,2}))?$/;

const usdFormatter = new Intl.NumberFormat('en-US', {
	style: 'currency',
	currency: 'USD',
	minimumFractionDigits: 2,
	maximumFractionDigits: 2
});

export function parseUsdToCents(value: string): DecimalInputResult {
	return parsePositiveDecimal(
		value,
		USD_SCALE,
		MAX_SAFE_INTEGER,
		{
			required: copy.expenseDraft.amountRequired,
			invalid: copy.expenseDraft.amountInvalid,
			positive: copy.expenseDraft.amountPositive,
			range: copy.expenseDraft.amountTooLarge
		}
	);
}

export function parsePercentageToBasisPoints(value: string): DecimalInputResult {
	return parsePositiveDecimal(
		value,
		BASIS_POINT_SCALE,
		MAX_PERCENTAGE_BASIS_POINTS,
		{
			required: copy.expenseDraft.percentageRequired,
			invalid: copy.expenseDraft.percentageInvalid,
			positive: copy.expenseDraft.percentagePositive,
			range: copy.expenseDraft.percentageTooLarge
		}
	);
}

export function formatUsd(cents: number): string {
	assertNonNegativeSafeInteger(cents, 'USD cents');

	const centsInteger = BigInt(cents);
	const wholeDollars = centsInteger / USD_SCALE;
	const fractionalCents = (centsInteger % USD_SCALE).toString().padStart(2, '0');

	return usdFormatter
		.formatToParts(wholeDollars)
		.map((part) => (part.type === 'fraction' ? fractionalCents : part.value))
		.join('');
}

export function formatCentsAsUsdInput(cents: number): string {
	assertNonNegativeSafeInteger(cents, 'USD cents');

	const centsInteger = BigInt(cents);
	const wholeDollars = centsInteger / USD_SCALE;
	const fractionalCents = (centsInteger % USD_SCALE).toString().padStart(2, '0');

	return `${wholeDollars}.${fractionalCents}`;
}

export function formatBasisPointsAsPercentageInput(basisPoints: number): string {
	if (
		!Number.isSafeInteger(basisPoints) ||
		basisPoints < 0 ||
		basisPoints > Number(MAX_PERCENTAGE_BASIS_POINTS)
	) {
		throw new RangeError('Percentage basis points must be an integer from 0 through 10000.');
	}

	const whole = Math.floor(basisPoints / 100);
	const fraction = basisPoints % 100;

	if (fraction === 0) {
		return String(whole);
	}
	if (fraction % 10 === 0) {
		return `${whole}.${fraction / 10}`;
	}

	return `${whole}.${String(fraction).padStart(2, '0')}`;
}

function parsePositiveDecimal(
	value: string,
	scale: bigint,
	maximum: bigint,
	messages: {
		required: string;
		invalid: string;
		positive: string;
		range: string;
	}
): DecimalInputResult {
	const normalized = value.trim();
	if (normalized === '') {
		return { ok: false, code: 'required', error: messages.required };
	}

	const match = decimalPattern.exec(normalized);
	if (!match) {
		return { ok: false, code: 'invalid-format', error: messages.invalid };
	}

	const whole = BigInt(match[1]);
	const fraction = BigInt((match[2] ?? '').padEnd(2, '0') || '0');
	const units = whole * scale + fraction;

	if (units === 0n) {
		return { ok: false, code: 'not-positive', error: messages.positive };
	}
	if (units > maximum) {
		return { ok: false, code: 'out-of-range', error: messages.range };
	}

	return { ok: true, value: Number(units) };
}

function assertNonNegativeSafeInteger(value: number, label: string): void {
	if (!Number.isSafeInteger(value) || value < 0) {
		throw new RangeError(`${label} must be a non-negative safe integer.`);
	}
}
