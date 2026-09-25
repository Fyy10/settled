import { describe, expect, it } from 'vitest';

import { MutationCoordinator } from './mutation-coordinator.svelte';

describe('MutationCoordinator', () => {
	it('allows only one operation until a retryable failure resets it', () => {
		const mutation = new MutationCoordinator<'save' | 'delete'>();

		expect(mutation.begin('save')).toBe(true);
		expect(mutation.begin('delete')).toBe(false);
		expect(mutation.operation).toBe('save');
		mutation.reset();
		expect(mutation.begin('delete')).toBe(true);
	});

	it.each(['committed', 'ambiguous', 'blocked'] as const)(
		'keeps controls locked after a %s outcome',
		(outcome) => {
			const mutation = new MutationCoordinator<'save' | 'delete'>();
			mutation.begin('save');
			if (outcome === 'committed') {
				mutation.markCommitted();
			} else if (outcome === 'ambiguous') {
				mutation.markAmbiguous();
			} else {
				mutation.markBlocked();
			}

			mutation.reset();
			expect(mutation.phase).toBe(outcome);
			expect(mutation.begin('delete')).toBe(false);
		}
	);
});
