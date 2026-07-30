import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { networkError } from '$lib/api/errors';
import { currentUserFixture } from '../../../tests/fixtures/api-contract';

import AppHeader from './app-header.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	logout: vi.fn(),
	clearSession: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/api/auth', () => ({ logout: mocks.logout }));
vi.mock('$lib/state/auth.svelte', () => ({
	clearSession: mocks.clearSession
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.logout.mockReset();
	mocks.clearSession.mockReset();
});

describe('AppHeader', () => {
	it('shows the signed-in account and logs out before replacing history', async () => {
		mocks.logout.mockResolvedValue(undefined);
		render(AppHeader, { user: currentUserFixture.user });

		await openAccountMenu();
		expect(screen.getByText(currentUserFixture.user.email)).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Log out' }));

		await waitFor(() => {
			expect(mocks.logout).toHaveBeenCalledOnce();
			expect(mocks.clearSession).toHaveBeenCalledOnce();
			expect(mocks.goto).toHaveBeenCalledWith('/login', { replaceState: true });
		});
	});

	it('disables repeated logout while the request is pending', async () => {
		const response = deferred<void>();
		mocks.logout.mockReturnValue(response.promise);
		render(AppHeader, { user: currentUserFixture.user });

		await openAccountMenu();
		const logoutItem = screen.getByRole('menuitem', { name: 'Log out' });
		await fireEvent.click(logoutItem);

		const pendingItem = await screen.findByRole('menuitem', { name: /signing out/i });
		expect(pendingItem).toHaveAttribute('data-disabled');
		await fireEvent.click(pendingItem);
		expect(mocks.logout).toHaveBeenCalledOnce();

		response.resolve();
		await waitFor(() => expect(mocks.goto).toHaveBeenCalled());
	});

	it('retains the authenticated header and focuses an error after a network failure', async () => {
		mocks.logout.mockRejectedValue(networkError(new TypeError('Failed to fetch')));
		render(AppHeader, { user: currentUserFixture.user });

		await openAccountMenu();
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Log out' }));

		const alert = await screen.findByRole('alert');
		expect(alert).toHaveTextContent(
			'Settled could not sign you out. Your current page is still open. Try again.'
		);
		await waitFor(() => expect(alert).toHaveFocus());
		expect(
			screen.getByRole('button', { name: `Open account menu for Alice` })
		).toBeInTheDocument();
		expect(mocks.clearSession).not.toHaveBeenCalled();
		expect(mocks.goto).not.toHaveBeenCalled();
	});
});

async function openAccountMenu(): Promise<void> {
	await fireEvent.click(
		screen.getByRole('button', { name: `Open account menu for Alice` })
	);
	await screen.findByRole('menu');
}

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
} {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((resolvePromise) => {
		resolve = resolvePromise;
	});

	return { promise, resolve };
}
