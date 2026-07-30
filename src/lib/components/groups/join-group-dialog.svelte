<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import UserPlusIcon from "@lucide/svelte/icons/user-plus";
	import { tick } from "svelte";

	import { joinGroup } from "$lib/api/groups";
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
		normalizeJoinCode,
		validateJoinCode,
		type JoinGroupFieldErrors
	} from "$lib/utils/group";

	type FormAlert = {
		title: string;
		description?: string;
	};

	let {
		onSuccess,
		triggerVariant = 'outline'
	}: {
		onSuccess: (group: GroupSummary) => void | Promise<void>;
		triggerVariant?: ButtonVariant;
	} = $props();

	const joinFieldMap = { joinCode: 'joinCode' } as const;

	let open = $state(false);
	let joinCode = $state('');
	let pending = $state(false);
	let fieldErrors = $state<JoinGroupFieldErrors>({});
	let formAlert = $state<FormAlert | null>(null);
	let triggerButton: HTMLButtonElement | null = $state(null);
	let joinCodeInput: HTMLInputElement | null = $state(null);
	let formAlertElement: HTMLDivElement | null = $state(null);

	function handleOpenChange(nextOpen: boolean): void {
		open = nextOpen;
		if (nextOpen) {
			joinCode = '';
			fieldErrors = {};
			formAlert = null;
		}
	}

	function updateJoinCode(value: string): void {
		joinCode = normalizeJoinCode(value);
		if (fieldErrors.joinCode) {
			fieldErrors = {};
		}
	}

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (pending) {
			return;
		}

		fieldErrors = validateJoinCode(joinCode);
		formAlert = null;
		if (fieldErrors.joinCode) {
			await focusFailure();
			return;
		}

		pending = true;
		let group: GroupSummary;
		try {
			group = await joinGroup({ joinCode });
		} catch (error) {
			applyApiError(error);
			pending = false;
			await focusFailure();
			return;
		}

		joinCode = '';
		pending = false;
		open = false;
		await onSuccess(group);
	}

	function applyApiError(error: unknown): void {
		fieldErrors = {};

		if (error instanceof ApiError && error.status === 404) {
			formAlert = { title: copy.groups.join.invalidCode };
			return;
		}

		if (isNetworkApiError(error)) {
			formAlert = {
				title: copy.auth.serviceUnavailableTitle,
				description: copy.auth.serviceUnavailableDescription
			};
			return;
		}

		if (error instanceof ApiError && Object.keys(error.fields).length > 0) {
			const mapped = mapApiFieldErrors(error.fields, joinFieldMap);
			fieldErrors = mapped.fieldErrors;

			const unknownMessages = Object.values(mapped.unknownFieldErrors);
			formAlert =
				unknownMessages.length > 0
					? {
							title: copy.groups.join.failure,
							description: unknownMessages.join(' ')
						}
					: null;
			return;
		}

		formAlert = { title: copy.groups.join.failure };
	}

	async function focusFailure(): Promise<void> {
		await tick();
		if (fieldErrors.joinCode) {
			joinCodeInput?.focus();
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
				<UserPlusIcon data-icon="inline-start" />
				{copy.groups.joinAction}
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
			joinCodeInput?.focus();
		}}
		onCloseAutoFocus={(event) => {
			event.preventDefault();
			triggerButton?.focus();
		}}
	>
		<Dialog.Header>
			<Dialog.Title>{copy.groups.join.title}</Dialog.Title>
			<Dialog.Description>{copy.groups.join.description}</Dialog.Description>
		</Dialog.Header>

		<form id="join-group-form" onsubmit={submit} novalidate>
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
					data-invalid={fieldErrors.joinCode ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Label for="join-group-code">
						{copy.groups.join.codeLabel}
					</Field.Label>
					<Input
						bind:ref={joinCodeInput}
						value={joinCode}
						id="join-group-code"
						name="joinCode"
						type="text"
						autocomplete="off"
						autocapitalize="characters"
						spellcheck={false}
						disabled={pending}
						required
						aria-invalid={fieldErrors.joinCode ? true : undefined}
						aria-describedby={fieldErrors.joinCode ? 'join-group-code-error' : undefined}
						oninput={(event) => updateJoinCode(event.currentTarget.value)}
					/>
					{#if fieldErrors.joinCode}
						<Field.Error id="join-group-code-error">
							{fieldErrors.joinCode}
						</Field.Error>
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
						{copy.groups.join.cancel}
					</Button>
				{/snippet}
			</Dialog.Close>
			<Button
				class="min-h-11"
				type="submit"
				form="join-group-form"
				disabled={pending}
			>
				{#if pending}
					<Spinner
						data-icon="inline-start"
						aria-label={copy.groups.join.pending}
					/>
					{copy.groups.join.pending}
				{:else}
					{copy.groups.join.submit}
				{/if}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
