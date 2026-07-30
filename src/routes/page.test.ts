import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { currentUserFixture } from '../tests/fixtures/api-contract';

import RootPage from './+page.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	ensureSession: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/state/auth.svelte', () => ({
	ensureSession: mocks.ensureSession
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.ensureSession.mockReset();
});

describe('root session bootstrap', () => {
	it('provides one stable main landmark while the session is unknown', () => {
		mocks.ensureSession.mockReturnValue(new Promise(() => {}));
		render(RootPage);

		expect(screen.getAllByRole('main')).toHaveLength(1);
		expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
		expect(
			screen.getByRole('heading', { level: 1, name: 'Opening your ledger' })
		).toBeInTheDocument();
		expect(screen.getByRole('status', { name: 'Checking your session' })).toBeInTheDocument();
	});

	it('replaces history with groups for an authenticated session', async () => {
		mocks.ensureSession.mockResolvedValue({
			status: 'authenticated',
			user: currentUserFixture.user
		});
		render(RootPage);

		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith('/groups', { replaceState: true })
		);
	});

	it('replaces history with login for an anonymous session', async () => {
		mocks.ensureSession.mockResolvedValue({ status: 'anonymous', user: null });
		render(RootPage);

		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith('/login', { replaceState: true })
		);
	});

	it('does not redirect a failed session check and permits retry', async () => {
		mocks.ensureSession
			.mockRejectedValueOnce(new TypeError('Failed to fetch'))
			.mockResolvedValueOnce({ status: 'anonymous', user: null });
		render(RootPage);

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'Settled can’t reach the server.'
			})
		).toBeInTheDocument();
		expect(screen.getByText('Check your connection and try again.')).toBeInTheDocument();
		expect(mocks.goto).not.toHaveBeenCalled();

		await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith('/login', { replaceState: true })
		);
	});
});
