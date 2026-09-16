<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import InfoIcon from "@lucide/svelte/icons/info";
	import { onDestroy, tick, untrack } from "svelte";
	import { toast } from "svelte-sonner";

	import {
		ApiError,
		isNetworkApiError,
		isNotFoundApiError,
		isUnauthorizedApiError,
		mapApiFieldErrors
	} from "$lib/api/errors";
	import type {
		GroupMember,
		Repayment,
		ReplaceRepaymentInput
	} from "$lib/api/types";
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
	import type { CommittedMutationResult } from "$lib/utils/committed-mutation";
	import {
		isRepaymentDraftDirty,
		toRepaymentInput,
		type RepaymentDraft,
		type RepaymentDraftError
	} from "$lib/utils/repayment-draft";

	type RepaymentFormField =
		| 'sender'
		| 'recipient'
		| 'amount'
		| 'date'
		| 'note';
	type NotFoundResolution = 'record' | 'handled' | 'retryable';

	let {
		mode,
		members,
		initialDraft,
		cancelHref,
		balancesHref,
		mutation,
		save,
		onCommitted,
		onNotFound,
		mutationDisabledReason = null
	}: {
		mode: 'create' | 'edit';
		members: readonly GroupMember[];
		initialDraft: RepaymentDraft;
		cancelHref: string;
		balancesHref: string;
		mutation: MutationCoordinator<'save' | 'delete'>;
		save: (
			input: ReplaceRepaymentInput,
			options: { signal: AbortSignal }
		) => Promise<Repayment>;
		onCommitted: (
			repayment: Repayment
		) => Promise<CommittedMutationResult>;
		onNotFound: () =>
			| NotFoundResolution
			| Promise<NotFoundResolution>;
		mutationDisabledReason?: string | null;
	} = $props();

	const initial = cloneDraft(untrack(() => initialDraft));
	let draft = $state<RepaymentDraft>(cloneDraft(initial));
	let fieldErrors = $state<
		Partial<Record<RepaymentFormField, string>>
	>({});
	let formErrors = $state<string[]>([]);
	let postCommitRecovery = $state(false);
	let senderTrigger: HTMLElement | null = $state(null);
	let recipientTrigger: HTMLElement | null = $state(null);
	let amountInput: HTMLInputElement | null = $state(null);
	let dateInput: HTMLInputElement | null = $state(null);
	let noteInput: HTMLInputElement | null = $state(null);
	let errorAlert: HTMLDivElement | null = $state(null);
	let controller: AbortController | null = null;

	const memberIds = $derived(members.map((member) => member.userId));
	const selectItems = $derived(
		members.map((member) => ({
			value: member.userId,
			label: member.displayName
		}))
	);
	const senderName = $derived(
		members.find((member) => member.userId === draft.fromUserId)
			?.displayName ?? copy.groups.repayments.fromPlaceholder
	);
	const recipientName = $derived(
		members.find((member) => member.userId === draft.toUserId)
			?.displayName ?? copy.groups.repayments.toPlaceholder
	);
	const draftResult = $derived(toRepaymentInput(draft, memberIds));
	const dirty = $derived(isRepaymentDraftDirty(draft, initial));
	const mutationPending = $derived(mutation.phase === 'pending');
	const pending = $derived(
		mutation.phase === 'pending' && mutation.operation === 'save'
	);
	const committed = $derived(mutation.phase === 'committed');
	const locked = $derived(mutation.phase !== 'idle');
	const submitDisabled = $derived(
		locked || mutationDisabledReason !== null
	);

	onDestroy(() => controller?.abort());

	function updateSender(value: string): void {
		draft.fromUserId = value;
		clearFieldError('sender');
		clearFieldError('recipient');
	}

	function updateRecipient(value: string): void {
		draft.toUserId = value;
		clearFieldError('recipient');
		clearFieldError('sender');
	}

	function swapPeople(): void {
		if (locked) {
			return;
		}
		const previousSender = draft.fromUserId;
		draft.fromUserId = draft.toUserId;
		draft.toUserId = previousSender;
		clearFieldError('sender');
		clearFieldError('recipient');
	}

	function updateAmount(value: string): void {
		draft.amount = value;
		clearFieldError('amount');
	}

	function updateDate(value: string): void {
		draft.repaymentDate = value;
		clearFieldError('date');
	}

	function updateNote(value: string): void {
		draft.note = value;
		clearFieldError('note');
	}

	function clearFieldError(field: RepaymentFormField): void {
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

		const inputResult = toRepaymentInput(draft, memberIds);
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

		let repayment: Repayment;
		try {
			repayment = await save(inputResult.value, {
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
				await handleNotFound();
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
				(error instanceof ApiError &&
					error.source === 'invalid-response')
			) {
				mutation.markAmbiguous();
				formErrors = [copy.groups.repayments.ambiguousFailure];
				await focusAlert();
				return;
			}

			mutation.reset();
			formErrors = [copy.groups.repayments.failure];
			await focusAlert();
			return;
		}

		mutation.markCommitted();
		await tick();
		toast.success(
			mode === 'create'
				? copy.groups.repayments.recorded
				: copy.groups.repayments.saved
		);
		let result: CommittedMutationResult;
		try {
			result = await onCommitted(repayment);
		} catch {
			result = { refresh: 'failed', navigation: 'failed' };
		}
		if (result.refresh === 'failed') {
			toast.error(copy.groups.repayments.refreshFailure);
		}
		if (result.navigation === 'failed') {
			postCommitRecovery = true;
			formErrors = [copy.groups.repayments.navigationFailure];
			toast.error(copy.groups.repayments.navigationFailure);
			await focusAlert();
		}
	}

	async function handleNotFound(): Promise<void> {
		let resolution: NotFoundResolution = 'record';
		try {
			resolution = await onNotFound();
		} catch {
			// The mutation did not commit; preserve the draft for recovery.
			resolution = 'retryable';
		}
		if (resolution === 'handled') {
			mutation.markBlocked();
			return;
		}
		if (resolution === 'record') {
			mutation.markBlocked();
			formErrors = [
				copy.groups.repayments.recordUnavailableTitle
			];
			await focusAlert();
			return;
		}

		mutation.reset();
		const refreshedResult = toRepaymentInput(
			draft,
			members.map((member) => member.userId)
		);
		if (!refreshedResult.ok) {
			applyDraftErrors(refreshedResult.errors);
			await focusFirstInvalid();
			return;
		}
		formErrors = [copy.groups.repayments.failure];
		await focusAlert();
	}

	function applyDraftErrors(
		errors: readonly RepaymentDraftError[]
	): void {
		const next: Partial<Record<RepaymentFormField, string>> = {};
		for (const error of errors) {
			const field = draftErrorField(error);
			next[field] ??= error.message;
		}
		fieldErrors = next;
		formErrors = [];
	}

	function applyBackendErrors(error: ApiError): void {
		const mapped = mapApiFieldErrors(error.fields, {
			fromUserId: 'sender',
			toUserId: 'recipient',
			amountCents: 'amount',
			repaymentDate: 'date',
			note: 'note'
		} satisfies Record<string, RepaymentFormField>);
		fieldErrors = mapped.fieldErrors;
		const unknownMessages = Object.entries(
			mapped.unknownFieldErrors
		).map(([field, message]) => `${field}: ${message}`);
		formErrors =
			unknownMessages.length > 0
				? [
						copy.groups.repayments.unknownFieldFailure,
						...unknownMessages
					]
				: Object.keys(mapped.fieldErrors).length === 0
					? [error.message]
					: [];
	}

	async function focusFirstInvalid(): Promise<void> {
		await tick();
		for (const [field, target] of [
			['sender', senderTrigger],
			['recipient', recipientTrigger],
			['amount', amountInput],
			['date', dateInput],
			['note', noteInput]
		] as const) {
			if (fieldErrors[field]) {
				target?.focus();
				return;
			}
		}
		await focusAlert();
	}

	async function focusAlert(): Promise<void> {
		await tick();
		errorAlert?.focus();
	}

	function draftErrorField(
		error: RepaymentDraftError
	): RepaymentFormField {
		switch (error.field) {
			case 'fromUserId':
				return 'sender';
			case 'toUserId':
				return 'recipient';
			case 'amountCents':
				return 'amount';
			case 'repaymentDate':
				return 'date';
			case 'note':
				return 'note';
		}
	}

	function cloneDraft(value: RepaymentDraft): RepaymentDraft {
		return { ...value };
	}
</script>

<UnsavedChangesGuard
	active={dirty && !committed}
	blocked={mutationPending}
	message={copy.groups.repayments.unsavedChanges}
/>

<form class="flex flex-col gap-6" novalidate onsubmit={submit}>
	<Alert.Root>
		<InfoIcon data-icon="inline-start" />
		<Alert.Title>{copy.groups.repayments.disclaimer}</Alert.Title>
	</Alert.Root>

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
					<a href={balancesHref}>
						{copy.groups.repayments.continueToBalances}
					</a>
				</Alert.Description>
			{/if}
		</Alert.Root>
	{/if}

	{#if mutationDisabledReason !== null}
		<Alert.Root id="repayment-mutation-disabled">
			<CircleAlertIcon data-icon="inline-start" />
			<Alert.Title>{mutationDisabledReason}</Alert.Title>
		</Alert.Root>
	{/if}

	<Field.FieldGroup>
		<Field.Field
			data-invalid={fieldErrors.sender ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="repayment-from">
				{copy.groups.repayments.fromLabel}
			</Field.FieldLabel>
			<Select.Root
				type="single"
				value={draft.fromUserId}
				onValueChange={updateSender}
				items={selectItems}
				disabled={locked}
			>
				<Select.Trigger
					bind:ref={senderTrigger}
					id="repayment-from"
					class="min-h-11 w-full"
					aria-invalid={fieldErrors.sender ? true : undefined}
					aria-describedby={fieldErrors.sender
						? 'repayment-from-error'
						: undefined}
				>
					<span class="truncate">{senderName}</span>
				</Select.Trigger>
				<Select.Content>
					<Select.Group>
						{#each members as member (member.userId)}
							<Select.Item
								value={member.userId}
								label={member.displayName}
								class="min-h-11"
								disabled={member.userId === draft.toUserId}
							/>
						{/each}
					</Select.Group>
				</Select.Content>
			</Select.Root>
			<Field.FieldError
				id="repayment-from-error"
				errors={fieldErrors.sender
					? [{ message: fieldErrors.sender }]
					: []}
			/>
		</Field.Field>

		<Field.Field
			data-invalid={fieldErrors.recipient ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="repayment-to">
				{copy.groups.repayments.toLabel}
			</Field.FieldLabel>
			<Select.Root
				type="single"
				value={draft.toUserId}
				onValueChange={updateRecipient}
				items={selectItems}
				disabled={locked}
			>
				<Select.Trigger
					bind:ref={recipientTrigger}
					id="repayment-to"
					class="min-h-11 w-full"
					aria-invalid={fieldErrors.recipient ? true : undefined}
					aria-describedby={fieldErrors.recipient
						? 'repayment-to-error'
						: undefined}
				>
					<span class="truncate">{recipientName}</span>
				</Select.Trigger>
				<Select.Content>
					<Select.Group>
						{#each members as member (member.userId)}
							<Select.Item
								value={member.userId}
								label={member.displayName}
								class="min-h-11"
								disabled={member.userId === draft.fromUserId}
							/>
						{/each}
					</Select.Group>
				</Select.Content>
			</Select.Root>
			<Field.FieldError
				id="repayment-to-error"
				errors={fieldErrors.recipient
					? [{ message: fieldErrors.recipient }]
					: []}
			/>
		</Field.Field>

		<Button
			type="button"
			variant="outline"
			class="min-h-11 self-start"
			disabled={locked}
			onclick={swapPeople}
		>
			{copy.groups.repayments.swapPeople}
		</Button>

		<Field.Field
			data-invalid={fieldErrors.amount ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="repayment-amount">
				{copy.groups.repayments.amountLabel}
			</Field.FieldLabel>
			<InputGroup.Root>
				<InputGroup.Addon>$</InputGroup.Addon>
				<InputGroup.Input
					bind:ref={amountInput}
					id="repayment-amount"
					inputmode="decimal"
					autocomplete="off"
					value={draft.amount}
					oninput={(event) =>
						updateAmount(event.currentTarget.value)}
					disabled={locked}
					aria-invalid={fieldErrors.amount ? true : undefined}
					aria-describedby={fieldErrors.amount
						? 'repayment-amount-error'
						: undefined}
				/>
			</InputGroup.Root>
			<Field.FieldError
				id="repayment-amount-error"
				errors={fieldErrors.amount
					? [{ message: fieldErrors.amount }]
					: []}
			/>
		</Field.Field>

		<Field.Field
			data-invalid={fieldErrors.date ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="repayment-date">
				{copy.groups.repayments.dateLabel}
			</Field.FieldLabel>
			<Input
				bind:ref={dateInput}
				id="repayment-date"
				type="date"
				value={draft.repaymentDate}
				oninput={(event) => updateDate(event.currentTarget.value)}
				disabled={locked}
				aria-invalid={fieldErrors.date ? true : undefined}
				aria-describedby={fieldErrors.date
					? 'repayment-date-error'
					: undefined}
			/>
			<Field.FieldError
				id="repayment-date-error"
				errors={fieldErrors.date
					? [{ message: fieldErrors.date }]
					: []}
			/>
		</Field.Field>

		<Field.Field
			data-invalid={fieldErrors.note ? true : undefined}
			data-disabled={locked ? true : undefined}
		>
			<Field.FieldLabel for="repayment-note">
				{copy.groups.repayments.noteLabel}
			</Field.FieldLabel>
			<Input
				bind:ref={noteInput}
				id="repayment-note"
				autocomplete="off"
				placeholder={copy.groups.repayments.notePlaceholder}
				value={draft.note}
				oninput={(event) => updateNote(event.currentTarget.value)}
				disabled={locked}
				aria-invalid={fieldErrors.note ? true : undefined}
				aria-describedby={fieldErrors.note
					? 'repayment-note-error'
					: undefined}
			/>
			<Field.FieldError
				id="repayment-note-error"
				errors={fieldErrors.note
					? [{ message: fieldErrors.note }]
					: []}
			/>
		</Field.Field>
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
				{copy.groups.repayments.cancel}
			</Button>
			<Button
				type="submit"
				class="min-h-11 sm:min-w-36"
				disabled={submitDisabled}
				title={mutationDisabledReason ?? undefined}
				aria-describedby={[
					mutationDisabledReason !== null
						? 'repayment-mutation-disabled'
						: '',
					!draftResult.ok
						? 'repayment-save-guidance'
						: ''
				]
					.filter(Boolean)
					.join(' ') || undefined}
			>
				{#if pending}
					<Spinner
						aria-label={mode === 'create'
							? copy.groups.repayments.recording
							: copy.groups.repayments.saving}
					/>
					{mode === 'create'
						? copy.groups.repayments.recording
						: copy.groups.repayments.saving}
				{:else}
					{mode === 'create'
						? copy.groups.repayments.record
						: copy.groups.repayments.save}
				{/if}
			</Button>
		</div>
		{#if !draftResult.ok}
			<p
				id="repayment-save-guidance"
				class="mx-auto mt-2 max-w-2xl text-right text-xs text-muted-foreground"
			>
				{copy.groups.repayments.saveGuidance}
			</p>
		{/if}
	</div>
</form>
