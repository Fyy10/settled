<script lang="ts">
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import { copy } from '$lib/copy/en';
	import { hasBlockingForms, hasDirtyForms } from '$lib/state/dirty-forms.svelte';
	import { networkState, startNetworkObserver } from '$lib/state/network.svelte';

	let updateAvailable = $state(false);
	let applying = $state(false);
	let failed = $state(false);
	let reloadPending = false;
	let reloadStarted = false;
	let disposed = false;
	let update: (() => Promise<void>) | undefined;
	const blocked = $derived(hasDirtyForms.current || hasBlockingForms.current);

	function requestReload(): void {
		if (disposed || reloadStarted) return;
		reloadPending = true;
		applying = false;
		updateAvailable = true;
		if (hasDirtyForms.current || hasBlockingForms.current) return;
		reloadStarted = true;
		window.location.reload();
	}

	onMount(() => {
		const stop = startNetworkObserver();
		const serviceWorkers = navigator.serviceWorker;
		let previousController = serviceWorkers?.controller;
		const controllerChanged = () => {
			const controller = serviceWorkers.controller;
			const replaced = previousController !== null && previousController !== undefined &&
				controller !== null && controller !== previousController;
			previousController = controller;
			// Claiming the first install must not reload the initial page.
			if (replaced) requestReload();
		};
		serviceWorkers?.addEventListener('controllerchange', controllerChanged);
		let stopWatching = () => {};
		function watchRegistration(registration: ServiceWorkerRegistration | undefined): void {
			if (!registration || disposed) return;
			stopWatching();
			const workers = new Set<ServiceWorker>();
			const checkWaiting = () => {
				if (!disposed && registration.waiting) updateAvailable = true;
			};
			const watchInstalling = () => {
				const worker = registration.installing;
				if (worker && !workers.has(worker)) {
					workers.add(worker);
					worker.addEventListener('statechange', checkWaiting);
				}
				checkWaiting();
			};
			registration.addEventListener('updatefound', watchInstalling);
			watchInstalling();
			stopWatching = () => {
				registration.removeEventListener('updatefound', watchInstalling);
				for (const worker of workers) worker.removeEventListener('statechange', checkWaiting);
			};
		}
		void import('virtual:pwa-register').then(({ registerSW }) => {
			if (disposed) return;
			update = registerSW({
				immediate: true,
				onRegisteredSW: (_url, registration) => watchRegistration(registration),
				onNeedRefresh: () => { if (!disposed) updateAvailable = true; },
				onNeedReload: requestReload
			});
		}).catch(() => { /* A failed registration must not interrupt the online app. */ });
		return () => {
			disposed = true;
			serviceWorkers?.removeEventListener('controllerchange', controllerChanged);
			stopWatching();
			stop();
		};
	});

	async function reload(): Promise<void> {
		if (disposed || reloadStarted || hasDirtyForms.current || hasBlockingForms.current || applying || !update) return;
		if (reloadPending) { requestReload(); return; }
		applying = true;
		failed = false;
		try {
			await update();
		} catch {
			failed = true;
		} finally {
			applying = false;
		}
	}
</script>

{#if !networkState.online || updateAvailable}
	<div class="mx-auto flex w-full max-w-5xl flex-col gap-2 px-4 py-2 sm:px-6" aria-live="polite">
		{#if !networkState.online}
			<Alert.Root role="status">
				<Alert.Title>{copy.pwa.offline}</Alert.Title>
				<Alert.Description>{copy.pwa.mutationDisabled}</Alert.Description>
			</Alert.Root>
		{/if}
		{#if updateAvailable}
			<Alert.Root role="status">
				<Alert.Title>{copy.pwa.updateAvailable}</Alert.Title>
				<Alert.Description>
					{#if blocked}
						<p>{copy.pwa.updateBlocked}</p>
					{:else}
						<Button variant="outline" disabled={applying} onclick={reload}>{copy.pwa.reload}</Button>
					{/if}
					{#if failed}<p>{copy.pwa.updateFailure}</p>{/if}
				</Alert.Description>
			</Alert.Root>
		{/if}
	</div>
{/if}
