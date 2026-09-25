<script lang="ts">
	import { registerDirtyForm } from "$lib/state/dirty-forms.svelte";
	import { networkState } from "$lib/state/network.svelte";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import UserMinusIcon from "@lucide/svelte/icons/user-minus";
	import { onDestroy, tick } from "svelte";
	import { toast } from "svelte-sonner";

	import { removeGroupMember } from "$lib/api/groups";
	import {
		ApiError,
		isConflictApiError,
		isCsrfApiError,
		isForbiddenApiError,
		isNetworkApiError,
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/client";
	import type { GroupMember } from "$lib/api/types";
	import * as Alert from "$lib/components/ui/alert";
	import * as AlertDialog from "$lib/components/ui/alert-dialog";
	import { Button } from "$lib/components/ui/button";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";

	import type { MemberNotFoundResolution } from "./member-removal";

	let {
		groupId,
		member,
		onCommitted,
		onNotFound,
		onProtectedError,
		onCloseFocus
	}: {
		groupId: string;
		member: GroupMember;
		onCommitted: (userId: string) => void | Promise<void>;
		onNotFound: (
			userId: string
		) => MemberNotFoundResolution | Promise<MemberNotFoundResolution>;
		onProtectedError: (error: ApiError) => boolean | Promise<boolean>;
		onCloseFocus: () => void;
	} = $props();

	let open = $state(false);
	let pending = $state(false);
	const mutationRegistration = registerDirtyForm();
	$effect.pre(() => mutationRegistration.update(false, pending));
	onDestroy(() => mutationRegistration.unregister());
	let committed = $state(false);
	let errorMessage = $state('');
	let triggerButton: HTMLButtonElement | null = $state(null);
	let errorAlert: HTMLDivElement | null = $state(null);
	let controller: AbortController | null = null;

	onDestroy(() => controller?.abort());

	function handleOpenChange(nextOpen: boolean): void {
		if ((pending || committed) && nextOpen !== open) {
			return;
		}

		open = nextOpen;
		if (nextOpen) {
			errorMessage = '';
		}
	}

	async function removeMember(): Promise<void> {
		if (!networkState.online || pending || committed) {
			return;
		}

		pending = true;
		mutationRegistration.update(false, true);
		errorMessage = '';
		controller?.abort();
		controller = new AbortController();
		try {
			await removeGroupMember(groupId, member.userId, {
				signal: controller.signal
			});
		} catch (error) {
			if (controller.signal.aborted) {
				return;
			}
			if (isConflictApiError(error)) {
				errorMessage = copy.groups.workspace.members.removeConflict;
				pending = false;
				await focusError();
				return;
			}
			if (isNotFoundApiError(error)) {
				let resolution: MemberNotFoundResolution = 'unresolved';
				try {
					resolution = await onNotFound(member.userId);
				} catch {
					// The DELETE was not committed; retain the retryable dialog.
				}
				if (resolution === 'removed') {
					await completeRemoval();
					return;
				}
				if (resolution === 'handled') {
					pending = false;
					open = false;
					return;
				}
				errorMessage = copy.groups.workspace.members.removeFailure;
				pending = false;
				await focusError();
				return;
			}
			if (
				error instanceof ApiError &&
				(isUnauthorizedApiError(error) ||
					(isForbiddenApiError(error) && !isCsrfApiError(error)))
			) {
				try {
					if (await onProtectedError(error)) {
						pending = false;
						open = false;
						return;
					}
				} catch {
					// Fall through to the ordinary pre-commit error.
				}
			}

			errorMessage = isNetworkApiError(error)
				? copy.auth.serviceUnavailableTitle
				: copy.groups.workspace.members.removeFailure;
			pending = false;
			await focusError();
			return;
		}

		await completeRemoval();
	}

	async function completeRemoval(): Promise<void> {
		committed = true;
		pending = false;
		open = false;
		await tick();
		onCloseFocus();
		toast.success(copy.groups.workspace.members.removed);
		try {
			await onCommitted(member.userId);
		} catch {
			toast.error(copy.groups.workspace.members.removeRefreshFailure);
		}
	}

	async function focusError(): Promise<void> {
		await tick();
		errorAlert?.focus();
	}
</script>

<AlertDialog.Root bind:open onOpenChange={handleOpenChange}>
	<AlertDialog.Trigger disabled={committed}>
		{#snippet child({ props })}
			<Button
				bind:ref={triggerButton}
				{...props}
				variant="outline"
				class="min-h-10 text-destructive"
				disabled={committed}
				aria-label={`${copy.groups.workspace.members.remove} ${member.displayName}`}
			>
				<UserMinusIcon data-icon="inline-start" />
				{committed
					? copy.groups.workspace.members.removed
					: copy.groups.workspace.members.remove}
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
			if (committed) {
				onCloseFocus();
			} else {
				triggerButton?.focus();
			}
		}}
	>
		<AlertDialog.Header>
			<AlertDialog.Title>
				Remove {member.displayName}?
			</AlertDialog.Title>
			<AlertDialog.Description>
				{member.displayName} will lose access to this group and its history.
			</AlertDialog.Description>
		</AlertDialog.Header>

		{#if errorMessage}
			<Alert.Root
				bind:ref={errorAlert}
				variant="destructive"
				tabindex={-1}
			>
				<CircleAlertIcon data-icon="inline-start" />
				<Alert.Title>{errorMessage}</Alert.Title>
			</Alert.Root>
		{/if}

		{#if !networkState.online}
			<Alert.Root id="remove-member-dialog-offline-reason">
				<Alert.Title>{networkState.mutationDisabledReason}</Alert.Title>
			</Alert.Root>
		{/if}

		<AlertDialog.Footer>
			<Button
				variant="outline"
				class="min-h-11"
				disabled={pending}
				onclick={() => {
					open = false;
				}}
			>
				{copy.groups.workspace.members.removeCancel}
			</Button>
			<Button
				aria-describedby={!networkState.online ? "remove-member-dialog-offline-reason" : undefined}
				variant="destructive"
				class="min-h-11"
				disabled={!networkState.online || pending}
				onclick={() => void removeMember()}
			>
				{#if pending}
					<Spinner
						data-icon="inline-start"
						aria-label={copy.groups.workspace.members.removePending}
					/>
					{copy.groups.workspace.members.removePending}
				{:else}
					{copy.groups.workspace.members.removeSubmit}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
