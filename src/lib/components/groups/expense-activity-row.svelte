<script lang="ts">
	import type { Expense } from "$lib/api/types";
	import { copy } from "$lib/copy/en";

	import MoneyAmount from "./money-amount.svelte";

	let {
		expense,
		payerName
	}: {
		expense: Expense;
		payerName: string;
	} = $props();

	const participantLabel = $derived(
		expense.splits.length === 1
			? copy.groups.workspace.activity.person
			: copy.groups.workspace.activity.people
	);
</script>

<article class="grid min-w-0 gap-2 py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:gap-4">
	<div class="flex min-w-0 flex-col gap-1">
		<p class="font-semibold break-words">{expense.description}</p>
		<p class="text-sm text-muted-foreground">
			<span class="break-words">{payerName}</span>
			{copy.groups.workspace.activity.paid}
			<span aria-hidden="true">·</span>
			{copy.groups.workspace.activity.splitWith}
			{expense.splits.length}
			{participantLabel}
		</p>
	</div>
	<MoneyAmount cents={expense.amountCents} />
</article>
