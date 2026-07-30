<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import TriangleAlertIcon from "@lucide/svelte/icons/triangle-alert";
	import { onDestroy, tick } from "svelte";

	import { dissolveGroup } from "$lib/api/groups";
	import {
		ApiError,
		isCsrfApiError,
		isForbiddenApiError,
		isNetworkApiError,
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/client";
	import type { GroupSummary } from "$lib/api/types";
	import * as Alert from "$lib/components/ui/alert";
	import * as AlertDialog from "$lib/components/ui/alert-dialog";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Field from "$lib/components/ui/field";
	import { Input } from "$lib/components/ui/input";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";

	let {
		group,
		onDissolved,
		onProtectedError
	}: {
		group: GroupSummary;
		onDissolved: () => void | Promise<void>;
		onProtectedError: (error: ApiError) => boolean | Promise<boolean>;
	} = $props();

	let open = $state(false);
	let confirmation = $state('');
	let pending = $state(false);
	let committed = $state(false);
	let formError = $state('');
	let postCommitError = $state(false);
	let triggerButton: HTMLButtonElement | null = $state(null);
	let confirmationInput: HTMLInputElement | null = $state(null);
	let formAlert: HTMLDivElement | null = $state(null);
	let controller: AbortController | null = null;

	const canDissolve = $derived(
		!pending && !committed && confirmation === group.name
	);

	onDestroy(() => controller?.abort());

	function handleOpenChange(nextOpen: boolean): void {
		if ((pending || committed) && !nextOpen) {
			return;
		}

		open = nextOpen;
		if (nextOpen) {
			confirmation = '';
			formError = '';
			postCommitError = false;
		}
	}

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!canDissolve) {
			await tick();
			confirmationInput?.focus();
			return;
		}

		pending = true;
		formError = '';
		controller?.abort();
		controller = new AbortController();
		try {
			await dissolveGroup(group.id, { signal: controller.signal });
		} catch (error) {
			if (controller.signal.aborted) {
				return;
			}
			if (
				error instanceof ApiError &&
				((isForbiddenApiError(error) && !isCsrfApiError(error)) ||
					isNotFoundApiError(error) ||
					isUnauthorizedApiError(error))
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
			formError = isNetworkApiError(error)
				? copy.auth.serviceUnavailableTitle
				: copy.groups.settings.danger.failure;
			pending = false;
			await tick();
			formAlert?.focus();
			return;
		}

		committed = true;
		pending = false;
		try {
			await onDissolved();
		} catch {
			postCommitError = true;
		}
	}
</script>

<Card.Root class="border-destructive/40" aria-labelledby="danger-zone-heading">
	<Card.Header>
		<Card.Title>
			<h2 id="danger-zone-heading">{copy.groups.settings.danger.title}</h2>
		</Card.Title>
		<Card.Description>{copy.groups.settings.danger.description}</Card.Description>
	</Card.Header>
	<Card.Content>
		<AlertDialog.Root bind:open onOpenChange={handleOpenChange}>
			<AlertDialog.Trigger>
				{#snippet child({ props })}
					<Button
						bind:ref={triggerButton}
						{...props}
						variant="destructive"
						class="min-h-11"
					>
						<TriangleAlertIcon data-icon="inline-start" />
						{copy.groups.settings.danger.action}
					</Button>
				{/snippet}
			</AlertDialog.Trigger>

			<AlertDialog.Content
				class="w-[calc(100%-2rem)] sm:max-w-md"
				onEscapeKeydown={(event) => {
					if (pending || committed) {
						event.preventDefault();
					}
				}}
				onOpenAutoFocus={(event) => {
					event.preventDefault();
					confirmationInput?.focus();
				}}
				onCloseAutoFocus={(event) => {
					event.preventDefault();
					if (!committed) {
						triggerButton?.focus();
					}
				}}
			>
				<AlertDialog.Header>
					<AlertDialog.Title>
						{copy.groups.settings.danger.dialogTitle}
					</AlertDialog.Title>
					<AlertDialog.Description>
						{copy.groups.settings.danger.dialogDescription}
						<strong class="mt-2 block break-words text-foreground">
							{group.name}
						</strong>
					</AlertDialog.Description>
				</AlertDialog.Header>

				<form id="dissolve-group-form" onsubmit={submit} novalidate>
					<Field.Group>
						{#if formError}
							<Alert.Root
								bind:ref={formAlert}
								variant="destructive"
								tabindex={-1}
							>
								<CircleAlertIcon data-icon="inline-start" />
								<Alert.Title>{formError}</Alert.Title>
							</Alert.Root>
						{/if}
						<Field.Field data-disabled={pending || committed ? true : undefined}>
							<Field.Label for="dissolve-group-confirmation">
								{copy.groups.settings.danger.confirmationLabel}
							</Field.Label>
							<Input
								bind:ref={confirmationInput}
								bind:value={confirmation}
								id="dissolve-group-confirmation"
								name="confirmation"
								type="text"
								autocomplete="off"
								disabled={pending || committed}
								aria-describedby="dissolve-group-confirmation-description"
							/>
							<Field.Description id="dissolve-group-confirmation-description">
								{copy.groups.settings.danger.confirmationDescription}
							</Field.Description>
						</Field.Field>
					</Field.Group>
				</form>

				{#if postCommitError}
					<Alert.Root variant="destructive">
						<CircleAlertIcon data-icon="inline-start" />
						<Alert.Title>
							{copy.groups.settings.danger.navigationFailure}
						</Alert.Title>
					</Alert.Root>
				{/if}

				<AlertDialog.Footer>
					<Button
						variant="outline"
						class="min-h-11"
						disabled={pending || committed}
						onclick={() => {
							open = false;
						}}
					>
						{copy.groups.settings.danger.cancel}
					</Button>
					<Button
						type="submit"
						form="dissolve-group-form"
						variant="destructive"
						class="min-h-11"
						disabled={!canDissolve}
					>
						{#if pending || committed}
							<Spinner
								data-icon="inline-start"
								aria-label={copy.groups.settings.danger.pending}
							/>
							{committed
								? copy.groups.settings.danger.success
								: copy.groups.settings.danger.pending}
						{:else}
							{copy.groups.settings.danger.submit}
						{/if}
					</Button>
					{#if postCommitError}
						<Button href="/groups" variant="outline" class="min-h-11">
							{copy.groups.settings.danger.continueToGroups}
						</Button>
					{/if}
				</AlertDialog.Footer>
			</AlertDialog.Content>
		</AlertDialog.Root>
	</Card.Content>
</Card.Root>
