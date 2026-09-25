export const LOCAL_API_BASE_URL = 'http://localhost:8080';

type ResolveApiBaseUrlOptions = {
	production: boolean;
};

export function resolveApiBaseUrl(
	value: string | undefined,
	{ production }: ResolveApiBaseUrlOptions
): string {
	if (value === undefined || value === '') {
		if (production) {
			throw new Error('PUBLIC_API_BASE_URL is required for production builds.');
		}

		return LOCAL_API_BASE_URL;
	}

	if (value.trim() !== value) {
		throw new Error('PUBLIC_API_BASE_URL must not include surrounding whitespace.');
	}

	if (value.endsWith('/')) {
		throw new Error('PUBLIC_API_BASE_URL must not include a trailing slash.');
	}

	let url: URL;

	try {
		url = new URL(value);
	} catch {
		throw new Error('PUBLIC_API_BASE_URL must be an absolute HTTP or HTTPS origin.');
	}

	if (url.protocol !== 'http:' && url.protocol !== 'https:') {
		throw new Error('PUBLIC_API_BASE_URL must use HTTP or HTTPS.');
	}

	if (
		url.username !== '' ||
		url.password !== '' ||
		url.pathname !== '/' ||
		url.search !== '' ||
		url.hash !== ''
	) {
		throw new Error('PUBLIC_API_BASE_URL must contain only an origin.');
	}

	if (production && url.protocol !== 'https:') {
		throw new Error('PUBLIC_API_BASE_URL must use HTTPS for production builds.');
	}

	return url.origin;
}
