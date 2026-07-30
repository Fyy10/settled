<script lang="ts">
	import { goto } from "$app/navigation";
	import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
	import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";

	import {
		isNotFoundApiError,
		isUnauthorizedApiError
	} from "$lib/api/errors";
	import { createExpense } from "$lib/api/expenses";
	import type { Expense } from "$lib/api/types";
	import ExpenseForm from "$lib/components/expenses/expense-form.svelte";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Card from "$lib/components/ui/card";
	import { Skeleton } from "$lib/components/ui/skeleton";
	import { copy } from "$lib/copy/en";
	import { authState } from "$lib/state/auth.svelte";
	import { useGroupAccounting } from "$lib/state/group-accounting.svelte";
	import { useGroupDetailContext } from "$lib/state/group-detail.svelte";
	import { MutationCoordinator } from "$lib/state/mutation-coordinator.svelte";
	import { finishCommittedMutation } from "$lib/utils/committed-mutation";
	import { currentLocalDate } from "$lib/utils/dates";
	import {
		createExpenseDraft,
		type ExpenseDraft,
		type ExpenseInput
	} from "$lib/utils/expense-draft";
	import { pageTitle } from "$lib/utils/page-title";

	const groupDetail = useGroupDetailContext();
	const accounting = useGroupAccounting();
	let mutation = $state(new MutationCoordinator<'save' | 'delete'>());
	let mutationGroupId = '';
	let initializedGroupId = '';
	let initialDraft = $state<ExpenseDraft | null>(null);
	let memberMismatch = $state(false);
	let mutationDisabledReason = $state<string | null>(null);

	const groupPath = $derived(
		`/groups/${encodeURIComponent(groupDetail.groupId)}`
	);
	const activityPath = $derived(`${groupPath}?view=activity`);

	$effect(() => {
		const groupId = groupDetail.groupId;
		const members = groupDetail.members;
		if (mutationGroupId !== groupId) {
			mutation = new MutationCoordinator<'save' | 'delete'>();
			mutationGroupId = groupId;
		}
		if (
			initializedGroupId !== '' &&
			initializedGroupId !== groupId
		) {
			initialDraft = null;
			initializedGroupId = '';
			memberMismatch = false;
		}
		if (
			groupDetail.status !== 'ready' ||
			groupDetail.group === null ||
			groupId === '' ||
			members.length === 0 ||
			initializedGroupId === groupId
		) {
			return;
		}

		const currentUserId =
			authState.current.status === 'authenticated'
				? authState.current.user.id
				: '';
		if (!members.some((member) => member.userId === currentUserId)) {
			memberMismatch = true;
			initializedGroupId = groupId;
			return;
		}
		initialDraft = createExpenseDraft({
			paidByUserId: currentUserId,
			expenseDate: currentLocalDate(),
			participantUserIds: members.map((member) => member.userId)
		});
		initializedGroupId = groupId;
	});

	function save(
		input: ExpenseInput,
		options: { signal: AbortSignal }
	): Promise<Expense> {
		return createExpense(groupDetail.groupId, input, options);
	}

	function finish(): ReturnType<typeof finishCommittedMutation> {
		return finishCommittedMutation({
			refresh: () => accounting.refreshAfterExpenseMutation(),
			navigate: navigateAfterRefresh
		});
	}

	function navigateAfterRefresh(): Promise<unknown> {
		const errors = [
			accounting.expenses.error,
			accounting.settlements.error
		];
		if (
			groupDetail.status !== 'ready' ||
			errors.some(
				(error) =>
					isUnauthorizedApiError(error) || isNotFoundApiError(error)
			)
		) {
			return Promise.resolve();
		}
		return goto(activityPath, { replaceState: true });
	}

	async function resolveNotFound(): Promise<
		'record' | 'handled' | 'retryable'
	> {
		try {
			await groupDetail.refreshDetail();
		} catch {
			// The route layout owns hidden-group and session states.
		}
		return groupDetail.status === 'ready' ? 'retryable' : 'handled';
	}
</script>

<svelte:head>
	<title>
		{pageTitle(
			groupDetail.group === null
				? copy.routes.addExpense.documentTitle
				: `${copy.routes.addExpense.documentTitle} · ${groupDetail.group.name}`
		)}
	</title>
</svelte:head>

{#if groupDetail.status === 'ready' && groupDetail.group !== null}
	<section
		class="mx-auto flex w-full max-w-2xl flex-col gap-5 pb-4"
		aria-labelledby="expense-page-heading"
	>
		<header class="flex flex-col gap-3">
			<Button
				href={activityPath}
				variant="ghost"
				class="min-h-11 self-start px-2"
			>
				<ArrowLeftIcon data-icon="inline-start" />
				{copy.groups.expenses.backToGroup}
			</Button>
			<div class="flex flex-col gap-1.5">
				<h1 id="expense-page-heading" tabindex="-1">
					{copy.routes.addExpense.title}
				</h1>
				<p class="text-muted-foreground">
					{copy.routes.addExpense.description}
				</p>
			</div>
		</header>

		<Card.Root class="overflow-visible">
			<Card.Content>
				{#if memberMismatch}
					<Alert.Root variant="destructive">
						<CircleAlertIcon data-icon="inline-start" />
						<Alert.Title>{copy.groups.expenses.memberMismatch}</Alert.Title>
					</Alert.Root>
				{:else if initialDraft !== null}
					{#key groupDetail.groupId}
						<ExpenseForm
							mode="create"
							members={groupDetail.members}
							{initialDraft}
							cancelHref={activityPath}
							{mutation}
							{save}
							onCommitted={finish}
							onNotFound={resolveNotFound}
							{mutationDisabledReason}
						/>
					{/key}
				{:else}
					<div
						class="flex flex-col gap-5"
						aria-label={copy.routes.addExpense.loadingLabel}
					>
						{#each [0, 1, 2, 3] as row (row)}
							<Skeleton class="h-14 w-full" />
						{/each}
					</div>
				{/if}
			</Card.Content>
		</Card.Root>
	</section>
{/if}
