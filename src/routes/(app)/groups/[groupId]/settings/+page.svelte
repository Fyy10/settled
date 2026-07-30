<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.settings.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.settings.title}
	description={copy.routes.settings.description}
	loadingLabel={copy.routes.settings.loadingLabel}
/>
