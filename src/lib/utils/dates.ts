import { copy } from '$lib/copy/en';

export type LocalDateResult =
	| { ok: true; value: string }
	| { ok: false; error: string };

const localDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/;

export function parseLocalDate(value: string): LocalDateResult {
	const match = localDatePattern.exec(value);
	if (!match) {
		return { ok: false, error: copy.expenseDraft.dateInvalid };
	}

	const year = Number(match[1]);
	const month = Number(match[2]);
	const day = Number(match[3]);
	if (
		year < 1 ||
		month < 1 ||
		month > 12 ||
		day < 1 ||
		day > daysInMonth(year, month)
	) {
		return { ok: false, error: copy.expenseDraft.dateInvalid };
	}

	return { ok: true, value };
}

export function isStrictLocalDate(value: string): boolean {
	return parseLocalDate(value).ok;
}

export function formatLocalDate(date: Date): string {
	if (Number.isNaN(date.getTime())) {
		throw new RangeError('Local date source must be valid.');
	}

	const year = date.getFullYear();
	if (year < 1 || year > 9_999) {
		throw new RangeError('Local date year must fit YYYY.');
	}

	return [
		String(year).padStart(4, '0'),
		String(date.getMonth() + 1).padStart(2, '0'),
		String(date.getDate()).padStart(2, '0')
	].join('-');
}

export function currentLocalDate(now: Date = new Date()): string {
	return formatLocalDate(now);
}

function daysInMonth(year: number, month: number): number {
	switch (month) {
		case 2:
			return isLeapYear(year) ? 29 : 28;
		case 4:
		case 6:
		case 9:
		case 11:
			return 30;
		default:
			return 31;
	}
}

function isLeapYear(year: number): boolean {
	return year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
}
