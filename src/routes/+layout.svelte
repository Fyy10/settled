<script lang="ts">
	import { navigating } from "$app/state";
	import favicon from "$lib/assets/favicon.svg";
	import RouteFocusManager from "$lib/components/app/route-focus-manager.svelte";
	import RouteLoadingAnnouncer from "$lib/components/app/route-loading-announcer.svelte";
	import SessionExpirationRedirect from "$lib/components/app/session-expiration-redirect.svelte";
	import PwaStatus from "$lib/components/app/pwa-status.svelte";
	import { Toaster } from "$lib/components/ui/sonner";
	import { copy } from "$lib/copy/en";

	import "./layout.css";

	let { children } = $props();
</script>

<svelte:head>
	<title>{copy.appName}</title>
	<link rel="icon" href={favicon} />
	<link rel="manifest" href="/manifest.webmanifest" />
	<meta name="theme-color" content="#2f6f58" />
</svelte:head>

<a
	href="#main-content"
	class="fixed top-3 left-3 -translate-y-20 rounded-md bg-primary px-4 py-2 font-medium text-primary-foreground transition-transform focus:translate-y-0"
>
	{copy.navigation.skipToContent}
</a>

<RouteFocusManager />
<RouteLoadingAnnouncer active={navigating.to !== null} />
<SessionExpirationRedirect />
<Toaster position="top-center" />
<PwaStatus />

{@render children?.()}
