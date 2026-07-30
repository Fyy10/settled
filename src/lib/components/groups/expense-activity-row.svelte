<script lang="ts">
	import PencilIcon from "@lucide/svelte/icons/pencil";

	import type { Expense } from "$lib/api/types";
	import { copy } from "$lib/copy/en";

	import MoneyAmount from "./money-amount.svelte";

	let {
		groupId,
		expense,
		payerName
	}: {
		groupId: string;
		expense: Expense;
		payerName: string;
	} = $props();

	const participantLabel = $derived(
		expense.splits.length === 1
			? copy.groups.workspace.activity.person
			: copy.groups.workspace.activity.people
	);
	const editHref = $derived(
		`/groups/${encodeURIComponent(groupId)}/expenses/${encodeURIComponent(expense.id)}/edit`
	);
</script>

<article class="min-w-0">
	<a
		href={editHref}
		class="-mx-2 grid min-h-11 min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 rounded-lg px-2 py-4 outline-none transition-colors hover:bg-muted/60 focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-4 sm:gap-4"
	>
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
		<PencilIcon
			data-icon="inline-end"
			class="text-muted-foreground"
			aria-hidden="true"
		/>
	</a>
</article>
