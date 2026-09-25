import { describe, expect, it } from 'vitest';

import { shouldFocusRouteHeading } from './route-focus';

const origin = 'https://settled.example';

describe('shouldFocusRouteHeading', () => {
	it('focuses the page heading when the route path changes', () => {
		expect(
			shouldFocusRouteHeading(
				new URL('/groups', origin),
				new URL('/groups/group-id/expenses/new', origin)
			)
		).toBe(true);
	});

	it('does not steal focus during initial rendering', () => {
		expect(shouldFocusRouteHeading(undefined, new URL('/groups', origin))).toBe(false);
		expect(shouldFocusRouteHeading(null, new URL('/groups', origin))).toBe(false);
	});

	it('preserves focus when only the group view query changes', () => {
		expect(
			shouldFocusRouteHeading(
				new URL('/groups/group-id?view=balances', origin),
				new URL('/groups/group-id?view=members', origin)
			)
		).toBe(false);
	});
});
