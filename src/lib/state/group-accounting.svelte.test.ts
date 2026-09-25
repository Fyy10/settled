import { describe, expect, it, vi } from 'vitest';

import {
	GroupAccountingState,
	type GroupAccountingLoaders
} from './group-accounting.svelte';

vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

describe('GroupAccountingState', () => {
	it('starts the three independent overview resources together', async () => {
		const expenses = deferred<{ expenses: never[]; members: never[] }>();
		const repayments = deferred<{ repayments: never[]; members: never[] }>();
		const settlements = deferred<{ settlements: never[]; members: never[] }>();
		const loaders = createLoaders({
			expenses: vi.fn(() => expenses.promise),
			repayments: vi.fn(() => repayments.promise),
			settlements: vi.fn(() => settlements.promise)
		});
		const state = new GroupAccountingState(loaders);
		state.activate('group-id');

		const request = state.ensureOverview();

		expect(loaders.expenses).toHaveBeenCalledOnce();
		expect(loaders.repayments).toHaveBeenCalledOnce();
		expect(loaders.settlements).toHaveBeenCalledOnce();
		expect(state.expenses.status).toBe('loading');
		expect(state.repayments.status).toBe('loading');
		expect(state.settlements.status).toBe('loading');

		expenses.resolve({ expenses: [], members: [] });
		repayments.resolve({ repayments: [], members: [] });
		settlements.resolve({ settlements: [], members: [] });
		await request;
		expect(state.expenses.status).toBe('ready');
		expect(state.repayments.status).toBe('ready');
		expect(state.settlements.status).toBe('ready');
	});

	it('refreshes only expenses and settlements after an expense mutation', async () => {
		const loaders = createLoaders();
		const state = new GroupAccountingState(loaders);
		state.activate('group-id');
		await state.ensureOverview();
		vi.mocked(loaders.expenses).mockClear();
		vi.mocked(loaders.repayments).mockClear();
		vi.mocked(loaders.settlements).mockClear();

		await state.refreshAfterExpenseMutation();

		expect(loaders.expenses).toHaveBeenCalledOnce();
		expect(loaders.repayments).not.toHaveBeenCalled();
		expect(loaders.settlements).toHaveBeenCalledOnce();
	});

	it('waits for both expense revalidations before reporting one failure', async () => {
		const expenses = deferred<{ expenses: never[]; members: never[] }>();
		const settlements = deferred<{ settlements: never[]; members: never[] }>();
		const state = new GroupAccountingState(
			createLoaders({
				expenses: () => expenses.promise,
				settlements: () => settlements.promise
			})
		);
		state.activate('group-id');
		const settled = vi.fn();

		const refresh = state
			.refreshAfterExpenseMutation()
			.then(settled, settled);
		expenses.reject(new Error('expense refresh failed'));
		await Promise.resolve();
		expect(settled).not.toHaveBeenCalled();

		settlements.resolve({ settlements: [], members: [] });
		await refresh;
		expect(settled).toHaveBeenCalledOnce();
	});

	it('refreshes only repayments and settlements after a repayment mutation', async () => {
		const loaders = createLoaders();
		const state = new GroupAccountingState(loaders);
		state.activate('group-id');
		await state.ensureOverview();
		vi.mocked(loaders.expenses).mockClear();
		vi.mocked(loaders.repayments).mockClear();
		vi.mocked(loaders.settlements).mockClear();

		await state.refreshAfterRepaymentMutation();

		expect(loaders.expenses).not.toHaveBeenCalled();
		expect(loaders.repayments).toHaveBeenCalledOnce();
		expect(loaders.settlements).toHaveBeenCalledOnce();
	});

	it('aborts and ignores stale work when the active group changes', async () => {
		const first = deferred<{ expenses: never[]; members: never[] }>();
		const expenses = vi
			.fn<GroupAccountingLoaders['expenses']>()
			.mockReturnValueOnce(first.promise)
			.mockResolvedValueOnce({ expenses: [], members: [] });
		const state = new GroupAccountingState(createLoaders({ expenses }));
		state.activate('first-group');

		const obsolete = state.refreshExpenses();
		const firstSignal = expenses.mock.calls[0][1].signal;
		state.activate('second-group');
		await state.refreshExpenses();

		expect(firstSignal.aborted).toBe(true);
		expect(state.groupId).toBe('second-group');
		expect(state.expenses.status).toBe('ready');
		first.resolve({ expenses: [], members: [] });
		await obsolete;
		expect(state.expenses.status).toBe('ready');
		expect(state.expenses.revision).toBe(1);
	});

	it('preserves the last response when revalidation fails', async () => {
		const expenses = vi
			.fn<GroupAccountingLoaders['expenses']>()
			.mockResolvedValueOnce({ expenses: [], members: [] })
			.mockRejectedValueOnce(new Error('offline'));
		const state = new GroupAccountingState(createLoaders({ expenses }));
		state.activate('group-id');
		await state.refreshExpenses();
		const previous = state.expenses.response;

		await expect(state.refreshExpenses()).rejects.toThrow('offline');

		expect(state.expenses.status).toBe('ready');
		expect(state.expenses.response).toBe(previous);
		expect(state.expenses.error).toEqual(new Error('offline'));
	});

	it('does not duplicate resources that ensureOverview already loaded', async () => {
		const loaders = createLoaders();
		const state = new GroupAccountingState(loaders);
		state.activate('group-id');

		await state.ensureOverview();
		await state.ensureOverview();

		expect(loaders.expenses).toHaveBeenCalledOnce();
		expect(loaders.repayments).toHaveBeenCalledOnce();
		expect(loaders.settlements).toHaveBeenCalledOnce();
	});

	it('clears loaded data and aborts pending work for a terminal route state', async () => {
		const pending = deferred<{ settlements: never[]; members: never[] }>();
		const settlements = vi.fn<GroupAccountingLoaders['settlements']>(
			(_groupId, _options) => pending.promise
		);
		const loaders = createLoaders({
			settlements
		});
		const state = new GroupAccountingState(loaders);
		state.activate('group-id');
		await state.refreshExpenses();
		const settlementRequest = state.refreshSettlements();
		const signal = settlements.mock.calls[0]![1].signal;

		state.clear();

		expect(signal.aborted).toBe(true);
		expect(state.groupId).toBe('');
		expect(state.expenses.response).toBeNull();
		expect(state.expenses.status).toBe('idle');
		pending.resolve({ settlements: [], members: [] });
		await settlementRequest;
		expect(state.settlements.response).toBeNull();
	});
});

function createLoaders(
	overrides: Partial<GroupAccountingLoaders> = {}
): GroupAccountingLoaders {
	return {
		expenses:
			overrides.expenses ??
			vi.fn().mockResolvedValue({ expenses: [], members: [] }),
		repayments:
			overrides.repayments ??
			vi.fn().mockResolvedValue({ repayments: [], members: [] }),
		settlements:
			overrides.settlements ??
			vi.fn().mockResolvedValue({ settlements: [], members: [] })
	};
}

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (reason: unknown) => void;
} {
	let resolve!: (value: T) => void;
	let reject!: (reason: unknown) => void;
	const promise = new Promise<T>((complete, fail) => {
		resolve = complete;
		reject = fail;
	});
	return { promise, resolve, reject };
}
