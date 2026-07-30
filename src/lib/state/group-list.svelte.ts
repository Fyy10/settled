import { getContext, setContext } from 'svelte';

import { listGroups } from '$lib/api/groups';
import { isUnauthorizedApiError } from '$lib/api/errors';
import type { GroupSummary } from '$lib/api/types';

export type GroupListStatus = 'idle' | 'loading' | 'ready' | 'error';

export type GroupListLoader = (options: {
	signal: AbortSignal;
}) => Promise<GroupSummary[]>;

const groupListContextKey = Symbol('group-list-context');

export class GroupListContext {
	status = $state<GroupListStatus>('idle');
	groups = $state<GroupSummary[]>([]);
	error = $state<unknown>(null);
	isRefreshing = $state(false);
	revision = $state(0);

	#loader: GroupListLoader;
	#controller: AbortController | null = null;
	#requestVersion = 0;

	constructor(loader: GroupListLoader = listGroups) {
		this.#loader = loader;
	}

	async refresh({ preserve = this.status === 'ready' } = {}): Promise<GroupSummary[]> {
		this.#controller?.abort();
		const controller = new AbortController();
		const requestVersion = ++this.#requestVersion;
		this.#controller = controller;

		if (preserve && this.status === 'ready') {
			this.isRefreshing = true;
			this.error = null;
		} else {
			this.status = 'loading';
			this.groups = [];
			this.error = null;
		}

		try {
			const groups = await this.#loader({ signal: controller.signal });
			if (this.#isCurrent(controller, requestVersion)) {
				this.groups = [...groups];
				this.status = 'ready';
				this.error = null;
				this.revision += 1;
			}

			return groups;
		} catch (error) {
			if (!this.#isCurrent(controller, requestVersion)) {
				throw error;
			}

			if (isUnauthorizedApiError(error)) {
				this.clear();
			} else if (preserve && this.status === 'ready') {
				this.error = error;
				this.isRefreshing = false;
			} else {
				this.groups = [];
				this.error = error;
				this.status = 'error';
			}

			throw error;
		} finally {
			if (this.#isCurrent(controller, requestVersion)) {
				this.#controller = null;
				this.isRefreshing = false;
			}
		}
	}

	clear(): void {
		this.#requestVersion += 1;
		this.#controller?.abort();
		this.#controller = null;
		this.groups = [];
		this.error = null;
		this.isRefreshing = false;
		this.status = 'idle';
		this.revision += 1;
	}

	dispose(): void {
		this.#requestVersion += 1;
		this.#controller?.abort();
		this.#controller = null;
		this.isRefreshing = false;
	}

	#isCurrent(controller: AbortController, requestVersion: number): boolean {
		return (
			!controller.signal.aborted &&
			this.#controller === controller &&
			this.#requestVersion === requestVersion
		);
	}
}

export function provideGroupListContext(
	context: GroupListContext
): GroupListContext {
	setContext(groupListContextKey, context);
	return context;
}

export function useGroupListContext(): GroupListContext {
	const context = getContext<GroupListContext | undefined>(groupListContextKey);
	if (context === undefined) {
		throw new Error('Group list context is unavailable outside the app route.');
	}

	return context;
}
