<script lang="ts">
	import ArrowRightIcon from "@lucide/svelte/icons/arrow-right";

	import type { Settlement } from "$lib/api/types";
	import { Button } from "$lib/components/ui/button";
	import { Separator } from "$lib/components/ui/separator";
	import { copy } from "$lib/copy/en";

	import MemberAvatar from "./member-avatar.svelte";
	import MoneyAmount from "./money-amount.svelte";

	let {
		groupId,
		settlement,
		fromName,
		toName
	}: {
		groupId: string;
		settlement: Settlement;
		fromName: string;
		toName: string;
	} = $props();

	const paymentHref = $derived.by(() => {
		const query = new URLSearchParams({
			from: settlement.fromUserId,
			to: settlement.toUserId,
			amountCents: String(settlement.amountCents)
		});

		return `/groups/${encodeURIComponent(groupId)}/repayments/new?${query.toString()}`;
	});
</script>

<article class="flex flex-col gap-4 py-4 lg:grid lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
	<p class="min-w-0 text-base leading-6 sm:sr-only">
		<span class="font-semibold break-words">{fromName}</span>
		<span class="text-muted-foreground">
			{copy.groups.workspace.balances.shouldPay}
		</span>
		<span class="font-semibold break-words">{toName}</span>
	</p>

	<div class="hidden min-w-0 items-center gap-3 sm:flex">
		<div class="flex min-w-0 max-w-48 items-center gap-2">
			<MemberAvatar displayName={fromName} />
			<span class="font-semibold break-words">{fromName}</span>
		</div>

		<div class="flex min-w-24 flex-1 items-center gap-2" aria-hidden="true">
			<span class="whitespace-nowrap text-xs text-muted-foreground">
				{copy.groups.workspace.balances.shouldPay}
			</span>
			<Separator class="min-w-8 flex-1" />
			<ArrowRightIcon class="size-4 shrink-0 text-muted-foreground" />
		</div>

		<div class="flex min-w-0 max-w-48 items-center gap-2">
			<MemberAvatar displayName={toName} />
			<span class="font-semibold break-words">{toName}</span>
		</div>
	</div>

	<div class="flex flex-wrap items-center justify-between gap-3 lg:justify-end">
		<MoneyAmount cents={settlement.amountCents} />
		<Button href={paymentHref} variant="outline" class="min-h-11">
			{copy.groups.workspace.recordPayment}
		</Button>
	</div>
</article>
