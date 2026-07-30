<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.addExpense.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.addExpense.title}
	description={copy.routes.addExpense.description}
	loadingLabel={copy.routes.addExpense.loadingLabel}
/>
