import { createRawSnippet } from 'svelte';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { currentUserFixture } from '../../tests/fixtures/api-contract';

import AuthLayout from './+layout.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	ensureSession: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/state/auth.svelte', () => ({
	ensureSession: mocks.ensureSession
}));

const children = createRawSnippet(() => ({
	render: () => '<h1>Authentication form</h1>'
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.ensureSession.mockReset();
});

describe('authentication route guard', () => {
	it('does not flash the form while session state is unknown', () => {
		mocks.ensureSession.mockReturnValue(new Promise(() => {}));
		render(AuthLayout, { children });

		expect(
			screen.getByRole('heading', { level: 1, name: 'Checking your session' })
		).toBeInTheDocument();
		expect(screen.queryByText('Authentication form')).not.toBeInTheDocument();
	});

	it('renders public auth content only for an anonymous session', async () => {
		mocks.ensureSession.mockResolvedValue({ status: 'anonymous', user: null });
		render(AuthLayout, { children });

		expect(await screen.findByText('Authentication form')).toBeInTheDocument();
		expect(mocks.goto).not.toHaveBeenCalled();
	});

	it('replaces an auth route with groups for an authenticated session', async () => {
		mocks.ensureSession.mockResolvedValue({
			status: 'authenticated',
			user: currentUserFixture.user
		});
		render(AuthLayout, { children });

		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith('/groups', { replaceState: true })
		);
		expect(screen.queryByText('Authentication form')).not.toBeInTheDocument();
	});

	it('keeps network failure distinct from anonymous and permits retry', async () => {
		mocks.ensureSession
			.mockRejectedValueOnce(new TypeError('Failed to fetch'))
			.mockResolvedValueOnce({ status: 'anonymous', user: null });
		render(AuthLayout, { children });

		expect(
			await screen.findByRole('heading', {
				level: 1,
				name: 'Settled can’t reach the server.'
			})
		).toBeInTheDocument();
		expect(mocks.goto).not.toHaveBeenCalled();

		await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
		expect(await screen.findByText('Authentication form')).toBeInTheDocument();
	});
});
