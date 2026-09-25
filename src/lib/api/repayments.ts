import { normalizeRepaymentNote } from '$lib/utils/repayment-draft';

import { request } from './client';
import { invalidResponseError } from './errors';
import type {
	CreateRepaymentInput,
	Repayment,
	RepaymentListResponse,
	ReplaceRepaymentInput
} from './types';
import {
	isRepaymentListResponse,
	isRepaymentResponse,
	requireApiPayload
} from './validation';

type RepaymentRequestOptions = {
	signal?: AbortSignal;
};

export async function listRepayments(
	groupId: string,
	options: RepaymentRequestOptions = {}
): Promise<RepaymentListResponse> {
	const payload = await request<unknown>(repaymentCollectionPath(groupId), {
		signal: options.signal,
		expectedStatus: 200
	});

	const response = requireApiPayload(
		payload,
		isRepaymentListResponse,
		'repayment-list response'
	);
	if (!isCoherentRepaymentList(response, groupId)) {
		throw invalidResponseError(
			200,
			undefined,
			'The server returned an incoherent repayment list.'
		);
	}

	return response;
}

export async function getRepayment(
	groupId: string,
	repaymentId: string,
	options: RepaymentRequestOptions = {}
): Promise<Repayment> {
	const payload = await request<unknown>(
		repaymentItemPath(groupId, repaymentId),
		{ signal: options.signal, expectedStatus: 200 }
	);

	return requireCoherentRepayment(
		payload,
		200,
		'repayment response',
		groupId,
		repaymentId
	);
}

export async function createRepayment(
	groupId: string,
	input: Readonly<CreateRepaymentInput>,
	options: RepaymentRequestOptions = {}
): Promise<Repayment> {
	const payload = await request<unknown>(repaymentCollectionPath(groupId), {
		method: 'POST',
		body: input,
		signal: options.signal,
		expectedStatus: 201
	});

	return requireCoherentRepayment(
		payload,
		201,
		'create-repayment response',
		groupId,
		undefined,
		input
	);
}

export async function replaceRepayment(
	groupId: string,
	repaymentId: string,
	input: Readonly<ReplaceRepaymentInput>,
	options: RepaymentRequestOptions = {}
): Promise<Repayment> {
	const payload = await request<unknown>(
		repaymentItemPath(groupId, repaymentId),
		{
			method: 'PUT',
			body: input,
			signal: options.signal,
			expectedStatus: 200
		}
	);

	return requireCoherentRepayment(
		payload,
		200,
		'replace-repayment response',
		groupId,
		repaymentId,
		input
	);
}

export async function deleteRepayment(
	groupId: string,
	repaymentId: string,
	options: RepaymentRequestOptions = {}
): Promise<void> {
	const payload = await request<unknown>(
		repaymentItemPath(groupId, repaymentId),
		{
			method: 'DELETE',
			signal: options.signal,
			expectedStatus: 204
		}
	);
	if (payload !== undefined) {
		throw invalidResponseError(
			204,
			undefined,
			'The server returned an invalid delete-repayment response.'
		);
	}
}

function repaymentCollectionPath(groupId: string): string {
	return `/api/groups/${encodeURIComponent(groupId)}/repayments`;
}

function repaymentItemPath(
	groupId: string,
	repaymentId: string
): string {
	return `${repaymentCollectionPath(groupId)}/${encodeURIComponent(repaymentId)}`;
}

function requireCoherentRepayment(
	payload: unknown,
	status: number,
	description: string,
	groupId: string,
	repaymentId?: string,
	input?: Readonly<CreateRepaymentInput>
): Repayment {
	if (!isRepaymentResponse(payload)) {
		throw invalidResponseError(
			status,
			undefined,
			`The server returned an invalid ${description}.`
		);
	}
	if (
		payload.repayment.groupId !== groupId ||
		(repaymentId !== undefined &&
			payload.repayment.id !== repaymentId)
	) {
		throw invalidResponseError(
			status,
			undefined,
			'The server returned a repayment for a different resource.'
		);
	}
	if (
		input !== undefined &&
		!matchesRepaymentInput(payload.repayment, input)
	) {
		throw invalidResponseError(
			status,
			undefined,
			'The server returned a repayment that does not match the submitted input.'
		);
	}

	return payload.repayment;
}

function isCoherentRepaymentList(
	response: RepaymentListResponse,
	groupId: string
): boolean {
	const repaymentIds = new Set<string>();
	const memberIds = new Set<string>();

	for (const member of response.members) {
		const identity = member.userId.toLocaleLowerCase('en-US');
		if (memberIds.has(identity)) {
			return false;
		}
		memberIds.add(identity);
	}

	for (const repayment of response.repayments) {
		const identity = repayment.id.toLocaleLowerCase('en-US');
		if (
			repayment.groupId !== groupId ||
			repaymentIds.has(identity)
		) {
			return false;
		}
		repaymentIds.add(identity);

		if (
			!memberIds.has(
				repayment.fromUserId.toLocaleLowerCase('en-US')
			) ||
			!memberIds.has(
				repayment.toUserId.toLocaleLowerCase('en-US')
			)
		) {
			return false;
		}
	}

	return true;
}

function matchesRepaymentInput(
	repayment: Repayment,
	input: Readonly<CreateRepaymentInput>
): boolean {
	return (
		repayment.fromUserId === input.fromUserId &&
		repayment.toUserId === input.toUserId &&
		repayment.amountCents === input.amountCents &&
		repayment.repaymentDate === input.repaymentDate &&
		repayment.note === normalizeRepaymentNote(input.note)
	);
}
