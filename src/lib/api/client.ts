import { clearCsrfToken, getCsrfToken } from '$lib/state/csrf';
import { createBasicAuthorization } from '$lib/utils/basic-auth';

import {
	ApiError,
	isCsrfApiError,
	isUnauthorizedApiError
} from './errors';
import { sendApiRequest, type ApiMethod } from './transport';
import type { LoginCredentials } from './types';

export {
	ApiError,
	apiFieldMap,
	classifyApiError,
	isConflictApiError,
	isCsrfApiError,
	isForbiddenApiError,
	isNotFoundApiError,
	isNetworkApiError,
	isUnauthorizedApiError,
	mapApiFieldErrors
} from './errors';
export type {
	ApiErrorCategory,
	ApiErrorDetails,
	MappedApiFieldErrors
} from './errors';

export type RequestOptions = {
	method?: ApiMethod;
	body?: unknown;
	signal?: AbortSignal;
	csrf?: boolean;
	retryCsrf?: boolean;
	expectedStatus?: number;
};

type UnauthorizedListener = (error: ApiError) => void;

const unsafeMethods = new Set<ApiMethod>(['POST', 'PUT', 'PATCH', 'DELETE']);
const unauthorizedListeners = new Set<UnauthorizedListener>();

export function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
	return requestInternal<T>(path, options);
}

export function requestWithBasicAuth<T>(
	path: string,
	credentials: Readonly<LoginCredentials>,
	options: Pick<RequestOptions, 'signal' | 'retryCsrf'> = {}
): Promise<T> {
	const authorization = createBasicAuthorization(credentials.email, credentials.password);

	return requestInternal<T>(
		path,
		{
			method: 'POST',
			signal: options.signal,
			retryCsrf: options.retryCsrf
		},
		authorization
	);
}

export function onUnauthorized(listener: UnauthorizedListener): () => void {
	unauthorizedListeners.add(listener);

	return () => {
		unauthorizedListeners.delete(listener);
	};
}

async function requestInternal<T>(
	path: string,
	options: RequestOptions,
	authorization?: string
): Promise<T> {
	const method = options.method ?? 'GET';
	const unsafe = unsafeMethods.has(method);
	validateCsrfOption(unsafe, options.csrf);
	throwIfAborted(options.signal);

	try {
		return await sendAttempt<T>(path, options, method, unsafe, authorization);
	} catch (error) {
		if (unsafe && options.retryCsrf !== false && isCsrfApiError(error)) {
			throwIfAborted(options.signal);
			await waitForSharedValue(
				getCsrfToken({ force: true }),
				options.signal
			);

			try {
				return await sendAttempt<T>(path, options, method, unsafe, authorization);
			} catch (retryError) {
				handleUnauthorized(retryError);
				throw retryError;
			}
		}

		handleUnauthorized(error);
		throw error;
	}
}

async function sendAttempt<T>(
	path: string,
	options: RequestOptions,
	method: ApiMethod,
	unsafe: boolean,
	authorization?: string
): Promise<T> {
	const headers = new Headers();
	if (authorization !== undefined) {
		headers.set('Authorization', authorization);
	}
	if (unsafe) {
		throwIfAborted(options.signal);
		headers.set(
			'X-CSRF-Token',
			await waitForSharedValue(getCsrfToken(), options.signal)
		);
	}

	return (await sendApiRequest(path, {
		method,
		body: options.body,
		signal: options.signal,
		headers,
		expectedStatus: options.expectedStatus
	})) as T;
}

function throwIfAborted(signal: AbortSignal | undefined): void {
	if (signal?.aborted) {
		throw abortReason(signal);
	}
}

function waitForSharedValue<T>(
	promise: Promise<T>,
	signal: AbortSignal | undefined
): Promise<T> {
	if (signal === undefined) {
		return promise;
	}
	if (signal.aborted) {
		return Promise.reject(abortReason(signal));
	}

	return new Promise<T>((resolve, reject) => {
		const abort = () => {
			reject(abortReason(signal));
		};
		signal.addEventListener('abort', abort, { once: true });

		promise.then(
			(value) => {
				signal.removeEventListener('abort', abort);
				if (signal.aborted) {
					reject(abortReason(signal));
					return;
				}
				resolve(value);
			},
			(error: unknown) => {
				signal.removeEventListener('abort', abort);
				reject(error);
			}
		);
	});
}

function abortReason(signal: AbortSignal): unknown {
	return (
		signal.reason ??
		new DOMException('The operation was aborted.', 'AbortError')
	);
}

function validateCsrfOption(unsafe: boolean, csrf: boolean | undefined): void {
	if (csrf === undefined) {
		return;
	}

	if (csrf !== unsafe) {
		throw new TypeError(
			unsafe
				? 'Unsafe API requests cannot bypass the shared CSRF path.'
				: 'Safe API requests cannot include a CSRF token.'
		);
	}
}

function handleUnauthorized(error: unknown): void {
	if (!isUnauthorizedApiError(error)) {
		return;
	}

	clearCsrfToken();
	for (const listener of unauthorizedListeners) {
		try {
			listener(error);
		} catch {
			// A lifecycle listener must not replace the authoritative API error.
		}
	}
}
