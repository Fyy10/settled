import { describe, expect, it } from 'vitest';

import { rootRedirectTarget } from './root-redirect';

describe('rootRedirectTarget', () => {
	it('sends an authenticated user to groups', () => {
		expect(rootRedirectTarget('authenticated')).toBe('/groups');
	});

	it('sends an anonymous user to login', () => {
		expect(rootRedirectTarget('anonymous')).toBe('/login');
	});
});
