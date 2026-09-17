import { withNetworkState } from '../../../tests/network';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import type { User } from '$lib/api/types';
import { currentUserFixture } from '../../../tests/fixtures/api-contract';

import LoginForm from './login-form.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	login: vi.fn(),
	setAuthenticated: vi.fn(),
	page: { url: undefined as unknown as URL }
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$app/state', () => ({ page: mocks.page }));
vi.mock('$lib/api/auth', () => ({ login: mocks.login }));
vi.mock('$lib/config/public', () => ({ API_BASE_URL: 'http://localhost:8080' }));
vi.mock('$lib/state/auth.svelte', () => ({
	setAuthenticated: mocks.setAuthenticated
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.login.mockReset();
	mocks.setAuthenticated.mockReset();
	mocks.page.url = new URL('https://settled.example/login');
});

describe('LoginForm', () => {
	it('retains editable entries and blocks direct submission while offline', async () => {
		await withNetworkState(async (setOnline) => {
			render(LoginForm);
			const input = screen.getByLabelText('Email');
			await fireEvent.input(input, { target: { value: 'person@example.com' } });
			await setOnline(false);
			const button = screen.getAllByRole('button', { name: 'Log in' }).at(-1)!;
			expect(button).toBeDisabled();
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.submit(input.closest('form')!);
			expect(mocks.login).not.toHaveBeenCalled();
			expect(input).toHaveValue('person@example.com');
			expect(input).toBeEnabled();
			await setOnline(true);
			expect(input).toHaveValue('person@example.com');
			expect(mocks.login).not.toHaveBeenCalled();
		});
	});

	it('uses authentication affordances and toggles password visibility', async () => {
		render(LoginForm);

		const email = screen.getByLabelText('Email');
		const password = screen.getByLabelText('Password');
		expect(email).toHaveAttribute('type', 'email');
		expect(email).toHaveAttribute('autocomplete', 'username');
		expect(email).toHaveAttribute('autocapitalize', 'none');
		expect(email).toHaveAttribute('spellcheck', 'false');
		expect(password).toHaveAttribute('type', 'password');
		expect(password).toHaveAttribute('autocomplete', 'current-password');

		const visibility = screen.getByRole('button', { name: 'Show password' });
		await fireEvent.click(visibility);
		expect(password).toHaveAttribute('type', 'text');
		expect(screen.getByRole('button', { name: 'Hide password' })).toHaveAttribute(
			'aria-pressed',
			'true'
		);
	});

	it('focuses the first invalid field before making a request', async () => {
		render(LoginForm);

		await fireEvent.click(screen.getByRole('button', { name: 'Log in' }));

		const email = screen.getByLabelText('Email');
		await waitFor(() => expect(email).toHaveFocus());
		expect(email).toHaveAttribute('aria-invalid', 'true');
		expect(screen.getByText('Enter your email address.')).toHaveAttribute(
			'id',
			'login-email-error'
		);
		expect(mocks.login).not.toHaveBeenCalled();
	});

	it('shows one focusable alert for invalid credentials', async () => {
		mocks.login.mockRejectedValue(
			new ApiError({
				status: 401,
				code: 'unauthorized',
				message: 'Authentication failed.',
				fields: {}
			})
		);
		render(LoginForm);
		await fillLogin('alice@example.com', 'not-the-password');

		await fireEvent.click(screen.getByRole('button', { name: 'Log in' }));

		const alert = await screen.findByRole('alert');
		expect(alert).toHaveTextContent('Email or password is incorrect.');
		await waitFor(() => expect(alert).toHaveFocus());
		expect(screen.queryByText(/email does not exist/i)).not.toBeInTheDocument();
	});

	it('disables repeated submission and preserves a non-ASCII password', async () => {
		const response = deferred<User>();
		mocks.login.mockReturnValue(response.promise);
		mocks.page.url = new URL(
			'https://settled.example/login?next=%2Fgroups%2Flake%3Fview%3Dactivity'
		);
		render(LoginForm);
		await fillLogin(' alice@example.com ', 'päss密碼');

		const submit = screen.getByRole('button', { name: 'Log in' });
		await fireEvent.click(submit);

		expect(screen.getByRole('button', { name: /logging in/i })).toBeDisabled();
		expect(mocks.login).toHaveBeenCalledWith({
			email: 'alice@example.com',
			password: 'päss密碼'
		});
		await fireEvent.click(submit);
		expect(mocks.login).toHaveBeenCalledOnce();

		response.resolve(currentUserFixture.user);
		await waitFor(() => {
			expect(mocks.setAuthenticated).toHaveBeenCalledWith(currentUserFixture.user);
			expect(mocks.goto).toHaveBeenCalledWith('/groups/lake?view=activity', {
				replaceState: true
			});
		});
	});

	it('announces an expired session and rejects an unsafe continuation', async () => {
		mocks.page.url = new URL(
			'https://settled.example/login?reason=session-expired&next=https%3A%2F%2Fevil.example'
		);
		mocks.login.mockResolvedValue(currentUserFixture.user);
		render(LoginForm);

		expect(screen.getByRole('alert')).toHaveTextContent(
			'Your session expired. Log in again to continue.'
		);

		await fillLogin('alice@example.com', 'correct horse');
		await fireEvent.click(screen.getByRole('button', { name: 'Log in' }));
		await waitFor(() =>
			expect(mocks.goto).toHaveBeenCalledWith('/groups', { replaceState: true })
		);
	});
});

async function fillLogin(email: string, password: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText('Email'), {
		target: { value: email }
	});
	await fireEvent.input(screen.getByLabelText('Password'), {
		target: { value: password }
	});
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
