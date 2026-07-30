<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import { onDestroy, tick, untrack } from "svelte";
	import { toast } from "svelte-sonner";

	import {
		ApiError,
		isNetworkApiError,
		isNotFoundApiError,
		isUnauthorizedApiError,
		mapApiFieldErrors
	} from "$lib/api/errors";
	import type { Expense, GroupMember } from "$lib/api/types";
	import UnsavedChangesGuard from "$lib/components/app/unsaved-changes-guard.svelte";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Field from "$lib/components/ui/field";
	import { Input } from "$lib/components/ui/input";
	import * as InputGroup from "$lib/components/ui/input-group";
	import * as Select from "$lib/components/ui/select";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { MutationCoordinator } from "$lib/state/mutation-coordinator.svelte";
	import {
		isExpenseDraftDirty,
		toExpenseInput,
		type ExpenseDraft,
		type ExpenseDraftError,
		type ExpenseInput
	} from "$lib/utils/expense-draft";
	import type { CommittedMutationResult } from "$lib/utils/committed-mutation";

	import SplitEditor from "./split-editor.svelte";

	type ExpenseFormField =
		| 'description'
		| 'amount'
		| 'payer'
		| 'date'
		| 'splits';

	type RecordNotFoundResolution = 'record' | 'handled' | 'retryable';

	let {
		mode,
		members,
		initialDraft,
		cancelHref,
		mutation,
		save,
		onCommitted,
		onNotFound,
		mutationDisabledReason = null
	}: {
		mode: 'create' | 'edit';
		members: readonly GroupMember[];
		initialDraft: ExpenseDraft;
		cancelHref: string;
		mutation: MutationCoordinator<'save' | 'delete'>;
		save: (
			input: ExpenseInput,
			options: { signal: AbortSignal }
		) => Promise<Expense>;
		onCommitted: (expense: Expense) => Promise<CommittedMutationResult>;
		onNotFound: () =>
			| RecordNotFoundResolution
			| Promise<RecordNotFoundResolution>;
		mutationDisabledReason?: string | null;
	} = $props();

	const initial = cloneDraft(untrack(() => initialDraft));
	let draft = $state<ExpenseDraft>(cloneDraft(initial));
	let fieldErrors = $state<Partial<Record<ExpenseFormField, string>>>({});
	let splitErrorParticipantUserId = $state<string | undefined>(undefined);
	let formErrors = $state<string[]>([]);
	let postCommitRecovery = $state(false);
	let descriptionInput: HTMLInputElement | null = $state(null);
	let amountInput: HTMLInputElement | null = $state(null);
	let payerTrigger: HTMLElement | null = $state(null);
	let dateInput: HTMLInputElement | null = $state(null);
	let splitEditor: HTMLElement | null = $state(null);
	let errorAlert: HTMLDivElement | null = $state(null);
	let controller: AbortController | null = null;

	const payerName = $derived(
		members.find((member) => member.userId === draft.paidByUserId)
			?.displayName ?? copy.groups.expenses.payerPlaceholder
	);
	const selectItems = $derived(
		members.map((member) => ({
			value: member.userId,
			label: member.displayName
		}))
	);
	const draftResult = $derived(toExpenseInput(draft));
	const dirty = $derived(isExpenseDraftDirty(draft, initial));
	const mutationPending = $derived(mutation.phase === 'pending');
	const pending = $derived(
		mutation.phase === 'pending' && mutation.operation === 'save'
	);
	const committed = $derived(mutation.phase === 'committed');
	const locked = $derived(mutation.phase !== 'idle');
	const submitDisabled = $derived(
		locked || mutationDisabledReason !== null || !draftResult.ok
	);

	onDestroy(() => controller?.abort());

	function updateDescription(value: string): void {
		draft.description = value;
		clearFieldError('description');
	}

	function updateAmount(value: string): void {
		draft.amount = value;
		clearFieldError('amount');
	}

	function updatePayer(value: string): void {
		draft.paidByUserId = value;
		clearFieldError('payer');
	}

	function updateDate(value: string): void {
		draft.expenseDate = value;
		clearFieldError('date');
	}

	function updateSplit(nextDraft: ExpenseDraft): void {
		draft = nextDraft;
		splitErrorParticipantUserId = undefined;
		clearFieldError('splits');
	}

	function clearFieldError(field: ExpenseFormField): void {
		if (fieldErrors[field] === undefined) {
			return;
		}
		const next = { ...fieldErrors };
		delete next[field];
		fieldErrors = next;
	}

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (
			locked ||
			mutationDisabledReason !== null ||
			!mutation.begin('save')
		) {
			return;
		}

		const inputResult = toExpenseInput(draft);
		if (!inputResult.ok) {
			mutation.reset();
			applyDraftErrors(inputResult.errors);
			await focusFirstInvalid();
			return;
		}

		fieldErrors = {};
		formErrors = [];
		controller?.abort();
		controller = new AbortController();

		let expense: Expense;
		try {
			expense = await save(inputResult.value, {
				signal: controller.signal
			});
		} catch (error) {
			if (controller.signal.aborted) {
				return;
			}
			if (isUnauthorizedApiError(error)) {
				mutation.reset();
				return;
			}
			if (isNotFoundApiError(error)) {
				let resolution: RecordNotFoundResolution = 'record';
				try {
					resolution = await onNotFound();
				} catch {
					// The mutation is still uncommitted; keep the form available.
				}
				if (resolution === 'handled') {
					mutation.markBlocked();
					return;
				}
				if (resolution === 'retryable') {
					mutation.reset();
					formErrors = [copy.groups.expenses.failure];
					await focusAlert();
					return;
				}
				mutation.markBlocked();
				formErrors = [copy.groups.expenses.recordUnavailableTitle];
				await focusAlert();
				return;
			}
			if (error instanceof ApiError && error.status === 422) {
				mutation.reset();
				applyBackendErrors(error);
				await focusFirstInvalid();
				return;
			}
			if (
				isNetworkApiError(error) ||
				(error instanceof ApiError && error.source === 'invalid-response')
			) {
				mutation.markAmbiguous();
				formErrors = [copy.groups.expenses.ambiguousFailure];
				await focusAlert();
				return;
			}

			mutation.reset();
			formErrors = [copy.groups.expenses.failure];
			await focusAlert();
			return;
		}

		mutation.markCommitted();
		await tick();
		toast.success(
			mode === 'create'
				? copy.groups.expenses.added
				: copy.groups.expenses.saved
		);
		let result: CommittedMutationResult;
		try {
			result = await onCommitted(expense);
		} catch {
			result = { refresh: 'failed', navigation: 'failed' };
		}
		if (result.refresh === 'failed') {
			toast.error(copy.groups.expenses.refreshFailure);
		}
		if (result.navigation === 'failed') {
			postCommitRecovery = true;
			formErrors = [copy.groups.expenses.navigationFailure];
			toast.error(copy.groups.expenses.navigationFailure);
			await focusAlert();
		}
	}

	function applyDraftErrors(errors: readonly ExpenseDraftError[]): void {
		const next: Partial<Record<ExpenseFormField, string>> = {};
		for (const error of errors) {
			const field = draftErrorField(error);
			next[field] ??= error.message;
		}
		fieldErrors = next;
		splitErrorParticipantUserId = errors.find((error) => {
			const field = draftErrorField(error);
			return field === 'splits' && error.participantUserId !== undefined;
		})?.participantUserId;
		formErrors = [];
	}

	function applyBackendErrors(error: ApiError): void {
		const mapped = mapApiFieldErrors(error.fields, {
			description: 'description',
			amountCents: 'amount',
			paidByUserId: 'payer',
			expenseDate: 'date',
			participantUserIds: 'splits',
			splits: 'splits',
			percentageSplits: 'splits'
		} satisfies Record<string, ExpenseFormField>);
		fieldErrors = mapped.fieldErrors;
		splitErrorParticipantUserId = undefined;
		const unknownMessages = Object.entries(mapped.unknownFieldErrors).map(
			([field, message]) => `${field}: ${message}`
		);
		formErrors =
			unknownMessages.length > 0
				? [copy.groups.expenses.unknownFieldFailure, ...unknownMessages]
				: Object.keys(mapped.fieldErrors).length === 0
					? [error.message]
					: [];
	}

	async function focusFirstInvalid(): Promise<void> {
		await tick();
		for (const [field, target] of [
			['description', descriptionInput],
			['amount', amountInput],
			['payer', payerTrigger],
			['date', dateInput]
		] as const) {
			if (fieldErrors[field]) {
				target?.focus();
				return;
			}
		}
		if (fieldErrors.splits) {
			const target = splitErrorParticipantUserId
				? splitEditor?.querySelector<HTMLElement>(
						`[data-split-input="${CSS.escape(splitErrorParticipantUserId)}"]`
					)
				: splitEditor?.querySelector<HTMLElement>(
						draft.splitMode === 'equal'
							? '[role="checkbox"], [data-slot="toggle-group-item"]'
							: '[data-split-input]'
					);
			target?.focus();
			return;
		}
		await focusAlert();
	}

	async function focusAlert(): Promise<void> {
		await tick();
		errorAlert?.focus();
	}

	function draftErrorField(error: ExpenseDraftError): ExpenseFormField {
		switch (error.field) {
			case 'description':
				return 'description';
			case 'amountCents':
				return 'amount';
			case 'paidByUserId':
				return 'payer';
			case 'expenseDate':
				return 'date';
			default:
				return 'splits';
		}
	}

	function cloneDraft(value: ExpenseDraft): ExpenseDraft {
		return {
			...value,
			participants: value.participants.map((participant) => ({
				...participant
			}))
		};
	}
</script>

<UnsavedChangesGuard
	active={dirty && !committed}
	blocked={mutationPending}
	message={copy.groups.expenses.unsavedChanges}
/>

<form class="flex flex-col gap-6" novalidate onsubmit={submit}>
	{#if formErrors.length > 0}
		<Alert.Root
			bind:ref={errorAlert}
			variant="destructive"
			tabindex={-1}
		>
			<CircleAlertIcon data-icon="inline-start" />
			<Alert.Title>{formErrors[0]}</Alert.Title>
			{#if formErrors.length > 1}
				<Alert.Description>
					<ul class="list-disc pl-4">
						{#each formErrors.slice(1) as message (message)}
							<li>{message}</li>
						{/each}
					</ul>
				</Alert.Description>
			{/if}
			{#if postCommitRecovery}
				<Alert.Description>
					<a href={cancelHref}>{copy.groups.expenses.continueToActivity}</a>
				</Alert.Description>
			{/if}
		</Alert.Root>
	{/if}

	{#if mutationDisabledReason !== null}
		<Alert.Root id="expense-mutation-disabled">
			<CircleAlertIcon data-icon="inline-start" />
			<Alert.Title>{mutationDisabledReason}</Alert.Title>
		</Alert.Root>
	{/if}

	<Field.FieldGroup>
		<Field.Field
			data-invalid={fieldErrors.description ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="expense-description">
				{copy.groups.expenses.descriptionLabel}
			</Field.FieldLabel>
			<Input
				bind:ref={descriptionInput}
				id="expense-description"
				autocomplete="off"
				placeholder={copy.groups.expenses.descriptionPlaceholder}
				value={draft.description}
				oninput={(event) => updateDescription(event.currentTarget.value)}
				disabled={locked}
				aria-invalid={fieldErrors.description ? true : undefined}
				aria-describedby={fieldErrors.description
					? 'expense-description-error'
					: undefined}
			/>
			<Field.FieldError
				id="expense-description-error"
				errors={fieldErrors.description
					? [{ message: fieldErrors.description }]
					: []}
			/>
		</Field.Field>

		<Field.Field
			data-invalid={fieldErrors.amount ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="expense-amount">
				{copy.groups.expenses.amountLabel}
			</Field.FieldLabel>
			<InputGroup.Root>
				<InputGroup.Addon>$</InputGroup.Addon>
				<InputGroup.Input
					bind:ref={amountInput}
					id="expense-amount"
					inputmode="decimal"
					autocomplete="off"
					value={draft.amount}
					oninput={(event) => updateAmount(event.currentTarget.value)}
					disabled={locked}
					aria-invalid={fieldErrors.amount ? true : undefined}
					aria-describedby={fieldErrors.amount
						? 'expense-amount-error'
						: undefined}
				/>
			</InputGroup.Root>
			<Field.FieldError
				id="expense-amount-error"
				errors={fieldErrors.amount ? [{ message: fieldErrors.amount }] : []}
			/>
		</Field.Field>

		<Field.Field
			data-invalid={fieldErrors.payer ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="expense-payer">
				{copy.groups.expenses.payerLabel}
			</Field.FieldLabel>
			<Select.Root
				type="single"
				value={draft.paidByUserId}
				onValueChange={updatePayer}
				items={selectItems}
				disabled={locked}
			>
				<Select.Trigger
					bind:ref={payerTrigger}
					id="expense-payer"
					class="min-h-11 w-full"
					aria-invalid={fieldErrors.payer ? true : undefined}
					aria-describedby={fieldErrors.payer
						? 'expense-payer-error'
						: undefined}
				>
					<span class="truncate">{payerName}</span>
				</Select.Trigger>
				<Select.Content>
					<Select.Group>
						{#each members as member (member.userId)}
							<Select.Item
								value={member.userId}
								label={member.displayName}
								class="min-h-11"
							/>
						{/each}
					</Select.Group>
				</Select.Content>
			</Select.Root>
			<Field.FieldError
				id="expense-payer-error"
				errors={fieldErrors.payer ? [{ message: fieldErrors.payer }] : []}
			/>
		</Field.Field>

		<Field.Field
			data-invalid={fieldErrors.date ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="expense-date">
				{copy.groups.expenses.dateLabel}
			</Field.FieldLabel>
			<Input
				bind:ref={dateInput}
				id="expense-date"
				type="date"
				value={draft.expenseDate}
				oninput={(event) => updateDate(event.currentTarget.value)}
				disabled={locked}
				aria-invalid={fieldErrors.date ? true : undefined}
				aria-describedby={fieldErrors.date
					? 'expense-date-error'
					: undefined}
			/>
			<Field.FieldError
				id="expense-date-error"
				errors={fieldErrors.date ? [{ message: fieldErrors.date }] : []}
			/>
		</Field.Field>

		<SplitEditor
			bind:ref={splitEditor}
			{draft}
			{members}
			disabled={locked}
			error={fieldErrors.splits}
			errorParticipantUserId={splitErrorParticipantUserId}
			editMode={mode === 'edit'}
			onChange={updateSplit}
		/>
	</Field.FieldGroup>

	<div
		class="sticky bottom-0 z-20 -mx-4 mt-2 border-t bg-background/95 px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] backdrop-blur supports-[backdrop-filter]:bg-background/85 sm:static sm:mx-0 sm:border-0 sm:bg-transparent sm:px-0 sm:pb-0 sm:pt-0 sm:backdrop-blur-none"
	>
		<div class="mx-auto flex max-w-2xl flex-col-reverse gap-2 sm:flex-row sm:justify-end">
			<Button
				href={cancelHref}
				variant="outline"
				class="min-h-11 sm:min-w-28"
				disabled={mutationPending || committed}
			>
				{copy.groups.expenses.cancel}
			</Button>
			<Button
				type="submit"
				class="min-h-11 sm:min-w-36"
				disabled={submitDisabled}
				title={mutationDisabledReason ?? undefined}
				aria-describedby={[
					mutationDisabledReason !== null
						? 'expense-mutation-disabled'
						: '',
					!draftResult.ok ? 'expense-save-disabled-reason' : ''
				]
					.filter(Boolean)
					.join(' ') || undefined}
			>
				{#if pending}
					<Spinner
						aria-label={mode === 'create'
							? copy.groups.expenses.adding
							: copy.groups.expenses.saving}
					/>
					{mode === 'create'
						? copy.groups.expenses.adding
						: copy.groups.expenses.saving}
				{:else}
					{mode === 'create'
						? copy.groups.expenses.add
						: copy.groups.expenses.save}
				{/if}
			</Button>
		</div>
		{#if !draftResult.ok}
			<p
				id="expense-save-disabled-reason"
				class="mx-auto mt-2 max-w-2xl text-right text-xs text-muted-foreground"
			>
				{copy.groups.expenses.saveDisabledReason}
			</p>
		{/if}
	</div>
</form>
