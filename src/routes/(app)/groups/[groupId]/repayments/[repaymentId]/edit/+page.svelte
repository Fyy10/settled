<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.editPayment.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.editPayment.title}
	description={copy.routes.editPayment.description}
	loadingLabel={copy.routes.editPayment.loadingLabel}
/>
