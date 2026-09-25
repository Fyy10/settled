import { request } from './client';
import { invalidResponseError } from './errors';
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
		{ signal: options.signal, expectedStatus: 200 }
	);

	const response = requireApiPayload(
		payload,
		isSettlementListResponse,
		'settlement-list response'
	);
	if (!isCoherentSettlementList(response)) {
		throw invalidResponseError(
			200,
			undefined,
			'The server returned an incoherent settlement list.'
		);
	}

	return response;
}

function isCoherentSettlementList(
	response: SettlementListResponse
): boolean {
	const memberIds = new Set<string>();
	for (const member of response.members) {
		const identity = member.userId.toLocaleLowerCase('en-US');
		if (memberIds.has(identity)) {
			return false;
		}
		memberIds.add(identity);
	}

	const pairs = new Set<string>();
	for (const settlement of response.settlements) {
		const fromIdentity =
			settlement.fromUserId.toLocaleLowerCase('en-US');
		const toIdentity = settlement.toUserId.toLocaleLowerCase('en-US');
		if (
			!memberIds.has(fromIdentity) ||
			!memberIds.has(toIdentity)
		) {
			return false;
		}

		const pair =
			fromIdentity < toIdentity
				? `${fromIdentity}\u0000${toIdentity}`
				: `${toIdentity}\u0000${fromIdentity}`;
		if (pairs.has(pair)) {
			return false;
		}
		pairs.add(pair);
	}

	return true;
}
