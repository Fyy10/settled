<script lang="ts">
	import RouteState from "$lib/components/app/route-state.svelte";
	import { copy } from "$lib/copy/en";
</script>

<main id="main-content" class="mx-auto flex min-h-dvh w-full max-w-3xl items-center px-4 py-10 sm:px-6">
	<RouteState
		title={copy.routes.root.title}
		description={copy.routes.root.description}
		loadingLabel={copy.loading.session}
	/>
</main>
