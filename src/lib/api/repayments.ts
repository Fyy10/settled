import { request } from './client';
import { invalidResponseError } from './errors';
import type { RepaymentListResponse } from './types';
import { isRepaymentListResponse, requireApiPayload } from './validation';

type RepaymentRequestOptions = {
	signal?: AbortSignal;
};

export async function listRepayments(
	groupId: string,
	options: RepaymentRequestOptions = {}
): Promise<RepaymentListResponse> {
	const payload = await request<unknown>(
		`/api/groups/${encodeURIComponent(groupId)}/repayments`,
		{ signal: options.signal }
	);

	const response = requireApiPayload(
		payload,
		isRepaymentListResponse,
		'repayment-list response'
	);
	if (response.repayments.some((repayment) => repayment.groupId !== groupId)) {
		throw invalidResponseError(
			200,
			undefined,
			'The server returned payment records for a different group.'
		);
	}

	return response;
}
