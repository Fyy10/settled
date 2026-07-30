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

	let state = $state<'checking' | 'error'>('checking');

	onMount(() => {
		void bootstrap();
	});

	async function bootstrap(): Promise<void> {
		state = 'checking';

		let session;
		try {
			session = await ensureSession();
		} catch {
			state = 'error';
			return;
		}

		await goto(session.status === 'authenticated' ? '/groups' : '/login', {
			replaceState: true
		});
	}
</script>

<main id="main-content" class="mx-auto flex min-h-dvh w-full max-w-md items-center px-4 py-10 sm:px-6">
	<section class="flex w-full flex-col gap-6" aria-labelledby="bootstrap-heading">
		<header class="flex items-center justify-center gap-3">
			<img src={favicon} alt="" class="size-10" />
			<span class="text-lg font-semibold tracking-tight">{copy.appName}</span>
		</header>

		<div class="border-y py-6">
			{#if state === 'checking'}
				<div class="flex items-start gap-3" aria-live="polite">
					<Spinner aria-label={copy.loading.session} />
					<div class="flex min-w-0 flex-col gap-1">
						<h1 id="bootstrap-heading" tabindex="-1">{copy.routes.root.title}</h1>
						<p class="text-sm text-muted-foreground">{copy.routes.root.description}</p>
					</div>
				</div>
			{:else}
				<div class="flex flex-col gap-4">
					<Alert.Root variant="destructive">
						<CircleAlertIcon data-icon="inline-start" />
						<Alert.Title>
							<h1 id="bootstrap-heading" tabindex="-1">
								{copy.auth.serviceUnavailableTitle}
							</h1>
						</Alert.Title>
						<Alert.Description>
							{copy.auth.serviceUnavailableDescription}
						</Alert.Description>
					</Alert.Root>
					<Button class="min-h-11" onclick={() => void bootstrap()}>
						{copy.auth.retry}
					</Button>
				</div>
			{/if}
		</div>
	</section>
</main>
