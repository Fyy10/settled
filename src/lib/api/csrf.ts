import { sendApiRequest } from './transport';
import type { CsrfResponse } from './types';
import { isCsrfResponse, requireApiPayload } from './validation';

export async function fetchCsrfToken(): Promise<string> {
	const payload = await sendApiRequest('/api/auth/csrf');
	const response = requireApiPayload<CsrfResponse>(payload, isCsrfResponse, 'CSRF response');

	return response.csrfToken;
}
