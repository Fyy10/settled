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
	groupDetailFixture,
	repaymentListFixture,
	settlementListFixture
} from '../../../../../../tests/fixtures/api-contract';

import RecordPaymentPage from './+page.svelte';

type RouteParams = {
	groupId: string;
};

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	createRepayment: vi.fn(),
	toastSuccess: vi.fn(),
	toastError: vi.fn(),
	page: {
		params: { groupId: '' },
		url: new URL('https://settled.example/')
	},
	setPage: (_params: RouteParams, _url: URL): void => undefined,
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
		repayments: { error: null as unknown },
		settlements: { error: null as unknown },
		refreshAfterRepaymentMutation: vi.fn()
	}
}));

vi.mock('$app/navigation', () => ({
	goto: mocks.goto,
	beforeNavigate: mocks.beforeNavigate
}));
vi.mock('$app/state', async () => {
	const { createSubscriber } = await import('svelte/reactivity');
	let updateParams = (): void => undefined;
	let updateUrl = (): void => undefined;
	const subscribeParams = createSubscriber((notify) => {
		updateParams = notify;
		return () => {
			updateParams = (): void => undefined;
		};
	});
	const subscribeUrl = createSubscriber((notify) => {
		updateUrl = notify;
		return () => {
			updateUrl = (): void => undefined;
		};
	});

	mocks.setPage = (params, url) => {
		mocks.page.params = params;
		mocks.page.url = url;
		updateParams();
		updateUrl();
	};

	return {
		page: {
			get params() {
				subscribeParams();
				return mocks.page.params;
			},
			get url() {
				subscribeUrl();
				return mocks.page.url;
			}
		}
	};
});
vi.mock('$lib/api/repayments', () => ({
	createRepayment: mocks.createRepayment
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
	mocks.createRepayment
		.mockReset()
		.mockResolvedValue(repaymentListFixture.repayments[0]);
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
	mocks.groupDetail.refreshDetail
		.mockReset()
		.mockResolvedValue(groupDetailFixture);
	mocks.accounting.repayments.error = null;
	mocks.accounting.settlements.error = null;
	mocks.accounting.refreshAfterRepaymentMutation
		.mockReset()
		.mockResolvedValue(undefined);
	setPageUrl('');
});

afterEach(cleanup);

describe('record payment route', () => {
	it('defaults to the authenticated member and local date without trusting absent hints', async () => {
		const { container } = render(RecordPaymentPage);

		expect(
			await screen.findByText(
				'Settled records a payment made outside the app. It does not send money.'
			)
		).toBeInTheDocument();
		expect(screen.getByLabelText('Paid by')).toHaveTextContent('Alice');
		expect(screen.getByLabelText('Paid to')).toHaveTextContent(
			'Choose who received payment'
		);
		expect(screen.getByLabelText('Amount')).toHaveValue('');
		expect(
			(screen.getByLabelText('Date') as HTMLInputElement).value
		).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		expect(container.querySelector('[data-slot="card"]')).toHaveClass(
			'overflow-visible'
		);
	});

	it('prefills the exact directional settlement hint after active-member validation', async () => {
		setPageUrl(settlementQuery());
		render(RecordPaymentPage);

		expect(await screen.findByLabelText('Paid by')).toHaveTextContent(
			'Bob'
		);
		expect(screen.getByLabelText('Paid to')).toHaveTextContent('Alice');
		expect(screen.getByLabelText('Amount')).toHaveValue('20.00');
	});

	it('reinitializes when only the settlement query changes', async () => {
		setPageUrl(settlementQuery());
		render(RecordPaymentPage);
		expect(await screen.findByLabelText('Amount')).toHaveValue('20.00');
		await fireEvent.input(screen.getByLabelText('Amount'), {
			target: { value: '99.00' }
		});

		const query = new URLSearchParams({
			from: currentUserFixture.user.id,
			to: groupDetailFixture.members[1].userId,
			amountCents: '500'
		});
		setPageUrl(`?${query.toString()}`);

		await waitFor(() =>
			expect(screen.getByLabelText('Paid by')).toHaveTextContent('Alice')
		);
		expect(screen.getByLabelText('Paid to')).toHaveTextContent('Bob');
		expect(screen.getByLabelText('Amount')).toHaveValue('5.00');
	});

	it('never combines a new route identity with the previous group member list', () => {
		mocks.setPage(
			{ groupId: 'different-group' },
			new URL(
				`https://settled.example/groups/different-group/repayments/new${settlementQuery()}`
			)
		);

		render(RecordPaymentPage);

		expect(screen.queryByLabelText('Paid by')).not.toBeInTheDocument();
		expect(
			screen.queryByText(
				'Settled records a payment made outside the app. It does not send money.'
			)
		).not.toBeInTheDocument();
	});

	it('posts the reviewed prefill, refreshes repayments and settlements, then replaces history with balances', async () => {
		setPageUrl(settlementQuery());
		render(RecordPaymentPage);
		await screen.findByLabelText('Amount');

		await fireEvent.click(
			screen.getByRole('button', { name: 'Record payment' })
		);

		await waitFor(() =>
			expect(mocks.createRepayment).toHaveBeenCalledOnce()
		);
		const settlement = settlementListFixture.settlements[0];
		expect(mocks.createRepayment).toHaveBeenCalledWith(
			groupDetailFixture.group.id,
			expect.objectContaining({
				fromUserId: settlement.fromUserId,
				toUserId: settlement.toUserId,
				amountCents: settlement.amountCents,
				note: null,
				repaymentDate: expect.stringMatching(
					/^\d{4}-\d{2}-\d{2}$/
				)
			}),
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
		await waitFor(() =>
			expect(
				mocks.accounting.refreshAfterRepaymentMutation
			).toHaveBeenCalledOnce()
		);
		expect(mocks.goto).toHaveBeenCalledWith(
			`/groups/${groupDetailFixture.group.id}?view=balances`,
			{ replaceState: true }
		);
		expect(mocks.toastSuccess).toHaveBeenCalledWith(
			'Payment recorded'
		);
	});

	it('keeps a POST 404 retryable after refreshing an accessible group and retains the draft', async () => {
		setPageUrl(settlementQuery());
		mocks.createRepayment.mockRejectedValue(notFoundError());
		render(RecordPaymentPage);
		await screen.findByLabelText('Amount');
		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Record payment' })
		);

		expect(
			await screen.findByText(
				'Settled could not save this payment record. Check the form and try again.'
			)
		).toBeInTheDocument();
		expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce();
		expect(screen.getByLabelText('Note (optional)')).toHaveValue('Cash');
		expect(
			screen.getByRole('button', { name: 'Record payment' })
		).toBeEnabled();
	});

	it('revalidates refreshed members after POST 404 and focuses a stale sender without clearing the draft', async () => {
		setPageUrl(settlementQuery());
		mocks.createRepayment.mockRejectedValue(notFoundError());
		const activeMembers = mocks.groupDetail.members;
		mocks.groupDetail.refreshDetail.mockImplementation(async () => {
			activeMembers.splice(1, 1);
			return {
				...groupDetailFixture,
				members: [...activeMembers]
			};
		});
		render(RecordPaymentPage);
		await screen.findByLabelText('Amount');
		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Record payment' })
		);

		expect(
			await screen.findByText(
				'Choose an active group member who paid.'
			)
		).toHaveAttribute('id', 'repayment-from-error');
		expect(screen.getByLabelText('Paid by')).toHaveFocus();
		expect(screen.getByLabelText('Note (optional)')).toHaveValue('Cash');
		expect(
			screen.queryByText(
				'Settled could not save this payment record. Check the form and try again.'
			)
		).not.toBeInTheDocument();
	});

	it('does not race a terminal refresh error with balances navigation', async () => {
		setPageUrl(settlementQuery());
		mocks.accounting.refreshAfterRepaymentMutation.mockImplementation(
			async () => {
				const error = notFoundError();
				mocks.accounting.repayments.error = error;
				throw error;
			}
		);
		render(RecordPaymentPage);
		await screen.findByLabelText('Amount');

		await fireEvent.click(
			screen.getByRole('button', { name: 'Record payment' })
		);

		await waitFor(() =>
			expect(
				mocks.accounting.refreshAfterRepaymentMutation
			).toHaveBeenCalledOnce()
		);
		expect(mocks.goto).not.toHaveBeenCalled();
		expect(mocks.toastSuccess).toHaveBeenCalledWith(
			'Payment recorded'
		);
		expect(mocks.toastError).toHaveBeenCalledWith(
			expect.stringContaining('balances could not be fully refreshed')
		);
	});

	it('fails safely when the authenticated account is absent from active members', async () => {
		mocks.auth.current = {
			status: 'authenticated',
			user: { ...currentUserFixture.user, id: 'not-a-member' }
		};

		render(RecordPaymentPage);

		expect(
			await screen.findByText(
				'Your active account is not a member of this group. Reload before recording a payment.'
			)
		).toBeInTheDocument();
		expect(
			screen.queryByRole('button', { name: 'Record payment' })
		).not.toBeInTheDocument();
	});
});

function setPageUrl(search: string): void {
	const groupId = groupDetailFixture.group.id;
	mocks.setPage(
		{ groupId },
		new URL(
			`https://settled.example/groups/${groupId}/repayments/new${search}`
		)
	);
}

function settlementQuery(): string {
	const settlement = settlementListFixture.settlements[0];
	return `?${new URLSearchParams({
		from: settlement.fromUserId,
		to: settlement.toUserId,
		amountCents: String(settlement.amountCents)
	}).toString()}`;
}

function notFoundError(): ApiError {
	return new ApiError({
		status: 404,
		code: 'not_found',
		message: 'Not found.',
		fields: {}
	});
}
