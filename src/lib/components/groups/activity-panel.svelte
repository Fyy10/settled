<script lang="ts">
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
	import ListPlusIcon from "@lucide/svelte/icons/list-plus";

	import type {
		ExpenseListResponse,
		GroupMember,
		RepaymentListResponse
	} from "$lib/api/types";
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
		formatActivityDate,
		groupActivity,
		memberDisplayName,
		mergeActivity
	} from "$lib/utils/group-workspace";

	import ExpenseActivityRow from "./expense-activity-row.svelte";
	import RepaymentActivityRow from "./repayment-activity-row.svelte";

	type PanelStatus = 'loading' | 'ready' | 'error';

	let {
		groupId,
		members,
		expenseStatus,
		expenseResponse,
		repaymentStatus,
		repaymentResponse,
		onRetryExpenses,
		onRetryRepayments
	}: {
		groupId: string;
		members: readonly GroupMember[];
		expenseStatus: PanelStatus;
		expenseResponse: ExpenseListResponse | null;
		repaymentStatus: PanelStatus;
		repaymentResponse: RepaymentListResponse | null;
		onRetryExpenses: () => void;
		onRetryRepayments: () => void;
	} = $props();

	const memberNames = $derived(
		buildMemberLookup(
			members,
			expenseResponse?.members ?? [],
			repaymentResponse?.members ?? []
		)
	);
	const activityGroups = $derived(
		groupActivity(
			mergeActivity(
				expenseResponse?.expenses ?? [],
				repaymentResponse?.repayments ?? []
			)
		)
	);
	const allReady = $derived(
		expenseStatus === 'ready' && repaymentStatus === 'ready'
	);

	const addExpenseHref = $derived(
		`/groups/${encodeURIComponent(groupId)}/expenses/new`
	);
	const recordPaymentHref = $derived(
		`/groups/${encodeURIComponent(groupId)}/repayments/new`
	);
</script>

<Card.Root aria-labelledby="activity-heading">
	<Card.Header>
		<Card.Title>
			<h2 id="activity-heading">{copy.groups.workspace.activity.title}</h2>
		</Card.Title>
		<Card.Description>
			{copy.groups.workspace.activity.description}
		</Card.Description>
	</Card.Header>

	<Card.Content class="flex flex-col gap-5">
		{#if expenseStatus === 'error'}
			<div class="flex flex-col gap-3">
				<Alert.Root variant="destructive">
					<CircleAlertIcon data-icon="inline-start" />
					<Alert.Title>
						{copy.groups.workspace.activity.expenseFailureTitle}
					</Alert.Title>
					<Alert.Description>
						{copy.groups.workspace.activity.partialFailureDescription}
					</Alert.Description>
				</Alert.Root>
				<Button class="min-h-11 self-start" onclick={onRetryExpenses}>
					{copy.groups.workspace.retry}
				</Button>
			</div>
		{/if}

		{#if repaymentStatus === 'error'}
			<div class="flex flex-col gap-3">
				<Alert.Root variant="destructive">
					<CircleAlertIcon data-icon="inline-start" />
					<Alert.Title>
						{copy.groups.workspace.activity.paymentFailureTitle}
					</Alert.Title>
					<Alert.Description>
						{copy.groups.workspace.activity.partialFailureDescription}
					</Alert.Description>
				</Alert.Root>
				<Button class="min-h-11 self-start" onclick={onRetryRepayments}>
					{copy.groups.workspace.retry}
				</Button>
			</div>
		{/if}

		{#if expenseStatus === 'loading' || repaymentStatus === 'loading'}
			<div class="flex flex-wrap gap-4" aria-live="polite">
				{#if expenseStatus === 'loading'}
					<div class="flex items-center gap-2">
						<Spinner aria-label={copy.groups.workspace.activity.loadingExpenses} />
						<span class="text-sm font-medium">
							{copy.groups.workspace.activity.loadingExpenses}
						</span>
					</div>
				{/if}
				{#if repaymentStatus === 'loading'}
					<div class="flex items-center gap-2">
						<Spinner aria-label={copy.groups.workspace.activity.loadingPayments} />
						<span class="text-sm font-medium">
							{copy.groups.workspace.activity.loadingPayments}
						</span>
					</div>
				{/if}
			</div>
		{/if}

		{#if activityGroups.length > 0}
			<div class="flex flex-col gap-6">
				{#each activityGroups as activityGroup (activityGroup.occurredOn)}
					<section
						class="flex flex-col gap-1"
						aria-labelledby={`activity-date-${activityGroup.occurredOn}`}
					>
						<h3
							id={`activity-date-${activityGroup.occurredOn}`}
							class="text-sm font-semibold text-muted-foreground"
						>
							{formatActivityDate(activityGroup.occurredOn)}
						</h3>
						<div>
							{#each activityGroup.items as item, index (`${item.kind}:${item.id}`)}
								{#if index > 0}
									<Separator />
								{/if}
								{#if item.kind === 'expense'}
									<ExpenseActivityRow
										expense={item.value}
										payerName={memberDisplayName(
											memberNames,
											item.value.paidByUserId,
											copy.groups.workspace.unknownMember
										)}
									/>
								{:else}
									<RepaymentActivityRow
										repayment={item.value}
										fromName={memberDisplayName(
											memberNames,
											item.value.fromUserId,
											copy.groups.workspace.unknownMember
										)}
										toName={memberDisplayName(
											memberNames,
											item.value.toUserId,
											copy.groups.workspace.unknownMember
										)}
									/>
								{/if}
							{/each}
						</div>
					</section>
				{/each}
			</div>
		{:else if allReady}
			<Empty.Root class="min-h-56">
				<Empty.Header>
					<Empty.Media variant="icon">
						<ListPlusIcon />
					</Empty.Media>
					<Empty.Title>{copy.groups.workspace.activity.emptyTitle}</Empty.Title>
					<Empty.Description>
						{copy.groups.workspace.activity.emptyDescription}
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<div class="flex flex-wrap justify-center gap-2">
						<Button href={addExpenseHref} class="min-h-11">
							{copy.groups.workspace.addExpense}
						</Button>
						<Button
							href={recordPaymentHref}
							variant="outline"
							class="min-h-11"
						>
							{copy.groups.workspace.recordPayment}
						</Button>
					</div>
				</Empty.Content>
			</Empty.Root>
		{:else if expenseStatus === 'loading' && repaymentStatus === 'loading'}
			<div class="flex flex-col gap-4" aria-hidden="true">
				{#each [0, 1, 2] as row (row)}
					<div class="flex items-center justify-between gap-4">
						<div class="flex min-w-0 flex-1 flex-col gap-2">
							<Skeleton class="h-4 w-2/5" />
							<Skeleton class="h-3 w-3/5" />
						</div>
						<Skeleton class="h-4 w-20" />
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
