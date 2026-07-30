import { request } from './client';
import type { SettlementListResponse } from './types';
import { isSettlementListResponse, requireApiPayload } from './validation';

type SettlementRequestOptions = {
	signal?: AbortSignal;
};

export async function listSettlements(
	groupId: string,
	options: SettlementRequestOptions = {}
): Promise<SettlementListResponse> {
	const payload = await request<unknown>(
		`/api/groups/${encodeURIComponent(groupId)}/settlements`,
		{ signal: options.signal }
	);

	return requireApiPayload(
		payload,
		isSettlementListResponse,
		'settlement-list response'
	);
}
