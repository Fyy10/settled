export const DEFAULT_AUTHENTICATED_PATH = '/groups';

const localOrigin = 'https://settled.invalid';
const unsafePathCharacters = /[\u0000-\u001f\u007f\\]/;

export function safeNextPath(value: string | null | undefined): string {
	if (
		value === undefined ||
		value === null ||
		!value.startsWith('/') ||
		value.startsWith('//') ||
		unsafePathCharacters.test(value)
	) {
		return DEFAULT_AUTHENTICATED_PATH;
	}

	try {
		const url = new URL(value, localOrigin);
		if (url.origin !== localOrigin || !url.pathname.startsWith('/')) {
			return DEFAULT_AUTHENTICATED_PATH;
		}
	} catch {
		return DEFAULT_AUTHENTICATED_PATH;
	}

	return value;
}

export function sessionExpiredLoginPath(next: string | null | undefined): string {
	const safeNext = safeNextPath(next);

	return `/login?reason=session-expired&next=${encodeURIComponent(safeNext)}`;
}
