<script lang="ts">
	import { goto } from "$app/navigation";
	import { browser } from "$app/environment";
	import { page } from "$app/state";
	import { onDestroy } from "svelte";

	import { onSessionExpired } from "$lib/state/auth.svelte";
	import { sessionExpiredLoginPath } from "$lib/utils/safe-next";

	const stopListening = browser
		? onSessionExpired(() => {
				const next = `${page.url.pathname}${page.url.search}${page.url.hash}`;
				void goto(sessionExpiredLoginPath(next), { replaceState: true });
			})
		: () => {};

	onDestroy(stopListening);
</script>
