import { describe, expect, it } from 'vitest';

import { LOCAL_API_BASE_URL, resolveApiBaseUrl } from './api-base-url';

const development = { production: false };
const production = { production: true };

describe('resolveApiBaseUrl', () => {
	it('uses the local API origin when development has no configured value', () => {
		expect(resolveApiBaseUrl(undefined, development)).toBe(LOCAL_API_BASE_URL);
		expect(resolveApiBaseUrl('', development)).toBe(LOCAL_API_BASE_URL);
	});

	it('accepts HTTP for local development and HTTPS for any environment', () => {
		expect(resolveApiBaseUrl('http://localhost:9090', development)).toBe(
			'http://localhost:9090'
		);
		expect(resolveApiBaseUrl('https://api.settled.example', development)).toBe(
			'https://api.settled.example'
		);
		expect(resolveApiBaseUrl('https://api.settled.example', production)).toBe(
			'https://api.settled.example'
		);
	});

	it('requires an explicit HTTPS origin for production', () => {
		expect(() => resolveApiBaseUrl(undefined, production)).toThrow(
			'PUBLIC_API_BASE_URL is required for production builds.'
		);
		expect(() => resolveApiBaseUrl('http://api.settled.example', production)).toThrow(
			'PUBLIC_API_BASE_URL must use HTTPS for production builds.'
		);
	});

	it.each([
		' https://api.settled.example',
		'https://api.settled.example ',
		'https://api.settled.example/'
	])('rejects whitespace or a trailing slash in %s', (value) => {
		expect(() => resolveApiBaseUrl(value, development)).toThrow();
	});

	it.each([
		'https://api.settled.example/api',
		'https://api.settled.example?region=west',
		'https://api.settled.example#api',
		'https://user:secret@api.settled.example'
	])('rejects an origin with additional URL data in %s', (value) => {
		expect(() => resolveApiBaseUrl(value, development)).toThrow(
			'PUBLIC_API_BASE_URL must contain only an origin.'
		);
	});

	it.each(['api.settled.example', '/api', 'not a URL', 'mailto:hello@settled.example'])(
		'rejects a malformed or unsupported URL in %s',
		(value) => {
			expect(() => resolveApiBaseUrl(value, development)).toThrow();
		}
	);
});
