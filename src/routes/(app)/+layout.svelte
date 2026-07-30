<script lang="ts">
	import favicon from "$lib/assets/favicon.svg";
	import { copy } from "$lib/copy/en";

	let { children } = $props();
</script>

<header class="border-b bg-background">
	<div class="mx-auto flex min-h-14 w-full max-w-6xl items-center px-4 sm:px-6">
		<a
			href="/groups"
			class="flex min-h-10 items-center gap-3 font-semibold"
			aria-label={copy.navigation.groupsHome}
		>
			<img src={favicon} alt="" class="size-8" />
			<span class="tracking-tight">{copy.appName}</span>
		</a>
	</div>
</header>

<main id="main-content" class="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6 sm:py-10">
	{@render children?.()}
</main>
