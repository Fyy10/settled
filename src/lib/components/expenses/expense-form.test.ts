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
import type { Expense } from '$lib/api/types';
import { MutationCoordinator } from '$lib/state/mutation-coordinator.svelte';
import type {
	ExpenseDraft,
	ExpenseInput
} from '$lib/utils/expense-draft';
import type { CommittedMutationResult } from '$lib/utils/committed-mutation';
import {
	expenseListFixture,
	groupDetailFixture
} from '../../../tests/fixtures/api-contract';

import ExpenseForm from './expense-form.svelte';

type NavigationCallback = (navigation: BeforeNavigate) => void;
type SaveExpense = (
	input: ExpenseInput,
	options: { signal: AbortSignal }
) => Promise<Expense>;
type FinishExpense = (
	expense: Expense
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

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('ExpenseForm', () => {
	it('retains editable entries and blocks direct submission while offline', async () => {
		await withNetworkState(async (setOnline) => {
			const save = vi.fn();
			renderForm({ save });
			const input = screen.getByLabelText('Description');
			await fireEvent.input(input, { target: { value: 'Offline dinner' } });
			await setOnline(false);
			const button = screen.getAllByRole('button', { name: 'Save changes' }).at(-1)!;
			expect(button).toBeDisabled();
			expect(
				screen.getByText('Reconnect to continue. Your entries will stay here.')
			).toBeInTheDocument();
			await fireEvent.submit(input.closest('form')!);
			expect(save).not.toHaveBeenCalled();
			expect(input).toHaveValue('Offline dinner');
			expect(input).toBeEnabled();
			await setOnline(true);
			expect(input).toHaveValue('Offline dinner');
			expect(save).not.toHaveBeenCalled();
		});
	});

	it('focuses fields in visual order for client validation', async () => {
		const draft = validDraft();
		draft.description = '';
		draft.amount = '';
		const { container } = renderForm({ initialDraft: draft });

		await fireEvent.submit(container.querySelector('form')!);

		expect(screen.getByLabelText('Description')).toHaveFocus();
		expect(screen.getByText('Enter a description.')).toHaveAttribute(
			'id',
			'expense-description-error'
		);
		expect(screen.getByLabelText('Description')).toHaveAttribute(
			'aria-describedby',
			'expense-description-error'
		);
	});

	it('maps every documented 422 field, reports unknown keys, and focuses description first', async () => {
		const save = vi.fn().mockRejectedValue(
			new ApiError({
				status: 422,
				code: 'validation_failed',
				message: 'Invalid expense.',
				fields: {
					description: 'Server description error.',
					amountCents: 'Server amount error.',
					paidByUserId: 'Server payer error.',
					expenseDate: 'Server date error.',
					splits: 'Server split error.',
					futureField: 'Future validation error.'
				}
			})
		);
		renderForm({ save });

		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));

		expect(await screen.findByText('Server description error.')).toBeInTheDocument();
		expect(screen.getByText('Server amount error.')).toBeInTheDocument();
		expect(screen.getByText('Server payer error.')).toBeInTheDocument();
		expect(screen.getByText('Server date error.')).toBeInTheDocument();
		expect(screen.getByText('Server split error.')).toBeInTheDocument();
		expect(
			screen.getByText(
				'The server reported a form issue that Settled could not match to a field.'
			)
		).toBeInTheDocument();
		expect(screen.getByText('futureField: Future validation error.')).toBeInTheDocument();
		expect(screen.getByLabelText('Description')).toHaveFocus();
	});

	it.each([
		['splits', 'exact'],
		['percentageSplits', 'percentage']
	] as const)(
		'focuses the first visible share for backend %s errors',
		async (apiField, mode) => {
			const draft =
				mode === 'exact' ? validDraft() : validPercentageDraft();
			const save = vi.fn().mockRejectedValue(
				new ApiError({
					status: 422,
					code: 'validation_failed',
					message: 'Invalid split.',
					fields: { [apiField]: 'Recheck the split.' }
				})
			);
			renderForm({ initialDraft: draft, save });

			await fireEvent.click(
				screen.getByRole('button', { name: 'Save changes' })
			);

			expect(await screen.findByText('Recheck the split.')).toBeInTheDocument();
			expect(
				screen.getByLabelText(
					mode === 'exact' ? 'Share for Alice' : 'Percentage for Alice'
				)
			).toHaveFocus();
		}
	);

	it('locks a committed save, surfaces follow-up failures, and offers navigation recovery', async () => {
		const save = vi.fn().mockResolvedValue(expenseListFixture.expenses[0]);
		const onCommitted = vi.fn().mockResolvedValue({
			refresh: 'failed',
			navigation: 'failed'
		});
		renderForm({ save, onCommitted });

		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));

		await waitFor(() => expect(onCommitted).toHaveBeenCalledOnce());
		expect(mocks.toastSuccess).toHaveBeenCalledWith('Changes saved');
		expect(mocks.toastError).toHaveBeenCalledWith(
			expect.stringContaining('some group information')
		);
		expect(
			screen.getByRole('link', { name: 'Return to group activity' })
		).toHaveAttribute('href', '/groups/group-id?view=activity');
		expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled();
		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
		expect(save).toHaveBeenCalledOnce();
	});

	it('treats a network result as ambiguous and prevents duplicate POST/PUT', async () => {
		const save = vi
			.fn()
			.mockRejectedValue(networkError(new TypeError('connection lost')));
		renderForm({ save });

		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));

		expect(
			await screen.findByText(
				'Settled could not confirm the result. Check group activity before trying again.'
			)
		).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled();
		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
		expect(save).toHaveBeenCalledOnce();
	});

	it('guards a dirty internal navigation and allows session expiry while pending', async () => {
		const pending = deferred<typeof expenseListFixture.expenses[0]>();
		const confirm = vi.mocked(globalThis.confirm);
		confirm.mockReturnValue(false);
		renderForm({ save: vi.fn(() => pending.promise) });

		await fireEvent.input(screen.getByLabelText('Description'), {
			target: { value: 'Updated dinner' }
		});
		const dirtyNavigation = navigation('/groups/group-id?view=activity');
		registeredCallback()(dirtyNavigation.value);
		expect(dirtyNavigation.cancel).toHaveBeenCalledOnce();

		await fireEvent.click(screen.getByRole('button', { name: 'Save changes' }));
		const sessionNavigation = navigation(
			'/login?reason=session-expired&next=%2Fgroups'
		);
		registeredCallback()(sessionNavigation.value);
		expect(sessionNavigation.cancel).not.toHaveBeenCalled();
	});

	it('programmatically describes an offline mutation guard from the disabled Save button', () => {
		renderForm({
			mutationDisabledReason: 'Changes are unavailable while Settled is offline.'
		});

		const save = screen.getByRole('button', { name: 'Save changes' });
		expect(save).toBeDisabled();
		expect(save).toHaveAttribute(
			'aria-describedby',
			'expense-mutation-disabled'
		);
		expect(
			screen.getByText('Changes are unavailable while Settled is offline.')
				.closest('[role="alert"]')
		).toHaveAttribute('id', 'expense-mutation-disabled');
	});
});

function renderForm({
	initialDraft = validDraft(),
	save = vi.fn().mockResolvedValue(expenseListFixture.expenses[0]),
	onCommitted = vi.fn().mockResolvedValue({
		refresh: 'succeeded',
		navigation: 'succeeded'
	}),
	mutationDisabledReason = null
}: {
	initialDraft?: ExpenseDraft;
	save?: SaveExpense;
	onCommitted?: FinishExpense;
	mutationDisabledReason?: string | null;
} = {}) {
	return render(ExpenseForm, {
		mode: 'edit',
		members: groupDetailFixture.members,
		initialDraft,
		cancelHref: '/groups/group-id?view=activity',
		mutation: new MutationCoordinator<'save' | 'delete'>(),
		save,
		onCommitted,
		onNotFound: vi.fn().mockResolvedValue('record'),
		mutationDisabledReason
	});
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

function validPercentageDraft(): ExpenseDraft {
	const draft = validDraft();
	draft.splitMode = 'percentage';
	draft.participants = draft.participants.map((participant) => ({
		...participant,
		exactAmount: '',
		seededExactAmount: '',
		percentage: '50',
		seededPercentage: '50'
	}));
	return draft;
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
