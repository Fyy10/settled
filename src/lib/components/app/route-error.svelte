<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";

	import { copy } from "$lib/copy/en";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";

	let {
		onRetry,
		backHref = '/',
		backLabel = copy.navigation.backToSettled
	}: {
		onRetry: () => void;
		backHref?: string;
		backLabel?: string;
	} = $props();
</script>

<section class="mx-auto flex w-full max-w-2xl flex-col gap-6" aria-labelledby="error-heading">
	<header class="flex flex-col gap-2">
		<h1 id="error-heading" tabindex="-1">{copy.errors.unexpectedTitle}</h1>
		<p class="text-muted-foreground">{copy.errors.unexpectedDescription}</p>
	</header>

	<Alert.Root variant="destructive">
		<CircleAlertIcon />
		<Alert.Title>{copy.errors.unexpectedSummary}</Alert.Title>
		<Alert.Description>{copy.errors.unexpectedDescription}</Alert.Description>
	</Alert.Root>

	<div class="flex flex-wrap gap-3">
		<Button onclick={onRetry} size="lg">{copy.errors.retry}</Button>
		<Button href={backHref} variant="outline">{backLabel}</Button>
	</div>
</section>
