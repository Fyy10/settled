<script lang="ts">
	import { registerDirtyForm } from "$lib/state/dirty-forms.svelte";
	import { networkState } from "$lib/state/network.svelte";
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
	import { deleteRepayment } from "$lib/api/repayments";
	import * as Alert from "$lib/components/ui/alert";
	import * as AlertDialog from "$lib/components/ui/alert-dialog";
	import { Button } from "$lib/components/ui/button";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { MutationCoordinator } from "$lib/state/mutation-coordinator.svelte";
	import type { CommittedMutationResult } from "$lib/utils/committed-mutation";

	let {
		groupId,
		repaymentId,
		activityHref,
		mutation,
		onCommitted,
		onNotFound,
		mutationDisabledReason: externalMutationDisabledReason = null
	}: {
		groupId: string;
		repaymentId: string;
		activityHref: string;
		mutation: MutationCoordinator<'save' | 'delete'>;
		onCommitted: () => Promise<CommittedMutationResult>;
		onNotFound: () =>
			| 'record'
			| 'handled'
			| Promise<'record' | 'handled'>;
		mutationDisabledReason?: string | null;
	} = $props();

	let open = $state(false);
	let errorMessage = $state('');
	let recoveryNeeded = $state(false);
	let trigger: HTMLButtonElement | null = $state(null);
	let errorAlert: HTMLDivElement | null = $state(null);
	let controller: AbortController | null = null;

	const mutationDisabledReason = $derived(networkState.mutationDisabledReason ?? externalMutationDisabledReason);

	const pending = $derived(
		mutation.phase === 'pending' && mutation.operation === 'delete'
	);
	const controlsLocked = $derived(mutation.phase !== 'idle');
	const triggerDisabled = $derived(
		pending || (controlsLocked && !recoveryNeeded)
	);

	const mutationRegistration = registerDirtyForm();
	$effect.pre(() => mutationRegistration.update(false, pending));
	onDestroy(() => mutationRegistration.unregister());

	onDestroy(() => controller?.abort());

	function changeOpen(nextOpen: boolean): void {
		if (pending && nextOpen !== open) {
			return;
		}
		open = nextOpen;
		if (nextOpen && !controlsLocked) {
			errorMessage = '';
			recoveryNeeded = false;
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
			await deleteRepayment(groupId, repaymentId, {
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
			if (
				isNetworkApiError(error) ||
				(error instanceof ApiError &&
					error.source === 'invalid-response')
			) {
				mutation.markAmbiguous();
				recoveryNeeded = true;
				errorMessage = copy.groups.repayments.ambiguousFailure;
				await focusError();
				return;
			}

			mutation.reset();
			errorMessage = copy.groups.repayments.deleteFailure;
			await focusError();
			return;
		}

		mutation.markCommitted();
		open = false;
		await tick();
		toast.success(copy.groups.repayments.deleted);
		let result: CommittedMutationResult;
		try {
			result = await onCommitted();
		} catch {
			result = { refresh: 'failed', navigation: 'failed' };
		}
		if (result.refresh === 'failed') {
			toast.error(copy.groups.repayments.deleteRefreshFailure);
		}
		if (result.navigation === 'failed') {
			recoveryNeeded = true;
			errorMessage =
				copy.groups.repayments.deleteNavigationFailure;
			open = true;
			await focusError();
			toast.error(
				copy.groups.repayments.deleteNavigationFailure
			);
		}
	}

	async function handleNotFound(): Promise<void> {
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
		errorMessage = copy.groups.repayments.recordUnavailableTitle;
		await focusError();
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
					? copy.groups.repayments.reviewDeleteResult
					: copy.groups.repayments.delete}
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
			<AlertDialog.Title>
				{copy.groups.repayments.deleteTitle}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{copy.groups.repayments.deleteDescription}
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if mutationDisabledReason !== null}
			<Alert.Root id="repayment-delete-disabled">
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
							{copy.groups.repayments.continueToActivity}
						</a>
					</Alert.Description>
				{/if}
			</Alert.Root>
		{/if}

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={pending}>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="outline"
						disabled={pending}
					>
						{copy.groups.repayments.deleteCancel}
					</Button>
				{/snippet}
			</AlertDialog.Cancel>
			<Button
				variant="destructive"
				disabled={controlsLocked || mutationDisabledReason !== null}
				aria-describedby={mutationDisabledReason !== null
					? 'repayment-delete-disabled'
					: undefined}
				onclick={(event) => {
					event.preventDefault();
					void remove();
				}}
			>
				{#if pending}
					<Spinner aria-label={copy.groups.repayments.deleting} />
					{copy.groups.repayments.deleting}
				{:else}
					{copy.groups.repayments.deleteSubmit}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
