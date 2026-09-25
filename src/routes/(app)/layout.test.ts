import { createRawSnippet } from 'svelte';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { currentUserFixture } from '../../tests/fixtures/api-contract';

import AppLayout from './+layout.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	ensureSession: vi.fn(),
	clearSession: vi.fn(),
	logout: vi.fn(),
	authState: {
		current: { status: 'unknown', user: null } as
			| { status: 'unknown'; user: null }
			| { status: 'anonymous'; user: null }
			| {
					status: 'authenticated';
					user: {
						id: string;
						email: string;
						displayName: string;
						createdAt: string;
						updatedAt: string;
					};
			  }
	}
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/api/auth', () => ({ logout: mocks.logout }));
vi.mock('$lib/config/public', () => ({ API_BASE_URL: 'http://localhost:8080' }));
vi.mock('$lib/state/auth.svelte', () => ({
	authState: mocks.authState,
	ensureSession: mocks.ensureSession,
	clearSession: mocks.clearSession
}));

const children = createRawSnippet(() => ({
	render: () => '<h1>Protected groups</h1>'
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.ensureSession.mockReset();
	mocks.clearSession.mockReset();
	mocks.logout.mockReset();
	mocks.authState.current = { status: 'unknown', user: null };
});

describe('authenticated application guard', () => {
	it('renders a stable shell skeleton while the session is unknown', () => {
		mocks.ensureSession.mockReturnValue(new Promise(() => {}));
		render(AppLayout, { children });

		expect(
			screen.getByRole('heading', { level: 1, name: 'Checking your session' })
		).toBeInTheDocument();
		expect(screen.queryByText('Protected groups')).not.toBeInTheDocument();
		expect(screen.getAllByRole('main')).toHaveLength(1);
	});

	it('renders protected content after an authenticated check', async () => {
		mocks.ensureSession.mockResolvedValue({
			status: 'authenticated',
			user: currentUserFixture.user
		});
		render(AppLayout, { children });

		expect(await screen.findByText('Protected groups')).toBeInTheDocument();
		expect(
			screen.getByRole('button', { name: 'Open account menu for Alice' })
		).toBeInTheDocument();
	});

	it('preserves the ready shell while reusing a cached authenticated session', () => {
		mocks.authState.current = {
			status: 'authenticated',
			user: currentUserFixture.user
		};
		mocks.ensureSession.mockReturnValue(new Promise(() => {}));
		render(AppLayout, { children });

		expect(screen.getByText('Protected groups')).toBeInTheDocument();
		expect(
			screen.queryByRole('heading', { level: 1, name: 'Checking your session' })
		).not.toBeInTheDocument();
	});

	it('replaces an app route with login when the session is anonymous', async () => {
		mocks.ensureSession.mockResolvedValue({ status: 'anonymous', user: null });
		render(AppLayout, { children });

		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith('/login', { replaceState: true })
		);
		expect(screen.queryByText('Protected groups')).not.toBeInTheDocument();
	});

	it('does not redirect a network failure and can recover on retry', async () => {
		mocks.ensureSession
			.mockRejectedValueOnce(new TypeError('Failed to fetch'))
			.mockResolvedValueOnce({
				status: 'authenticated',
				user: currentUserFixture.user
			});
		render(AppLayout, { children });

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'Settled can’t reach the server.'
			})
		).toBeInTheDocument();
		expect(mocks.goto).not.toHaveBeenCalled();

		await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
		expect(await screen.findByText('Protected groups')).toBeInTheDocument();
	});
});
