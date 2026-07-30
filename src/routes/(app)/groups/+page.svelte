<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.groups.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.groups.title}
	description={copy.routes.groups.description}
	loadingLabel={copy.routes.groups.loadingLabel}
/>
