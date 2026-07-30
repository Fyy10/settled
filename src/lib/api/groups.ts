import { request } from './client';
import { invalidResponseError } from './errors';
import type {
	CreateGroupInput,
	GroupDetailResponse,
	GroupSummary,
	JoinGroupInput
} from './types';
import {
	isGroupDetailResponse,
	isGroupListResponse,
	isGroupResponse,
	requireApiPayload
} from './validation';

type GroupRequestOptions = {
	signal?: AbortSignal;
};

export async function listGroups(
	options: GroupRequestOptions = {}
): Promise<GroupSummary[]> {
	const payload = await request<unknown>('/api/groups', {
		signal: options.signal
	});
	const response = requireApiPayload(
		payload,
		isGroupListResponse,
		'group-list response'
	);

	return response.groups;
}

export async function getGroupDetail(
	groupId: string,
	options: GroupRequestOptions = {}
): Promise<GroupDetailResponse> {
	const payload = await request<unknown>(
		`/api/groups/${encodeURIComponent(groupId)}`,
		{ signal: options.signal }
	);

	const response = requireApiPayload(
		payload,
		isGroupDetailResponse,
		'group-detail response'
	);
	if (response.group.id !== groupId) {
		throw invalidResponseError(
			200,
			undefined,
			'The server returned group detail for a different group.'
		);
	}

	return response;
}

export async function createGroup(
	input: Readonly<CreateGroupInput>,
	options: GroupRequestOptions = {}
): Promise<GroupSummary> {
	const payload = await request<unknown>('/api/groups', {
		method: 'POST',
		body: input,
		signal: options.signal
	});
	const response = requireApiPayload(
		payload,
		isGroupResponse,
		'create-group response'
	);

	return response.group;
}

export async function joinGroup(
	input: Readonly<JoinGroupInput>,
	options: GroupRequestOptions = {}
): Promise<GroupSummary> {
	const payload = await request<unknown>('/api/groups/join', {
		method: 'POST',
		body: input,
		signal: options.signal
	});
	const response = requireApiPayload(
		payload,
		isGroupResponse,
		'join-group response'
	);

	return response.group;
}
