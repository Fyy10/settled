import { getContext, setContext } from 'svelte';

import { getGroupDetail } from '$lib/api/groups';
import {
	isNotFoundApiError,
	isUnauthorizedApiError
} from '$lib/api/errors';
import type {
	GroupDetailResponse,
	GroupMember,
	GroupSummary
} from '$lib/api/types';

export type GroupDetailStatus = 'loading' | 'ready' | 'error' | 'hidden';

export type GroupDetailLoader = (
	groupId: string,
	options: { signal: AbortSignal }
) => Promise<GroupDetailResponse>;

const groupDetailContextKey = Symbol('group-detail-context');

export class GroupDetailContext {
	groupId = $state('');
	status = $state<GroupDetailStatus>('loading');
	group = $state<GroupSummary | null>(null);
	members = $state<GroupMember[]>([]);
	error = $state<unknown>(null);
	isRefreshing = $state(false);
	revision = $state(0);

	#loader: GroupDetailLoader;
	#controller: AbortController | null = null;
	#pending: Promise<GroupDetailResponse> | null = null;
	#requestVersion = 0;

	constructor(loader: GroupDetailLoader = getGroupDetail) {
		this.#loader = loader;
	}

	load(groupId: string): Promise<GroupDetailResponse> {
		if (groupId === this.groupId) {
			if (this.#pending !== null) {
				return this.#pending;
			}
			if (this.status === 'ready' && this.group !== null) {
				return Promise.resolve({
					group: this.group,
					members: [...this.members]
				});
			}
		}

		this.#beginGroup(groupId);
		return this.#request(false);
	}

	refreshDetail(): Promise<GroupDetailResponse> {
		if (this.groupId === '') {
			return Promise.reject(new Error('Group detail has no active group ID.'));
		}
		if (this.#pending !== null) {
			return this.#pending;
		}

		return this.#request(this.status === 'ready' && this.group !== null);
	}

	replaceDetail(detail: GroupDetailResponse): void {
		if (detail.group.id !== this.groupId) {
			throw new Error('Group detail does not match the active route.');
		}

		this.#applyDetail(detail);
	}

	updateGroup(group: GroupSummary): void {
		if (group.id !== this.groupId || this.group === null) {
			throw new Error('Group summary does not match the active route.');
		}

		this.group = group;
		this.status = 'ready';
		this.error = null;
		this.revision += 1;
	}

	markHidden(): void {
		this.#cancelPending();
		this.group = null;
		this.members = [];
		this.error = null;
		this.isRefreshing = false;
		this.status = 'hidden';
		this.revision += 1;
	}

	clear(): void {
		this.#cancelPending();
		this.group = null;
		this.members = [];
		this.error = null;
		this.isRefreshing = false;
		this.status = 'loading';
		this.revision += 1;
	}

	dispose(): void {
		this.#cancelPending();
	}

	#beginGroup(groupId: string): void {
		this.#cancelPending();
		this.groupId = groupId;
		this.group = null;
		this.members = [];
		this.error = null;
		this.isRefreshing = false;
		this.status = 'loading';
		this.revision += 1;
	}

	#request(preserveReadyData: boolean): Promise<GroupDetailResponse> {
		const controller = new AbortController();
		const requestVersion = ++this.#requestVersion;
		this.#controller = controller;

		if (preserveReadyData) {
			this.isRefreshing = true;
			this.error = null;
		} else {
			this.status = 'loading';
		}

		const request = this.#loader(this.groupId, { signal: controller.signal })
			.then((detail) => {
				if (this.#isCurrent(controller, requestVersion)) {
					this.#applyDetail(detail);
				}

				return detail;
			})
			.catch((error: unknown) => {
				if (!this.#isCurrent(controller, requestVersion)) {
					throw error;
				}

				if (isNotFoundApiError(error)) {
					this.markHidden();
				} else if (isUnauthorizedApiError(error)) {
					this.clear();
				} else if (preserveReadyData) {
					this.error = error;
					this.isRefreshing = false;
				} else {
					this.group = null;
					this.members = [];
					this.error = error;
					this.status = 'error';
				}

				throw error;
			})
			.finally(() => {
				if (this.#isCurrent(controller, requestVersion)) {
					this.#controller = null;
					this.#pending = null;
					this.isRefreshing = false;
				}
			});

		this.#pending = request;
		return request;
	}

	#applyDetail(detail: GroupDetailResponse): void {
		if (detail.group.id !== this.groupId) {
			throw new Error('Group detail does not match the active route.');
		}

		this.group = detail.group;
		this.members = [...detail.members];
		this.error = null;
		this.isRefreshing = false;
		this.status = 'ready';
		this.revision += 1;
	}

	#cancelPending(): void {
		this.#requestVersion += 1;
		this.#controller?.abort();
		this.#controller = null;
		this.#pending = null;
	}

	#isCurrent(controller: AbortController, requestVersion: number): boolean {
		return (
			!controller.signal.aborted &&
			this.#controller === controller &&
			this.#requestVersion === requestVersion
		);
	}
}

export function provideGroupDetailContext(
	context: GroupDetailContext
): GroupDetailContext {
	setContext(groupDetailContextKey, context);
	return context;
}

export function useGroupDetailContext(): GroupDetailContext {
	const context = getContext<GroupDetailContext | undefined>(
		groupDetailContextKey
	);
	if (context === undefined) {
		throw new Error('Group detail context is unavailable outside its route.');
	}

	return context;
}
