<script lang="ts">
	import { copy } from "$lib/copy/en";

	const DEFAULT_DELAY = 300;
	const MINIMUM_HOLD = 1_000;

	let {
		active,
		delay = DEFAULT_DELAY,
		minimumHold = MINIMUM_HOLD,
		message = copy.loading.route
	}: {
		active: boolean;
		delay?: number;
		minimumHold?: number;
		message?: string;
	} = $props();

	let announcement = $state('');
	let announcedAt = $state<number | null>(null);

	$effect(() => {
		const currentMessage = message;

		if (active) {
			if (announcement !== '') {
				return;
			}

			const showTimeout = window.setTimeout(() => {
				announcement = currentMessage;
				announcedAt = Date.now();
			}, delay);

			return () => window.clearTimeout(showTimeout);
		}

		if (announcement === '' || announcedAt === null) {
			return;
		}

		// Keep an announced message available for at least one second so assistive
		// technology has time to consume it after navigation finishes.
		const remainingHold = Math.max(0, minimumHold - (Date.now() - announcedAt));
		const clearTimeout = window.setTimeout(() => {
			announcement = '';
			announcedAt = null;
		}, remainingHold);

		return () => window.clearTimeout(clearTimeout);
	});
</script>

<div class="sr-only" aria-live="polite" aria-atomic="true">{announcement}</div>
