import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor
} from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import {
	expenseListFixture,
	groupDetailFixture
} from '../../../../../../../tests/fixtures/api-contract';

import EditExpensePage from './+page.svelte';

type RouteParams = {
	groupId: string;
	expenseId: string;
};

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	getExpense: vi.fn(),
	replaceExpense: vi.fn(),
	deleteExpense: vi.fn(),
	toastSuccess: vi.fn(),
	toastError: vi.fn(),
	page: {
		params: {
			groupId: '',
			expenseId: ''
		}
	},
	setPageParams: (_params: RouteParams): void => undefined,
	groupDetail: {
		groupId: '',
		status: 'ready' as 'loading' | 'ready' | 'error' | 'hidden',
		group: null as (typeof groupDetailFixture)['group'] | null,
		members: [] as (typeof groupDetailFixture)['members'],
		refreshDetail: vi.fn()
	},
	accounting: {
		expenses: { error: null as unknown },
		settlements: { error: null as unknown },
		refreshAfterExpenseMutation: vi.fn()
	}
}));

vi.mock('$app/navigation', () => ({
	goto: mocks.goto,
	beforeNavigate: mocks.beforeNavigate
}));
vi.mock('$app/state', async () => {
	const { createSubscriber } = await import('svelte/reactivity');
	let update = (): void => undefined;
	const subscribe = createSubscriber((notify) => {
		update = notify;
		return () => {
			update = (): void => undefined;
		};
	});

	mocks.setPageParams = (params) => {
		mocks.page.params = params;
		update();
	};

	return {
		page: {
			get params() {
				subscribe();
				return mocks.page.params;
			}
		}
	};
});
vi.mock('$lib/api/expenses', () => ({
	getExpense: mocks.getExpense,
	replaceExpense: mocks.replaceExpense,
	deleteExpense: mocks.deleteExpense
}));
vi.mock('$lib/state/group-detail.svelte', () => ({
	useGroupDetailContext: () => mocks.groupDetail
}));
vi.mock('$lib/state/group-accounting.svelte', () => ({
	useGroupAccounting: () => mocks.accounting
}));
vi.mock('svelte-sonner', () => ({
	toast: {
		success: mocks.toastSuccess,
		error: mocks.toastError
	}
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.beforeNavigate.mockReset();
	mocks.getExpense
		.mockReset()
		.mockResolvedValue(expenseListFixture.expenses[0]);
	mocks.replaceExpense
		.mockReset()
		.mockResolvedValue(expenseListFixture.expenses[0]);
	mocks.deleteExpense.mockReset().mockResolvedValue(undefined);
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
	mocks.setPageParams({
		groupId: groupDetailFixture.group.id,
		expenseId: expenseListFixture.expenses[0].id
	});
	mocks.groupDetail.groupId = groupDetailFixture.group.id;
	mocks.groupDetail.status = 'ready';
	mocks.groupDetail.group = groupDetailFixture.group;
	mocks.groupDetail.members = [...groupDetailFixture.members];
	mocks.groupDetail.refreshDetail.mockReset().mockResolvedValue(groupDetailFixture);
	mocks.accounting.expenses.error = null;
	mocks.accounting.settlements.error = null;
	mocks.accounting.refreshAfterExpenseMutation
		.mockReset()
		.mockResolvedValue(undefined);
});

afterEach(cleanup);

describe('edit expense route', () => {
	it('starts the target GET while parent group detail is still loading', async () => {
		const target = deferred<typeof expenseListFixture.expenses[0]>();
		mocks.groupDetail.status = 'loading';
		mocks.groupDetail.group = null;
		mocks.getExpense.mockReturnValue(target.promise);

		const result = render(EditExpensePage);

		await waitFor(() => expect(mocks.getExpense).toHaveBeenCalledOnce());
		expect(mocks.getExpense).toHaveBeenCalledWith(
			groupDetailFixture.group.id,
			expenseListFixture.expenses[0].id,
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
		const signal = mocks.getExpense.mock.calls[0][2].signal;
		result.unmount();
		expect(signal.aborted).toBe(true);
	});

	it('opens persisted splits as exact amounts in stable member order', async () => {
		const { container } = render(EditExpensePage);

		expect(await screen.findByLabelText('Share for Alice')).toHaveValue('27.00');
		expect(screen.getByLabelText('Share for Bob')).toHaveValue('27.00');
		expect(screen.getByRole('radio', { name: 'Exact amounts' })).toBeChecked();
		expect(screen.getByLabelText('Description')).toHaveValue('Groceries');
		expect(container.querySelector('[data-slot="card"]')).toHaveClass(
			'overflow-visible'
		);
	});

	it('reinitializes expense A after a pending B route is replaced by A', async () => {
		const pendingB = deferred<typeof expenseListFixture.expenses[0]>();
		const expenseA = expenseListFixture.expenses[0];
		const expenseBId = '00000000-0000-4000-8000-000000000099';
		mocks.getExpense.mockImplementation(
			(_groupId: string, expenseId: string) =>
				expenseId === expenseBId
					? pendingB.promise
					: Promise.resolve(expenseA)
		);

		render(EditExpensePage);
		expect(await screen.findByLabelText('Description')).toHaveValue(
			expenseA.description
		);

		mocks.setPageParams({
			groupId: groupDetailFixture.group.id,
			expenseId: expenseBId
		});
		await waitFor(() =>
			expect(mocks.getExpense).toHaveBeenCalledWith(
				groupDetailFixture.group.id,
				expenseBId,
				expect.objectContaining({ signal: expect.any(AbortSignal) })
			)
		);
		await waitFor(() =>
			expect(screen.queryByLabelText('Description')).not.toBeInTheDocument()
		);
		const pendingSignal = mocks.getExpense.mock.calls.find(
			(call) => call[1] === expenseBId
		)?.[2].signal;

		mocks.setPageParams({
			groupId: groupDetailFixture.group.id,
			expenseId: expenseA.id
		});

		await waitFor(() => expect(mocks.getExpense).toHaveBeenCalledTimes(3));
		expect(pendingSignal?.aborted).toBe(true);
		expect(await screen.findByLabelText('Description')).toHaveValue(
			expenseA.description
		);
	});

	it('revalidates group detail before deciding an initial GET 404 is a missing record', async () => {
		const detail = deferred<typeof groupDetailFixture>();
		mocks.getExpense.mockRejectedValue(
			new ApiError({
				status: 404,
				code: 'not_found',
				message: 'Not found.',
				fields: {}
			})
		);
		mocks.groupDetail.refreshDetail.mockReturnValue(detail.promise);
		render(EditExpensePage);

		await waitFor(() => expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce());
		expect(
			screen.queryByText('This record isn’t available.')
		).not.toBeInTheDocument();

		detail.resolve(groupDetailFixture);
		expect(
			await screen.findByText('This record isn’t available.')
		).toBeInTheDocument();
	});

	it('fails safely when persisted payer or split members are not active', async () => {
		mocks.getExpense.mockResolvedValue({
			...expenseListFixture.expenses[0],
			splits: [
				{
					userId: 'removed-member',
					amountCents: expenseListFixture.expenses[0].amountCents
				}
			]
		});

		render(EditExpensePage);

		expect(
			await screen.findByText('Settled couldn’t load this expense.')
		).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
	});

	it('does not race a terminal refresh error with group Activity navigation', async () => {
		mocks.accounting.refreshAfterExpenseMutation.mockImplementation(
			async () => {
				const error = new ApiError({
					status: 404,
					code: 'not_found',
					message: 'Not found.',
					fields: {}
				});
				mocks.accounting.expenses.error = error;
				throw error;
			}
		);
		render(EditExpensePage);
		await screen.findByLabelText('Description');

		await fireEvent.input(screen.getByLabelText('Description'), {
			target: { value: 'Updated groceries' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));

		await waitFor(() => expect(mocks.replaceExpense).toHaveBeenCalledOnce());
		await waitFor(() =>
			expect(mocks.accounting.refreshAfterExpenseMutation).toHaveBeenCalledOnce()
		);
		expect(mocks.goto).not.toHaveBeenCalled();
		expect(mocks.toastSuccess).toHaveBeenCalledWith('Changes saved');
		expect(mocks.toastError).toHaveBeenCalledWith(
			expect.stringContaining('some group information')
		);
	});
});

function deferred<T>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
} {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((complete) => {
		resolve = complete;
	});
	return { promise, resolve };
}
