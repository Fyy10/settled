<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.recordPayment.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.recordPayment.title}
	description={copy.routes.recordPayment.description}
	loadingLabel={copy.routes.recordPayment.loadingLabel}
/>
