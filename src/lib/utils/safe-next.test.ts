import { describe, expect, it } from 'vitest';

import {
	DEFAULT_AUTHENTICATED_PATH,
	safeNextPath,
	sessionExpiredLoginPath
} from './safe-next';

describe('safeNextPath', () => {
	it.each([
		'/',
		'/groups',
		'/groups/9a41c3a3-169f-4219-998b-822d31352f90?tab=activity',
		'/groups#balances'
	])('accepts the local path %s', (value) => {
		expect(safeNextPath(value)).toBe(value);
	});

	it.each([
		undefined,
		null,
		'',
		'groups',
		'https://evil.example/groups',
		'//evil.example/groups',
		'///evil.example/groups',
		'/\\evil.example/groups',
		'/groups\u0000/hidden'
	])('falls back for an unsafe next value', (value) => {
		expect(safeNextPath(value)).toBe(DEFAULT_AUTHENTICATED_PATH);
	});
});

describe('sessionExpiredLoginPath', () => {
	it('encodes only a validated local continuation', () => {
		expect(sessionExpiredLoginPath('/groups/one?tab=activity')).toBe(
			'/login?reason=session-expired&next=%2Fgroups%2Fone%3Ftab%3Dactivity'
		);
		expect(sessionExpiredLoginPath('//evil.example')).toBe(
			'/login?reason=session-expired&next=%2Fgroups'
		);
	});
});
