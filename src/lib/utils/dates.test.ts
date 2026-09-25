import { describe, expect, it } from 'vitest';

import { currentLocalDate, formatLocalDate, isStrictLocalDate, parseLocalDate } from './dates';

describe('strict local calendar dates', () => {
	it.each([
		'1904-02-29',
		'2000-02-29',
		'2024-02-29',
		'2026-07-29',
		'9999-12-31'
	])('accepts %s without timezone conversion', (value) => {
		expect(parseLocalDate(value)).toEqual({ ok: true, value });
		expect(isStrictLocalDate(value)).toBe(true);
	});

	it.each([
		'',
		'0000-01-01',
		'2026',
		'2026-07',
		'2026-7-29',
		'2026-07-9',
		'1900-02-29',
		'2026-02-29',
		'2024-02-30',
		'2026-00-01',
		'2026-13-01',
		'2026-07-29T00:00:00Z',
		' 2026-07-29',
		'2026-07-29 '
	])('rejects %j', (value) => {
		expect(parseLocalDate(value).ok).toBe(false);
		expect(isStrictLocalDate(value)).toBe(false);
	});

	it('formats the local calendar fields rather than a UTC-shifted date', () => {
		const localEvening = new Date(2026, 6, 29, 23, 45, 0);

		expect(formatLocalDate(localEvening)).toBe('2026-07-29');
		expect(currentLocalDate(localEvening)).toBe('2026-07-29');
	});

	it('rejects an invalid Date source', () => {
		expect(() => formatLocalDate(new Date(Number.NaN))).toThrow(RangeError);
	});
});
