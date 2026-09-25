<script lang="ts">
	import { networkState } from "$lib/state/network.svelte";
	import { goto } from "$app/navigation";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import LogOutIcon from "@lucide/svelte/icons/log-out";
	import { onDestroy, tick } from "svelte";

	import { logout } from "$lib/api/auth";
	import type { User } from "$lib/api/types";
	import favicon from "$lib/assets/favicon.svg";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
	import { Separator } from "$lib/components/ui/separator";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { clearSession } from "$lib/state/auth.svelte";
	import {
		registerDirtyForm,
		hasBlockingForms,
		hasDirtyForms
	} from "$lib/state/dirty-forms.svelte";

	let { user }: { user: User } = $props();

	let menuOpen = $state(false);
	let pending = $state(false);
	const mutationRegistration = registerDirtyForm();
	$effect.pre(() => mutationRegistration.update(false, pending));
	onDestroy(() => mutationRegistration.unregister());
	let failure = $state(false);
	let failureAlert: HTMLDivElement | null = $state(null);

	async function submitLogout(): Promise<void> {
		if (!networkState.online || pending || hasBlockingForms.current) {
			return;
		}
		if (
			hasDirtyForms.current &&
			!globalThis.confirm(copy.auth.accountMenu.logoutDirtyConfirmation)
		) {
			return;
		}

		pending = true;
		mutationRegistration.update(false, true);
		failure = false;

		try {
			await logout();
			clearSession();
			await goto('/login', { replaceState: true });
		} catch {
			pending = false;
			menuOpen = false;
			failure = true;
			await tick();
			failureAlert?.focus();
		}
	}
</script>

<header class="bg-background">
	<div class="mx-auto flex min-h-14 w-full max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
		<a
			href="/groups"
			class="flex min-h-11 min-w-0 items-center gap-3 font-semibold"
			aria-label={copy.navigation.groupsHome}
		>
			<img src={favicon} alt="" class="size-8 shrink-0" />
			<span class="truncate tracking-tight">{copy.appName}</span>
		</a>

		<DropdownMenu.Root bind:open={menuOpen}>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="ghost"
						class="min-h-11 min-w-0 max-w-44 shrink justify-start px-3 sm:max-w-56"
						aria-label={`${copy.auth.accountMenu.open} ${user.displayName}`}
					>
						<span class="truncate">{user.displayName}</span>
						{#if pending}
							<Spinner
								data-icon="inline-end"
								aria-label={copy.auth.accountMenu.pending}
							/>
						{:else}
							<ChevronDownIcon data-icon="inline-end" />
						{/if}
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>

			<DropdownMenu.Content align="end" class="w-64">
				<DropdownMenu.Label class="flex min-w-0 flex-col gap-0.5">
					<span>{copy.auth.accountMenu.label}</span>
					<span class="truncate font-normal text-foreground">{user.email}</span>
				</DropdownMenu.Label>
				{#if !networkState.online}
					<p id="app-header-offline-reason" class="px-2 py-1.5 text-sm text-muted-foreground">{networkState.mutationDisabledReason}</p>
				{/if}
				<DropdownMenu.Separator />
				<DropdownMenu.Group>
					<DropdownMenu.Item
						class="min-h-11"
						disabled={!networkState.online || pending || hasBlockingForms.current}
						aria-describedby={!networkState.online ? "app-header-offline-reason" : undefined}
						onSelect={(event) => {
							event.preventDefault();
							void submitLogout();
						}}
					>
						{#if pending}
							<Spinner
								data-icon="inline-start"
								aria-label={copy.auth.accountMenu.pending}
							/>
							{copy.auth.accountMenu.pending}
						{:else}
							<LogOutIcon data-icon="inline-start" />
							{copy.auth.accountMenu.logout}
						{/if}
					</DropdownMenu.Item>
				</DropdownMenu.Group>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>

	{#if failure}
		<div class="mx-auto w-full max-w-6xl px-4 pb-3 sm:px-6">
			<Alert.Root
				bind:ref={failureAlert}
				variant="destructive"
				tabindex={-1}
			>
				<CircleAlertIcon data-icon="inline-start" />
				<Alert.Title>{copy.auth.accountMenu.failure}</Alert.Title>
			</Alert.Root>
		</div>
	{/if}
</header>

<Separator />
