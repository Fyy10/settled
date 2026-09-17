<script lang="ts">
	import { registerDirtyForm } from "$lib/state/dirty-forms.svelte";
	import { networkState } from "$lib/state/network.svelte";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import { onDestroy, tick, untrack } from "svelte";
	import { toast } from "svelte-sonner";

	import { renameGroup } from "$lib/api/groups";
	import {
		ApiError,
		isCsrfApiError,
		isForbiddenApiError,
		isNetworkApiError,
		isNotFoundApiError,
		isUnauthorizedApiError,
		mapApiFieldErrors
	} from "$lib/api/client";
	import type { GroupSummary } from "$lib/api/types";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
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
		group,
		onSaved,
		onProtectedError
	}: {
		group: GroupSummary;
		onSaved: (group: GroupSummary) => boolean | Promise<boolean>;
		onProtectedError: (error: ApiError) => boolean | Promise<boolean>;
	} = $props();

	const renameFieldMap = { name: 'name' } as const;

	let name = $state('');
	let syncedName = $state('');
	let pending = $state(false);
	const mutationRegistration = registerDirtyForm();
	$effect.pre(() => mutationRegistration.update(name !== syncedName, pending));
	onDestroy(() => mutationRegistration.unregister());
	let fieldErrors = $state<CreateGroupFieldErrors>({});
	let formAlert = $state<FormAlert | null>(null);
	let nameInput: HTMLInputElement | null = $state(null);
	let formAlertElement: HTMLDivElement | null = $state(null);
	let controller: AbortController | null = null;

	$effect.pre(() => {
		const currentName = group.name;
		if (currentName === syncedName) {
			return;
		}

		untrack(() => {
			syncedName = currentName;
			if (!pending) {
				name = currentName;
				fieldErrors = {};
			}
		});
	});

	onDestroy(() => controller?.abort());

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
		controller?.abort();
		controller = new AbortController();
		let renamed: GroupSummary;
		try {
			renamed = await renameGroup(
				group.id,
				{ name: name.trim() },
				{ signal: controller.signal }
			);
		} catch (error) {
			if (controller.signal.aborted) {
				return;
			}
			if (await handleProtectedError(error)) {
				pending = false;
				return;
			}
			applyApiError(error);
			pending = false;
			await focusFailure();
			return;
		}

		syncedName = renamed.name;
		name = renamed.name;
		fieldErrors = {};
		let refreshed = false;
		try {
			refreshed = await onSaved(renamed);
		} catch {
			// The rename is committed; only its explicit revalidation failed.
		}
		pending = false;
		toast.success(copy.groups.settings.name.success);
		if (!refreshed) {
			formAlert = { title: copy.groups.settings.name.refreshFailure };
		}
	}

	async function handleProtectedError(error: unknown): Promise<boolean> {
		if (
			error instanceof ApiError &&
			((isForbiddenApiError(error) && !isCsrfApiError(error)) ||
				isNotFoundApiError(error) ||
				isUnauthorizedApiError(error))
		) {
			return await onProtectedError(error);
		}

		return false;
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
			const mapped = mapApiFieldErrors(error.fields, renameFieldMap);
			fieldErrors = mapped.fieldErrors;
			const unknownMessages = Object.values(mapped.unknownFieldErrors);
			formAlert =
				unknownMessages.length > 0
					? {
							title: copy.groups.settings.name.failure,
							description: unknownMessages.join(' ')
						}
					: null;
			return;
		}

		formAlert = { title: copy.groups.settings.name.failure };
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

<Card.Root aria-labelledby="group-name-heading">
	<Card.Header>
		<Card.Title>
			<h2 id="group-name-heading">{copy.groups.settings.name.title}</h2>
		</Card.Title>
		<Card.Description>{copy.groups.settings.name.description}</Card.Description>
	</Card.Header>
	<Card.Content>
		<form onsubmit={submit} novalidate>
			{#if !networkState.online}
				<Alert.Root id="rename-group-form-offline-reason">
					<Alert.Title>{networkState.mutationDisabledReason}</Alert.Title>
				</Alert.Root>
			{/if}

			<Field.Group>
				{#if formAlert}
					<Alert.Root
						bind:ref={formAlertElement}
						variant={formAlert.title === copy.groups.settings.name.refreshFailure
							? 'default'
							: 'destructive'}
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
					orientation="horizontal"
					data-invalid={fieldErrors.name ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Content>
						<Field.Label for="settings-group-name">
							{copy.groups.settings.name.label}
						</Field.Label>
						<Input
							bind:ref={nameInput}
							bind:value={name}
							id="settings-group-name"
							name="name"
							type="text"
							autocomplete="off"
							disabled={pending}
							required
							aria-invalid={fieldErrors.name ? true : undefined}
							aria-describedby={fieldErrors.name
								? 'settings-group-name-error'
								: undefined}
						/>
						{#if fieldErrors.name}
							<Field.Error id="settings-group-name-error">
								{fieldErrors.name}
							</Field.Error>
						{/if}
					</Field.Content>
					<Button
						aria-describedby={!networkState.online ? "rename-group-form-offline-reason" : undefined}
						type="submit"
						class="min-h-11 sm:self-end"
						disabled={!networkState.online || pending}
					>
						{#if pending}
							<Spinner
								data-icon="inline-start"
								aria-label={copy.groups.settings.name.pending}
							/>
							{copy.groups.settings.name.pending}
						{:else}
							{copy.groups.settings.name.submit}
						{/if}
					</Button>
				</Field.Field>
			</Field.Group>
		</form>
	</Card.Content>
</Card.Root>
