import { describe, expect, it } from 'vitest';

import {
	formatGroupUpdatedAt,
	formatMemberCount,
	normalizeJoinCode,
	validateCreateGroupName,
	validateJoinCode
} from './group';

describe('group form helpers', () => {
	it('validates a trimmed group name by Unicode code point', () => {
		expect(validateCreateGroupName('   ')).toEqual({
			name: 'Enter a group name.'
		});
		expect(validateCreateGroupName(` ${'𠮷'.repeat(160)} `)).toEqual({});
		expect(validateCreateGroupName('𠮷'.repeat(161))).toEqual({
			name: 'Group name must not exceed 160 characters.'
		});
	});

	it('normalizes pasted join codes and requires a visible value', () => {
		expect(normalizeJoinCode('  abcd-1234  ')).toBe('ABCD-1234');
		expect(validateJoinCode('\t\n')).toEqual({
			joinCode: 'Enter a group code.'
		});
		expect(validateJoinCode(' ABCD1234 ')).toEqual({});
	});
});

describe('group display helpers', () => {
	it('pluralizes API-provided member counts', () => {
		expect(formatMemberCount(0)).toBe('0 members');
		expect(formatMemberCount(1)).toBe('1 member');
		expect(formatMemberCount(3)).toBe('3 members');
	});

	it('formats valid update timestamps and safely retains an invalid value', () => {
		expect(formatGroupUpdatedAt('2026-06-30T18:00:00Z')).toMatch(
			/Jun 30, 2026/
		);
		expect(formatGroupUpdatedAt('not-a-timestamp')).toBe('not-a-timestamp');
	});
});
