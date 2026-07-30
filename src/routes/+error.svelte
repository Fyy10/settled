<script lang="ts">
	import RouteError from "$lib/components/app/route-error.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";

	function retry(): void {
		window.location.reload();
	}
</script>

<svelte:head>
	<title>{pageTitle(copy.errors.documentTitle)}</title>
</svelte:head>

<main id="main-content" class="mx-auto flex min-h-dvh w-full max-w-3xl items-center px-4 py-10 sm:px-6">
	<RouteError onRetry={retry} />
</main>
