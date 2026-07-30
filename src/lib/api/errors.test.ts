import { describe, expect, it } from 'vitest';

import {
	ApiError,
	apiFieldMap,
	classifyApiError,
	invalidResponseError,
	isNotFoundApiError,
	mapApiFieldErrors,
	networkError
} from './errors';

describe('ApiError', () => {
	it.each([
		[401, 'unauthorized', 'unauthorized'],
		[403, 'csrf_required', 'csrf'],
		[403, 'csrf_invalid', 'csrf'],
		[422, 'validation_failed', 'api']
	] as const)('classifies status %i and code %s as %s', (status, code, expected) => {
		const error = new ApiError({
			status,
			code,
			message: 'Request failed.',
			fields: {}
		});

		expect(classifyApiError(error)).toBe(expected);
	});

	it('distinguishes network and unknown failures', () => {
		expect(classifyApiError(networkError(new TypeError('Failed to fetch')))).toBe('network');
		expect(classifyApiError(invalidResponseError(200))).toBe('unknown');
		expect(classifyApiError(new Error('Unexpected'))).toBe('unknown');
	});

	it('recognizes only HTTP 404 errors as hidden-resource failures', () => {
		expect(
			isNotFoundApiError(
				new ApiError({
					status: 404,
					code: 'not_found',
					message: 'Not found.',
					fields: {}
				})
			)
		).toBe(true);
		expect(isNotFoundApiError(invalidResponseError(404))).toBe(true);
		expect(isNotFoundApiError(new Error('Not found.'))).toBe(false);
	});

	it('retains stable details without sharing the mutable field input', () => {
		const fields = { email: 'Email is required.' };
		const error = new ApiError({
			status: 422,
			code: 'validation_failed',
			message: 'One or more fields are invalid.',
			fields,
			requestId: 'request-123'
		});
		fields.email = 'Changed';

		expect(error).toMatchObject({
			name: 'ApiError',
			status: 422,
			code: 'validation_failed',
			message: 'One or more fields are invalid.',
			fields: { email: 'Email is required.' },
			requestId: 'request-123',
			source: 'http'
		});
	});
});

describe('API field mapping', () => {
	it('maps documented API keys and preserves unknown keys for a form alert', () => {
		const mapped = mapApiFieldErrors(
			{
				amountCents: 'Enter an amount.',
				participantUserIds: 'Choose participants.',
				futureField: 'This field changed.'
			},
			apiFieldMap
		);

		expect(mapped.fieldErrors).toEqual({
			amount: 'Enter an amount.',
			splits: 'Choose participants.'
		});
		expect(mapped.unknownFieldErrors).toEqual({
			futureField: 'This field changed.'
		});
	});

	it('keeps the first error when multiple API keys map to one form control', () => {
		const mapped = mapApiFieldErrors(
			{
				splits: 'Split amounts are invalid.',
				percentageSplits: 'Percentages are invalid.'
			},
			apiFieldMap
		);

		expect(mapped.fieldErrors.splits).toBe('Split amounts are invalid.');
	});
});
