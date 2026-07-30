import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import type { BeforeNavigate } from '@sveltejs/kit';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { networkError } from '$lib/api/errors';
import { MutationCoordinator } from '$lib/state/mutation-coordinator.svelte';
import type { ExpenseDraft } from '$lib/utils/expense-draft';
import {
	expenseListFixture,
	groupDetailFixture
} from '../../../tests/fixtures/api-contract';

import DeleteExpenseDialog from './delete-expense-dialog.svelte';
import ExpenseForm from './expense-form.svelte';

type NavigationCallback = (navigation: BeforeNavigate) => void;

const mocks = vi.hoisted(() => ({
	deleteExpense: vi.fn(),
	callbacks: [] as NavigationCallback[],
	toastSuccess: vi.fn(),
	toastError: vi.fn()
}));

vi.mock('$lib/api/expenses', () => ({
	deleteExpense: mocks.deleteExpense
}));
vi.mock('$app/navigation', () => ({
	beforeNavigate: (callback: NavigationCallback) => {
		mocks.callbacks.push(callback);
	}
}));
vi.mock('svelte-sonner', () => ({
	toast: {
		success: mocks.toastSuccess,
		error: mocks.toastError
	}
}));

beforeEach(() => {
	mocks.deleteExpense.mockReset().mockResolvedValue(undefined);
	mocks.callbacks.length = 0;
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
	vi.stubGlobal('confirm', vi.fn());
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('DeleteExpenseDialog', () => {
	it('commits once, refreshes, and reports the exact delete toast', async () => {
		const onCommitted = vi.fn().mockResolvedValue({
			refresh: 'succeeded',
			navigation: 'succeeded'
		});
		renderDialog({ onCommitted });

		await confirmDelete();

		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
		expect(mocks.deleteExpense).toHaveBeenCalledWith(
			'group-id',
			'expense-id',
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
		expect(mocks.toastSuccess).toHaveBeenCalledWith('Expense deleted');
		expect(screen.getByRole('button', { name: 'Delete expense' })).toBeDisabled();
	});

	it('keeps an ambiguous delete closeable, focusable, reopenable, and non-repeatable', async () => {
		mocks.deleteExpense.mockRejectedValue(
			networkError(new TypeError('connection lost'))
		);
		renderDialog();

		await confirmDelete();

		const dialog = await screen.findByRole('alertdialog');
		expect(
			within(dialog).getByText(
				'Settled could not confirm the result. Check group activity before trying again.'
			)
		).toBeInTheDocument();
		expect(
			within(dialog).getByRole('link', { name: 'Return to group activity' })
		).toHaveAttribute('href', '/groups/group-id?view=activity');

		await fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);
		const recoveryTrigger = screen.getByRole('button', {
			name: 'Review delete result'
		});
		expect(recoveryTrigger).toHaveFocus();

		await fireEvent.click(recoveryTrigger);
		const reopened = await screen.findByRole('alertdialog');
		expect(
			within(reopened).getByRole('button', { name: 'Delete expense' })
		).toBeDisabled();
		expect(mocks.deleteExpense).toHaveBeenCalledOnce();
	});

	it('marks a dirty edit clean before committed navigation', async () => {
		const mutation = new MutationCoordinator<'save' | 'delete'>();
		renderEditForm(mutation);
		const onCommitted = vi.fn(async () => {
			const nav = navigation('/groups/group-id?view=activity');
			expect(mocks.callbacks).toHaveLength(1);
			mocks.callbacks[0](nav.value);
			expect(nav.cancel).not.toHaveBeenCalled();
			expect(globalThis.confirm).not.toHaveBeenCalled();
			return {
				refresh: 'succeeded' as const,
				navigation: 'succeeded' as const
			};
		});
		renderDialog({ mutation, onCommitted });

		await fireEvent.input(screen.getByLabelText('Description'), {
			target: { value: 'Changed before delete' }
		});
		await confirmDelete();

		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
		expect(mutation.phase).toBe('committed');
	});

	it('prevents Delete from dispatching while Save is in flight', async () => {
		const mutation = new MutationCoordinator<'save' | 'delete'>();
		const save = deferred<typeof expenseListFixture.expenses[0]>();
		renderEditForm(mutation, vi.fn(() => save.promise));
		renderDialog({ mutation });

		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));

		expect(screen.getByRole('button', { name: 'Delete expense' })).toBeDisabled();
		expect(mocks.deleteExpense).not.toHaveBeenCalled();
	});

	it('keeps a navigation-failure recovery link reachable after closing', async () => {
		renderDialog({
			onCommitted: vi.fn().mockResolvedValue({
				refresh: 'succeeded',
				navigation: 'failed'
			})
		});

		await confirmDelete();

		const dialog = await screen.findByRole('alertdialog');
		expect(
			within(dialog).getByRole('link', { name: 'Return to group activity' })
		).toBeInTheDocument();
		await fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
		const recoveryTrigger = await screen.findByRole('button', {
			name: 'Review delete result'
		});
		expect(recoveryTrigger).toHaveFocus();
		await fireEvent.click(recoveryTrigger);
		expect(
			within(await screen.findByRole('alertdialog')).getByRole('link', {
				name: 'Return to group activity'
			})
		).toBeInTheDocument();
	});
});

function renderDialog({
	mutation = new MutationCoordinator<'save' | 'delete'>(),
	onCommitted = vi.fn().mockResolvedValue({
		refresh: 'succeeded',
		navigation: 'succeeded'
	})
}: {
	mutation?: MutationCoordinator<'save' | 'delete'>;
	onCommitted?: () => Promise<{
		refresh: 'succeeded' | 'failed';
		navigation: 'succeeded' | 'failed';
	}>;
} = {}) {
	return render(DeleteExpenseDialog, {
		groupId: 'group-id',
		expenseId: 'expense-id',
		activityHref: '/groups/group-id?view=activity',
		mutation,
		onCommitted,
		onNotFound: vi.fn().mockResolvedValue('record')
	});
}

function renderEditForm(
	mutation: MutationCoordinator<'save' | 'delete'>,
	save = vi.fn().mockResolvedValue(expenseListFixture.expenses[0])
) {
	return render(ExpenseForm, {
		mode: 'edit',
		members: groupDetailFixture.members,
		initialDraft: validDraft(),
		cancelHref: '/groups/group-id?view=activity',
		mutation,
		save,
		onCommitted: vi.fn().mockResolvedValue({
			refresh: 'succeeded',
			navigation: 'succeeded'
		}),
		onNotFound: vi.fn().mockResolvedValue('record')
	});
}

async function confirmDelete(): Promise<void> {
	await fireEvent.click(screen.getByRole('button', { name: 'Delete expense' }));
	const dialog = await screen.findByRole('alertdialog');
	await fireEvent.click(
		within(dialog).getByRole('button', { name: 'Delete expense' })
	);
}

function navigation(path: string): {
	value: BeforeNavigate;
	cancel: ReturnType<typeof vi.fn>;
} {
	const cancel = vi.fn();
	return {
		value: {
			type: 'goto',
			from: null,
			to: {
				params: {},
				route: { id: null },
				url: new URL(path, 'https://settled.example')
			},
			willUnload: false,
			complete: Promise.resolve(),
			delta: undefined,
			cancel
		},
		cancel
	};
}

function validDraft(): ExpenseDraft {
	return {
		description: 'Shared dinner',
		amount: '54.00',
		paidByUserId: groupDetailFixture.members[0].userId,
		expenseDate: '2026-06-30',
		splitMode: 'exact',
		participants: groupDetailFixture.members.map((member) => ({
			userId: member.userId,
			exactAmount: '27.00',
			seededExactAmount: '27.00',
			percentage: '',
			seededPercentage: ''
		}))
	};
}

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
