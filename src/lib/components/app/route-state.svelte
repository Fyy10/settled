<script lang="ts">
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { Spinner } from "$lib/components/ui/spinner";

	let {
		title,
		description,
		loadingLabel
	}: {
		title: string;
		description: string;
		loadingLabel: string;
	} = $props();
</script>

<section class="mx-auto flex w-full max-w-3xl flex-col gap-8" aria-labelledby="route-heading">
	<header class="flex max-w-2xl flex-col gap-2">
		<h1 id="route-heading" tabindex="-1">{title}</h1>
		<p class="text-muted-foreground">{description}</p>
	</header>

	<div class="overflow-hidden rounded-lg border bg-card">
		<div class="flex min-h-14 items-center gap-3 border-b px-4">
			<Spinner aria-label={loadingLabel} />
			<p class="text-sm font-medium">{loadingLabel}</p>
		</div>

		<div class="flex flex-col" aria-hidden="true">
			{#each [0, 1, 2] as row (row)}
				<div class="grid min-h-18 grid-cols-[minmax(0,1fr)_5rem] items-center gap-4 border-b px-4 last:border-b-0">
					<div class="flex min-w-0 flex-col gap-2">
						<Skeleton class="h-3.5 w-2/5" />
						<Skeleton class="h-3 w-3/4" />
					</div>
					<Skeleton class="h-4 w-full" />
				</div>
			{/each}
		</div>
	</div>
</section>
