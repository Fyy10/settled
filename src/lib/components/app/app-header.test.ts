import { withNetworkState } from '../../../tests/network';
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor
} from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { networkError } from '$lib/api/errors';
import { registerDirtyForm } from '$lib/state/dirty-forms.svelte';
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
	vi.stubGlobal('confirm', vi.fn());
});

afterEach(async () => {
	cleanup();
	await waitFor(() => {
		expect(document.body.style.overflow).not.toBe('hidden');
		expect(document.body.style.pointerEvents).not.toBe('none');
	});
	vi.unstubAllGlobals();
});

describe('AppHeader', () => {
	it('blocks offline mutations and permits retry after reconnecting', async () => {
		await withNetworkState(async (setOnline) => {
			render(AppHeader, { user: currentUserFixture.user });
			await openAccountMenu();
			await setOnline(false);
			const button = screen.getAllByRole('menuitem', { name: 'Log out' }).at(-1)!;
			expect(button).toHaveAttribute('data-disabled');
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.click(button);
			expect(mocks.logout).not.toHaveBeenCalled();
			await setOnline(true);
			expect(button).not.toHaveAttribute('data-disabled');
			expect(mocks.logout).not.toHaveBeenCalled();
		});
	});

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

	it('confirms a dirty draft before logout and does not mutate on decline', async () => {
		const registration = registerDirtyForm(true);
		const confirm = vi.mocked(globalThis.confirm);
		confirm.mockReturnValue(false);
		render(AppHeader, { user: currentUserFixture.user });

		await openAccountMenu();
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Log out' }));

		expect(confirm).toHaveBeenCalledWith(
			'Discard your unsaved changes and log out?'
		);
		expect(mocks.logout).not.toHaveBeenCalled();

		confirm.mockReturnValue(true);
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Log out' }));
		await waitFor(() => expect(mocks.logout).toHaveBeenCalledOnce());
		registration.unregister();
	});

	it('does not start logout while a form mutation is pending', async () => {
		const registration = registerDirtyForm(true, true);
		render(AppHeader, { user: currentUserFixture.user });

		await openAccountMenu();
		expect(screen.getByRole('menuitem', { name: 'Log out' })).toHaveAttribute(
			'data-disabled'
		);
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Log out' }));
		expect(mocks.logout).not.toHaveBeenCalled();
		registration.unregister();
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
