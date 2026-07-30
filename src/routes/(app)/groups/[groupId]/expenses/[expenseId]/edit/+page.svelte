<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
	import { pageTitle } from "$lib/utils/page-title";
</script>

<svelte:head>
	<title>{pageTitle(copy.routes.editExpense.documentTitle)}</title>
</svelte:head>

<RouteState
	title={copy.routes.editExpense.title}
	description={copy.routes.editExpense.description}
	loadingLabel={copy.routes.editExpense.loadingLabel}
/>
