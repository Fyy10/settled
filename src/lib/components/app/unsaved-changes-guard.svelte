<script lang="ts">
	import { beforeNavigate } from "$app/navigation";
	import { onDestroy, untrack } from "svelte";

	import { registerDirtyForm } from "$lib/state/dirty-forms.svelte";

	let {
		active,
		blocked = false,
		message
	}: {
		active: boolean;
		blocked?: boolean;
		message: string;
	} = $props();

	// Register the initial value synchronously so global consumers cannot miss a
	// dirty form during the first render.
	const registration = registerDirtyForm(
		untrack(() => active),
		untrack(() => blocked)
	);

	$effect(() => {
		registration.update(active, blocked);
	});

	onDestroy(() => {
		registration.unregister();
	});

	beforeNavigate((navigation) => {
		const isAuthoritativeAuthExit =
			navigation.to?.url.pathname === '/login';
		if (isAuthoritativeAuthExit) {
			return;
		}
		if (blocked) {
			navigation.cancel();
			return;
		}
		if (!active) {
			return;
		}

		if (navigation.type === 'leave') {
			navigation.cancel();
			return;
		}

		if (!globalThis.confirm(message)) {
			navigation.cancel();
		}
	});
</script>
