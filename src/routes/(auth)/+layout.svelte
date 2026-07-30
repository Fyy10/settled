<script lang="ts">
	import { goto } from "$app/navigation";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import { onMount } from "svelte";

	import favicon from "$lib/assets/favicon.svg";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { ensureSession } from "$lib/state/auth.svelte";

	let { children } = $props();

	let gate = $state<'checking' | 'ready' | 'error'>('checking');

	onMount(() => {
		void verifySession();
	});

	async function verifySession(): Promise<void> {
		gate = 'checking';

		let session;
		try {
			session = await ensureSession();
		} catch {
			gate = 'error';
			return;
		}

		if (session.status === 'authenticated') {
			await goto('/groups', { replaceState: true });
			return;
		}

		gate = 'ready';
	}
</script>

<main id="main-content" class="mx-auto flex min-h-dvh w-full max-w-md items-center px-4 py-10 sm:px-6">
	<div class="flex w-full flex-col gap-8">
		<header class="flex flex-col items-center gap-3 text-center">
			<a
				href="/"
				class="flex min-h-11 items-center gap-3 font-semibold"
				aria-label={copy.navigation.brandHome}
			>
				<img src={favicon} alt="" class="size-9" />
				<span class="text-lg tracking-tight">{copy.appName}</span>
			</a>
			<p class="text-sm text-muted-foreground">{copy.tagline}</p>
		</header>

		{#if gate === 'ready'}
			{@render children?.()}
		{:else}
			<section class="border-y py-6" aria-labelledby="auth-gate-heading">
				{#if gate === 'checking'}
					<div class="flex items-start gap-3" aria-live="polite">
						<Spinner aria-label={copy.loading.session} />
						<div class="flex min-w-0 flex-col gap-1">
							<h1 id="auth-gate-heading" tabindex="-1">
								{copy.auth.checkingTitle}
							</h1>
							<p class="text-sm text-muted-foreground">
								{copy.auth.checkingDescription}
							</p>
						</div>
					</div>
				{:else}
					<div class="flex flex-col gap-4">
						<Alert.Root variant="destructive">
							<CircleAlertIcon data-icon="inline-start" />
							<Alert.Title>
								<h1 id="auth-gate-heading" tabindex="-1">
									{copy.auth.serviceUnavailableTitle}
								</h1>
							</Alert.Title>
							<Alert.Description>
								{copy.auth.serviceUnavailableDescription}
							</Alert.Description>
						</Alert.Root>
						<Button class="min-h-11" onclick={() => void verifySession()}>
							{copy.auth.retry}
						</Button>
					</div>
				{/if}
			</section>
		{/if}
	</div>
</main>
