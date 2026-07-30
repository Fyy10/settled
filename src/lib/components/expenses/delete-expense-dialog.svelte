<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import { onDestroy, tick } from "svelte";
	import { toast } from "svelte-sonner";

	import {
		ApiError,
		isNetworkApiError,
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/errors";
	import { deleteExpense } from "$lib/api/expenses";
	import * as Alert from "$lib/components/ui/alert";
	import * as AlertDialog from "$lib/components/ui/alert-dialog";
	import { Button } from "$lib/components/ui/button";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { MutationCoordinator } from "$lib/state/mutation-coordinator.svelte";
	import type { CommittedMutationResult } from "$lib/utils/committed-mutation";

	let {
		groupId,
		expenseId,
		activityHref,
		mutation,
		onCommitted,
		onNotFound,
		mutationDisabledReason = null
	}: {
		groupId: string;
		expenseId: string;
		activityHref: string;
		mutation: MutationCoordinator<'save' | 'delete'>;
		onCommitted: () => Promise<CommittedMutationResult>;
		onNotFound: () => 'record' | 'handled' | Promise<'record' | 'handled'>;
		mutationDisabledReason?: string | null;
	} = $props();

	let open = $state(false);
	let errorMessage = $state('');
	let recoveryNeeded = $state(false);
	let trigger: HTMLButtonElement | null = $state(null);
	let errorAlert: HTMLDivElement | null = $state(null);
	let controller: AbortController | null = null;

	const pending = $derived(
		mutation.phase === 'pending' && mutation.operation === 'delete'
	);
	const committed = $derived(mutation.phase === 'committed');
	const controlsLocked = $derived(mutation.phase !== 'idle');
	const triggerDisabled = $derived(
		pending || (controlsLocked && !recoveryNeeded)
	);

	onDestroy(() => controller?.abort());

	function changeOpen(nextOpen: boolean): void {
		if (pending && nextOpen !== open) {
			return;
		}
		open = nextOpen;
		if (nextOpen) {
			if (!controlsLocked) {
				errorMessage = '';
				recoveryNeeded = false;
			}
		}
	}

	async function remove(): Promise<void> {
		if (
			controlsLocked ||
			mutationDisabledReason !== null ||
			!mutation.begin('delete')
		) {
			return;
		}

		errorMessage = '';
		controller?.abort();
		controller = new AbortController();
		try {
			await deleteExpense(groupId, expenseId, {
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
				let resolution: 'record' | 'handled' = 'record';
				try {
					resolution = await onNotFound();
				} catch {
					// Keep the ordinary record-unavailable state.
				}
				if (resolution === 'handled') {
					mutation.markBlocked();
					open = false;
					return;
				}
				mutation.markBlocked();
				recoveryNeeded = true;
				errorMessage = copy.groups.expenses.recordUnavailableTitle;
				await focusError();
				return;
			}
			if (
				isNetworkApiError(error) ||
				(error instanceof ApiError && error.source === 'invalid-response')
			) {
				mutation.markAmbiguous();
				recoveryNeeded = true;
				errorMessage = copy.groups.expenses.ambiguousFailure;
				await focusError();
				return;
			}

			mutation.reset();
			errorMessage = copy.groups.expenses.deleteFailure;
			await focusError();
			return;
		}

		mutation.markCommitted();
		open = false;
		await tick();
		toast.success(copy.groups.expenses.deleted);
		let result: CommittedMutationResult;
		try {
			result = await onCommitted();
		} catch {
			result = { refresh: 'failed', navigation: 'failed' };
		}
		if (result.refresh === 'failed') {
			toast.error(copy.groups.expenses.refreshFailure);
		}
		if (result.navigation === 'failed') {
			recoveryNeeded = true;
			errorMessage = copy.groups.expenses.navigationFailure;
			open = true;
			await focusError();
			toast.error(copy.groups.expenses.navigationFailure);
		}
	}

	async function focusError(): Promise<void> {
		await tick();
		errorAlert?.focus();
	}
</script>

<AlertDialog.Root bind:open onOpenChange={changeOpen}>
	<AlertDialog.Trigger disabled={triggerDisabled}>
		{#snippet child({ props })}
			<Button
				bind:ref={trigger}
				{...props}
				variant="outline"
				class="min-h-11 text-destructive"
				disabled={triggerDisabled}
			>
				<Trash2Icon data-icon="inline-start" />
				{recoveryNeeded
					? copy.groups.expenses.reviewDeleteResult
					: copy.groups.expenses.delete}
			</Button>
		{/snippet}
	</AlertDialog.Trigger>

	<AlertDialog.Content
		class="w-[calc(100%-2rem)] sm:max-w-md"
		onEscapeKeydown={(event) => {
			if (pending) {
				event.preventDefault();
			}
		}}
		onCloseAutoFocus={(event) => {
			event.preventDefault();
			if (!triggerDisabled) {
				trigger?.focus();
			}
		}}
	>
		<AlertDialog.Header>
			<AlertDialog.Title>{copy.groups.expenses.deleteTitle}</AlertDialog.Title>
			<AlertDialog.Description>
				{copy.groups.expenses.deleteDescription}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if mutationDisabledReason !== null}
			<Alert.Root id="expense-delete-disabled">
				<CircleAlertIcon data-icon="inline-start" />
				<Alert.Title>{mutationDisabledReason}</Alert.Title>
			</Alert.Root>
		{/if}

		{#if errorMessage}
			<Alert.Root
				bind:ref={errorAlert}
				variant="destructive"
				tabindex={-1}
			>
				<CircleAlertIcon data-icon="inline-start" />
				<Alert.Title>{errorMessage}</Alert.Title>
				{#if recoveryNeeded}
					<Alert.Description>
						<a href={activityHref}>
							{copy.groups.expenses.continueToActivity}
						</a>
					</Alert.Description>
				{/if}
			</Alert.Root>
		{/if}

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={pending}>
				{copy.groups.expenses.deleteCancel}
			</AlertDialog.Cancel>
			<Button
				variant="destructive"
				disabled={controlsLocked || mutationDisabledReason !== null}
				aria-describedby={mutationDisabledReason !== null
					? 'expense-delete-disabled'
					: undefined}
				onclick={(event) => {
					event.preventDefault();
					void remove();
				}}
			>
				{#if pending}
					<Spinner aria-label={copy.groups.expenses.deleting} />
					{copy.groups.expenses.deleting}
				{:else}
					{copy.groups.expenses.deleteSubmit}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
