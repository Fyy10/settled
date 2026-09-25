import { describe, expect, it } from 'vitest';

import {
	formatBasisPointsAsPercentageInput,
	formatCentsAsUsdInput,
	formatUsd,
	parsePercentageToBasisPoints,
	parseUsdToCents
} from './money';

describe('parseUsdToCents', () => {
	it.each([
		['54', 5_400],
		['54.5', 5_450],
		['54.00', 5_400],
		['0.01', 1],
		['000.29', 29],
		[' \t90.07\n', 9_007],
		['90071992547409.91', Number.MAX_SAFE_INTEGER]
	])('parses %j without floating-point multiplication', (value, expected) => {
		expect(parseUsdToCents(value)).toEqual({ ok: true, value: expected });
	});

	it.each([
		'',
		'   ',
		'.50',
		'1.',
		'+1',
		'-1',
		'1,000',
		'1e2',
		'$1',
		'1.001',
		'one'
	])('rejects malformed input %j', (value) => {
		expect(parseUsdToCents(value).ok).toBe(false);
	});

	it.each(['0', '0.0', '0.00'])('rejects zero input %j', (value) => {
		expect(parseUsdToCents(value)).toMatchObject({
			ok: false,
			code: 'not-positive'
		});
	});

	it('rejects values one cent beyond the safe-integer boundary', () => {
		expect(parseUsdToCents('90071992547409.92')).toMatchObject({
			ok: false,
			code: 'out-of-range'
		});
		expect(parseUsdToCents('999999999999999999999999999999.99')).toMatchObject({
			ok: false,
			code: 'out-of-range'
		});
	});
});

describe('USD formatting', () => {
	it.each([
		[0, '$0.00'],
		[1, '$0.01'],
		[5_400, '$54.00'],
		[123_456_789, '$1,234,567.89'],
		[Number.MAX_SAFE_INTEGER, '$90,071,992,547,409.91']
	])('formats %d cents exactly', (cents, expected) => {
		expect(formatUsd(cents)).toBe(expected);
	});

	it('formats exact editable decimal strings', () => {
		expect(formatCentsAsUsdInput(1)).toBe('0.01');
		expect(formatCentsAsUsdInput(5_450)).toBe('54.50');
		expect(formatCentsAsUsdInput(Number.MAX_SAFE_INTEGER)).toBe(
			'90071992547409.91'
		);
	});

	it.each([-1, 1.5, Number.NaN, Number.POSITIVE_INFINITY, Number.MAX_SAFE_INTEGER + 1])(
		'rejects unsafe cents %s',
		(cents) => {
			expect(() => formatUsd(cents)).toThrow(RangeError);
			expect(() => formatCentsAsUsdInput(cents)).toThrow(RangeError);
		}
	);
});

describe('percentage basis points', () => {
	it.each([
		['0.01', 1],
		['1', 100],
		['33.34', 3_334],
		['50.5', 5_050],
		['100.00', 10_000],
		[' 25 ', 2_500]
	])('converts %j percent to basis points', (value, expected) => {
		expect(parsePercentageToBasisPoints(value)).toEqual({
			ok: true,
			value: expected
		});
	});

	it.each(['', '0', '0.00', '.5', '1.', '-1', '1.001', '100.01'])(
		'rejects invalid percentage %j',
		(value) => {
			expect(parsePercentageToBasisPoints(value).ok).toBe(false);
		}
	);

	it.each([
		[1, '0.01'],
		[100, '1'],
		[3_334, '33.34'],
		[5_050, '50.5'],
		[10_000, '100']
	])('formats %d basis points as %s', (basisPoints, expected) => {
		expect(formatBasisPointsAsPercentageInput(basisPoints)).toBe(expected);
	});
});
