<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.register.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.register.title}
	description={copy.routes.register.description}
	loadingLabel={copy.routes.register.loadingLabel}
/>
