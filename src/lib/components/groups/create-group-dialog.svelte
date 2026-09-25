<script lang="ts">
	import { registerDirtyForm } from "$lib/state/dirty-forms.svelte";
	import { networkState } from "$lib/state/network.svelte";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import { onDestroy, tick } from "svelte";

	import { createGroup } from "$lib/api/groups";
	import {
		ApiError,
		isNetworkApiError,
		mapApiFieldErrors
	} from "$lib/api/client";
	import type { GroupSummary } from "$lib/api/types";
	import * as Alert from "$lib/components/ui/alert";
	import { Button, type ButtonVariant } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import * as Field from "$lib/components/ui/field";
	import { Input } from "$lib/components/ui/input";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import {
		validateCreateGroupName,
		type CreateGroupFieldErrors
	} from "$lib/utils/group";

	type FormAlert = {
		title: string;
		description?: string;
	};

	let {
		onSuccess,
		triggerVariant = 'default'
	}: {
		onSuccess: (group: GroupSummary) => void | Promise<void>;
		triggerVariant?: ButtonVariant;
	} = $props();

	const createFieldMap = { name: 'name' } as const;

	let open = $state(false);
	let name = $state('');
	let pending = $state(false);
	const mutationRegistration = registerDirtyForm();
	$effect.pre(() => mutationRegistration.update(open && Boolean(name), pending));
	onDestroy(() => mutationRegistration.unregister());
	let fieldErrors = $state<CreateGroupFieldErrors>({});
	let formAlert = $state<FormAlert | null>(null);
	let triggerButton: HTMLButtonElement | null = $state(null);
	let nameInput: HTMLInputElement | null = $state(null);
	let formAlertElement: HTMLDivElement | null = $state(null);

	function handleOpenChange(nextOpen: boolean): void {
		open = nextOpen;
		if (nextOpen) {
			name = '';
			fieldErrors = {};
			formAlert = null;
		}
	}

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!networkState.online || pending) {
			return;
		}

		fieldErrors = validateCreateGroupName(name);
		formAlert = null;
		if (fieldErrors.name) {
			await focusFailure();
			return;
		}

		pending = true;
		mutationRegistration.update(false, true);
		let group: GroupSummary;
		try {
			group = await createGroup({ name: name.trim() });
		} catch (error) {
			applyApiError(error);
			pending = false;
			await focusFailure();
			return;
		}

		name = '';
		pending = false;
		open = false;
		await onSuccess(group);
	}

	function applyApiError(error: unknown): void {
		fieldErrors = {};

		if (isNetworkApiError(error)) {
			formAlert = {
				title: copy.auth.serviceUnavailableTitle,
				description: copy.auth.serviceUnavailableDescription
			};
			return;
		}

		if (error instanceof ApiError && Object.keys(error.fields).length > 0) {
			const mapped = mapApiFieldErrors(error.fields, createFieldMap);
			fieldErrors = mapped.fieldErrors;

			const unknownMessages = Object.values(mapped.unknownFieldErrors);
			formAlert =
				unknownMessages.length > 0
					? {
							title: copy.groups.create.failure,
							description: unknownMessages.join(' ')
						}
					: null;
			return;
		}

		formAlert = { title: copy.groups.create.failure };
	}

	async function focusFailure(): Promise<void> {
		await tick();
		if (fieldErrors.name) {
			nameInput?.focus();
			return;
		}

		formAlertElement?.focus();
	}
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
	<Dialog.Trigger>
		{#snippet child({ props })}
			<Button
				bind:ref={triggerButton}
				{...props}
				variant={triggerVariant}
				class="min-h-11"
				disabled={pending}
			>
				<PlusIcon data-icon="inline-start" />
				{copy.groups.createAction}
			</Button>
		{/snippet}
	</Dialog.Trigger>

	<Dialog.Content
		class="sm:max-w-md"
		showCloseButton={!pending}
		onEscapeKeydown={(event) => {
			if (pending) {
				event.preventDefault();
			}
		}}
		onInteractOutside={(event) => {
			if (pending) {
				event.preventDefault();
			}
		}}
		onOpenAutoFocus={(event) => {
			event.preventDefault();
			nameInput?.focus();
		}}
		onCloseAutoFocus={(event) => {
			event.preventDefault();
			triggerButton?.focus();
		}}
	>
		<Dialog.Header>
			<Dialog.Title>{copy.groups.create.title}</Dialog.Title>
			<Dialog.Description>{copy.groups.create.description}</Dialog.Description>
		</Dialog.Header>

		<form id="create-group-form" onsubmit={submit} novalidate>
			{#if !networkState.online}
				<Alert.Root id="create-group-dialog-offline-reason">
					<Alert.Title>{networkState.mutationDisabledReason}</Alert.Title>
				</Alert.Root>
			{/if}

			<Field.Group>
				{#if formAlert}
					<Alert.Root
						bind:ref={formAlertElement}
						variant="destructive"
						tabindex={-1}
					>
						<CircleAlertIcon data-icon="inline-start" />
						<Alert.Title>{formAlert.title}</Alert.Title>
						{#if formAlert.description}
							<Alert.Description>{formAlert.description}</Alert.Description>
						{/if}
					</Alert.Root>
				{/if}

				<Field.Field
					data-invalid={fieldErrors.name ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Label for="create-group-name">
						{copy.groups.create.nameLabel}
					</Field.Label>
					<Input
						bind:ref={nameInput}
						bind:value={name}
						id="create-group-name"
						name="name"
						type="text"
						autocomplete="off"
						disabled={pending}
						required
						aria-invalid={fieldErrors.name ? true : undefined}
						aria-describedby={fieldErrors.name ? 'create-group-name-error' : undefined}
					/>
					{#if fieldErrors.name}
						<Field.Error id="create-group-name-error">{fieldErrors.name}</Field.Error>
					{/if}
				</Field.Field>
			</Field.Group>
		</form>

		<Dialog.Footer>
			<Dialog.Close>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="outline"
						class="min-h-11"
						disabled={pending}
					>
						{copy.groups.create.cancel}
					</Button>
				{/snippet}
			</Dialog.Close>
			<Button
				aria-describedby={!networkState.online ? "create-group-dialog-offline-reason" : undefined}
				class="min-h-11"
				type="submit"
				form="create-group-form"
				disabled={!networkState.online || pending}
			>
				{#if pending}
					<Spinner
						data-icon="inline-start"
						aria-label={copy.groups.create.pending}
					/>
					{copy.groups.create.pending}
				{:else}
					{copy.groups.create.submit}
				{/if}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
