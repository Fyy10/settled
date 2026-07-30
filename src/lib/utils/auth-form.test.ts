import { describe, expect, it } from 'vitest';

import { validateLoginForm, validateRegistrationForm } from './auth-form';

describe('login form validation', () => {
	it('requires credentials and rejects an obviously malformed email', () => {
		expect(validateLoginForm({ email: '', password: '' })).toEqual({
			email: 'Enter your email address.',
			password: 'Enter your password.'
		});
		expect(validateLoginForm({ email: 'alice.example.com', password: 'password' })).toEqual({
			email: 'Enter a valid email address.'
		});
		expect(
			validateLoginForm({ email: 'alice:admin@example.com', password: 'password' })
		).toEqual({ email: 'Enter a valid email address.' });
	});

	it('does not trim or impose registration rules on a login password', () => {
		expect(
			validateLoginForm({ email: ' alice@example.com ', password: ' 密 ' })
		).toEqual({});
	});
});

describe('registration form validation', () => {
	it('requires all fields and catches an obvious email shape', () => {
		expect(validateRegistrationForm({ displayName: ' ', email: '', password: '' })).toEqual({
			displayName: 'Enter your display name.',
			email: 'Enter your email address.',
			password: 'Use 8–128 characters.'
		});
		expect(
			validateRegistrationForm({
				displayName: 'Alice',
				email: 'alice.example.com',
				password: 'password'
			})
		).toEqual({ email: 'Enter a valid email address.' });
	});

	it('counts display names by Unicode code point', () => {
		expect(
			validateRegistrationForm({
				displayName: '𠮷'.repeat(120),
				email: 'alice@example.com',
				password: 'password'
			})
		).toEqual({});
		expect(
			validateRegistrationForm({
				displayName: '𠮷'.repeat(121),
				email: 'alice@example.com',
				password: 'password'
			})
		).toEqual({
			displayName: 'Display name must not exceed 120 characters.'
		});
	});

	it('enforces the backend password boundary in UTF-8 bytes', () => {
		expect(
			validateRegistrationForm({
				displayName: 'Alice',
				email: 'alice@example.com',
				password: '密密密'
			})
		).toEqual({});
		expect(
			validateRegistrationForm({
				displayName: 'Alice',
				email: 'alice@example.com',
				password: `${'a'.repeat(126)}密`
			})
		).toEqual({ password: 'Use 8–128 characters.' });
		expect(
			validateRegistrationForm({
				displayName: 'Alice',
				email: 'alice@example.com',
				password: 'a'.repeat(128)
			})
		).toEqual({});
	});
});
