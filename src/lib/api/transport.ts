import { API_BASE_URL } from '$lib/config/public';

import { ApiError, invalidResponseError, networkError } from './errors';
import { isApiErrorBody, isJsonObject } from './validation';

export type ApiMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export type ApiTransportOptions = {
	method?: ApiMethod;
	body?: unknown;
	signal?: AbortSignal;
	headers?: HeadersInit;
	expectedStatus?: number;
};

export async function sendApiRequest(
	path: string,
	options: ApiTransportOptions = {}
): Promise<unknown> {
	const method = options.method ?? 'GET';
	const hasBody = options.body !== undefined;

	if (method === 'GET' && hasBody) {
		throw new TypeError('GET API requests cannot include a body.');
	}

	const headers = new Headers(options.headers);
	headers.set('Accept', 'application/json');

	let body: string | undefined;
	if (hasBody) {
		body = JSON.stringify(options.body);
		if (body === undefined) {
			throw new TypeError('The API request body must be JSON serializable.');
		}
		headers.set('Content-Type', 'application/json');
	} else {
		headers.delete('Content-Type');
	}

	const url = resolveApiPath(path);
	let response: Response;
	try {
		response = await fetch(url, {
			method,
			headers,
			credentials: 'include',
			body,
			signal: options.signal
		});
	} catch (error) {
		if (isAbortError(error)) {
			throw error;
		}

		throw networkError(error);
	}

	const requestId = response.headers.get('X-Request-ID') ?? undefined;

	if (!response.ok) {
		throw await responseApiError(response, requestId);
	}

	if (
		options.expectedStatus !== undefined &&
		response.status !== options.expectedStatus
	) {
		throw invalidResponseError(
			response.status,
			requestId,
			`The server returned HTTP ${response.status}; expected HTTP ${options.expectedStatus}.`
		);
	}

	if (response.status === 204) {
		return undefined;
	}

	const payload = await readJson(response, requestId);
	if (!isJsonObject(payload)) {
		throw invalidResponseError(response.status, requestId);
	}

	return payload;
}

function resolveApiPath(path: string): string {
	if (path !== '/api' && !path.startsWith('/api/')) {
		throw new TypeError('API request paths must begin with /api/.');
	}

	const url = new URL(path, API_BASE_URL);
	const apiOrigin = new URL(API_BASE_URL).origin;

	if (
		url.origin !== apiOrigin ||
		(url.pathname !== '/api' && !url.pathname.startsWith('/api/'))
	) {
		throw new TypeError('API request paths must stay on the configured API origin.');
	}

	return url.href;
}

async function responseApiError(
	response: Response,
	requestId: string | undefined
): Promise<ApiError> {
	if (!hasJsonContentType(response)) {
		return unknownHttpError(response.status, requestId);
	}

	let payload: unknown;
	try {
		payload = await response.json();
	} catch {
		return unknownHttpError(response.status, requestId);
	}

	if (!isApiErrorBody(payload)) {
		return unknownHttpError(response.status, requestId);
	}

	return new ApiError({
		status: response.status,
		code: payload.error.code,
		message: payload.error.message,
		fields: payload.error.fields ?? {},
		requestId
	});
}

async function readJson(response: Response, requestId: string | undefined): Promise<unknown> {
	if (!hasJsonContentType(response)) {
		throw invalidResponseError(response.status, requestId);
	}

	try {
		return await response.json();
	} catch {
		throw invalidResponseError(response.status, requestId);
	}
}

function hasJsonContentType(response: Response): boolean {
	const contentType = response.headers.get('Content-Type');

	return contentType !== null && /^application\/(?:[\w.-]+\+)?json(?:\s*;|$)/i.test(contentType);
}

function unknownHttpError(status: number, requestId: string | undefined): ApiError {
	return new ApiError({
		status,
		code: 'unknown_error',
		message: 'The server returned an unexpected error response.',
		fields: {},
		requestId
	});
}

function isAbortError(error: unknown): boolean {
	return (
		typeof error === 'object' &&
		error !== null &&
		'name' in error &&
		error.name === 'AbortError'
	);
}
