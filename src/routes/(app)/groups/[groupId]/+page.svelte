<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.group.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.group.title}
	description={copy.routes.group.description}
	loadingLabel={copy.routes.group.loadingLabel}
/>
