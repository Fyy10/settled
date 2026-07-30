<script lang="ts">
	import { afterNavigate } from "$app/navigation";
	import { shouldFocusRouteHeading } from "$lib/utils/route-focus";

	afterNavigate(({ from, to }) => {
		if (!to || !shouldFocusRouteHeading(from?.url, to.url)) {
			return;
		}

		window.requestAnimationFrame(() => {
			document.querySelector<HTMLElement>("#main-content h1")?.focus({ preventScroll: true });
		});
	});
</script>
