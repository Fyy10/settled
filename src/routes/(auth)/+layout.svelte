<script lang="ts">
	import favicon from "$lib/assets/favicon.svg";
	import { copy } from "$lib/copy/en";

	let { children } = $props();
</script>

<main id="main-content" class="mx-auto flex min-h-dvh w-full max-w-md items-center px-4 py-10 sm:px-6">
	<div class="flex w-full flex-col gap-8">
		<header class="flex flex-col items-center gap-3 text-center">
			<a
				href="/"
				class="flex min-h-10 items-center gap-3 font-semibold"
				aria-label={copy.navigation.brandHome}
			>
				<img src={favicon} alt="" class="size-9" />
				<span class="text-lg tracking-tight">{copy.appName}</span>
			</a>
			<p class="text-sm text-muted-foreground">{copy.tagline}</p>
		</header>

		{@render children?.()}
	</div>
</main>
