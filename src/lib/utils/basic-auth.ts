const BYTE_CHUNK_SIZE = 0x8000;

export function createBasicAuthorization(email: string, password: string): string {
	if (email.includes(':')) {
		throw new TypeError('A Basic Auth email address cannot contain a colon.');
	}

	const bytes = new TextEncoder().encode(`${email}:${password}`);
	let binary = '';

	for (let offset = 0; offset < bytes.length; offset += BYTE_CHUNK_SIZE) {
		binary += String.fromCharCode(...bytes.subarray(offset, offset + BYTE_CHUNK_SIZE));
	}

	return `Basic ${btoa(binary)}`;
}
