import { getContext, setContext } from 'svelte';

import { listExpenses } from '$lib/api/expenses';
import { listRepayments } from '$lib/api/repayments';
import { listSettlements } from '$lib/api/settlements';
import type {
	ExpenseListResponse,
	RepaymentListResponse,
	SettlementListResponse
} from '$lib/api/types';

export type AccountingResourceStatus = 'idle' | 'loading' | 'ready' | 'error';

export type AccountingResourceState<T> = {
	status: AccountingResourceStatus;
	response: T | null;
	error: unknown;
	isRefreshing: boolean;
	revision: number;
};

export type AccountingLoader<T> = (
	groupId: string,
	options: { signal: AbortSignal }
) => Promise<T>;

export type GroupAccountingLoaders = {
	expenses: AccountingLoader<ExpenseListResponse>;
	repayments: AccountingLoader<RepaymentListResponse>;
	settlements: AccountingLoader<SettlementListResponse>;
};

type ResourceName = keyof GroupAccountingLoaders;
type ResourceResponse<K extends ResourceName> = Awaited<
	ReturnType<GroupAccountingLoaders[K]>
>;

const groupAccountingContextKey = Symbol('group-accounting-context');

export class GroupAccountingState {
	groupId = $state('');
	expenses = $state<AccountingResourceState<ExpenseListResponse>>(
		initialResource()
	);
	repayments = $state<AccountingResourceState<RepaymentListResponse>>(
		initialResource()
	);
	settlements = $state<AccountingResourceState<SettlementListResponse>>(
		initialResource()
	);

	#loaders: GroupAccountingLoaders;
	#controllers: Partial<Record<ResourceName, AbortController>> = {};
	#versions: Record<ResourceName, number> = {
		expenses: 0,
		repayments: 0,
		settlements: 0
	};

	constructor(
		loaders: Partial<GroupAccountingLoaders> = {}
	) {
		this.#loaders = {
			expenses: loaders.expenses ?? listExpenses,
			repayments: loaders.repayments ?? listRepayments,
			settlements: loaders.settlements ?? listSettlements
		};
	}

	activate(groupId: string): void {
		if (groupId === this.groupId) {
			return;
		}

		this.#cancelAll();
		this.groupId = groupId;
		this.expenses = initialResource();
		this.repayments = initialResource();
		this.settlements = initialResource();
	}

	async ensureOverview(): Promise<void> {
		if (this.groupId === '') {
			throw new Error('Group accounting has no active group ID.');
		}

		const requests: Promise<unknown>[] = [];
		if (this.expenses.status === 'idle') {
			requests.push(this.refreshExpenses());
		}
		if (this.repayments.status === 'idle') {
			requests.push(this.refreshRepayments());
		}
		if (this.settlements.status === 'idle') {
			requests.push(this.refreshSettlements());
		}

		await Promise.allSettled(requests);
	}

	refreshExpenses(): Promise<ExpenseListResponse> {
		return this.#refresh('expenses');
	}

	refreshRepayments(): Promise<RepaymentListResponse> {
		return this.#refresh('repayments');
	}

	refreshSettlements(): Promise<SettlementListResponse> {
		return this.#refresh('settlements');
	}

	async refreshAfterExpenseMutation(): Promise<void> {
		await settleRefreshes([
			this.refreshExpenses(),
			this.refreshSettlements()
		]);
	}

	async refreshAfterRepaymentMutation(): Promise<void> {
		await settleRefreshes([
			this.refreshRepayments(),
			this.refreshSettlements()
		]);
	}

	dispose(): void {
		this.#cancelAll();
	}

	clear(): void {
		this.#cancelAll();
		this.groupId = '';
		this.expenses = initialResource();
		this.repayments = initialResource();
		this.settlements = initialResource();
	}

	async #refresh<K extends ResourceName>(
		resource: K
	): Promise<ResourceResponse<K>> {
		if (this.groupId === '') {
			throw new Error('Group accounting has no active group ID.');
		}

		this.#controllers[resource]?.abort();
		const controller = new AbortController();
		const requestVersion = ++this.#versions[resource];
		const groupId = this.groupId;
		this.#controllers[resource] = controller;

		const current = this.#getResource(resource);
		const preserve = current.response !== null;
		this.#setResource(resource, {
			...current,
			status: 'loading',
			error: null,
			isRefreshing: preserve
		});

		try {
			const response = await this.#loaders[resource](groupId, {
				signal: controller.signal
			});
			if (this.#isCurrent(resource, controller, requestVersion, groupId)) {
				this.#setResource(resource, {
					status: 'ready',
					response: response as ResourceResponse<K>,
					error: null,
					isRefreshing: false,
					revision: current.revision + 1
				});
			}

			return response as ResourceResponse<K>;
		} catch (error) {
			if (this.#isCurrent(resource, controller, requestVersion, groupId)) {
				this.#setResource(resource, {
					status: preserve ? 'ready' : 'error',
					response: preserve ? current.response : null,
					error,
					isRefreshing: false,
					revision: current.revision
				});
			}
			throw error;
		} finally {
			if (this.#isCurrent(resource, controller, requestVersion, groupId)) {
				delete this.#controllers[resource];
			}
		}
	}

	#getResource<K extends ResourceName>(
		resource: K
	): AccountingResourceState<ResourceResponse<K>> {
		return this[resource] as AccountingResourceState<ResourceResponse<K>>;
	}

	#setResource<K extends ResourceName>(
		resource: K,
		state: AccountingResourceState<ResourceResponse<K>>
	): void {
		if (resource === 'expenses') {
			this.expenses = state as AccountingResourceState<ExpenseListResponse>;
		} else if (resource === 'repayments') {
			this.repayments =
				state as AccountingResourceState<RepaymentListResponse>;
		} else {
			this.settlements =
				state as AccountingResourceState<SettlementListResponse>;
		}
	}

	#isCurrent(
		resource: ResourceName,
		controller: AbortController,
		requestVersion: number,
		groupId: string
	): boolean {
		return (
			!controller.signal.aborted &&
			this.#controllers[resource] === controller &&
			this.#versions[resource] === requestVersion &&
			this.groupId === groupId
		);
	}

	#cancelAll(): void {
		for (const resource of [
			'expenses',
			'repayments',
			'settlements'
		] as const) {
			this.#versions[resource] += 1;
			this.#controllers[resource]?.abort();
			delete this.#controllers[resource];
		}
	}
}

export function provideGroupAccounting(
	state = new GroupAccountingState()
): GroupAccountingState {
	setContext(groupAccountingContextKey, state);
	return state;
}

export function useGroupAccounting(): GroupAccountingState {
	const state = getContext<GroupAccountingState | undefined>(
		groupAccountingContextKey
	);
	if (state === undefined) {
		throw new Error('Group accounting context is unavailable outside its route.');
	}

	return state;
}

function initialResource<T>(): AccountingResourceState<T> {
	return {
		status: 'idle',
		response: null,
		error: null,
		isRefreshing: false,
		revision: 0
	};
}

async function settleRefreshes(
	requests: readonly Promise<unknown>[]
): Promise<void> {
	const results = await Promise.allSettled(requests);
	const failure = results.find(
		(result): result is PromiseRejectedResult => result.status === 'rejected'
	);
	if (failure !== undefined) {
		throw failure.reason;
	}
}
