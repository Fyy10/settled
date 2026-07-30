import { describe, expect, it } from 'vitest';

import { createBasicAuthorization } from './basic-auth';

describe('createBasicAuthorization', () => {
	it('encodes email and non-ASCII passwords as UTF-8 before Base64', () => {
		const authorization = createBasicAuthorization('alice@example.com', 'påss🔐漢字');

		expect(decodeBasicAuthorization(authorization)).toBe(
			'alice@example.com:påss🔐漢字'
		);
	});

	it('preserves password whitespace and colons', () => {
		const authorization = createBasicAuthorization('alice@example.com', '  pass:word  ');

		expect(decodeBasicAuthorization(authorization)).toBe(
			'alice@example.com:  pass:word  '
		);
	});

	it('encodes values larger than the bounded conversion chunk', () => {
		const password = '密'.repeat(40_000);

		expect(decodeBasicAuthorization(createBasicAuthorization('a@example.com', password))).toBe(
			`a@example.com:${password}`
		);
	});

	it('rejects an email containing the Basic Auth delimiter', () => {
		expect(() => createBasicAuthorization('alice:admin@example.com', 'password')).toThrow(
			'A Basic Auth email address cannot contain a colon.'
		);
	});
});

function decodeBasicAuthorization(authorization: string): string {
	const binary = atob(authorization.slice('Basic '.length));
	const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));

	return new TextDecoder().decode(bytes);
}
