import { withNetworkState } from '../../../tests/network';
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor
} from '@testing-library/svelte';
import type { BeforeNavigate } from '@sveltejs/kit';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, networkError } from '$lib/api/errors';
import type {
	Repayment,
	ReplaceRepaymentInput
} from '$lib/api/types';
import { MutationCoordinator } from '$lib/state/mutation-coordinator.svelte';
import type { CommittedMutationResult } from '$lib/utils/committed-mutation';
import type { RepaymentDraft } from '$lib/utils/repayment-draft';
import {
	groupDetailFixture,
	repaymentListFixture
} from '../../../tests/fixtures/api-contract';

import RepaymentForm from './repayment-form.svelte';

type NavigationCallback = (navigation: BeforeNavigate) => void;
type SaveRepayment = (
	input: ReplaceRepaymentInput,
	options: { signal: AbortSignal }
) => Promise<Repayment>;
type FinishRepayment = (
	repayment: Repayment
) => Promise<CommittedMutationResult>;

const mocks = vi.hoisted(() => ({
	callbacks: [] as NavigationCallback[],
	toastSuccess: vi.fn(),
	toastError: vi.fn()
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
	mocks.callbacks.length = 0;
	mocks.toastSuccess.mockReset();
	mocks.toastError.mockReset();
	vi.stubGlobal('confirm', vi.fn());
});

afterEach(async () => {
	cleanup();
	await waitFor(() => {
		expect(document.body.style.overflow).toBe('');
		expect(document.body.style.pointerEvents).toBe('');
	});
	vi.unstubAllGlobals();
});

describe('RepaymentForm', () => {
	it('retains editable entries and blocks direct submission while offline', async () => {
		await withNetworkState(async (setOnline) => {
			const save = vi.fn();
			renderForm({ save });
			const input = screen.getByLabelText('Note (optional)');
			await fireEvent.input(input, { target: { value: 'Offline payment' } });
			await setOnline(false);
			const button = screen.getAllByRole('button', { name: 'Save changes' }).at(-1)!;
			expect(button).toBeDisabled();
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.submit(input.closest('form')!);
			expect(save).not.toHaveBeenCalled();
			expect(input).toHaveValue('Offline payment');
			expect(input).toBeEnabled();
			await setOnline(true);
			expect(input).toHaveValue('Offline payment');
			expect(save).not.toHaveBeenCalled();
		});
	});

	it('always explains the off-app record and allows another member as sender', () => {
		const { container } = renderForm();

		expect(
			screen.getByText(
				'Settled records a payment made outside the app. It does not send money.'
			)
		).toBeInTheDocument();
		expect(screen.getByLabelText('Paid by')).toHaveTextContent('Bob');
		expect(screen.getByLabelText('Paid to')).toHaveTextContent('Alice');
		expect(screen.getByLabelText('Note (optional)')).not.toHaveAttribute(
			'maxlength'
		);
		expect(
			Array.from(container.querySelectorAll('label')).map(
				(label) => label.textContent?.trim()
			)
		).toEqual([
			'Paid by',
			'Paid to',
			'Amount',
			'Date',
			'Note (optional)'
		]);
	});

	it('disables the current recipient in the sender choices', async () => {
		renderForm();

		const trigger = screen.getByLabelText('Paid by');
		await fireEvent.keyDown(trigger, { key: 'ArrowDown' });
		const aliceOption = await screen.findByRole('option', {
			name: 'Alice'
		});
		const bobOption = screen.getByRole('option', { name: 'Bob' });
		expect(aliceOption).toHaveAttribute('data-disabled');
		expect(bobOption).not.toHaveAttribute('data-disabled');
		await fireEvent.keyDown(aliceOption, { key: 'Escape' });
		await waitFor(() =>
			expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
		);
	});

	it('swaps a two-member payment direction and saves the corrected participants', async () => {
		const initialDraft = validDraft();
		const save = vi.fn().mockResolvedValue({
			...repaymentListFixture.repayments[0],
			fromUserId: initialDraft.toUserId,
			toUserId: initialDraft.fromUserId
		});
		renderForm({ initialDraft, save });

		await fireEvent.click(screen.getByRole('button', { name: 'Swap people' }));

		expect(screen.getByLabelText('Paid by')).toHaveTextContent('Alice');
		expect(screen.getByLabelText('Paid to')).toHaveTextContent('Bob');
		expect(save).not.toHaveBeenCalled();
		const dirtyNavigation = navigation('/groups/group-id?view=activity');
		registeredCallback()(dirtyNavigation.value);
		expect(dirtyNavigation.cancel).toHaveBeenCalledOnce();

		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));

		expect(save).toHaveBeenCalledExactlyOnceWith(
			{
				fromUserId: initialDraft.toUserId,
				toUserId: initialDraft.fromUserId,
				amountCents: 2000,
				repaymentDate: initialDraft.repaymentDate,
				note: initialDraft.note
			},
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
	});

	it('retains same-user validation and focuses the recipient', async () => {
		const draft = validDraft();
		draft.toUserId = draft.fromUserId;
		const save = vi.fn();
		renderForm({ initialDraft: draft, save });

		const button = screen.getByRole('button', {
			name: 'Save changes'
		});
		expect(button).toBeEnabled();
		await fireEvent.click(button);

		expect(
			screen.getByText(
				'Paid by and Paid to must be different people.'
			)
		).toHaveAttribute('id', 'repayment-to-error');
		expect(screen.getByLabelText('Paid to')).toHaveFocus();
		expect(save).not.toHaveBeenCalled();
	});

	it('shows malformed amount feedback and focuses the amount after a normal save activation', async () => {
		const save = vi.fn();
		renderForm({
			initialDraft: {
				...validDraft(),
				amount: '1.234'
			},
			save
		});

		const button = screen.getByRole('button', {
			name: 'Save changes'
		});
		expect(button).toBeEnabled();
		await fireEvent.click(button);

		expect(
			screen.getByText(
				'Enter a dollar amount with up to two decimal places.'
			)
		).toHaveAttribute('id', 'repayment-amount-error');
		expect(screen.getByLabelText('Amount')).toHaveFocus();
		expect(save).not.toHaveBeenCalled();
	});

	it('shows malformed note feedback and focuses the note after a normal save activation', async () => {
		const save = vi.fn();
		renderForm({
			initialDraft: {
				...validDraft(),
				note: 'cash\u202epaid'
			},
			save
		});

		const button = screen.getByRole('button', {
			name: 'Save changes'
		});
		expect(button).toBeEnabled();
		await fireEvent.click(button);

		expect(
			screen.getByText(
				'Note contains unsupported control characters.'
			)
		).toHaveAttribute('id', 'repayment-note-error');
		expect(screen.getByLabelText('Note (optional)')).toHaveFocus();
		expect(save).not.toHaveBeenCalled();
	});

	it('maps every documented 422 field, keeps unknown keys visible, and focuses sender first', async () => {
		const save = vi.fn().mockRejectedValue(
			new ApiError({
				status: 422,
				code: 'validation_failed',
				message: 'Invalid payment record.',
				fields: {
					fromUserId: 'Server sender error.',
					toUserId: 'Server recipient error.',
					amountCents: 'Server amount error.',
					repaymentDate: 'Server date error.',
					note: 'Server note error.',
					futureField: 'Future validation error.'
				}
			})
		);
		renderForm({ save });

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		expect(
			await screen.findByText('Server sender error.')
		).toBeInTheDocument();
		expect(screen.getByText('Server recipient error.')).toBeInTheDocument();
		expect(screen.getByText('Server amount error.')).toBeInTheDocument();
		expect(screen.getByText('Server date error.')).toBeInTheDocument();
		expect(screen.getByText('Server note error.')).toBeInTheDocument();
		expect(
			screen.getByText(
				'The server reported a form issue that Settled could not match to a field.'
			)
		).toBeInTheDocument();
		expect(
			screen.getByText('futureField: Future validation error.')
		).toBeInTheDocument();
		expect(screen.getByLabelText('Paid by')).toHaveFocus();

		await fireEvent.click(screen.getByRole('button', { name: 'Swap people' }));
		expect(screen.queryByText('Server sender error.')).not.toBeInTheDocument();
		expect(screen.queryByText('Server recipient error.')).not.toBeInTheDocument();
		expect(screen.getByLabelText('Amount')).toHaveAttribute('aria-invalid', 'true');
	});

	it('normalizes the note, locks pending submission, and prevents duplicate saves', async () => {
		const pending = deferred<Repayment>();
		const save = vi.fn(() => pending.promise);
		const onCommitted = vi.fn().mockResolvedValue({
			refresh: 'succeeded',
			navigation: 'succeeded'
		});
		renderForm({
			initialDraft: {
				...validDraft(),
				note: '\u0085Venmo\u3000'
			},
			save,
			onCommitted
		});

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		expect(save).toHaveBeenCalledWith(
			expect.objectContaining({ note: 'Venmo' }),
			expect.objectContaining({ signal: expect.any(AbortSignal) })
		);
		expect(
			screen.getByRole('button', { name: /Saving changes/ })
		).toBeDisabled();
		await fireEvent.click(
			screen.getByRole('button', { name: /Saving changes/ })
		);
		expect(save).toHaveBeenCalledOnce();
		const swapButton = screen.getByRole('button', { name: 'Swap people' });
		expect(swapButton).toBeDisabled();
		await fireEvent.click(swapButton);
		expect(screen.getByLabelText('Paid by')).toHaveTextContent('Bob');
		expect(screen.getByLabelText('Paid to')).toHaveTextContent('Alice');
		pending.resolve(repaymentListFixture.repayments[0]);
		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
	});

	it('treats a network result as ambiguous and keeps the draft locked', async () => {
		const save = vi
			.fn()
			.mockRejectedValue(networkError(new TypeError('connection lost')));
		renderForm({ save });

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		expect(
			await screen.findByText(
				'Settled could not confirm the result. Check group activity before trying again.'
			)
		).toBeInTheDocument();
		expect(
			screen.getByRole('button', { name: 'Save changes' })
		).toBeDisabled();
		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);
		expect(save).toHaveBeenCalledOnce();
	});

	it('locks a committed save and offers balances recovery after follow-up failures', async () => {
		const save = vi
			.fn()
			.mockResolvedValue(repaymentListFixture.repayments[0]);
		const onCommitted = vi.fn().mockResolvedValue({
			refresh: 'failed',
			navigation: 'failed'
		});
		renderForm({ save, onCommitted });

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);

		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
		expect(mocks.toastSuccess).toHaveBeenCalledWith('Changes saved');
		expect(mocks.toastError).toHaveBeenCalledWith(
			expect.stringContaining('balances could not be fully refreshed')
		);
		expect(
			screen.getByRole('link', { name: 'Return to group balances' })
		).toHaveAttribute('href', '/groups/group-id?view=balances');
		expect(
			screen.getByRole('button', { name: 'Save changes' })
		).toBeDisabled();
	});

	it('guards dirty internal navigation while allowing session expiry during a pending save', async () => {
		const pending = deferred<Repayment>();
		const onCommitted = vi.fn().mockResolvedValue({
			refresh: 'succeeded',
			navigation: 'succeeded'
		});
		const confirm = vi.mocked(globalThis.confirm);
		confirm.mockReturnValue(false);
		renderForm({
			save: vi.fn(() => pending.promise),
			onCommitted
		});

		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});
		const dirtyNavigation = navigation(
			'/groups/group-id?view=activity'
		);
		registeredCallback()(dirtyNavigation.value);
		expect(dirtyNavigation.cancel).toHaveBeenCalledOnce();

		await fireEvent.click(
			screen.getByRole('button', { name: 'Save changes' })
		);
		const sessionNavigation = navigation(
			'/login?reason=session-expired&next=%2Fgroups'
		);
		registeredCallback()(sessionNavigation.value);
		expect(sessionNavigation.cancel).not.toHaveBeenCalled();
		pending.resolve(repaymentListFixture.repayments[0]);
		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
	});

	it('retains the draft and describes the offline mutation guard', async () => {
		const save = vi.fn();
		renderForm({
			mutationDisabledReason:
				'Changes are unavailable while Settled is offline.'
		});

		await fireEvent.input(screen.getByLabelText('Note (optional)'), {
			target: { value: 'Cash' }
		});
		const button = screen.getByRole('button', {
			name: 'Save changes'
		});
		expect(button).toBeDisabled();
		expect(button).toHaveAttribute(
			'aria-describedby',
			'repayment-mutation-disabled'
		);
		expect(screen.getByLabelText('Note (optional)')).toHaveValue('Cash');
		expect(
			screen
				.getByText(
					'Changes are unavailable while Settled is offline.'
				)
				.closest('[role="alert"]')
		).toHaveAttribute('id', 'repayment-mutation-disabled');
		expect(save).not.toHaveBeenCalled();
	});
});

function renderForm({
	initialDraft = validDraft(),
	save = vi
		.fn()
		.mockResolvedValue(repaymentListFixture.repayments[0]),
	onCommitted = vi.fn().mockResolvedValue({
		refresh: 'succeeded',
		navigation: 'succeeded'
	}),
	mutationDisabledReason = null
}: {
	initialDraft?: RepaymentDraft;
	save?: SaveRepayment;
	onCommitted?: FinishRepayment;
	mutationDisabledReason?: string | null;
} = {}) {
	return render(RepaymentForm, {
		mode: 'edit',
		members: groupDetailFixture.members,
		initialDraft,
		cancelHref: '/groups/group-id?view=activity',
		balancesHref: '/groups/group-id?view=balances',
		mutation: new MutationCoordinator<'save' | 'delete'>(),
		save,
		onCommitted,
		onNotFound: vi.fn().mockResolvedValue('record'),
		mutationDisabledReason
	});
}

function validDraft(): RepaymentDraft {
	const repayment = repaymentListFixture.repayments[0];
	return {
		fromUserId: repayment.fromUserId,
		toUserId: repayment.toUserId,
		amount: '20.00',
		repaymentDate: repayment.repaymentDate,
		note: repayment.note ?? ''
	};
}

function registeredCallback(): NavigationCallback {
	expect(mocks.callbacks).toHaveLength(1);
	return mocks.callbacks[0];
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
				scroll: null,
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
