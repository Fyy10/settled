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
	currentUserFixture,
	expenseListFixture,
	groupDetailFixture
} from '../../../../../../tests/fixtures/api-contract';

import AddExpensePage from './+page.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	createExpense: vi.fn(),
	refreshAccounting: vi.fn(),
	refreshDetail: vi.fn(),
	toastSuccess: vi.fn(),
	toastError: vi.fn(),
	auth: {
		current: {
			status: 'authenticated',
			user: {
				id: '',
				email: '',
				displayName: '',
				createdAt: '',
				updatedAt: ''
			}
		}
	},
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
vi.mock('$lib/api/expenses', () => ({
	createExpense: mocks.createExpense
}));
vi.mock('$lib/state/auth.svelte', () => ({
	authState: mocks.auth
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
	mocks.createExpense
		.mockReset()
		.mockResolvedValue(expenseListFixture.expenses[0]);
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
	mocks.auth.current = {
		status: 'authenticated',
		user: { ...currentUserFixture.user }
	};
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

describe('add expense route', () => {
	it('defaults to the authenticated member, local date, equal mode, and every member in API order', async () => {
		const { container } = render(AddExpensePage);

		expect(await screen.findByText('Review split')).toBeInTheDocument();
		expect(screen.getByLabelText('Paid by')).toHaveTextContent('Alice');
		expect(
			(screen.getByLabelText('Date') as HTMLInputElement).value
		).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		expect(screen.getByRole('radio', { name: 'Equal' })).toBeChecked();
		expect(screen.getAllByRole('checkbox')).toHaveLength(2);
		expect(screen.getAllByRole('checkbox').every((item) => item.getAttribute('aria-checked') === 'true')).toBe(
			true
		);
		expect(container.querySelector('[data-slot="card"]')).toHaveClass(
			'overflow-visible'
		);
	});

	it('posts the equal draft, waits for accounting refresh, and replaces history with Activity', async () => {
		render(AddExpensePage);
		await screen.findByText('Review split');
		await fireEvent.input(screen.getByLabelText('Description'), {
			target: { value: 'Shared dinner' }
		});
		await fireEvent.input(screen.getByLabelText('Amount'), {
			target: { value: '54.00' }
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Add expense' }));

		await waitFor(() => expect(mocks.createExpense).toHaveBeenCalledOnce());
		expect(mocks.createExpense).toHaveBeenCalledWith(
			groupDetailFixture.group.id,
			expect.objectContaining({
				paidByUserId: currentUserFixture.user.id,
				description: 'Shared dinner',
				amountCents: 5_400,
				splitMode: 'equal',
				participantUserIds: groupDetailFixture.members.map(
					(member) => member.userId
				)
			}),
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
		await waitFor(() =>
			expect(mocks.accounting.refreshAfterExpenseMutation).toHaveBeenCalledOnce()
		);
		expect(mocks.goto).toHaveBeenCalledWith(
			`/groups/${groupDetailFixture.group.id}?view=activity`,
			{ replaceState: true }
		);
		expect(mocks.toastSuccess).toHaveBeenCalledWith('Expense added');
	});

	it('keeps a create 404 retryable when group detail still exists', async () => {
		mocks.createExpense.mockRejectedValue(
			new ApiError({
				status: 404,
				code: 'not_found',
				message: 'Not found.',
				fields: {}
			})
		);
		render(AddExpensePage);
		await screen.findByText('Review split');
		await fireEvent.input(screen.getByLabelText('Description'), {
			target: { value: 'Shared dinner' }
		});
		await fireEvent.input(screen.getByLabelText('Amount'), {
			target: { value: '54.00' }
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Add expense' }));

		expect(
			await screen.findByText(
				'Settled could not save this expense. Check the form and try again.'
			)
		).toBeInTheDocument();
		expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce();
		expect(screen.getByRole('button', { name: 'Add expense' })).toBeEnabled();
	});

	it('fails safely instead of attributing a payer when the authenticated user is absent', async () => {
		mocks.auth.current = {
			status: 'authenticated',
			user: {
				...currentUserFixture.user,
				id: 'not-a-member'
			}
		};

		render(AddExpensePage);

		expect(
			await screen.findByText(
				'Your active account is not a member of this group. Reload before adding an expense.'
			)
		).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Add expense' })).not.toBeInTheDocument();
	});
});
