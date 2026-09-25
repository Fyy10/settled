import { describe, expect, it } from 'vitest';

import { pageTitle } from './page-title';

describe('pageTitle', () => {
	it('uses the product name for the root document', () => {
		expect(pageTitle()).toBe('Settled');
	});

	it('uses the same title pattern for named routes', () => {
		expect(pageTitle('Log in')).toBe('Log in · Settled');
		expect(pageTitle('Groups')).toBe('Groups · Settled');
		expect(pageTitle('Add expense')).toBe('Add expense · Settled');
	});
});
