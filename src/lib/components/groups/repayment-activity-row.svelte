<script lang="ts">
	import type { Repayment } from "$lib/api/types";
	import { copy } from "$lib/copy/en";

	import MoneyAmount from "./money-amount.svelte";

	let {
		repayment,
		fromName,
		toName
	}: {
		repayment: Repayment;
		fromName: string;
		toName: string;
	} = $props();
</script>

<article class="grid min-w-0 gap-2 py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:gap-4">
	<div class="flex min-w-0 flex-col gap-1">
		<p class="font-semibold break-words">
			{fromName} {copy.groups.workspace.activity.paid} {toName}
		</p>
		{#if repayment.note}
			<p class="break-words text-sm">{repayment.note}</p>
		{/if}
		<p class="text-xs text-muted-foreground">
			{copy.groups.workspace.activity.recordedOutside}
		</p>
	</div>
	<MoneyAmount cents={repayment.amountCents} />
</article>
