import { dev } from '$app/environment';
import { env } from '$env/dynamic/public';

import { resolveApiBaseUrl } from './api-base-url';

export const API_BASE_URL = resolveApiBaseUrl(env.PUBLIC_API_BASE_URL, {
	production: !dev
});
