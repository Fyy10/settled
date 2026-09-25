import { withNetworkState } from '../../../tests/network';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import type { User } from '$lib/api/types';
import { currentUserFixture } from '../../../tests/fixtures/api-contract';

import RegisterForm from './register-form.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	register: vi.fn(),
	setAuthenticated: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$lib/api/auth', () => ({ register: mocks.register }));
vi.mock('$lib/config/public', () => ({ API_BASE_URL: 'http://localhost:8080' }));
vi.mock('$lib/state/auth.svelte', () => ({
	setAuthenticated: mocks.setAuthenticated
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.register.mockReset();
	mocks.setAuthenticated.mockReset();
});

describe('RegisterForm', () => {
	it('retains editable entries and blocks direct submission while offline', async () => {
		await withNetworkState(async (setOnline) => {
			render(RegisterForm);
			const input = screen.getByLabelText('Email');
			await fireEvent.input(input, { target: { value: 'person@example.com' } });
			await setOnline(false);
			const button = screen.getAllByRole('button', { name: 'Create account' }).at(-1)!;
			expect(button).toBeDisabled();
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.submit(input.closest('form')!);
			expect(mocks.register).not.toHaveBeenCalled();
			expect(input).toHaveValue('person@example.com');
			expect(input).toBeEnabled();
			await setOnline(true);
			expect(input).toHaveValue('person@example.com');
			expect(mocks.register).not.toHaveBeenCalled();
		});
	});

	it('renders the actual password rule and focuses the first invalid field', async () => {
		render(RegisterForm);

		expect(screen.getByText('Use 8–128 characters.')).toHaveAttribute(
			'id',
			'register-password-help'
		);
		expect(screen.getByLabelText('Password')).toHaveAttribute(
			'autocomplete',
			'new-password'
		);

		await fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

		const displayName = screen.getByLabelText('Display name');
		await waitFor(() => expect(displayName).toHaveFocus());
		expect(displayName).toHaveAttribute('aria-invalid', 'true');
		expect(mocks.register).not.toHaveBeenCalled();
	});

	it('maps a duplicate email conflict to the email field and focuses it', async () => {
		mocks.register.mockRejectedValue(
			new ApiError({
				status: 409,
				code: 'conflict',
				message: 'Email is already registered.',
				fields: {}
			})
		);
		render(RegisterForm);
		await fillRegistration('Alice', 'alice@example.com', 'password');

		await fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

		const email = screen.getByLabelText('Email');
		expect(await screen.findByText('An account already uses this email.')).toHaveAttribute(
			'id',
			'register-email-error'
		);
		await waitFor(() => expect(email).toHaveFocus());
		expect(email).toHaveAttribute('aria-describedby', 'register-email-error');
	});

	it('maps known API fields and keeps unknown field errors visible', async () => {
		mocks.register.mockRejectedValue(
			new ApiError({
				status: 422,
				code: 'validation_failed',
				message: 'One or more fields are invalid.',
				fields: {
					displayName: 'Choose a shorter display name.',
					password: 'Password is not accepted.',
					futurePolicy: 'Accept the current account policy.'
				}
			})
		);
		render(RegisterForm);
		await fillRegistration('Alice', 'alice@example.com', 'password');

		await fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

		expect(await screen.findByText('Choose a shorter display name.')).toBeInTheDocument();
		expect(screen.getByText('Password is not accepted.')).toBeInTheDocument();
		const alert = screen
			.getByText('Accept the current account policy.')
			.closest('[role="alert"]');
		expect(alert).not.toBeNull();
		expect(alert).toHaveTextContent('Accept the current account policy.');
		await waitFor(() => expect(screen.getByLabelText('Display name')).toHaveFocus());
	});

	it('disables repeated submission and establishes the returned session', async () => {
		const response = deferred<User>();
		mocks.register.mockReturnValue(response.promise);
		render(RegisterForm);
		await fillRegistration(' Alice ', ' alice@example.com ', '密密密');

		const submit = screen.getByRole('button', { name: 'Create account' });
		await fireEvent.click(submit);

		expect(screen.getByRole('button', { name: /creating account/i })).toBeDisabled();
		expect(mocks.register).toHaveBeenCalledWith({
			displayName: 'Alice',
			email: 'alice@example.com',
			password: '密密密'
		});
		await fireEvent.click(submit);
		expect(mocks.register).toHaveBeenCalledOnce();

		response.resolve(currentUserFixture.user);
		await waitFor(() => {
			expect(mocks.setAuthenticated).toHaveBeenCalledWith(currentUserFixture.user);
			expect(mocks.goto).toHaveBeenCalledWith('/groups', { replaceState: true });
		});
	});
});

async function fillRegistration(
	displayName: string,
	email: string,
	password: string
): Promise<void> {
	await fireEvent.input(screen.getByLabelText('Display name'), {
		target: { value: displayName }
	});
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
