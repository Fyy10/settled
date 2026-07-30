import { clearCsrfToken, setCsrfToken } from '$lib/state/csrf';

import { request, requestWithBasicAuth } from './client';
import type {
	AuthResponse,
	CurrentUserResponse,
	LoginCredentials,
	RegisterInput,
	User
} from './types';
import {
	isAuthResponse,
	isCurrentUserResponse,
	requireApiPayload
} from './validation';

type AuthRequestOptions = {
	signal?: AbortSignal;
};

export async function register(
	input: Readonly<RegisterInput>,
	options: AuthRequestOptions = {}
): Promise<User> {
	const payload = await request<unknown>('/api/auth/register', {
		method: 'POST',
		body: input,
		signal: options.signal
	});
	const response = requireApiPayload<AuthResponse>(
		payload,
		isAuthResponse,
		'registration response'
	);

	setCsrfToken(response.csrfToken);
	return response.user;
}

export async function login(
	credentials: Readonly<LoginCredentials>,
	options: AuthRequestOptions = {}
): Promise<User> {
	const payload = await requestWithBasicAuth<unknown>('/api/auth/login', credentials, {
		signal: options.signal
	});
	const response = requireApiPayload<AuthResponse>(payload, isAuthResponse, 'login response');

	setCsrfToken(response.csrfToken);
	return response.user;
}

export async function logout(options: AuthRequestOptions = {}): Promise<void> {
	await request<void>('/api/auth/logout', {
		method: 'POST',
		signal: options.signal
	});
	clearCsrfToken();
}

export async function getCurrentUser(options: AuthRequestOptions = {}): Promise<User> {
	const payload = await request<unknown>('/api/me', {
		signal: options.signal
	});
	const response = requireApiPayload<CurrentUserResponse>(
		payload,
		isCurrentUserResponse,
		'current-user response'
	);

	return response.user;
}
