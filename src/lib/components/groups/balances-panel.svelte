<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import CircleCheckIcon from "@lucide/svelte/icons/circle-check";

	import type { SettlementListResponse } from "$lib/api/types";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import * as Empty from "$lib/components/ui/empty";
	import { Separator } from "$lib/components/ui/separator";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { Spinner } from "$lib/components/ui/spinner";
	import { copy } from "$lib/copy/en";
	import {
		buildMemberLookup,
		memberDisplayName
	} from "$lib/utils/group-workspace";

	import SettlementRow from "./settlement-row.svelte";

	let {
		groupId,
		status,
		response,
		onRetry
	}: {
		groupId: string;
		status: 'loading' | 'ready' | 'error';
		response: SettlementListResponse | null;
		onRetry: () => void;
	} = $props();

	const members = $derived(
		buildMemberLookup(response?.members ?? [])
	);
</script>

<Card.Root aria-labelledby="balances-heading">
	<Card.Header>
		<Card.Title>
			<h2 id="balances-heading">{copy.groups.workspace.balances.title}</h2>
		</Card.Title>
		<Card.Description>
			{copy.groups.workspace.balances.description}
		</Card.Description>
	</Card.Header>

	<Card.Content>
		{#if status === 'loading' && response === null}
			<div class="flex flex-col gap-4">
				<div class="flex items-center gap-3" aria-live="polite">
					<Spinner aria-label={copy.groups.workspace.balances.loading} />
					<span class="text-sm font-medium">
						{copy.groups.workspace.balances.loading}
					</span>
				</div>
				<div class="flex flex-col gap-5" aria-hidden="true">
					{#each [0, 1] as row (row)}
						<div class="flex items-center justify-between gap-4">
							<div class="flex min-w-0 flex-1 items-center gap-3">
								<Skeleton class="size-8 rounded-full" />
								<Skeleton class="h-4 w-3/5 max-w-72" />
							</div>
							<Skeleton class="h-10 w-28" />
						</div>
					{/each}
				</div>
			</div>
		{/if}

		{#if status === 'error'}
			<div class="flex flex-col gap-4">
				<Alert.Root variant="destructive">
					<CircleAlertIcon data-icon="inline-start" />
					<Alert.Title>
						{copy.groups.workspace.balances.failureTitle}
					</Alert.Title>
					<Alert.Description>
						{copy.groups.workspace.balances.failureDescription}
					</Alert.Description>
				</Alert.Root>
				<Button class="min-h-11 self-start" onclick={onRetry}>
					{copy.groups.workspace.retry}
				</Button>
			</div>
		{/if}

		{#if status !== 'error' && response !== null && response.settlements.length === 0}
			<Empty.Root class="min-h-56">
				<Empty.Header>
					<Empty.Media variant="icon">
						<CircleCheckIcon />
					</Empty.Media>
					<Empty.Title>{copy.groups.workspace.balances.emptyTitle}</Empty.Title>
					<Empty.Description>
						{copy.groups.workspace.balances.emptyDescription}
					</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else if status !== 'error' && response !== null && response.settlements.length > 0}
			<div>
				{#each response.settlements as settlement, index (`${settlement.fromUserId}:${settlement.toUserId}`)}
					{#if index > 0}
						<Separator />
					{/if}
					<SettlementRow
						{groupId}
						{settlement}
						fromName={memberDisplayName(
							members,
							settlement.fromUserId,
							copy.groups.workspace.unknownMember
						)}
						toName={memberDisplayName(
							members,
							settlement.toUserId,
							copy.groups.workspace.unknownMember
						)}
					/>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
