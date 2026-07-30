import {
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, networkError } from '$lib/api/errors';
import { GroupAccountingState } from '$lib/state/group-accounting.svelte';
import {
	expenseListFixture,
	groupDetailFixture,
	repaymentListFixture,
	settlementListFixture
} from '../../../../tests/fixtures/api-contract';

import GroupPage from './+page.svelte';

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	page: {
		url: new URL('https://settled.test/groups/group-id?view=balances')
	},
	context: {
		groupId: '',
		status: 'ready' as 'loading' | 'ready' | 'error' | 'hidden',
		group: null as (typeof groupDetailFixture)['group'] | null,
		members: [] as (typeof groupDetailFixture)['members'][number][],
		clear: vi.fn(),
		markHidden: vi.fn(),
		refreshDetail: vi.fn()
	},
	accounting: null as GroupAccountingState | null,
	removeGroupMember: vi.fn(),
	listExpenses: vi.fn(),
	listRepayments: vi.fn(),
	listSettlements: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$app/state', () => ({ page: mocks.page }));
vi.mock('$lib/api/expenses', () => ({ listExpenses: mocks.listExpenses }));
vi.mock('$lib/api/groups', () => ({
	removeGroupMember: mocks.removeGroupMember
}));
vi.mock('$lib/api/repayments', () => ({
	listRepayments: mocks.listRepayments
}));
vi.mock('$lib/api/settlements', () => ({
	listSettlements: mocks.listSettlements
}));
vi.mock('$lib/state/group-detail.svelte', () => ({
	useGroupDetailContext: () => mocks.context
}));
vi.mock('$lib/state/group-accounting.svelte', async (importOriginal) => {
	const actual =
		await importOriginal<typeof import('$lib/state/group-accounting.svelte')>();
	return {
		...actual,
		useGroupAccounting: () => mocks.accounting
	};
});
vi.mock('$lib/config/public', () => ({
	API_BASE_URL: 'http://localhost:8080'
}));

beforeEach(() => {
	mocks.goto.mockReset().mockResolvedValue(undefined);
	mocks.page.url = new URL(
		`https://settled.test/groups/${groupDetailFixture.group.id}?view=balances`
	);
	mocks.context.groupId = groupDetailFixture.group.id;
	mocks.context.status = 'ready';
	mocks.context.group = groupDetailFixture.group;
	mocks.context.members = [...groupDetailFixture.members];
	mocks.context.clear.mockReset();
	mocks.context.markHidden.mockReset();
	mocks.context.refreshDetail.mockReset().mockResolvedValue(groupDetailFixture);
	mocks.removeGroupMember.mockReset().mockResolvedValue(undefined);
	mocks.listExpenses.mockReset().mockResolvedValue(expenseListFixture);
	mocks.listRepayments.mockReset().mockResolvedValue(repaymentListFixture);
	mocks.listSettlements.mockReset().mockResolvedValue(settlementListFixture);
	mocks.accounting = new GroupAccountingState({
		expenses: mocks.listExpenses,
		repayments: mocks.listRepayments,
		settlements: mocks.listSettlements
	});
	mocks.accounting.activate(groupDetailFixture.group.id);
});

describe('group workspace loading and navigation', () => {
	it('starts all overview requests together and renders the responsive workspace', async () => {
		const expensePending = deferred();
		const repaymentPending = deferred();
		const settlementPending = deferred();
		mocks.listExpenses.mockReturnValue(expensePending.promise);
		mocks.listRepayments.mockReturnValue(repaymentPending.promise);
		mocks.listSettlements.mockReturnValue(settlementPending.promise);

		const { unmount } = render(GroupPage);

		await waitFor(() => {
			expect(mocks.listExpenses).toHaveBeenCalledOnce();
			expect(mocks.listRepayments).toHaveBeenCalledOnce();
			expect(mocks.listSettlements).toHaveBeenCalledOnce();
		});
		expect(screen.getByRole('heading', { level: 1, name: 'Lake Trip' })).toBeInTheDocument();
		expect(screen.getByRole('tablist', { name: 'Group views' })).toBeInTheDocument();
		expect(screen.getAllByRole('link', { name: 'Add expense' })).toHaveLength(2);
		expect(screen.getAllByRole('link', { name: 'Record payment' })).toHaveLength(2);

		const signals = [
			mocks.listExpenses.mock.calls[0][1].signal,
			mocks.listRepayments.mock.calls[0][1].signal,
			mocks.listSettlements.mock.calls[0][1].signal
		];
		unmount();
		expect(signals.every((signal) => !signal.aborted)).toBe(true);
		mocks.accounting?.dispose();
		expect(signals.every((signal) => signal.aborted)).toBe(true);
	});

	it('normalizes duplicate view parameters to one balances value', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?source=share&view=members&view=activity`
		);

		render(GroupPage);

		await waitFor(() => {
			expect(mocks.goto).toHaveBeenCalledWith(
				`/groups/${groupDetailFixture.group.id}?source=share&view=balances`,
				{
					replaceState: true,
					noScroll: true,
					keepFocus: true
				}
			);
		});
	});

	it('changes only the view query through keyboard-capable tabs', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?source=share&view=balances#ledger`
		);
		render(GroupPage);

		await fireEvent.click(screen.getByRole('tab', { name: 'Members' }));

		expect(mocks.goto).toHaveBeenCalledWith(
			`/groups/${groupDetailFixture.group.id}?source=share&view=members#ledger`,
			{
				replaceState: true,
				noScroll: true,
				keepFocus: true
			}
		);
	});
});

describe('independent group workspace failures', () => {
	it('keeps loaded payment activity when the expense request fails', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?view=activity`
		);
		mocks.listExpenses.mockRejectedValue(
			networkError(new TypeError('Failed to fetch'))
		);

		render(GroupPage);

		expect(
			await screen.findByText('Settled couldn’t load expenses.')
		).toBeInTheDocument();
		expect(screen.getByText('Bob paid Alice')).toBeInTheDocument();
		expect(screen.getAllByText('Recorded outside Settled')).toHaveLength(2);
		await fireEvent.click(screen.getByRole('tab', { name: 'Balances' }));
		expect(
			await screen.findByRole('heading', { level: 2, name: 'Balances' })
		).toBeInTheDocument();
		expect(mocks.context.markHidden).not.toHaveBeenCalled();
	});

	it('retries only the failed activity resource', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?view=activity`
		);
		mocks.listExpenses
			.mockRejectedValueOnce(networkError(new TypeError('Failed to fetch')))
			.mockResolvedValueOnce({ expenses: [], members: expenseListFixture.members });
		mocks.listRepayments.mockResolvedValue({
			repayments: [],
			members: repaymentListFixture.members
		});

		render(GroupPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Retry' })
		);

		expect(await screen.findByText('No activity yet')).toBeInTheDocument();
		expect(mocks.listExpenses).toHaveBeenCalledTimes(2);
		expect(mocks.listRepayments).toHaveBeenCalledOnce();
		expect(mocks.listSettlements).toHaveBeenCalledOnce();
	});

	it('exposes a hidden-group 404 to the route boundary through accounting state', async () => {
		mocks.listSettlements.mockRejectedValue(
			new ApiError({
				status: 404,
				code: 'not_found',
				message: 'Not found.',
				fields: {}
			})
		);

		render(GroupPage);

		await waitFor(() =>
			expect(mocks.accounting?.settlements.error).toMatchObject({
				status: 404,
				code: 'not_found'
			})
		);
		expect(mocks.context.markHidden).not.toHaveBeenCalled();
	});

	it('exposes a 401 to the global route/session boundary through accounting state', async () => {
		mocks.listRepayments.mockRejectedValue(
			new ApiError({
				status: 401,
				code: 'unauthorized',
				message: 'Authentication is required.',
				fields: {}
			})
		);

		render(GroupPage);

		await waitFor(() =>
			expect(mocks.accounting?.repayments.error).toMatchObject({
				status: 401,
				code: 'unauthorized'
			})
		);
		expect(mocks.context.clear).not.toHaveBeenCalled();
	});
});

describe('authoritative balances', () => {
	it('shows the documented empty state only when the backend returns no settlements', async () => {
		mocks.listSettlements.mockResolvedValue({
			settlements: [],
			members: settlementListFixture.members
		});

		render(GroupPage);

		expect(await screen.findByText('All settled')).toBeInTheDocument();
		expect(
			screen.getByText('There are no current balances in this group.')
		).toBeInTheDocument();
	});
});

describe('owner member removal revalidation', () => {
	it('treats detail 200 without the target as a proven concurrent removal', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?view=members`
		);
		mocks.removeGroupMember.mockRejectedValue(notFoundError());
		const withoutTarget = {
			...groupDetailFixture,
			group: { ...groupDetailFixture.group, memberCount: 1 },
			members: [groupDetailFixture.members[0]]
		};
		mocks.context.refreshDetail.mockResolvedValue(withoutTarget);
		render(GroupPage);
		await openBobRemoval();

		await fireEvent.click(
			within(screen.getByRole('alertdialog')).getByRole('button', {
				name: 'Remove member'
			})
		);

		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);
		expect(mocks.removeGroupMember).toHaveBeenCalledOnce();
		expect(mocks.context.refreshDetail).toHaveBeenCalledTimes(2);
		expect(mocks.listExpenses).toHaveBeenCalledTimes(2);
		expect(mocks.listRepayments).toHaveBeenCalledTimes(2);
		expect(mocks.listSettlements).toHaveBeenCalledTimes(2);
		await waitFor(() =>
			expect(screen.getByRole('heading', { name: 'Members' })).toHaveFocus()
		);
	});

	it('keeps the member and ordinary failure when detail 200 still contains the target', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?view=members`
		);
		mocks.removeGroupMember.mockRejectedValue(notFoundError());
		mocks.context.refreshDetail.mockResolvedValue(groupDetailFixture);
		render(GroupPage);
		await openBobRemoval();

		await fireEvent.click(
			within(screen.getByRole('alertdialog')).getByRole('button', {
				name: 'Remove member'
			})
		);

		expect(
			await screen.findByText('Settled could not remove this member. Try again.')
		).toBeInTheDocument();
		expect(screen.getByRole('alertdialog')).toBeInTheDocument();
		expect(mocks.context.refreshDetail).toHaveBeenCalledOnce();
		expect(mocks.listExpenses).toHaveBeenCalledOnce();
		expect(mocks.removeGroupMember).toHaveBeenCalledOnce();
	});

	it('promotes detail 404 to hidden group without retrying member DELETE', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?view=members`
		);
		mocks.removeGroupMember.mockRejectedValue(notFoundError());
		mocks.context.refreshDetail.mockImplementation(async () => {
			mocks.context.status = 'hidden';
			mocks.context.markHidden();
			throw notFoundError();
		});
		render(GroupPage);
		await openBobRemoval();

		await fireEvent.click(
			within(screen.getByRole('alertdialog')).getByRole('button', {
				name: 'Remove member'
			})
		);

		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);
		expect(mocks.context.markHidden).toHaveBeenCalledOnce();
		expect(mocks.removeGroupMember).toHaveBeenCalledOnce();
		expect(mocks.listExpenses).toHaveBeenCalledOnce();
	});

	it('reports only revalidation failure after a committed 204 and disables repeat removal', async () => {
		mocks.page.url = new URL(
			`https://settled.test/groups/${groupDetailFixture.group.id}?view=members`
		);
		mocks.context.refreshDetail.mockRejectedValue(
			networkError(new TypeError('Offline'))
		);
		mocks.listSettlements
			.mockResolvedValueOnce(settlementListFixture)
			.mockRejectedValueOnce(networkError(new TypeError('Offline')));
		render(GroupPage);
		await openBobRemoval();

		await fireEvent.click(
			within(screen.getByRole('alertdialog')).getByRole('button', {
				name: 'Remove member'
			})
		);

		expect(
			await screen.findByText(
				'The member was removed, but some group information could not be refreshed.'
			)
		).toBeInTheDocument();
		const trigger = screen.getByRole('button', { name: 'Remove member Bob' });
		expect(trigger).toBeDisabled();
		await fireEvent.click(trigger);
		expect(mocks.removeGroupMember).toHaveBeenCalledOnce();
	});
});

async function openBobRemoval(): Promise<void> {
	const trigger = await screen.findByRole('button', {
		name: 'Remove member Bob'
	});
	await fireEvent.click(trigger);
	await screen.findByRole('alertdialog');
}

function notFoundError(): ApiError {
	return new ApiError({
		status: 404,
		code: 'not_found',
		message: 'Not found.',
		fields: {}
	});
}

function deferred<T = unknown>(): {
	promise: Promise<T>;
	resolve: (value: T) => void;
} {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((complete) => {
		resolve = complete;
	});

	return { promise, resolve };
}
