<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import EyeIcon from "@lucide/svelte/icons/eye";
	import EyeOffIcon from "@lucide/svelte/icons/eye-off";
	import InfoIcon from "@lucide/svelte/icons/info";
	import { tick } from "svelte";

	import { login } from "$lib/api/auth";
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
		validateLoginForm,
		type LoginFieldErrors
	} from "$lib/utils/auth-form";
	import { safeNextPath } from "$lib/utils/safe-next";

	type FormAlert = {
		title: string;
		description?: string;
	};

	const loginFieldMap = {
		email: 'email',
		password: 'password'
	} as const;

	let email = $state('');
	let password = $state('');
	let passwordVisible = $state(false);
	let pending = $state(false);
	let fieldErrors = $state<LoginFieldErrors>({});
	let formAlert = $state<FormAlert | null>(null);
	let emailInput: HTMLInputElement | null = $state(null);
	let passwordInput: HTMLInputElement | null = $state(null);
	let formAlertElement: HTMLDivElement | null = $state(null);

	const sessionExpired = $derived(
		page.url.searchParams.get('reason') === 'session-expired'
	);

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (pending) {
			return;
		}

		fieldErrors = validateLoginForm({ email, password });
		formAlert = null;
		if (Object.keys(fieldErrors).length > 0) {
			await focusFailure();
			return;
		}

		pending = true;
		let failed = false;

		try {
			const user = await login({
				email: email.trim(),
				password
			});
			setAuthenticated(user);
			await goto(safeNextPath(page.url.searchParams.get('next')), {
				replaceState: true
			});
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

		if (error instanceof ApiError && error.status === 401) {
			formAlert = { title: copy.auth.login.invalidCredentials };
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
			const mapped = mapApiFieldErrors(error.fields, loginFieldMap);
			fieldErrors = mapped.fieldErrors;

			const unknownMessages = Object.values(mapped.unknownFieldErrors);
			if (unknownMessages.length > 0) {
				formAlert = {
					title: copy.auth.login.failure,
					description: unknownMessages.join(' ')
				};
			} else {
				formAlert = null;
			}
			return;
		}

		formAlert = { title: copy.auth.login.failure };
	}

	async function focusFailure(): Promise<void> {
		await tick();

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
			<h1 tabindex="-1">{copy.auth.login.title}</h1>
		</Card.Title>
		<Card.Description>{copy.auth.login.description}</Card.Description>
	</Card.Header>

	<Card.Content>
		<form id="login-form" onsubmit={submit} novalidate>
			<Field.Group>
				{#if sessionExpired}
					<Alert.Root>
						<InfoIcon data-icon="inline-start" />
						<Alert.Title>{copy.auth.login.sessionExpired}</Alert.Title>
					</Alert.Root>
				{/if}

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
					data-invalid={fieldErrors.email ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Label for="login-email">{copy.auth.login.emailLabel}</Field.Label>
					<Input
						bind:ref={emailInput}
						bind:value={email}
						id="login-email"
						name="email"
						type="email"
						autocomplete="username"
						autocapitalize="none"
						spellcheck={false}
						disabled={pending}
						required
						aria-invalid={fieldErrors.email ? true : undefined}
						aria-describedby={fieldErrors.email ? 'login-email-error' : undefined}
					/>
					{#if fieldErrors.email}
						<Field.Error id="login-email-error">{fieldErrors.email}</Field.Error>
					{/if}
				</Field.Field>

				<Field.Field
					data-invalid={fieldErrors.password ? true : undefined}
					data-disabled={pending ? true : undefined}
				>
					<Field.Label for="login-password">{copy.auth.login.passwordLabel}</Field.Label>
					<InputGroup.Root>
						<InputGroup.Input
							bind:ref={passwordInput}
							bind:value={password}
							id="login-password"
							name="password"
							type={passwordVisible ? 'text' : 'password'}
							autocomplete="current-password"
							disabled={pending}
							required
							aria-invalid={fieldErrors.password ? true : undefined}
							aria-describedby={fieldErrors.password ? 'login-password-error' : undefined}
						/>
						<InputGroup.Addon align="inline-end">
							<InputGroup.Button
								size="icon-sm"
								disabled={pending}
								aria-label={passwordVisible
									? copy.auth.login.hidePassword
									: copy.auth.login.showPassword}
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
					{#if fieldErrors.password}
						<Field.Error id="login-password-error">{fieldErrors.password}</Field.Error>
					{/if}
				</Field.Field>
			</Field.Group>
		</form>
	</Card.Content>

	<Card.Footer class="flex-col gap-4">
		<Button class="min-h-11 w-full" type="submit" form="login-form" disabled={pending}>
			{#if pending}
				<Spinner data-icon="inline-start" aria-label={copy.auth.login.pending} />
				{copy.auth.login.pending}
			{:else}
				{copy.auth.login.submit}
			{/if}
		</Button>
		<p class="text-center text-sm text-muted-foreground">
			{copy.auth.login.newAccountPrompt}
			<a class="font-medium text-foreground underline underline-offset-4" href="/register">
				{copy.auth.login.newAccountAction}
			</a>
		</p>
	</Card.Footer>
</Card.Root>
