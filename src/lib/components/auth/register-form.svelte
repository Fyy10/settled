<script lang="ts">
	import { goto } from "$app/navigation";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import { tick } from "svelte";

	import { register } from "$lib/api/auth";
	import {
		ApiError,
		isNetworkApiError,
		mapApiFieldErrors
	} from "$lib/api/client";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Field from "$lib/components/ui/field";
	import { Input } from "$lib/components/ui/input";
	import * as InputGroup from "$lib/components/ui/input-group";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { setAuthenticated } from "$lib/state/auth.svelte";
	import {
		validateRegistrationForm,
		type RegistrationFieldErrors
	} from "$lib/utils/auth-form";

	type FormAlert = {
		title: string;
		description?: string;
	};

	const registrationFieldMap = {
		displayName: 'displayName',
		email: 'email',
		password: 'password'
	} as const;

	let displayName = $state('');
	let email = $state('');
	let password = $state('');
	let passwordVisible = $state(false);
	let pending = $state(false);
	let fieldErrors = $state<RegistrationFieldErrors>({});
	let formAlert = $state<FormAlert | null>(null);
	let displayNameInput: HTMLInputElement | null = $state(null);
	let emailInput: HTMLInputElement | null = $state(null);
	let passwordInput: HTMLInputElement | null = $state(null);
	let formAlertElement: HTMLDivElement | null = $state(null);

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (pending) {
			return;
		}

		fieldErrors = validateRegistrationForm({ displayName, email, password });
		formAlert = null;
		if (Object.keys(fieldErrors).length > 0) {
			await focusFailure();
			return;
		}

		pending = true;
		let failed = false;

		try {
			const user = await register({
				displayName: displayName.trim(),
				email: email.trim(),
				password
			});
			setAuthenticated(user);
			await goto('/groups', { replaceState: true });
		} catch (error) {
			applyApiError(error);
			failed = true;
		} finally {
			pending = false;
		}

		if (failed) {
			await focusFailure();
		}
	}

	function applyApiError(error: unknown): void {
		fieldErrors = {};

		if (error instanceof ApiError && error.status === 409) {
			fieldErrors = { email: copy.auth.register.emailConflict };
			formAlert = null;
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
			const mapped = mapApiFieldErrors(error.fields, registrationFieldMap);
			fieldErrors = mapped.fieldErrors;

			const unknownMessages = Object.values(mapped.unknownFieldErrors);
			if (unknownMessages.length > 0) {
				formAlert = {
					title: copy.auth.register.failure,
					description: unknownMessages.join(' ')
				};
			} else {
				formAlert = null;
			}
			return;
		}

		formAlert = { title: copy.auth.register.failure };
	}

	async function focusFailure(): Promise<void> {
		await tick();

		if (fieldErrors.displayName) {
			displayNameInput?.focus();
			return;
		}
		if (fieldErrors.email) {
			emailInput?.focus();
			return;
		}
		if (fieldErrors.password) {
			passwordInput?.focus();
			return;
		}

		formAlertElement?.focus();
	}
</script>

<Card.Root class="w-full">
	<Card.Header class="border-b">
		<Card.Title>
			<h1 tabindex="-1">{copy.auth.register.title}</h1>
		</Card.Title>
		<Card.Description>{copy.auth.register.description}</Card.Description>
	</Card.Header>

	<Card.Content>
		<form id="register-form" onsubmit={submit} novalidate>
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
					data-invalid={fieldErrors.displayName ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Label for="register-display-name">
						{copy.auth.register.displayNameLabel}
					</Field.Label>
					<Input
						bind:ref={displayNameInput}
						bind:value={displayName}
						id="register-display-name"
						name="displayName"
						type="text"
						autocomplete="name"
						disabled={pending}
						required
						aria-invalid={fieldErrors.displayName ? true : undefined}
						aria-describedby={fieldErrors.displayName
							? 'register-display-name-error'
							: undefined}
					/>
					{#if fieldErrors.displayName}
						<Field.Error id="register-display-name-error">
							{fieldErrors.displayName}
						</Field.Error>
					{/if}
				</Field.Field>

				<Field.Field
					data-invalid={fieldErrors.email ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Label for="register-email">{copy.auth.register.emailLabel}</Field.Label>
					<Input
						bind:ref={emailInput}
						bind:value={email}
						id="register-email"
						name="email"
						type="email"
						autocomplete="username"
						autocapitalize="none"
						spellcheck={false}
						disabled={pending}
						required
						aria-invalid={fieldErrors.email ? true : undefined}
						aria-describedby={fieldErrors.email ? 'register-email-error' : undefined}
					/>
					{#if fieldErrors.email}
						<Field.Error id="register-email-error">{fieldErrors.email}</Field.Error>
					{/if}
				</Field.Field>

				<Field.Field
					data-invalid={fieldErrors.password ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Label for="register-password">
						{copy.auth.register.passwordLabel}
					</Field.Label>
					<InputGroup.Root>
						<InputGroup.Input
							bind:ref={passwordInput}
							bind:value={password}
							id="register-password"
							name="password"
							type={passwordVisible ? 'text' : 'password'}
							autocomplete="new-password"
							disabled={pending}
							required
							aria-invalid={fieldErrors.password ? true : undefined}
							aria-describedby={fieldErrors.password
								? 'register-password-help register-password-error'
								: 'register-password-help'}
						/>
						<InputGroup.Addon align="inline-end">
							<InputGroup.Button
								size="icon-sm"
								disabled={pending}
								aria-label={passwordVisible
									? copy.auth.register.hidePassword
									: copy.auth.register.showPassword}
								aria-pressed={passwordVisible}
								onclick={() => (passwordVisible = !passwordVisible)}
							>
								{#if passwordVisible}
									<EyeOffIcon data-icon="inline-start" />
								{:else}
									<EyeIcon data-icon="inline-start" />
								{/if}
							</InputGroup.Button>
						</InputGroup.Addon>
					</InputGroup.Root>
					<Field.Description id="register-password-help">
						{copy.auth.register.passwordGuidance}
					</Field.Description>
					{#if fieldErrors.password}
						<Field.Error id="register-password-error">{fieldErrors.password}</Field.Error>
					{/if}
				</Field.Field>
			</Field.Group>
		</form>
	</Card.Content>

	<Card.Footer class="flex-col gap-4">
		<Button class="min-h-11 w-full" type="submit" form="register-form" disabled={pending}>
			{#if pending}
				<Spinner data-icon="inline-start" aria-label={copy.auth.register.pending} />
				{copy.auth.register.pending}
			{:else}
				{copy.auth.register.submit}
			{/if}
		</Button>
		<p class="text-center text-sm text-muted-foreground">
			{copy.auth.register.existingAccountPrompt}
			<a class="font-medium text-foreground underline underline-offset-4" href="/login">
				{copy.auth.register.existingAccountAction}
			</a>
		</p>
	</Card.Footer>
</Card.Root>
