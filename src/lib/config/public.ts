import { dev } from '$app/environment';
import { PUBLIC_API_BASE_URL } from '$env/static/public';

import { resolveApiBaseUrl } from './api-base-url';

export const API_BASE_URL = resolveApiBaseUrl(PUBLIC_API_BASE_URL, {
	production: !dev
});
