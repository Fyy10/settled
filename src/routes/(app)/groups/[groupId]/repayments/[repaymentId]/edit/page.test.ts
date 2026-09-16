import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError } from '$lib/api/errors';
import {
	groupDetailFixture,
	repaymentListFixture
} from '../../../../../../../tests/fixtures/api-contract';

import EditPaymentPage from './+page.svelte';

type RouteParams = {
	groupId: string;
	repaymentId: string;
};

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	getRepayment: vi.fn(),
	replaceRepayment: vi.fn(),
	deleteRepayment: vi.fn(),
	toastSuccess: vi.fn(),
	toastError: vi.fn(),
	page: {
		params: { groupId: '', repaymentId: '' }
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
vi.mock('$lib/api/repayments', () => ({
	getRepayment: mocks.getRepayment,
	replaceRepayment: mocks.replaceRepayment,
	deleteRepayment: mocks.deleteRepayment
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
	mocks.getRepayment
		.mockReset()
		.mockResolvedValue(repaymentListFixture.repayments[0]);
	mocks.replaceRepayment
		.mockReset()
		.mockResolvedValue(repaymentListFixture.repayments[0]);
	mocks.deleteRepayment.mockReset().mockResolvedValue(undefined);
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
	mocks.setPageParams({
		groupId: groupDetailFixture.group.id,
		repaymentId: repaymentListFixture.repayments[0].id
	});
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
});

afterEach(async () => {
	cleanup();
	await waitFor(() => {
		expect(document.body.style.overflow).toBe('');
		expect(document.body.style.pointerEvents).toBe('');
	});
});

describe('edit payment route', () => {
	it('starts the target GET while parent group detail is still loading and aborts on teardown', async () => {
		const target = deferred<typeof repaymentListFixture.repayments[0]>();
		mocks.groupDetail.status = 'loading';
		mocks.groupDetail.group = null;
		mocks.getRepayment.mockReturnValue(target.promise);

		const result = render(EditPaymentPage);

		await waitFor(() => expect(mocks.getRepayment).toHaveBeenCalledOnce());
		expect(mocks.getRepayment).toHaveBeenCalledWith(
			groupDetailFixture.group.id,
			repaymentListFixture.repayments[0].id,
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
		const signal = mocks.getRepayment.mock.calls[0][2].signal;
		result.unmount();
		expect(signal.aborted).toBe(true);
	});

	it('opens the persisted direction, amount, date, and optional note', async () => {
		const { container } = render(EditPaymentPage);

		expect(await screen.findByLabelText('Paid by')).toHaveTextContent(
			'Bob'
		);
		expect(screen.getByLabelText('Paid to')).toHaveTextContent('Alice');
		expect(screen.getByLabelText('Amount')).toHaveValue('20.00');
		expect(screen.getByLabelText('Date')).toHaveValue('2026-06-30');
		expect(screen.getByLabelText('Note (optional)')).toHaveValue('Venmo');
		expect(container.querySelector('[data-slot="card"]')).toHaveClass(
			'overflow-visible'
		);
	});

	it('reinitializes repayment A after a pending B route is replaced by A', async () => {
		const pendingB =
			deferred<typeof repaymentListFixture.repayments[0]>();
		const repaymentA = repaymentListFixture.repayments[0];
		const repaymentBId =
			'00000000-0000-4000-8000-000000000099';
		mocks.getRepayment.mockImplementation(
			(_groupId: string, repaymentId: string) =>
				repaymentId === repaymentBId
					? pendingB.promise
					: Promise.resolve(repaymentA)
		);

		render(EditPaymentPage);
		expect(await screen.findByLabelText('Amount')).toHaveValue('20.00');

		mocks.setPageParams({
			groupId: groupDetailFixture.group.id,
			repaymentId: repaymentBId
		});
		await waitFor(() =>
			expect(mocks.getRepayment).toHaveBeenCalledWith(
				groupDetailFixture.group.id,
				repaymentBId,
				expect.objectContaining({ signal: expect.any(AbortSignal) })
			)
		);
		await waitFor(() =>
			expect(screen.queryByLabelText('Amount')).not.toBeInTheDocument()
		);
		const pendingSignal = mocks.getRepayment.mock.calls.find(
			(call) => call[1] === repaymentBId
		)?.[2].signal;

		mocks.setPageParams({
			groupId: groupDetailFixture.group.id,
			repaymentId: repaymentA.id
		});

		await waitFor(() => expect(mocks.getRepayment).toHaveBeenCalledTimes(3));
		expect(pendingSignal?.aborted).toBe(true);
		expect(await screen.findByLabelText('Amount')).toHaveValue('20.00');
	});

	it('revalidates group detail before showing an initial record 404', async () => {
		const detail = deferred<typeof groupDetailFixture>();
		mocks.getRepayment.mockRejectedValue(notFoundError());
		mocks.groupDetail.refreshDetail.mockReturnValue(detail.promise);
		render(EditPaymentPage);

		await waitFor(() =>
			expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce()
		);
		expect(
			screen.queryByText('This record isn’t available.')
		).not.toBeInTheDocument();

		detail.resolve(groupDetailFixture);
		expect(
			await screen.findByText('This record isn’t available.')
		).toBeInTheDocument();
	});

	it('lets the parent boundary own a hidden group instead of claiming a missing record', async () => {
		mocks.getRepayment.mockRejectedValue(notFoundError());
		mocks.groupDetail.refreshDetail.mockImplementation(async () => {
			mocks.groupDetail.status = 'hidden';
			mocks.groupDetail.group = null;
			return groupDetailFixture;
		});

		render(EditPaymentPage);

		await waitFor(() =>
			expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce()
		);
		expect(
			screen.queryByText('This record isn’t available.')
		).not.toBeInTheDocument();
		expect(screen.queryByLabelText('Paid by')).not.toBeInTheDocument();
	});

	it('keeps a PUT 404 retryable when re-GET proves the record still exists', async () => {
		mocks.getRepayment
			.mockResolvedValueOnce(repaymentListFixture.repayments[0])
			.mockResolvedValueOnce(repaymentListFixture.repayments[0]);
		mocks.replaceRepayment.mockRejectedValue(notFoundError());
		render(EditPaymentPage);
		await screen.findByLabelText('Note (optional)');
		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		expect(
			await screen.findByText(
				'Settled could not save this payment record. Check the form and try again.'
			)
		).toBeInTheDocument();
		expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce();
		expect(mocks.getRepayment).toHaveBeenCalledTimes(2);
		expect(screen.getByLabelText('Note (optional)')).toHaveValue('Cash');
		expect(
			screen.getByRole('button', { name: 'Save changes' })
		).toBeEnabled();
	});

	it('shows the local unavailable state when PUT 404 re-GET confirms deletion', async () => {
		mocks.getRepayment
			.mockResolvedValueOnce(repaymentListFixture.repayments[0])
			.mockRejectedValueOnce(notFoundError());
		mocks.replaceRepayment.mockRejectedValue(notFoundError());
		render(EditPaymentPage);
		await screen.findByLabelText('Note (optional)');
		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		expect(
			await screen.findByText('This record isn’t available.')
		).toBeInTheDocument();
		expect(screen.queryByLabelText('Note (optional)')).not.toBeInTheDocument();
	});

	it('saves, refreshes authoritative payment/balance data, and replaces history with balances', async () => {
		render(EditPaymentPage);
		await screen.findByLabelText('Note (optional)');
		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		await waitFor(() =>
			expect(mocks.replaceRepayment).toHaveBeenCalledOnce()
		);
		expect(mocks.replaceRepayment).toHaveBeenCalledWith(
			groupDetailFixture.group.id,
			repaymentListFixture.repayments[0].id,
			expect.objectContaining({ note: 'Cash' }),
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
	});

	it('uses one shared coordinator so pending delete locks Save, then refreshes and replaces to Activity', async () => {
		const pending = deferred<void>();
		mocks.deleteRepayment.mockReturnValue(pending.promise);
		render(EditPaymentPage);
		await screen.findByLabelText('Amount');

		await fireEvent.click(
			screen.getByRole('button', {
				name: 'Delete payment record'
			})
		);
		const dialog = await screen.findByRole('alertdialog');
		await fireEvent.click(
			within(dialog).getByRole('button', {
				name: 'Delete payment record'
			})
		);

		expect(
			screen.getByRole('button', { name: 'Save changes' })
		).toBeDisabled();
		expect(mocks.deleteRepayment).toHaveBeenCalledOnce();
		pending.resolve();
		await waitFor(() =>
			expect(
				mocks.accounting.refreshAfterRepaymentMutation
			).toHaveBeenCalledOnce()
		);
		expect(mocks.goto).toHaveBeenCalledWith(
			`/groups/${groupDetailFixture.group.id}?view=activity`,
			{ replaceState: true }
		);
		expect(mocks.toastSuccess).toHaveBeenCalledWith(
			'Payment record deleted'
		);
	});

	it('classifies DELETE 404 as a local missing record when group access remains ready', async () => {
		mocks.deleteRepayment.mockRejectedValue(notFoundError());
		render(EditPaymentPage);
		await screen.findByLabelText('Amount');

		await fireEvent.click(
			screen.getByRole('button', {
				name: 'Delete payment record'
			})
		);
		const dialog = await screen.findByRole('alertdialog');
		await fireEvent.click(
			within(dialog).getByRole('button', {
				name: 'Delete payment record'
			})
		);

		expect(
			await screen.findByText('This record isn’t available.')
		).toBeInTheDocument();
		expect(mocks.groupDetail.refreshDetail).toHaveBeenCalledOnce();
		expect(mocks.getRepayment).toHaveBeenCalledOnce();
		expect(screen.queryByLabelText('Amount')).not.toBeInTheDocument();
		expect(mocks.goto).not.toHaveBeenCalled();
		expect(mocks.toastSuccess).not.toHaveBeenCalled();
	});

	it('lets the parent boundary own DELETE 404 when group access becomes hidden', async () => {
		mocks.deleteRepayment.mockRejectedValue(notFoundError());
		mocks.groupDetail.refreshDetail.mockImplementation(async () => {
			mocks.groupDetail.status = 'hidden';
			mocks.groupDetail.group = null;
			return groupDetailFixture;
		});
		render(EditPaymentPage);
		await screen.findByLabelText('Amount');

		await fireEvent.click(
			screen.getByRole('button', {
				name: 'Delete payment record'
			})
		);
		const dialog = await screen.findByRole('alertdialog');
		await fireEvent.click(
			within(dialog).getByRole('button', {
				name: 'Delete payment record'
			})
		);

		await waitFor(() =>
			expect(
				mocks.groupDetail.refreshDetail
			).toHaveBeenCalledOnce()
		);
		await waitFor(() =>
			expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
		);
		expect(
			screen.queryByText('This record isn’t available.')
		).not.toBeInTheDocument();
		expect(mocks.getRepayment).toHaveBeenCalledOnce();
		expect(mocks.goto).not.toHaveBeenCalled();
		expect(mocks.toastSuccess).not.toHaveBeenCalled();
	});

	it('does not race a terminal repayment refresh with balances navigation', async () => {
		mocks.accounting.refreshAfterRepaymentMutation.mockImplementation(
			async () => {
				const error = notFoundError();
				mocks.accounting.repayments.error = error;
				throw error;
			}
		);
		render(EditPaymentPage);
		await screen.findByLabelText('Note (optional)');
		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		await waitFor(() =>
			expect(
				mocks.accounting.refreshAfterRepaymentMutation
			).toHaveBeenCalledOnce()
		);
		expect(mocks.goto).not.toHaveBeenCalled();
		expect(mocks.toastError).toHaveBeenCalledWith(
			expect.stringContaining('balances could not be fully refreshed')
		);
	});
});

function notFoundError(): ApiError {
	return new ApiError({
		status: 404,
		code: 'not_found',
		message: 'Not found.',
		fields: {}
	});
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
