<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.login.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.login.title}
	description={copy.routes.login.description}
	loadingLabel={copy.routes.login.loadingLabel}
/>
