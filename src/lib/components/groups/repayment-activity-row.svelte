<script lang="ts">
	import PencilIcon from "@lucide/svelte/icons/pencil";

	import type { Repayment } from "$lib/api/types";
	import { copy } from "$lib/copy/en";

	import MoneyAmount from "./money-amount.svelte";

	let {
		groupId,
		repayment,
		fromName,
		toName
	}: {
		groupId: string;
		repayment: Repayment;
		fromName: string;
		toName: string;
	} = $props();

	const editHref = $derived(
		`/groups/${encodeURIComponent(groupId)}/repayments/${encodeURIComponent(repayment.id)}/edit`
	);
</script>

<article class="min-w-0">
	<a
		href={editHref}
		class="-mx-2 grid min-h-11 min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 rounded-lg px-2 py-4 outline-none transition-colors hover:bg-muted/60 focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-4 sm:gap-4"
	>
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
		<PencilIcon
			data-icon="inline-end"
			class="text-muted-foreground"
			aria-hidden="true"
		/>
	</a>
</article>
