<script lang="ts">
	import { formatUsd } from "$lib/utils/money";

	let { cents }: { cents: number } = $props();
</script>

<span class="whitespace-nowrap font-semibold tabular-nums">{formatUsd(cents)}</span>
