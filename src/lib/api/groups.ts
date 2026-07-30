import { request } from './client';
import { invalidResponseError } from './errors';
import type {
	CreateGroupInput,
	GroupDetailResponse,
	GroupSummary,
	JoinGroupInput,
	RenameGroupInput
} from './types';
import {
	isGroupDetailResponse,
	isGroupListResponse,
	isGroupResponse,
	isJoinCodeResponse,
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

export async function renameGroup(
	groupId: string,
	input: Readonly<RenameGroupInput>,
	options: GroupRequestOptions = {}
): Promise<GroupSummary> {
	const payload = await request<unknown>(
		`/api/groups/${encodeURIComponent(groupId)}`,
		{
			method: 'PATCH',
			body: input,
			signal: options.signal,
			expectedStatus: 200
		}
	);
	const response = requireApiPayload(
		payload,
		isGroupResponse,
		'rename-group response'
	);
	requireMatchingGroupId(response.group, groupId);

	return response.group;
}

export async function getGroupJoinCode(
	groupId: string,
	options: GroupRequestOptions = {}
): Promise<string> {
	const payload = await request<unknown>(
		`/api/groups/${encodeURIComponent(groupId)}/join-code`,
		{ signal: options.signal, expectedStatus: 200 }
	);
	const response = requireApiPayload(
		payload,
		isJoinCodeResponse,
		'group-code response'
	);

	return response.joinCode;
}

export async function removeGroupMember(
	groupId: string,
	userId: string,
	options: GroupRequestOptions = {}
): Promise<void> {
	const payload = await request<unknown>(
		`/api/groups/${encodeURIComponent(groupId)}/members/${encodeURIComponent(userId)}`,
		{
			method: 'DELETE',
			signal: options.signal,
			expectedStatus: 204
		}
	);
	requireNoContent(payload, 'remove-member response');
}

export async function dissolveGroup(
	groupId: string,
	options: GroupRequestOptions = {}
): Promise<void> {
	const payload = await request<unknown>(`/api/groups/${encodeURIComponent(groupId)}`, {
		method: 'DELETE',
		signal: options.signal,
		expectedStatus: 204
	});
	requireNoContent(payload, 'dissolve-group response');
}

function requireMatchingGroupId(group: GroupSummary, groupId: string): void {
	if (group.id !== groupId) {
		throw invalidResponseError(
			200,
			undefined,
			'The server returned a different group.'
		);
	}
}

function requireNoContent(payload: unknown, description: string): void {
	if (payload !== undefined) {
		throw invalidResponseError(
			204,
			undefined,
			`The server returned an invalid ${description}.`
		);
	}
}
