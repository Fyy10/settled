<script lang="ts">
	import { goto } from "$app/navigation";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import { onMount } from "svelte";

	import favicon from "$lib/assets/favicon.svg";
	import AppHeader from "$lib/components/app/app-header.svelte";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import { Separator } from "$lib/components/ui/separator";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import { authState, ensureSession } from "$lib/state/auth.svelte";

	let { children } = $props();

	const initialSession = authState.current;
	let gate = $state<'checking' | 'ready' | 'error'>(
		initialSession.status === 'authenticated' ? 'ready' : 'checking'
	);
	let user = $state(initialSession.status === 'authenticated' ? initialSession.user : null);

	onMount(() => {
		void verifySession();
	});

	async function verifySession(): Promise<void> {
		if (user === null) {
			gate = 'checking';
		}

		let session;
		try {
			session = await ensureSession();
		} catch {
			gate = 'error';
			return;
		}

		if (session.status === 'anonymous') {
			await goto('/login', { replaceState: true });
			return;
		}

		user = session.user;
		gate = 'ready';
	}
</script>

{#if gate === 'ready' && user}
	<AppHeader {user} />
	<main id="main-content" class="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6 sm:py-10">
		{@render children?.()}
	</main>
{:else}
	<header class="bg-background">
		<div class="mx-auto flex min-h-14 w-full max-w-6xl items-center px-4 sm:px-6">
			<a
				href="/groups"
				class="flex min-h-11 items-center gap-3 font-semibold"
				aria-label={copy.navigation.groupsHome}
			>
				<img src={favicon} alt="" class="size-8" />
				<span class="tracking-tight">{copy.appName}</span>
			</a>
		</div>
	</header>
	<Separator />

	<main id="main-content" class="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6 sm:py-10">
		<section class="mx-auto flex w-full max-w-3xl flex-col gap-8" aria-labelledby="app-gate-heading">
			{#if gate === 'error'}
				<div class="flex max-w-xl flex-col gap-4">
					<Alert.Root variant="destructive">
						<CircleAlertIcon data-icon="inline-start" />
						<Alert.Title>
							<h1 id="app-gate-heading" tabindex="-1">
								{copy.auth.serviceUnavailableTitle}
							</h1>
						</Alert.Title>
						<Alert.Description>
							{copy.auth.serviceUnavailableDescription}
						</Alert.Description>
					</Alert.Root>
					<Button class="min-h-11 self-start" onclick={() => void verifySession()}>
						{copy.auth.retry}
					</Button>
				</div>
			{:else}
				<header class="flex max-w-2xl flex-col gap-2">
					<div class="flex items-center gap-3" aria-live="polite">
						<Spinner aria-label={copy.loading.session} />
						<h1 id="app-gate-heading" tabindex="-1">
							{copy.auth.checkingTitle}
						</h1>
					</div>
					<p class="text-muted-foreground">{copy.auth.checkingDescription}</p>
				</header>

				<div class="overflow-hidden rounded-lg border bg-card" aria-hidden="true">
					{#each [0, 1, 2] as row (row)}
						<div class="grid min-h-18 grid-cols-[minmax(0,1fr)_5rem] items-center gap-4 border-b px-4 last:border-b-0">
							<div class="flex min-w-0 flex-col gap-2">
								<Skeleton class="h-3.5 w-2/5" />
								<Skeleton class="h-3 w-3/4" />
							</div>
							<Skeleton class="h-4 w-full" />
						</div>
					{/each}
				</div>
			{/if}
		</section>
	</main>
{/if}
