import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
	within
} from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { networkError } from '$lib/api/errors';
import { MutationCoordinator } from '$lib/state/mutation-coordinator.svelte';

import DeleteRepaymentDialog from './delete-repayment-dialog.svelte';

const mocks = vi.hoisted(() => ({
	deleteRepayment: vi.fn(),
	toastSuccess: vi.fn(),
	toastError: vi.fn()
}));

vi.mock('$lib/api/repayments', () => ({
	deleteRepayment: mocks.deleteRepayment
}));
vi.mock('svelte-sonner', () => ({
	toast: {
		success: mocks.toastSuccess,
		error: mocks.toastError
	}
}));

beforeEach(() => {
	mocks.deleteRepayment.mockReset().mockResolvedValue(undefined);
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
});

afterEach(async () => {
	cleanup();
	await waitFor(() => {
		expect(document.body.style.overflow).toBe('');
		expect(document.body.style.pointerEvents).toBe('');
	});
});

describe('DeleteRepaymentDialog', () => {
	it('deletes once, waits for authoritative follow-up, and reports the exact success', async () => {
		const onCommitted = vi.fn().mockResolvedValue({
			refresh: 'succeeded',
			navigation: 'succeeded'
		});
		renderDialog({ onCommitted });

		await openAndConfirm();

		await waitFor(() =>
			expect(mocks.deleteRepayment).toHaveBeenCalledOnce()
		);
		expect(mocks.deleteRepayment).toHaveBeenCalledWith(
			'group/id',
			'repayment/id',
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
		expect(mocks.toastSuccess).toHaveBeenCalledWith(
			'Payment record deleted'
		);
	});

	it('keeps the dialog pending and prevents duplicate delete requests', async () => {
		const pending = deferred<void>();
		mocks.deleteRepayment.mockReturnValue(pending.promise);
		const mutation = new MutationCoordinator<'save' | 'delete'>();
		const onCommitted = vi.fn().mockResolvedValue({
			refresh: 'succeeded',
			navigation: 'succeeded'
		});
		renderDialog({ mutation, onCommitted });

		await openAndConfirm();

		expect(mutation.phase).toBe('pending');
		expect(mutation.operation).toBe('delete');
		const dialog = screen.getByRole('alertdialog');
		expect(
			within(dialog).getByRole('button', {
				name: /Deleting payment record/
			})
		).toBeDisabled();
		expect(
			within(dialog).getByRole('button', { name: 'Cancel' })
		).toBeDisabled();
		await fireEvent.click(
			within(dialog).getByRole('button', {
				name: /Deleting payment record/
			})
		);
		expect(mocks.deleteRepayment).toHaveBeenCalledOnce();
		pending.resolve();
		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
	});

	it('treats a network response as ambiguous and keeps recovery visible', async () => {
		mocks.deleteRepayment.mockRejectedValue(
			networkError(new TypeError('connection lost'))
		);
		renderDialog();

		await openAndConfirm();

		expect(
			await screen.findByText(
				'Settled could not confirm the result. Check group activity before trying again.'
			)
		).toBeInTheDocument();
		expect(
			screen.getByRole('link', {
				name: 'Return to group activity'
			})
		).toHaveAttribute(
			'href',
			'/groups/group-id?view=activity'
		);
		expect(mocks.deleteRepayment).toHaveBeenCalledOnce();
	});

	it('allows review while offline but disables and describes confirmation', async () => {
		renderDialog({
			mutationDisabledReason:
				'Changes are unavailable while Settled is offline.'
		});

		await fireEvent.click(
			screen.getByRole('button', {
				name: 'Delete payment record'
			})
		);
		const dialog = await screen.findByRole('alertdialog');
		const confirm = within(dialog).getByRole('button', {
			name: 'Delete payment record'
		});
		expect(confirm).toBeDisabled();
		expect(confirm).toHaveAttribute(
			'aria-describedby',
			'repayment-delete-disabled'
		);
		expect(
			within(dialog)
				.getByText(
					'Changes are unavailable while Settled is offline.'
				)
				.closest('[role="alert"]')
		).toHaveAttribute('id', 'repayment-delete-disabled');
		expect(mocks.deleteRepayment).not.toHaveBeenCalled();
	});
});

function renderDialog({
	mutation = new MutationCoordinator<'save' | 'delete'>(),
	onCommitted = vi.fn().mockResolvedValue({
		refresh: 'succeeded',
		navigation: 'succeeded'
	}),
	mutationDisabledReason = null
}: {
	mutation?: MutationCoordinator<'save' | 'delete'>;
	onCommitted?: () => Promise<{
		refresh: 'succeeded' | 'failed';
		navigation: 'succeeded' | 'failed';
	}>;
	mutationDisabledReason?: string | null;
} = {}) {
	return render(DeleteRepaymentDialog, {
		groupId: 'group/id',
		repaymentId: 'repayment/id',
		activityHref: '/groups/group-id?view=activity',
		mutation,
		onCommitted,
		onNotFound: vi.fn().mockResolvedValue('record'),
		mutationDisabledReason
	});
}

async function openAndConfirm(): Promise<void> {
	await fireEvent.click(
		screen.getByRole('button', { name: 'Delete payment record' })
	);
	const dialog = await screen.findByRole('alertdialog');
	await fireEvent.click(
		within(dialog).getByRole('button', {
			name: 'Delete payment record'
		})
	);
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
