import { render, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import SessionExpirationRedirect from './session-expiration-redirect.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	stopListening: vi.fn(),
	listener: undefined as (() => void) | undefined,
	page: {
		url: new URL(
			'https://settled.example/groups/lake?view=activity#latest'
		)
	}
}));

vi.mock('$app/environment', () => ({ browser: true }));
vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$app/state', () => ({ page: mocks.page }));
vi.mock('$lib/state/auth.svelte', () => ({
	onSessionExpired: (listener: () => void) => {
		mocks.listener = listener;
		return mocks.stopListening;
	}
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.stopListening.mockReset();
	mocks.listener = undefined;
	mocks.page.url = new URL(
		'https://settled.example/groups/lake?view=activity#latest'
	);
});

describe('SessionExpirationRedirect', () => {
	it('replaces history with login and a safe local continuation', async () => {
		const result = render(SessionExpirationRedirect);
		expect(mocks.listener).toBeTypeOf('function');

		mocks.listener?.();

		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith(
				'/login?reason=session-expired&next=%2Fgroups%2Flake%3Fview%3Dactivity%23latest',
				{ replaceState: true }
			)
		);

		result.unmount();
		expect(mocks.stopListening).toHaveBeenCalledOnce();
	});
});
